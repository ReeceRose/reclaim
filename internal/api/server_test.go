package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"reclaim/internal/config"
	"reclaim/internal/store"
)

// fakeScanner satisfies ScanTrigger without touching the filesystem.
type fakeScanner struct{ calls int32 }

func (f *fakeScanner) Scan(_ context.Context, _ string, _ bool) (*store.ScanRun, error) {
	atomic.AddInt32(&f.calls, 1)
	return &store.ScanRun{ID: 1, FilesScanned: 3, FilesAdded: 1}, nil
}

func testConfig() *config.Config {
	return &config.Config{
		EncodeWindowStart: 0,
		EncodeWindowEnd:   6 * time.Hour,
		ScanInterval:      24 * time.Hour,
		ProbeConcurrency:  4,
		MoviesPath:        "/media/movies",
		TVPath:            "/media/tv",
	}
}

func newTestServer(t *testing.T, disableAuth bool) (*Server, http.Handler, *store.Store, *fakeScanner) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	fs := &fakeScanner{}
	srv := New(Deps{
		Store:       st,
		Scanner:     fs,
		Live:        config.NewLive(testConfig()),
		MoviesPath:  "/media/movies",
		TVPath:      "/media/tv",
		DisableAuth: disableAuth,
	})
	return srv, srv.Handler(), st, fs
}

// completeSetup marks the store set up and returns a valid session cookie.
func completeSetup(t *testing.T, st *store.Store) *http.Cookie {
	t.Helper()
	if err := st.Settings.CompleteSetup(context.Background(), "admin", "password123"); err != nil {
		t.Fatalf("complete setup: %v", err)
	}
	w := httptest.NewRecorder()
	IssueSession(w, "admin", st.Settings.SessionSecret(), false)
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie issued")
	}
	return cookies[0]
}

func doReq(h http.Handler, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	r := httptest.NewRequest(method, path, rdr)
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return m
}

// --- Auth round-trip ------------------------------------------------------

func TestSetupLoginLogoutRoundTrip(t *testing.T) {
	_, h, _, _ := newTestServer(t, false)

	// First setup succeeds and logs the user in.
	w := doReq(h, http.MethodPost, "/api/setup", map[string]string{
		"username": "admin", "password": "password123",
	}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("setup: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if len(w.Result().Cookies()) == 0 {
		t.Fatal("setup did not issue a session cookie")
	}

	// Second setup is a conflict.
	w = doReq(h, http.MethodPost, "/api/setup", map[string]string{
		"username": "x", "password": "password123",
	}, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("repeat setup: want 409, got %d", w.Code)
	}

	// Login issues a working cookie.
	w = doReq(h, http.MethodPost, "/api/login", map[string]string{
		"username": "admin", "password": "password123",
	}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("login: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login issued no cookie")
	}
	cookie := cookies[0]

	// The cookie unlocks a protected route.
	w = doReq(h, http.MethodGet, "/api/stats", nil, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("stats with cookie: want 200, got %d", w.Code)
	}

	// Wrong password is rejected.
	w = doReq(h, http.MethodPost, "/api/login", map[string]string{
		"username": "admin", "password": "wrongpass1",
	}, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("bad login: want 401, got %d", w.Code)
	}
}

func TestProtectedRoutesRequireCookie(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	completeSetup(t, st)

	w := doReq(h, http.MethodGet, "/api/stats", nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no cookie: want 401, got %d", w.Code)
	}
}

func TestDisableAuthBypassesGate(t *testing.T) {
	_, h, _, _ := newTestServer(t, true)
	w := doReq(h, http.MethodGet, "/api/stats", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("DISABLE_AUTH: want 200, got %d", w.Code)
	}
}

func TestCredentialChangeTakesEffect(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)

	w := doReq(h, http.MethodPut, "/api/settings/credentials", map[string]string{
		"username": "admin2", "password": "newpassword1",
	}, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("change creds: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	// Old credentials no longer work.
	w = doReq(h, http.MethodPost, "/api/login", map[string]string{
		"username": "admin", "password": "password123",
	}, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("old creds: want 401, got %d", w.Code)
	}

	// New credentials work immediately, no restart.
	w = doReq(h, http.MethodPost, "/api/login", map[string]string{
		"username": "admin2", "password": "newpassword1",
	}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("new creds: want 200, got %d", w.Code)
	}
}

func TestSessionEndpointReportsState(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)

	// Before setup.
	w := doReq(h, http.MethodGet, "/api/session", nil, nil)
	body := decodeBody(t, w)
	if body["setup_complete"] != false {
		t.Errorf("pre-setup: setup_complete = %v, want false", body["setup_complete"])
	}

	cookie := completeSetup(t, st)
	w = doReq(h, http.MethodGet, "/api/session", nil, cookie)
	body = decodeBody(t, w)
	if body["setup_complete"] != true || body["authenticated"] != true {
		t.Errorf("post-setup: got %+v", body)
	}
	if body["username"] != "admin" {
		t.Errorf("username = %v, want admin", body["username"])
	}
}

// --- Candidates pagination ------------------------------------------------

func TestCandidatesKeysetPaginationNoDupesOrGaps(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	ctx := context.Background()

	const n = 7
	for i := 0; i < n; i++ {
		codec := "h264"
		if _, err := st.Media.Insert(ctx, &store.MediaFile{
			Path:                  filepath.Join("/media/movies", string(rune('a'+i))+".mkv"),
			LibraryType:           "movie",
			SizeBytes:             int64((i + 1) * 1000),
			Mtime:                 1,
			Fingerprint:           "fp" + string(rune('a'+i)),
			VideoCodec:            &codec,
			PredictedSavingsBytes: int64((i + 1) * 100),
			Status:                "active",
		}); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	seen := make(map[float64]bool)
	path := "/api/candidates?limit=2"
	pages := 0
	for {
		w := doReq(h, http.MethodGet, path, nil, cookie)
		if w.Code != http.StatusOK {
			t.Fatalf("candidates: want 200, got %d (%s)", w.Code, w.Body.String())
		}
		body := decodeBody(t, w)
		items, _ := body["items"].([]any)
		for _, it := range items {
			m := it.(map[string]any)
			id := m["id"].(float64)
			if seen[id] {
				t.Fatalf("duplicate id %v across pages", id)
			}
			seen[id] = true
		}
		if len(items) < 2 {
			break
		}
		cur, ok := body["next_cursor"].(map[string]any)
		if !ok {
			break
		}
		path = "/api/candidates?limit=2" +
			"&after_savings=" + strconv.FormatInt(int64(cur["after_savings"].(float64)), 10) +
			"&after_id=" + strconv.FormatInt(int64(cur["after_id"].(float64)), 10)
		pages++
		if pages > 20 {
			t.Fatal("pagination did not terminate")
		}
	}
	if len(seen) != n {
		t.Fatalf("walked %d distinct candidates, want %d", len(seen), n)
	}
}

// --- Jobs -----------------------------------------------------------------

func TestCreateJobsEchoesResolvedSelection(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	ctx := context.Background()

	codec := "h264"
	id, err := st.Media.Insert(ctx, &store.MediaFile{
		Path: "/media/movies/a.mkv", LibraryType: "movie", SizeBytes: 5000,
		Mtime: 1, Fingerprint: "fpa", VideoCodec: &codec,
		PredictedSavingsBytes: 2000, Status: "active",
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	hevc := "hevc"
	hevcID, err := st.Media.Insert(ctx, &store.MediaFile{
		Path: "/media/movies/b.mkv", LibraryType: "movie", SizeBytes: 5000,
		Mtime: 1, Fingerprint: "fpb", VideoCodec: &hevc, IsAlreadyHEVC: true, Status: "active",
	})
	if err != nil {
		t.Fatalf("insert hevc: %v", err)
	}

	w := doReq(h, http.MethodPost, "/api/jobs", map[string]any{
		"file_ids": []int64{id, hevcID, 9999},
	}, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("create jobs: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	queued, _ := body["queued"].([]any)
	skipped, _ := body["skipped"].([]any)
	if len(queued) != 1 {
		t.Fatalf("queued = %d, want 1 (only the eligible h264 file)", len(queued))
	}
	if len(skipped) != 2 {
		t.Fatalf("skipped = %d, want 2 (hevc + missing id)", len(skipped))
	}
	q0 := queued[0].(map[string]any)
	if int64(q0["media_file_id"].(float64)) != id {
		t.Errorf("queued media_file_id = %v, want %d", q0["media_file_id"], id)
	}

	// Listing shows the queued job with position 1.
	w = doReq(h, http.MethodGet, "/api/jobs?status=queued", nil, cookie)
	body = decodeBody(t, w)
	items := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("job list = %d, want 1", len(items))
	}
	if pos := items[0].(map[string]any)["queue_position"].(float64); pos != 1 {
		t.Errorf("queue_position = %v, want 1", pos)
	}
}

// --- Dry run --------------------------------------------------------------

func TestDryRunProjectsSavingsWithoutQueueing(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	ctx := context.Background()

	codec := "h264"
	var ids []int64
	for i := 0; i < 3; i++ {
		id, err := st.Media.Insert(ctx, &store.MediaFile{
			Path: filepath.Join("/media/movies", string(rune('a'+i))+".mkv"), LibraryType: "movie",
			SizeBytes: 1000, Mtime: 1, Fingerprint: "fp" + string(rune('a'+i)),
			VideoCodec: &codec, PredictedSavingsBytes: 400, Status: "active",
		})
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
		ids = append(ids, id)
	}

	w := doReq(h, http.MethodGet,
		"/api/dry-run?ids="+strconv.FormatInt(ids[0], 10)+","+strconv.FormatInt(ids[1], 10),
		nil, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("dry-run: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["file_count"].(float64) != 2 {
		t.Errorf("file_count = %v, want 2", body["file_count"])
	}
	if body["predicted_savings_bytes"].(float64) != 800 {
		t.Errorf("predicted_savings_bytes = %v, want 800", body["predicted_savings_bytes"])
	}

	// No jobs were created.
	jobs, err := st.Jobs.ListAll(ctx)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("dry-run created %d jobs, want 0", len(jobs))
	}
}

// --- Settings live config -------------------------------------------------

func TestSettingsLiveUpdate(t *testing.T) {
	srv, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)

	w := doReq(h, http.MethodPut, "/api/settings", map[string]any{
		"scan_interval":     "12h",
		"scan_anchor":       "02:30",
		"probe_concurrency": 8,
	}, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("put settings: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if srv.live.ScanInterval() != 12*time.Hour {
		t.Errorf("scan interval not applied: %v", srv.live.ScanInterval())
	}
	if srv.live.ScanAnchor() != "02:30" {
		t.Errorf("scan anchor not applied: %v", srv.live.ScanAnchor())
	}
	if srv.live.ProbeConcurrency() != 8 {
		t.Errorf("probe concurrency not applied: %d", srv.live.ProbeConcurrency())
	}

	body := decodeBody(t, w)
	if body["scan_interval"] != "12h0m0s" {
		t.Errorf("scan_interval echoed = %v", body["scan_interval"])
	}
	if body["scan_anchor"] != "02:30" {
		t.Errorf("scan_anchor echoed = %v", body["scan_anchor"])
	}
	if body["movies_path"] != "/media/movies" {
		t.Errorf("movies_path = %v, want read-only /media/movies", body["movies_path"])
	}
}

func TestSettingsRejectsBadValue(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	w := doReq(h, http.MethodPut, "/api/settings", map[string]any{
		"encode_window_start": "99:99",
	}, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad window: want 400, got %d", w.Code)
	}
}
