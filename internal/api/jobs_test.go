package api

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"reclaim/internal/store"
)

// seedQueuedJobs inserts n h264 media files of the given sizes and queues one
// job per file through the API, returning the media file ids.
func seedQueuedJobs(t *testing.T, h http.Handler, st *store.Store, cookie *http.Cookie, sizes ...int64) []int64 {
	t.Helper()
	ctx := context.Background()
	codec := "h264"
	duration := 3600.0
	ids := make([]int64, 0, len(sizes))
	for i, size := range sizes {
		id, err := st.Media.Insert(ctx, &store.MediaFile{
			Path:        "/media/movies/seed" + string(rune('a'+i)) + ".mkv",
			LibraryType: "movie", SizeBytes: size, Mtime: 1,
			Fingerprint: "fp" + string(rune('a'+i)), VideoCodec: &codec,
			Status: "active", DurationSeconds: &duration,
		})
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
		ids = append(ids, id)
	}

	w := doReq(h, http.MethodPost, "/api/jobs", map[string]any{"file_ids": ids}, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("queue jobs: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	return ids
}

func num(t *testing.T, body map[string]any, key string) float64 {
	t.Helper()
	v, ok := body[key].(float64)
	if !ok {
		t.Fatalf("%s missing or not a number in %v", key, body)
	}
	return v
}

// The queue summary describes the whole queue, not the requested page, and must
// survive past offset 0 — numbered pagination reads total_count on every page.
func TestJobsQueueSummaryOnEveryPage(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	seedQueuedJobs(t, h, st, cookie, 1000, 2000, 3000)

	for _, path := range []string{
		"/api/jobs?status=queued&order=queue&limit=1&offset=0",
		"/api/jobs?status=queued&order=queue&limit=1&offset=2",
	} {
		w := doReq(h, http.MethodGet, path, nil, cookie)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: want 200, got %d (%s)", path, w.Code, w.Body.String())
		}
		body := decodeBody(t, w)
		if items := body["items"].([]any); len(items) != 1 {
			t.Fatalf("%s: items = %d, want 1", path, len(items))
		}
		if got := num(t, body, "total_count"); got != 3 {
			t.Errorf("%s: total_count = %v, want 3", path, got)
		}
		if got := num(t, body, "queued_count"); got != 3 {
			t.Errorf("%s: queued_count = %v, want 3", path, got)
		}
		if got := num(t, body, "queue_total_original_bytes"); got != 6000 {
			t.Errorf("%s: queue_total_original_bytes = %v, want 6000", path, got)
		}
		if got := num(t, body, "queue_total_predicted_savings_bytes"); got <= 0 || got >= 6000 {
			t.Errorf("%s: queue_total_predicted_savings_bytes = %v, want a positive fraction of 6000", path, got)
		}
	}
}

// A queued-only listing has no finished jobs to summarize, and a history
// listing has no queue — neither block should leak into the other.
func TestJobsSummaryBlocksAreScopedToStatusFilter(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	seedQueuedJobs(t, h, st, cookie, 1000)

	w := doReq(h, http.MethodGet, "/api/jobs?status=queued", nil, cookie)
	if _, ok := decodeBody(t, w)["history"]; ok {
		t.Error("queued-only listing carries a history block")
	}

	w = doReq(h, http.MethodGet, "/api/jobs?status=completed,failed", nil, cookie)
	body := decodeBody(t, w)
	if _, ok := body["queue_total_original_bytes"]; ok {
		t.Error("history-only listing carries queue byte totals")
	}
	if _, ok := body["history"]; !ok {
		t.Error("history-only listing is missing its history block")
	}
}

func TestJobsHistorySummaryCountsCompletedOnly(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	ctx := context.Background()
	seedQueuedJobs(t, h, st, cookie, 1000, 5000)

	// First job runs for 300s and lands at 400 bytes.
	done, err := st.Jobs.ClaimNextQueued(ctx, 1000)
	if err != nil {
		t.Fatalf("claim first: %v", err)
	}
	if err := st.Jobs.Transition(ctx, done.ID, "running", "verifying"); err != nil {
		t.Fatalf("verifying: %v", err)
	}
	if err := st.Jobs.MarkCompleted(ctx, done.ID, 400, 1300); err != nil {
		t.Fatalf("complete: %v", err)
	}

	// Second job fails. It never swapped a file, so its 5000 bytes must not
	// appear in the byte totals.
	bad, err := st.Jobs.ClaimNextQueued(ctx, 2000)
	if err != nil {
		t.Fatalf("claim second: %v", err)
	}
	if err := st.Jobs.MarkFailed(ctx, bad.ID, "ffmpeg exploded", 2100); err != nil {
		t.Fatalf("fail: %v", err)
	}

	w := doReq(h, http.MethodGet, "/api/jobs?status=completed,failed&order=recent", nil, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("history: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	hist, ok := decodeBody(t, w)["history"].(map[string]any)
	if !ok {
		t.Fatalf("history block missing: %s", w.Body.String())
	}
	for _, tc := range []struct {
		key  string
		want float64
	}{
		{"completed_count", 1},
		{"failed_count", 1},
		{"cancelled_count", 0},
		{"original_size_bytes", 1000},
		{"output_size_bytes", 400},
		{"bytes_saved", 600},
		{"encode_seconds", 300},
	} {
		if got := num(t, hist, tc.key); got != tc.want {
			t.Errorf("history.%s = %v, want %v", tc.key, got, tc.want)
		}
	}
}

// A dismissed job is hidden from the list, so it must drop out of the totals
// that label that list too.
func TestJobsHistorySummaryExcludesDismissed(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	ctx := context.Background()
	seedQueuedJobs(t, h, st, cookie, 1000)

	job, err := st.Jobs.ClaimNextQueued(ctx, 1000)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	_ = st.Jobs.Transition(ctx, job.ID, "running", "verifying")
	if err := st.Jobs.MarkCompleted(ctx, job.ID, 400, 1300); err != nil {
		t.Fatalf("complete: %v", err)
	}

	w := doReq(h, http.MethodDelete, "/api/jobs/"+strconv.FormatInt(job.ID, 10), nil, cookie)
	if w.Code != http.StatusNoContent {
		t.Fatalf("dismiss: want 204, got %d (%s)", w.Code, w.Body.String())
	}

	w = doReq(h, http.MethodGet, "/api/jobs?status=completed,failed", nil, cookie)
	body := decodeBody(t, w)
	if got := num(t, body, "total_count"); got != 0 {
		t.Errorf("total_count = %v, want 0", got)
	}
	hist := body["history"].(map[string]any)
	if got := num(t, hist, "completed_count"); got != 0 {
		t.Errorf("history.completed_count = %v, want 0", got)
	}
	if got := num(t, hist, "bytes_saved"); got != 0 {
		t.Errorf("history.bytes_saved = %v, want 0", got)
	}
}
