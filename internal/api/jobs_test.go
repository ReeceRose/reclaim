package api

import (
	"context"
	"net/http"
	"slices"
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

// queuedFileOrder lists the queued jobs' media file ids in queue order, along
// with the job id behind each.
func queuedFileOrder(t *testing.T, h http.Handler, cookie *http.Cookie, query string) (files, jobIDs []int64) {
	t.Helper()
	w := doReq(h, http.MethodGet, "/api/jobs?status=queued&order=queue"+query, nil, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	for _, it := range decodeBody(t, w)["items"].([]any) {
		m := it.(map[string]any)
		files = append(files, int64(m["media_file_id"].(float64)))
		jobIDs = append(jobIDs, int64(m["id"].(float64)))
	}
	return files, jobIDs
}

func TestJobsSearchFiltersPageAndCount(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	ids := seedQueuedJobs(t, h, st, cookie, 1000, 2000, 3000)

	files, _ := queuedFileOrder(t, h, cookie, "&search=SEEDB")
	if len(files) != 1 || files[0] != ids[1] {
		t.Fatalf("search: want [%d], got %v", ids[1], files)
	}
	body := decodeBody(t, doReq(h, http.MethodGet, "/api/jobs?status=queued&search=seedb", nil, cookie))
	if got := num(t, body, "total_count"); got != 1 {
		t.Errorf("total_count: want 1, got %v", got)
	}
	if got := num(t, body, "queued_count"); got != 3 {
		t.Errorf("queued_count describes the whole queue: want 3, got %v", got)
	}
	fq := body["filtered_queue"].(map[string]any)
	if num(t, fq, "count") != 1 || num(t, fq, "original_size_bytes") != 2000 {
		t.Errorf("filtered_queue totals the match only: got %v", fq)
	}
	item := body["items"].([]any)[0].(map[string]any)
	if got := num(t, item, "queue_position"); got != 2 {
		t.Errorf("queue_position is queue-wide: want 2, got %v", got)
	}
}

func TestJobsReorder(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	ids := seedQueuedJobs(t, h, st, cookie, 1000, 2000, 3000, 4000)
	_, jobIDs := queuedFileOrder(t, h, cookie, "")

	w := doReq(h, http.MethodPost, "/api/jobs/reorder", map[string]any{
		"job_ids": []int64{jobIDs[3], jobIDs[2]}, "position": "top",
	}, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("reorder top: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	files, _ := queuedFileOrder(t, h, cookie, "")
	want := []int64{ids[2], ids[3], ids[0], ids[1]}
	if !slices.Equal(files, want) {
		t.Fatalf("after top: want %v, got %v", want, files)
	}

	w = doReq(h, http.MethodPost, "/api/jobs/reorder", map[string]any{
		"filter": map[string]any{"search": "seedc"}, "position": "bottom",
	}, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("reorder bottom: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	files, _ = queuedFileOrder(t, h, cookie, "")
	want = []int64{ids[3], ids[0], ids[1], ids[2]}
	if !slices.Equal(files, want) {
		t.Fatalf("after bottom: want %v, got %v", want, files)
	}

	// Up/down swap with the neighbour; a job already at the end stays put.
	files, jobIDs = queuedFileOrder(t, h, cookie, "")
	jobOf := make(map[int64]int64, len(files))
	for i, f := range files {
		jobOf[f] = jobIDs[i]
	}
	for _, step := range []struct {
		files    []int64
		position string
		want     []int64
	}{
		{[]int64{ids[1]}, "up", []int64{ids[3], ids[1], ids[0], ids[2]}},
		{[]int64{ids[3]}, "up", []int64{ids[3], ids[1], ids[0], ids[2]}},
		{[]int64{ids[3], ids[0]}, "down", []int64{ids[1], ids[3], ids[2], ids[0]}},
		{[]int64{ids[0]}, "down", []int64{ids[1], ids[3], ids[2], ids[0]}},
		{[]int64{ids[2], ids[0]}, "up", []int64{ids[1], ids[2], ids[0], ids[3]}},
	} {
		sel := make([]int64, len(step.files))
		for i, f := range step.files {
			sel[i] = jobOf[f]
		}
		w := doReq(h, http.MethodPost, "/api/jobs/reorder", map[string]any{"job_ids": sel, "position": step.position}, cookie)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: want 200, got %d (%s)", step.position, w.Code, w.Body.String())
		}
		files, _ = queuedFileOrder(t, h, cookie, "")
		if !slices.Equal(files, step.want) {
			t.Fatalf("%s %v: want %v, got %v", step.position, step.files, step.want, files)
		}
	}
	want = []int64{ids[1]}

	job, err := st.Jobs.ClaimNextQueued(context.Background(), 1)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if job.MediaFileID != want[0] {
		t.Errorf("worker claims queue order: want file %d, got %d", want[0], job.MediaFileID)
	}

	for _, body := range []map[string]any{
		{"job_ids": []int64{jobIDs[0]}, "position": "middle"},
		{"position": "top"},
		{"job_ids": []int64{jobIDs[0]}, "filter": map[string]any{"search": "x"}, "position": "top"},
	} {
		if w := doReq(h, http.MethodPost, "/api/jobs/reorder", body, cookie); w.Code != http.StatusBadRequest {
			t.Errorf("%v: want 400, got %d", body, w.Code)
		}
	}
}

func TestJobsSortQueue(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	ids := seedQueuedJobs(t, h, st, cookie, 2000, 4000, 1000, 3000)

	w := doReq(h, http.MethodPost, "/api/jobs/sort", map[string]any{"by": "size_desc"}, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("sort: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	files, _ := queuedFileOrder(t, h, cookie, "")
	if want := []int64{ids[1], ids[3], ids[0], ids[2]}; !slices.Equal(files, want) {
		t.Fatalf("size_desc: want %v, got %v", want, files)
	}

	w = doReq(h, http.MethodPost, "/api/jobs/sort", map[string]any{
		"by": "queued_at_asc", "filter": map[string]any{"search": "seed"},
	}, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("sort all: want 200, got %d", w.Code)
	}
	files, _ = queuedFileOrder(t, h, cookie, "")
	if !slices.Equal(files, ids) {
		t.Fatalf("queued_at_asc restores the original order: want %v, got %v", ids, files)
	}

	// Sorting a subset deals it back into the slots it held (1st and 3rd),
	// leaving the jobs between them where they were.
	_, jobIDs := queuedFileOrder(t, h, cookie, "")
	if _, err := st.Jobs.ApplyQueueOrder(context.Background(), []int64{jobIDs[2], jobIDs[0]}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	files, _ = queuedFileOrder(t, h, cookie, "")
	if want := []int64{ids[2], ids[1], ids[0], ids[3]}; !slices.Equal(files, want) {
		t.Fatalf("subset sort: want %v, got %v", want, files)
	}

	if w := doReq(h, http.MethodPost, "/api/jobs/sort", map[string]any{"by": "nope"}, cookie); w.Code != http.StatusBadRequest {
		t.Errorf("unknown key: want 400, got %d", w.Code)
	}
}

func TestJobsBulkCancel(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	ids := seedQueuedJobs(t, h, st, cookie, 1000, 2000, 3000)

	w := doReq(h, http.MethodPost, "/api/jobs/cancel", map[string]any{
		"filter": map[string]any{"search": "seeda"},
	}, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("bulk cancel: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if got := num(t, decodeBody(t, w), "cancelled"); got != 1 {
		t.Errorf("cancelled: want 1, got %v", got)
	}
	files, _ := queuedFileOrder(t, h, cookie, "")
	if want := ids[1:]; !slices.Equal(files, want) {
		t.Fatalf("remaining: want %v, got %v", want, files)
	}

	if w := doReq(h, http.MethodPost, "/api/jobs/cancel", map[string]any{}, cookie); w.Code != http.StatusBadRequest {
		t.Errorf("empty selection: want 400, got %d", w.Code)
	}
}
