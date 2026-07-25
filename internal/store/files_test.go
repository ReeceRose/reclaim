package store

import (
	"context"
	"testing"
)

func TestFiles_includesAllStatusesAndDerivesCandidateStates(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	candidateID, err := s.Media.insertFile(ctx, testFile{path: "/m/a-candidate.mkv", size: 1000, codec: "h264", height: 1080, savings: 400})
	if err != nil {
		t.Fatal(err)
	}
	hevcID, err := s.Media.insertFile(ctx, testFile{path: "/m/b-hevc.mkv", size: 1000, codec: "hevc", height: 1080, hevc: true})
	if err != nil {
		t.Fatal(err)
	}
	missingID, err := s.Media.insertFile(ctx, testFile{path: "/m/c-missing.mkv", size: 1000, codec: "h264", height: 1080, status: "missing"})
	if err != nil {
		t.Fatal(err)
	}
	probeID, err := s.Media.insertFile(ctx, testFile{path: "/m/d-probe.mkv", size: 1000, probeErr: "boom"})
	if err != nil {
		t.Fatal(err)
	}
	unknownID, err := s.Media.insertFile(ctx, testFile{path: "/m/e-unknown.mkv", size: 1000})
	if err != nil {
		t.Fatal(err)
	}
	queuedID, err := s.Media.insertFile(ctx, testFile{path: "/m/f-queued.mkv", size: 1000, codec: "h264", height: 1080, savings: 400})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Jobs.insertJobWithStatus(ctx, queuedID, "queued"); err != nil {
		t.Fatal(err)
	}
	completedID, err := s.Media.insertFile(ctx, testFile{path: "/m/g-completed.mkv", size: 1000, codec: "h264", height: 1080, savings: 400})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Jobs.insertJobWithStatus(ctx, completedID, "completed"); err != nil {
		t.Fatal(err)
	}

	files, err := s.Media.Files(ctx, FileQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 7 {
		t.Fatalf("want all 7 files, got %d", len(files))
	}
	if files[0].ID != candidateID || files[6].ID != completedID {
		t.Fatalf("default path order unexpected: %+v", files)
	}

	states, err := s.Media.CandidateStates(ctx, files)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64]CandidateState{
		candidateID: CandidateStateCandidate,
		hevcID:      CandidateStateAlreadyHEVC,
		missingID:   CandidateStateMissing,
		probeID:     CandidateStateProbeFailed,
		unknownID:   CandidateStateUnknownCodec,
		queuedID:    CandidateStateQueued,
		completedID: CandidateStateCompleted,
	}
	for id, state := range want {
		if states[id] != state {
			t.Fatalf("file %d state = %q, want %q", id, states[id], state)
		}
	}
}

func TestFiles_filtersByCandidateState(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/candidate.mkv", size: 1000, codec: "h264", height: 1080, savings: 400}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/hevc.mkv", size: 1000, codec: "hevc", height: 1080, hevc: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/missing.mkv", size: 1000, codec: "h264", height: 1080, status: "missing"}); err != nil {
		t.Fatal(err)
	}

	count := func(state CandidateState) int {
		got, err := s.Media.Files(ctx, FileQuery{Filter: FileFilter{CandidateState: string(state)}})
		if err != nil {
			t.Fatal(err)
		}
		return len(got)
	}
	if n := count(CandidateStateCandidate); n != 1 {
		t.Fatalf("candidate: want 1, got %d", n)
	}
	if n := count(CandidateStateAlreadyHEVC); n != 1 {
		t.Fatalf("already_hevc: want 1, got %d", n)
	}
	if n := count(CandidateStateMissing); n != 1 {
		t.Fatalf("missing: want 1, got %d", n)
	}
}

func TestFiles_filtersUnknownBuckets(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	noCodecID, err := s.Media.insertFile(ctx, testFile{path: "/m/no-codec.mkv", size: 1000})
	if err != nil {
		t.Fatal(err)
	}
	noHeightID, err := s.Media.insertFile(ctx, testFile{path: "/m/no-height.mkv", size: 1000, codec: "h264"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/known.mkv", size: 1000, codec: "h264", height: 1080}); err != nil {
		t.Fatal(err)
	}

	byCodec, err := s.Media.Files(ctx, FileQuery{Filter: FileFilter{VideoCodec: "unknown"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(byCodec) != 1 || byCodec[0].ID != noCodecID {
		t.Fatalf("unknown codec: got %+v, want only %d", byCodec, noCodecID)
	}

	byResolution, err := s.Media.Files(ctx, FileQuery{Filter: FileFilter{Height: "unknown"}})
	if err != nil {
		t.Fatal(err)
	}
	gotIDs := map[int64]bool{}
	for _, f := range byResolution {
		gotIDs[f.ID] = true
	}
	if len(byResolution) != 2 || !gotIDs[noCodecID] || !gotIDs[noHeightID] {
		t.Fatalf("unknown resolution: got %+v, want %d and %d", byResolution, noCodecID, noHeightID)
	}
}

// TestTVSeasonsAcrossShows_Ranking verifies seasons from different shows are
// ranked together by total size (default) and by predicted savings, that the
// search filter scopes to a single series, and that eligibility gating drives
// the savings totals.
func TestTVSeasonsAcrossShows_Ranking(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	codec := "h264"
	insert := func(path, title string, season int, size, savings int64, hevc bool) {
		t.Helper()
		if _, err := s.Media.Insert(ctx, &MediaFile{
			Path:                  path,
			LibraryType:           "tv",
			SizeBytes:             size,
			Status:                "active",
			VideoCodec:            &codec,
			IsAlreadyHEVC:         hevc,
			PredictedSavingsBytes: savings,
			SeriesTitle:           &title,
			SeasonNumber:          &season,
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Breaking Bad S1: big on disk, small savings (mostly already HEVC).
	insert("/tv/Breaking Bad/S01E01.mkv", "Breaking Bad", 1, 5000, 100, true)
	insert("/tv/Breaking Bad/S01E02.mkv", "Breaking Bad", 1, 5000, 100, true)
	// The Wire S1: smaller on disk, large savings (eligible h264).
	insert("/tv/The Wire/S01E01.mkv", "The Wire", 1, 3000, 2000, false)
	insert("/tv/The Wire/S01E02.mkv", "The Wire", 1, 3000, 2000, false)

	bySize, err := s.Media.TVSeasonsAcrossShows(ctx, "", SeasonSortSizeDesc, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(bySize) != 2 {
		t.Fatalf("len(bySize) = %d, want 2", len(bySize))
	}
	if bySize[0].SeriesTitle != "Breaking Bad" || bySize[0].TotalBytes != 10000 {
		t.Fatalf("size_desc first = %+v, want Breaking Bad @ 10000", bySize[0])
	}

	bySavings, err := s.Media.TVSeasonsAcrossShows(ctx, "", SeasonSortSavingsDesc, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if bySavings[0].SeriesTitle != "The Wire" || bySavings[0].PredictedSavingsBytes != 4000 {
		t.Fatalf("savings_desc first = %+v, want The Wire @ 4000", bySavings[0])
	}
	// Already-HEVC episodes are ineligible, so their savings must not count.
	if bySavings[1].SeriesTitle != "Breaking Bad" || bySavings[1].PredictedSavingsBytes != 0 {
		t.Fatalf("savings_desc second = %+v, want Breaking Bad @ 0", bySavings[1])
	}

	filtered, err := s.Media.TVSeasonsAcrossShows(ctx, "wire", SeasonSortSizeDesc, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].SeriesTitle != "The Wire" {
		t.Fatalf("search 'wire' = %+v, want single The Wire row", filtered)
	}

	total, err := s.Media.CountTVSeasonsAcrossShows(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("count = %d, want 2", total)
	}
}

// TestTVShowSeasons_EpisodeIDs verifies each season row carries the IDs of
// every episode file in that season, grouped correctly and excluding other
// shows/seasons. Consumers (bulk rescan) rely on this to target the right
// files without a separate lookup.
func TestTVShowSeasons_EpisodeIDs(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	insertEpisode := func(path, title string, season int) int64 {
		t.Helper()
		id, err := s.Media.Insert(ctx, &MediaFile{
			Path:         path,
			LibraryType:  "tv",
			SizeBytes:    1000,
			Status:       "active",
			SeriesTitle:  &title,
			SeasonNumber: &season,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}

	title := "Breaking Bad"
	s1e1 := insertEpisode("/tv/Breaking Bad/S01E01.mkv", title, 1)
	s1e2 := insertEpisode("/tv/Breaking Bad/S01E02.mkv", title, 1)
	s2e1 := insertEpisode("/tv/Breaking Bad/S02E01.mkv", title, 2)
	// Different show — must not leak into Breaking Bad's seasons.
	otherTitle := "The Wire"
	insertEpisode("/tv/The Wire/S01E01.mkv", otherTitle, 1)

	seasons, err := s.Media.TVShowSeasons(ctx, title)
	if err != nil {
		t.Fatal(err)
	}
	if len(seasons) != 2 {
		t.Fatalf("len(seasons) = %d, want 2", len(seasons))
	}

	byNum := map[int]TVSeasonRow{}
	for _, sn := range seasons {
		byNum[sn.Season] = sn
	}

	gotS1 := map[int64]bool{}
	for _, id := range byNum[1].EpisodeIDs {
		gotS1[id] = true
	}
	if len(byNum[1].EpisodeIDs) != 2 || !gotS1[s1e1] || !gotS1[s1e2] {
		t.Fatalf("season 1 episode ids = %v, want [%d %d]", byNum[1].EpisodeIDs, s1e1, s1e2)
	}

	if len(byNum[2].EpisodeIDs) != 1 || byNum[2].EpisodeIDs[0] != s2e1 {
		t.Fatalf("season 2 episode ids = %v, want [%d]", byNum[2].EpisodeIDs, s2e1)
	}
}
