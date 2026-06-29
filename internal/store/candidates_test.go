package store

import (
	"context"
	"testing"
)

func TestCandidates_excludesHEVCMissingProbeErrorAndQueued(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// A normal candidate.
	want, err := s.Media.insertFile(ctx, testFile{path: "/m/keep.mkv", size: 1000, codec: "h264", height: 1080, savings: 400})
	if err != nil {
		t.Fatal(err)
	}
	// Already HEVC → excluded.
	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/hevc.mkv", size: 1000, codec: "hevc", height: 1080, hevc: true}); err != nil {
		t.Fatal(err)
	}
	// Missing → excluded.
	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/missing.mkv", size: 1000, codec: "h264", height: 1080, savings: 400, status: "missing"}); err != nil {
		t.Fatal(err)
	}
	// Probe error → excluded.
	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/err.mkv", size: 1000, probeErr: "boom"}); err != nil {
		t.Fatal(err)
	}
	// Already queued → excluded.
	queuedID, err := s.Media.insertFile(ctx, testFile{path: "/m/queued.mkv", size: 1000, codec: "h264", height: 1080, savings: 400})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Jobs.insertJobWithStatus(ctx, queuedID, "queued"); err != nil {
		t.Fatal(err)
	}
	// Completed job → excluded.
	completedID, err := s.Media.insertFile(ctx, testFile{path: "/m/done.mkv", size: 1000, codec: "h264", height: 1080, savings: 400})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Jobs.insertJobWithStatus(ctx, completedID, "completed"); err != nil {
		t.Fatal(err)
	}
	// Failed job → NOT excluded (eligible for retry).
	failedID, err := s.Media.insertFile(ctx, testFile{path: "/m/retry.mkv", size: 1000, codec: "h264", height: 1080, savings: 300})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Jobs.insertJobWithStatus(ctx, failedID, "failed"); err != nil {
		t.Fatal(err)
	}

	got, err := s.Media.Candidates(ctx, CandidateQuery{})
	if err != nil {
		t.Fatal(err)
	}

	gotIDs := map[int64]bool{}
	for _, f := range got {
		gotIDs[f.ID] = true
	}
	if !gotIDs[want] || !gotIDs[failedID] {
		t.Fatalf("expected keep(%d) and retry(%d) in candidates, got %v", want, failedID, gotIDs)
	}
	if len(got) != 2 {
		t.Fatalf("expected exactly 2 candidates, got %d: %+v", len(got), got)
	}
}

func TestCandidates_savingsDescOrder(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i, sav := range []int64{100, 5000, 250, 999} {
		if _, err := s.Media.insertFile(ctx, testFile{
			path:    "/m/f" + string(rune('a'+i)) + ".mkv",
			size:    10000,
			codec:   "h264",
			height:  1080,
			savings: sav,
		}); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.Media.Candidates(ctx, CandidateQuery{Sort: SortSavingsDesc})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("want 4, got %d", len(got))
	}
	want := []int64{5000, 999, 250, 100}
	for i, f := range got {
		if f.PredictedSavingsBytes != want[i] {
			t.Fatalf("position %d: savings=%d, want %d", i, f.PredictedSavingsBytes, want[i])
		}
	}
}

func TestCandidates_keysetPaginationNoDupesOrGaps(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	const n = 25
	for i := 0; i < n; i++ {
		// Deliberately include ties in savings to exercise the (savings, id) tiebreak.
		sav := int64((i % 5) * 100)
		if _, err := s.Media.insertFile(ctx, testFile{
			path:    "/m/file" + string(rune('a'+i)) + ".mkv",
			size:    10000,
			codec:   "h264",
			height:  1080,
			savings: sav,
		}); err != nil {
			t.Fatal(err)
		}
	}

	seen := map[int64]bool{}
	var afterSavings, afterID *int64
	pages := 0
	for {
		page, err := s.Media.Candidates(ctx, CandidateQuery{
			Sort:         SortSavingsDesc,
			Limit:        7,
			AfterSavings: afterSavings,
			AfterID:      afterID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		pages++
		if pages > 100 {
			t.Fatal("pagination did not terminate")
		}
		for _, f := range page {
			if seen[f.ID] {
				t.Fatalf("duplicate row %d across pages", f.ID)
			}
			seen[f.ID] = true
		}
		last := page[len(page)-1]
		sav := last.PredictedSavingsBytes
		id := last.ID
		afterSavings, afterID = &sav, &id
	}

	if len(seen) != n {
		t.Fatalf("keyset walk saw %d distinct rows, want %d (gap or dupe)", len(seen), n)
	}
}

func TestCandidates_filters(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	mk := func(path, lib, codec string, height int) {
		if _, err := s.Media.insertFile(ctx, testFile{
			path: path, libraryType: lib, size: 1000, codec: codec, height: height, savings: 100,
		}); err != nil {
			t.Fatal(err)
		}
	}
	mk("/tv/a.mkv", "tv", "h264", 480)
	mk("/tv/b.mkv", "tv", "mpeg2video", 1080)
	mk("/m/c.mkv", "movie", "h264", 2160)
	mk("/m/d.mkv", "movie", "h264", 1080)

	count := func(q CandidateQuery) int {
		got, err := s.Media.Candidates(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		return len(got)
	}

	if n := count(CandidateQuery{Filter: CandidateFilter{LibraryType: "tv"}}); n != 2 {
		t.Fatalf("library tv: want 2, got %d", n)
	}
	if n := count(CandidateQuery{Filter: CandidateFilter{VideoCodec: "h264"}}); n != 3 {
		t.Fatalf("codec h264: want 3, got %d", n)
	}
	if n := count(CandidateQuery{Filter: CandidateFilter{Height: "480"}}); n != 1 {
		t.Fatalf("height 480: want 1, got %d", n)
	}
	if n := count(CandidateQuery{Filter: CandidateFilter{Height: "2160"}}); n != 1 {
		t.Fatalf("height 2160: want 1, got %d", n)
	}
	if n := count(CandidateQuery{Filter: CandidateFilter{Height: "1080"}}); n != 2 {
		t.Fatalf("height 1080: want 2, got %d", n)
	}
	if n := count(CandidateQuery{Filter: CandidateFilter{Height: resBandUHD}}); n != 1 {
		t.Fatalf("height uhd: want 1, got %d", n)
	}
	if n := count(CandidateQuery{Filter: CandidateFilter{Height: resBandFHD}}); n != 2 {
		t.Fatalf("height fhd: want 2, got %d", n)
	}
	if n := count(CandidateQuery{Filter: CandidateFilter{Height: resBandSD}}); n != 1 {
		t.Fatalf("height sd: want 1, got %d", n)
	}
}

func TestCandidates_resolutionFilterUsesWidthForCroppedFiles(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for _, tf := range []testFile{
		{path: "/m/8k-crop.mkv", size: 1000, codec: "h264", width: 7680, height: 3200, savings: 100},
		{path: "/m/uhd-crop.mkv", size: 1000, codec: "h264", width: 3840, height: 1600, savings: 100},
		{path: "/m/qhd-crop.mkv", size: 1000, codec: "h264", width: 2560, height: 1080, savings: 100},
		{path: "/m/fhd-crop.mkv", size: 1000, codec: "h264", width: 1920, height: 800, savings: 100},
		{path: "/m/hd-crop.mkv", size: 1000, codec: "h264", width: 1280, height: 536, savings: 100},
	} {
		if _, err := s.Media.insertFile(ctx, tf); err != nil {
			t.Fatal(err)
		}
	}

	count := func(height string) int {
		got, err := s.Media.Candidates(ctx, CandidateQuery{Filter: CandidateFilter{Height: height}})
		if err != nil {
			t.Fatal(err)
		}
		return len(got)
	}

	for _, band := range []string{resBand8K, resBandUHD, resBandQHD, resBandFHD, resBandHD} {
		if n := count(band); n != 1 {
			t.Fatalf("height %s: want 1, got %d", band, n)
		}
	}
}

func TestCandidates_libraryTypeSort(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for _, tf := range []testFile{
		{path: "/tv/a.mkv", libraryType: "tv", size: 1000, codec: "h264", height: 1080, savings: 100},
		{path: "/m/b.mkv", libraryType: "movie", size: 1000, codec: "h264", height: 1080, savings: 500},
		{path: "/tv/c.mkv", libraryType: "tv", size: 1000, codec: "h264", height: 1080, savings: 300},
	} {
		if _, err := s.Media.insertFile(ctx, tf); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.Media.Candidates(ctx, CandidateQuery{Sort: SortLibraryType})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3, got %d", len(got))
	}
	// movie sorts before tv; within tv, higher savings first.
	if got[0].LibraryType != "movie" || got[0].PredictedSavingsBytes != 500 {
		t.Fatalf("first = %+v, want movie/500", got[0])
	}
	if got[1].LibraryType != "tv" || got[1].PredictedSavingsBytes != 300 {
		t.Fatalf("second = %+v, want tv/300", got[1])
	}
	if got[2].LibraryType != "tv" || got[2].PredictedSavingsBytes != 100 {
		t.Fatalf("third = %+v, want tv/100", got[2])
	}
}
