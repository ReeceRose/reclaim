package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"reclaim/internal/store"
)

type capturedEvent struct {
	event string
	data  map[string]any
}

type fakeHub struct {
	mu     sync.Mutex
	events []capturedEvent
}

func (h *fakeHub) Broadcast(event string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	m, _ := data.(map[string]any)
	h.events = append(h.events, capturedEvent{event: event, data: m})
}

func (h *fakeHub) all() []capturedEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]capturedEvent(nil), h.events...)
}

// testClock is a manually advanced clock so batching windows are exercised
// without sleeping.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestNotifier(t *testing.T) (*Notifier, *store.Store, *fakeHub, *testClock) {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	hub := &fakeHub{}
	clock := &testClock{now: time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)}

	n := New(st)
	n.SetBroadcaster(hub)
	n.now = clock.Now
	return n, st, hub, clock
}

func setNotify(t *testing.T, st *store.Store, cfg store.NotifySettings) {
	t.Helper()
	if err := st.Settings.SetNotify(context.Background(), cfg); err != nil {
		t.Fatalf("set notify settings: %v", err)
	}
}

func insertCandidate(t *testing.T, st *store.Store, path, series string, season int) int64 {
	t.Helper()
	codec := "h264"
	f := &store.MediaFile{
		Path:                  path,
		LibraryType:           store.LibraryTypeTV,
		SizeBytes:             4 * 1024 * 1024 * 1024,
		VideoCodec:            &codec,
		PredictedSavingsBytes: 1024 * 1024 * 1024,
		Status:                store.MediaStatusActive,
		SeriesTitle:           &series,
		SeasonNumber:          &season,
	}
	id, err := st.Media.Insert(context.Background(), f)
	if err != nil {
		t.Fatalf("insert %s: %v", path, err)
	}
	return id
}

func eventCount(t *testing.T, st *store.Store) int {
	t.Helper()
	events, err := st.Events.List(context.Background(), store.EventFilter{Limit: 50})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	return len(events)
}

// The core promise: a season landing file by file produces one notification,
// and only once the library has been quiet for the configured delay.
func TestNotifier_holdsBatchUntilQuiet(t *testing.T) {
	n, st, hub, clock := newTestNotifier(t)
	ctx := context.Background()
	setNotify(t, st, store.NotifySettings{
		Enabled: true, DelaySeconds: 900, WebhookFormat: store.WebhookFormatJSON,
	})

	for i, path := range []string{
		"/tv/Severance/Season 3/S03E01.mkv",
		"/tv/Severance/Season 3/S03E02.mkv",
		"/tv/Severance/Season 3/S03E03.mkv",
	} {
		n.Add(insertCandidate(t, st, path, "Severance", 3))
		if i < 2 {
			// Each new arrival restarts the quiet period.
			clock.advance(5 * time.Minute)
			n.step(ctx)
			if got := eventCount(t, st); got != 0 {
				t.Fatalf("event written after %d additions; want none until the batch settles", i+1)
			}
		}
	}

	clock.advance(14 * time.Minute)
	n.step(ctx)
	if got := eventCount(t, st); got != 0 {
		t.Fatalf("event written before the delay elapsed (got %d)", got)
	}

	clock.advance(2 * time.Minute)
	n.step(ctx)

	events, err := st.Events.List(ctx, store.EventFilter{Limit: 50})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want exactly 1 for the batch", len(events))
	}
	if events[0].Type != store.EventCandidatesAdded {
		t.Errorf("event type = %q, want %q", events[0].Type, store.EventCandidatesAdded)
	}
	if !strings.Contains(events[0].Message, "3 new re-encode candidates") {
		t.Errorf("message = %q, want it to count all 3 files", events[0].Message)
	}
	if !strings.Contains(events[0].Message, "Severance · Season 3") {
		t.Errorf("message = %q, want the season named", events[0].Message)
	}

	broadcasts := hub.all()
	if len(broadcasts) != 1 || broadcasts[0].event != "event_created" {
		t.Fatalf("broadcasts = %+v, want one event_created", broadcasts)
	}
	if n.Pending() != 0 {
		t.Errorf("pending = %d after flush, want 0", n.Pending())
	}
}

// A trickle that never goes quiet still has to be announced eventually.
func TestNotifier_flushesOverdueBatch(t *testing.T) {
	n, st, _, clock := newTestNotifier(t)
	ctx := context.Background()
	setNotify(t, st, store.NotifySettings{
		Enabled: true, DelaySeconds: 600, WebhookFormat: store.WebhookFormatJSON,
	})

	// One arrival every 5 minutes never satisfies the 10-minute quiet period,
	// but the batch is capped at 4× the delay.
	for i := range 12 {
		n.Add(insertCandidate(t, st, "/tv/Show/Season 1/S01E"+string(rune('a'+i))+".mkv", "Show", 1))
		clock.advance(5 * time.Minute)
		n.step(ctx)
	}

	if got := eventCount(t, st); got != 1 {
		t.Fatalf("got %d events, want 1 from the max-wait cap", got)
	}
}

func TestNotifier_disabledDropsPending(t *testing.T) {
	n, st, _, clock := newTestNotifier(t)
	ctx := context.Background()
	setNotify(t, st, store.NotifySettings{
		Enabled: false, DelaySeconds: 60, WebhookFormat: store.WebhookFormatJSON,
	})

	n.Add(insertCandidate(t, st, "/tv/Show/Season 1/S01E01.mkv", "Show", 1))
	n.step(ctx)

	if n.Pending() != 0 {
		t.Errorf("pending = %d while disabled, want the backlog dropped", n.Pending())
	}

	// Re-enabling must not replay what landed while notifications were off.
	setNotify(t, st, store.NotifySettings{
		Enabled: true, DelaySeconds: 60, WebhookFormat: store.WebhookFormatJSON,
	})
	clock.advance(2 * time.Minute)
	n.step(ctx)

	if got := eventCount(t, st); got != 0 {
		t.Errorf("got %d events, want none replayed", got)
	}
}

// Rows are queued at insert time and a lot can happen during the delay. Only
// what is still a candidate at send time is announced.
func TestNotifier_skipsRowsThatStoppedBeingCandidates(t *testing.T) {
	n, st, _, _ := newTestNotifier(t)
	ctx := context.Background()
	setNotify(t, st, store.NotifySettings{
		Enabled: true, DelaySeconds: 0, WebhookFormat: store.WebhookFormatJSON,
	})

	keep := insertCandidate(t, st, "/tv/Show/Season 1/S01E01.mkv", "Show", 1)

	// A rename looks like an insert to the scanner: it probes the destination
	// path into a new row, then folds it into the original via RecordMove, which
	// deletes the duplicate. The queued ID must not survive that.
	original := insertCandidate(t, st, "/tv/Show/Season 1/S01E02.mkv", "Show", 1)
	renamed := "/tv/Show/Season 1/S01E02 - Title.mkv"
	duplicate := insertCandidate(t, st, renamed, "Show", 1)
	n.Add(keep, duplicate)

	if err := st.Media.RecordMove(ctx, original, duplicate, renamed); err != nil {
		t.Fatalf("record move: %v", err)
	}

	if err := n.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	events, err := st.Events.List(ctx, store.EventFilter{Limit: 50})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if !strings.Contains(events[0].Message, "1 new re-encode candidate ·") {
		t.Errorf("message = %q, want the singular form (one surviving row)", events[0].Message)
	}
}

// Every queued row reconciled away means there is nothing to announce at all.
func TestNotifier_silentWhenNothingSurvives(t *testing.T) {
	n, st, hub, _ := newTestNotifier(t)
	ctx := context.Background()
	setNotify(t, st, store.NotifySettings{
		Enabled: true, DelaySeconds: 0, WebhookFormat: store.WebhookFormatJSON,
	})

	n.Add(4242) // never existed
	if err := n.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if got := eventCount(t, st); got != 0 {
		t.Errorf("got %d events, want none", got)
	}
	if got := hub.all(); len(got) != 0 {
		t.Errorf("broadcasts = %+v, want none", got)
	}
}

func TestNotifier_postsWebhook(t *testing.T) {
	n, st, _, _ := newTestNotifier(t)
	ctx := context.Background()

	type received struct {
		contentType string
		body        []byte
	}
	got := make(chan received, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- received{contentType: r.Header.Get("Content-Type"), body: body}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	setNotify(t, st, store.NotifySettings{
		Enabled: true, DelaySeconds: 0,
		WebhookURL: srv.URL, WebhookFormat: store.WebhookFormatJSON,
	})

	n.Add(insertCandidate(t, st, "/tv/Severance/Season 3/S03E01.mkv", "Severance", 3))
	if err := n.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	select {
	case r := <-got:
		if r.contentType != "application/json" {
			t.Errorf("content type = %q, want application/json", r.contentType)
		}
		var payload WebhookPayload
		if err := json.Unmarshal(r.body, &payload); err != nil {
			t.Fatalf("unmarshal body %s: %v", r.body, err)
		}
		if payload.Event != store.EventCandidatesAdded {
			t.Errorf("event = %q, want %q", payload.Event, store.EventCandidatesAdded)
		}
		if payload.Count != 1 || payload.Titles != 1 {
			t.Errorf("counts = %d files / %d titles, want 1/1", payload.Count, payload.Titles)
		}
		if payload.Title != "Severance" || payload.LibraryType != store.LibraryTypeTV {
			t.Errorf("title = %q (%s), want the series", payload.Title, payload.LibraryType)
		}
		if len(payload.Seasons) != 1 || payload.Seasons[0].Number != 3 {
			t.Errorf("seasons = %+v, want season 3", payload.Seasons)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("webhook was never called")
	}
}

// A receiver that rejects the delivery must not stop the events-feed entry,
// which was already written by then.
func TestNotifier_webhookFailureKeepsEvent(t *testing.T) {
	n, st, _, _ := newTestNotifier(t)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	setNotify(t, st, store.NotifySettings{
		Enabled: true, DelaySeconds: 0,
		WebhookURL: srv.URL, WebhookFormat: store.WebhookFormatJSON,
	})

	n.Add(insertCandidate(t, st, "/tv/Show/Season 1/S01E01.mkv", "Show", 1))
	err := n.Flush(ctx)

	if err == nil {
		t.Fatal("expected the webhook failure to be returned")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %v, want it to quote the status", err)
	}
	if got := eventCount(t, st); got != 1 {
		t.Errorf("got %d events, want the feed entry to survive the failed webhook", got)
	}
}

func TestSendTest_deliversSample(t *testing.T) {
	n, st, _, _ := newTestNotifier(t)
	ctx := context.Background()

	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// The stored URL is empty: the override is what makes an unsaved URL testable.
	setNotify(t, st, store.NotifySettings{
		Enabled: true, DelaySeconds: 900, WebhookFormat: store.WebhookFormatJSON,
	})

	if err := n.SendTest(ctx, srv.URL, store.WebhookFormatSlack); err != nil {
		t.Fatalf("send test: %v", err)
	}

	select {
	case body := <-got:
		var payload struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			t.Fatalf("unmarshal %s: %v", body, err)
		}
		if !strings.Contains(payload.Text, "Reclaim test") {
			t.Errorf("text = %q, want it marked as a test", payload.Text)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("webhook was never called")
	}

	// A test send must not write to the events feed.
	if got := eventCount(t, st); got != 0 {
		t.Errorf("got %d events, want none from a test send", got)
	}
}

func TestSendTest_requiresURL(t *testing.T) {
	n, st, _, _ := newTestNotifier(t)
	setNotify(t, st, store.DefaultNotifySettings())

	if err := n.SendTest(context.Background(), "", ""); err == nil {
		t.Fatal("expected an error with no webhook configured")
	}
}

func TestBuildBody_perFormat(t *testing.T) {
	// Two seasons, so the body has detail lines to carry.
	summary := Split([]store.MediaFile{
		tvFile("/tv/Severance/Season 1/S01E01.mkv", "Severance", 1, 1024*1024*1024),
		tvFile("/tv/Severance/Season 3/S03E01.mkv", "Severance", 3, 1024*1024*1024),
	})[0]
	message := summary.Message()

	t.Run("discord", func(t *testing.T) {
		body, ct, err := buildBody(store.WebhookFormatDiscord, summary, 0)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if ct != "application/json" {
			t.Errorf("content type = %q", ct)
		}
		var payload struct {
			Embeds []struct {
				Title       string `json:"title"`
				Description string `json:"description"`
			} `json:"embeds"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal %s: %v", body, err)
		}
		if len(payload.Embeds) != 1 || payload.Embeds[0].Title != message {
			t.Errorf("embeds = %+v, want one titled %q", payload.Embeds, message)
		}
		if !strings.Contains(payload.Embeds[0].Description, "Season 3 —") {
			t.Errorf("description = %q, want the seasons listed", payload.Embeds[0].Description)
		}
	})

	t.Run("ntfy", func(t *testing.T) {
		body, ct, err := buildBody(store.WebhookFormatNtfy, summary, 0)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if !strings.HasPrefix(ct, "text/plain") {
			t.Errorf("content type = %q, want text/plain", ct)
		}
		if !strings.Contains(string(body), "Season 3 —") {
			t.Errorf("body = %q, want the seasons listed", body)
		}
	})

	t.Run("unknown format falls back to json", func(t *testing.T) {
		body, ct, err := buildBody("carrier-pigeon", summary, 99)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if ct != "application/json" {
			t.Errorf("content type = %q", ct)
		}
		var payload WebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal %s: %v", body, err)
		}
		if payload.OccurredAt != 99 {
			t.Errorf("occurred_at = %d, want 99", payload.OccurredAt)
		}
	})
}

// ntfy takes the title out of band, so the header has to carry the message.
func TestPostWebhook_ntfySetsTitleHeader(t *testing.T) {
	n, _, _, _ := newTestNotifier(t)

	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get("Title")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	summary := Split([]store.MediaFile{movieFile("/movies/Dune (2021)/Dune (2021).mkv", 100)})[0]
	cfg := store.NotifySettings{WebhookURL: srv.URL, WebhookFormat: store.WebhookFormatNtfy}

	if err := postWebhook(context.Background(), n.client, cfg, summary, 0); err != nil {
		t.Fatalf("post: %v", err)
	}
	if title := <-got; !strings.Contains(title, "Dune (2021)") {
		t.Errorf("Title header = %q, want the message", title)
	}
}

// insertMovie adds a movie-library candidate.
func insertMovie(t *testing.T, st *store.Store, path string) int64 {
	t.Helper()
	codec := "h264"
	id, err := st.Media.Insert(context.Background(), &store.MediaFile{
		Path:                  path,
		LibraryType:           store.LibraryTypeMovies,
		SizeBytes:             8 * 1024 * 1024 * 1024,
		VideoCodec:            &codec,
		PredictedSavingsBytes: 2 * 1024 * 1024 * 1024,
		Status:                store.MediaStatusActive,
	})
	if err != nil {
		t.Fatalf("insert %s: %v", path, err)
	}
	return id
}

// Two shows and a movie arriving in the same window must never share a message:
// one notification per title, each naming what it is about.
func TestNotifier_oneNotificationPerTitle(t *testing.T) {
	n, st, hub, _ := newTestNotifier(t)
	ctx := context.Background()

	var posts struct {
		mu     sync.Mutex
		bodies []string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		posts.mu.Lock()
		posts.bodies = append(posts.bodies, string(body))
		posts.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	setNotify(t, st, store.NotifySettings{
		Enabled: true, DelaySeconds: 0,
		WebhookURL: srv.URL, WebhookFormat: store.WebhookFormatJSON,
	})

	n.Add(insertCandidate(t, st, "/tv/Severance/Season 3/S03E01.mkv", "Severance", 3))
	n.Add(insertCandidate(t, st, "/tv/Severance/Season 3/S03E02.mkv", "Severance", 3))
	n.Add(insertCandidate(t, st, "/tv/The Wire/Season 1/S01E01.mkv", "The Wire", 1))
	n.Add(insertMovie(t, st, "/movies/Dune (2021)/Dune (2021).mkv"))

	if err := n.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	events, err := st.Events.List(ctx, store.EventFilter{Limit: 50})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3 (Severance, The Wire, Dune):\n%s", len(events), messagesOf(events))
	}

	byTitle := map[string]string{}
	for _, e := range events {
		switch {
		case strings.HasPrefix(e.Message, "Severance"):
			byTitle["Severance"] = e.Message
		case strings.HasPrefix(e.Message, "The Wire"):
			byTitle["The Wire"] = e.Message
		case strings.HasPrefix(e.Message, "Dune"):
			byTitle["Dune"] = e.Message
		}
	}
	if len(byTitle) != 3 {
		t.Fatalf("messages did not lead with one title each:\n%s", messagesOf(events))
	}
	if !strings.Contains(byTitle["Severance"], "2 new re-encode candidates") {
		t.Errorf("Severance message = %q, want both its episodes", byTitle["Severance"])
	}
	// No message may mention another title.
	for title, msg := range byTitle {
		for _, other := range []string{"Severance", "The Wire", "Dune"} {
			if other != title && strings.Contains(msg, other) {
				t.Errorf("%q message leaked %q: %q", title, other, msg)
			}
		}
	}

	if got := len(hub.all()); got != 3 {
		t.Errorf("broadcasts = %d, want one per title", got)
	}

	posts.mu.Lock()
	defer posts.mu.Unlock()
	if len(posts.bodies) != 3 {
		t.Fatalf("webhook posts = %d, want one per title", len(posts.bodies))
	}
	for _, body := range posts.bodies {
		var payload WebhookPayload
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			t.Fatalf("unmarshal %s: %v", body, err)
		}
		if payload.Title == "" || payload.Titles != 1 {
			t.Errorf("payload covers %d titles (%q), want exactly one", payload.Titles, payload.Title)
		}
	}
}

// Past the per-flush cap the notifications would be noise (and would trip a chat
// service's rate limit), so the whole flush collapses into one rollup.
func TestNotifier_rollsUpBulkArrival(t *testing.T) {
	n, st, _, _ := newTestNotifier(t)
	ctx := context.Background()
	setNotify(t, st, store.NotifySettings{
		Enabled: true, DelaySeconds: 0, WebhookFormat: store.WebhookFormatJSON,
	})

	for i := range maxTitlesPerFlush + 3 {
		name := string(rune('A' + i))
		n.Add(insertMovie(t, st, "/movies/"+name+"/"+name+".mkv"))
	}

	if err := n.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	events, err := st.Events.List(ctx, store.EventFilter{Limit: 50})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1 rollup:\n%s", len(events), messagesOf(events))
	}
	want := fmt.Sprintf("%d new re-encode candidates across %d titles", maxTitlesPerFlush+3, maxTitlesPerFlush+3)
	if !strings.Contains(events[0].Message, want) {
		t.Errorf("message = %q, want it to contain %q", events[0].Message, want)
	}
}

func messagesOf(events []store.Event) string {
	var b strings.Builder
	for _, e := range events {
		b.WriteString("  " + e.Message + "\n")
	}
	return b.String()
}
