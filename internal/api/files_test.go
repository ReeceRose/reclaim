package api

import (
	"context"
	"encoding/json"
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
}
