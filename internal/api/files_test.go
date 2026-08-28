package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"reclaim/internal/store"
)

func insertAPIMedia(t *testing.T, st *store.Store, f *store.MediaFile) int64 {
	t.Helper()
	if f.LibraryType == "" {
		f.LibraryType = store.LibraryTypeMovies
	}
	if f.SizeBytes == 0 {
		f.SizeBytes = 1000
	}
	if f.Fingerprint == "" {
		f.Fingerprint = "fp-" + f.Path
	}
	if f.Status == "" {
		f.Status = store.MediaStatusActive
	}
	id, err := st.Media.Insert(context.Background(), f)
	if err != nil {
		t.Fatalf("insert media: %v", err)
	}
	return id
}

func TestHandleFilesIncludesHEVCMissingAndCandidateState(t *testing.T) {
	_, h, st, _ := newTestServer(t, true)

	h264 := "h264"
	hevc := "hevc"
	insertAPIMedia(t, st, &store.MediaFile{
		Path:                  "/media/movies/a-candidate.mkv",
		VideoCodec:            &h264,
		PredictedSavingsBytes: 400,
	})
	insertAPIMedia(t, st, &store.MediaFile{
		Path:          "/media/movies/b-hevc.mkv",
		VideoCodec:    &hevc,
		IsAlreadyHEVC: true,
	})
	insertAPIMedia(t, st, &store.MediaFile{
		Path:       "/media/movies/c-missing.mkv",
		VideoCodec: &h264,
		Status:     store.MediaStatusMissing,
	})

	w := doReq(h, http.MethodGet, "/api/files?sort=path_asc", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("files: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var body struct {
		Items      []mediaFileDTO `json:"items"`
		TotalCount int            `json:"total_count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.TotalCount != 3 || len(body.Items) != 3 {
		t.Fatalf("want 3 files, got total=%d items=%d", body.TotalCount, len(body.Items))
	}
	if body.Items[0].CandidateState != string(store.CandidateStateCandidate) {
		t.Fatalf("first state = %q", body.Items[0].CandidateState)
	}
	if body.Items[1].CandidateState != string(store.CandidateStateAlreadyHEVC) {
		t.Fatalf("second state = %q", body.Items[1].CandidateState)
	}
	if body.Items[2].CandidateState != string(store.CandidateStateMissing) {
		t.Fatalf("third state = %q", body.Items[2].CandidateState)
	}
}

// A later season must page independently of the earlier ones — paging across the
// whole show returned an empty list for every season past the first page.
func TestHandleGroupedFileEpisodesPagesWithinSeason(t *testing.T) {
	_, h, st, _ := newTestServer(t, true)

	h264 := "h264"
	title := "Harbor Lights"
	for season := 1; season <= 3; season++ {
		for ep := 1; ep <= 4; ep++ {
			s, e := season, ep
			insertAPIMedia(t, st, &store.MediaFile{
				Path:         fmt.Sprintf("/media/tv/Harbor Lights/Season %02d/Harbor.Lights.S%02dE%02d.mkv", s, s, e),
				LibraryType:  store.LibraryTypeTV,
				VideoCodec:   &h264,
				SeriesTitle:  &title,
				SeasonNumber: &s,
			})
		}
	}

	w := doReq(h, http.MethodGet, "/api/files/grouped/episodes?series=Harbor+Lights&season=3&limit=4", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("grouped episodes: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var body struct {
		Episodes   []episodeDTO `json:"episodes"`
		TotalCount int          `json:"total_count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Episodes) != 4 || body.TotalCount != 4 {
		t.Fatalf("want 4 episodes of season 3, got total=%d episodes=%d", body.TotalCount, len(body.Episodes))
	}
	for i, ep := range body.Episodes {
		if ep.Season != 3 {
			t.Fatalf("episode %d in season %d, want 3", i, ep.Season)
		}
		if ep.Episode == nil || *ep.Episode != i+1 {
			t.Fatalf("episode %d parsed as %v", i, ep.Episode)
		}
	}
}

func TestHandleGroupedFilesSummarizesAllTVFiles(t *testing.T) {
	_, h, st, _ := newTestServer(t, true)

	h264 := "h264"
	hevc := "hevc"
	title := "Harbor Lights"
	season := 1
	insertAPIMedia(t, st, &store.MediaFile{
		Path:                  "/media/tv/Harbor Lights/Season 01/Harbor.Lights.S01E01.mkv",
		LibraryType:           store.LibraryTypeTV,
		VideoCodec:            &h264,
		PredictedSavingsBytes: 400,
		SizeBytes:             1000,
		SeriesTitle:           &title,
		SeasonNumber:          &season,
	})
	insertAPIMedia(t, st, &store.MediaFile{
		Path:          "/media/tv/Harbor Lights/Season 01/Harbor.Lights.S01E02.mkv",
		LibraryType:   store.LibraryTypeTV,
		VideoCodec:    &hevc,
		IsAlreadyHEVC: true,
		SizeBytes:     700,
		SeriesTitle:   &title,
		SeasonNumber:  &season,
	})

	w := doReq(h, http.MethodGet, "/api/files/grouped", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("grouped files: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var body struct {
		Series []librarySeriesSummary `json:"series"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Series) != 1 {
		t.Fatalf("want 1 series, got %+v", body.Series)
	}
	got := body.Series[0]
	if got.FileCount != 2 || got.EligibleCount != 1 || got.PredictedSavingsBytes != 400 {
		t.Fatalf("unexpected summary: %+v", got)
	}

	// One of two episodes is done, so the show is partly converted and nothing
	// else.
	for _, tc := range []struct {
		progress string
		want     int
	}{
		{"partial", 1},
		{"converted", 0},
		{"unconverted", 0},
		{"missing", 0},
	} {
		w := doReq(h, http.MethodGet, "/api/files/grouped?progress="+tc.progress, nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("progress %q: want 200, got %d (%s)", tc.progress, w.Code, w.Body.String())
		}
		var page struct {
			Series     []librarySeriesSummary `json:"series"`
			TotalCount int                    `json:"total_count"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if len(page.Series) != tc.want || page.TotalCount != tc.want {
			t.Fatalf("progress %q: got %d series / total %d, want %d", tc.progress, len(page.Series), page.TotalCount, tc.want)
		}
	}

	if w := doReq(h, http.MethodGet, "/api/files/grouped?progress=bogus", nil, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("bad progress: want 400, got %d (%s)", w.Code, w.Body.String())
	}
	if w := doReq(h, http.MethodGet, "/api/seasons?progress=bogus", nil, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("bad seasons progress: want 400, got %d (%s)", w.Code, w.Body.String())
	}
}
