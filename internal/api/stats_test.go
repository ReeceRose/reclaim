package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"reclaim/internal/store"
)

func seedCompletedEncode(t *testing.T, st *store.Store, path, codec string, size, output int64, completedAt int64) {
	t.Helper()
	ctx := context.Background()
	c := codec
	w, h := 1920, 1080
	fileID, err := st.Media.Insert(ctx, &store.MediaFile{
		Path:        path,
		LibraryType: "movies",
		SizeBytes:   size,
		Mtime:       1,
		Fingerprint: "fp-" + path,
		VideoCodec:  &c,
		Width:       &w,
		Height:      &h,
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("insert media: %v", err)
	}
	jobID, err := st.Jobs.Create(ctx, &store.TranscodeJob{
		MediaFileID:       fileID,
		ProfileID:         1,
		Status:            "queued",
		QueuedAt:          completedAt - 3610,
		OriginalSizeBytes: size,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := st.Jobs.Transition(ctx, jobID, "queued", "running"); err != nil {
		t.Fatalf("to running: %v", err)
	}
	if err := st.Jobs.Transition(ctx, jobID, "running", "verifying"); err != nil {
		t.Fatalf("to verifying: %v", err)
	}
	if _, err := st.CommitEncodeSwap(ctx, fileID, jobID, output, "fp-new-"+path, completedAt, "done", ""); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestStatsIncludesRealizedSavings(t *testing.T) {
	_, h, st, _ := newTestServer(t, true)
	seedCompletedEncode(t, st, "/movies/a.mkv", "h264", 1000, 400, 1_700_003_600)

	w := doReq(h, http.MethodGet, "/api/stats", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("stats: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	savings, ok := body["savings"].(map[string]any)
	if !ok {
		t.Fatalf("stats response has no savings block: %s", w.Body.String())
	}
	if got := savings["bytes_saved"].(float64); got != 600 {
		t.Errorf("bytes_saved: want 600, got %v", got)
	}
	if got := savings["files_encoded"].(float64); got != 1 {
		t.Errorf("files_encoded: want 1, got %v", got)
	}
	if got := savings["compression_ratio"].(float64); got < 0.39 || got > 0.41 {
		t.Errorf("compression_ratio: want ~0.4, got %v", got)
	}
}

func TestSavingsReportShape(t *testing.T) {
	_, h, st, _ := newTestServer(t, true)
	now := time.Now().Unix()
	seedCompletedEncode(t, st, "/movies/a.mkv", "h264", 1000, 400, now-7200)
	seedCompletedEncode(t, st, "/movies/b.mkv", "mpeg4", 4000, 1000, now-3600)

	w := doReq(h, http.MethodGet, "/api/stats/savings?days=30", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("savings: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var got struct {
		Summary struct {
			FilesEncoded int64 `json:"files_encoded"`
			BytesSaved   int64 `json:"bytes_saved"`
			BestPath     string
		} `json:"summary"`
		ByCodec []struct {
			Key        string `json:"key"`
			BytesSaved int64  `json:"bytes_saved"`
		} `json:"by_codec"`
		Daily []struct {
			Day        string `json:"day"`
			BytesSaved int64  `json:"bytes_saved"`
		} `json:"daily"`
		TopWins []struct {
			Path       string `json:"path"`
			BytesSaved int64  `json:"bytes_saved"`
		} `json:"top_wins"`
		JobOutcomes map[string]int64 `json:"job_outcomes"`
		Days        int              `json:"days"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.Summary.FilesEncoded != 2 || got.Summary.BytesSaved != 3600 {
		t.Errorf("summary: files=%d saved=%d", got.Summary.FilesEncoded, got.Summary.BytesSaved)
	}
	if got.Days != 30 {
		t.Errorf("days: want 30, got %d", got.Days)
	}
	if len(got.ByCodec) != 2 {
		t.Fatalf("by_codec: want 2 buckets, got %d", len(got.ByCodec))
	}
	if got.ByCodec[0].Key != "mpeg4" || got.ByCodec[0].BytesSaved != 3000 {
		t.Errorf("top codec: %+v", got.ByCodec[0])
	}
	if len(got.TopWins) != 2 || got.TopWins[0].Path != "/movies/b.mkv" {
		t.Errorf("top_wins: %+v", got.TopWins)
	}
	if len(got.Daily) == 0 {
		t.Error("daily series is empty")
	}
	if got.JobOutcomes["completed"] != 2 {
		t.Errorf("job_outcomes completed: want 2, got %d", got.JobOutcomes["completed"])
	}
}

func TestSavingsRejectsBadDays(t *testing.T) {
	_, h, _, _ := newTestServer(t, true)
	for _, q := range []string{"?days=0", "?days=-5", "?days=abc", "?days=99999"} {
		w := doReq(h, http.MethodGet, "/api/stats/savings"+q, nil, nil)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", q, w.Code)
		}
	}
}
