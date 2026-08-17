package store

import (
	"context"
	"testing"
)

func TestNotify_defaultsOnFreshDB(t *testing.T) {
	s := openTestStore(t)

	got := s.Settings.Notify(context.Background())
	want := DefaultNotifySettings()
	if got != want {
		t.Errorf("notify settings on fresh DB = %+v, want %+v", got, want)
	}
}

func TestSetNotify_roundTrips(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	want := NotifySettings{
		Enabled:       false,
		DelaySeconds:  60,
		WebhookURL:    "https://example.test/hook",
		WebhookFormat: WebhookFormatDiscord,
	}
	if err := s.Settings.SetNotify(ctx, want); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := s.Settings.Notify(ctx); got != want {
		t.Errorf("notify settings = %+v, want %+v", got, want)
	}
}

func TestSetNotify_trimsWebhookURL(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	cfg := DefaultNotifySettings()
	cfg.WebhookURL = "  https://example.test/hook  "
	if err := s.Settings.SetNotify(ctx, cfg); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := s.Settings.Notify(ctx).WebhookURL; got != "https://example.test/hook" {
		t.Errorf("webhook url = %q, want it trimmed", got)
	}
}

func TestSetNotify_rejectsBadValues(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	cases := map[string]NotifySettings{
		"unknown format": {DelaySeconds: 60, WebhookFormat: "carrier-pigeon"},
		"negative delay": {DelaySeconds: -1, WebhookFormat: WebhookFormatJSON},
		"delay too long": {DelaySeconds: MaxNotifyDelaySeconds + 1, WebhookFormat: WebhookFormatJSON},
		"non-http URL":   {DelaySeconds: 60, WebhookFormat: WebhookFormatJSON, WebhookURL: "ftp://example.test"},
		"hostless URL":   {DelaySeconds: 60, WebhookFormat: WebhookFormatJSON, WebhookURL: "https://"},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if err := s.Settings.SetNotify(ctx, cfg); err == nil {
				t.Fatalf("expected error for %s", name)
			}
		})
	}

	// A rejected write must leave the stored settings untouched.
	if got := s.Settings.Notify(ctx); got != DefaultNotifySettings() {
		t.Errorf("settings after rejected writes = %+v, want defaults", got)
	}
}

// A format written directly to the column, outside SetNotify's validation, must
// not reach the sender as-is.
func TestNotify_fallsBackOnGarbageFormat(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.w.ExecContext(ctx,
		"UPDATE settings SET notify_webhook_format = 'smoke-signal' WHERE id = 1",
	); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if got := s.Settings.Notify(ctx).WebhookFormat; got != WebhookFormatJSON {
		t.Errorf("webhook format = %q, want %q", got, WebhookFormatJSON)
	}
}

func TestCandidatesByID_returnsOnlyLiveCandidates(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	candidate, err := s.Media.Insert(ctx, testFile{
		path: "/tv/Show/S01E01.mkv", codec: "h264", savings: 500,
	}.toMedia())
	if err != nil {
		t.Fatalf("insert candidate: %v", err)
	}
	bigger, err := s.Media.Insert(ctx, testFile{
		path: "/tv/Show/S01E02.mkv", codec: "h264", savings: 900,
	}.toMedia())
	if err != nil {
		t.Fatalf("insert bigger: %v", err)
	}
	alreadyHEVC, err := s.Media.Insert(ctx, testFile{
		path: "/tv/Show/S01E03.mkv", codec: "hevc", hevc: true,
	}.toMedia())
	if err != nil {
		t.Fatalf("insert hevc: %v", err)
	}
	missing, err := s.Media.Insert(ctx, testFile{
		path: "/tv/Show/S01E04.mkv", codec: "h264", savings: 400, status: "missing",
	}.toMedia())
	if err != nil {
		t.Fatalf("insert missing: %v", err)
	}

	got, err := s.Media.CandidatesByID(ctx, []int64{candidate, bigger, alreadyHEVC, missing, 9999})
	if err != nil {
		t.Fatalf("candidates by id: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2 (%+v)", len(got), got)
	}
	// Ranked by predicted savings, so the bigger win leads.
	if got[0].ID != bigger || got[1].ID != candidate {
		t.Errorf("ids = [%d %d], want [%d %d] (savings desc)", got[0].ID, got[1].ID, bigger, candidate)
	}
}

func TestCandidatesByID_emptyInput(t *testing.T) {
	s := openTestStore(t)

	got, err := s.Media.CandidatesByID(context.Background(), nil)
	if err != nil {
		t.Fatalf("candidates by id: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d rows, want 0", len(got))
	}
}

func TestScansCompletedBefore(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	first, err := s.Scans.Create(ctx, "startup", 100)
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if n, err := s.Scans.CompletedBefore(ctx, first); err != nil || n != 0 {
		t.Fatalf("before first scan = %d (err %v), want 0", n, err)
	}

	done := int64(200)
	if err := s.Scans.Complete(ctx, &ScanRun{ID: first, CompletedAt: &done}); err != nil {
		t.Fatalf("complete first: %v", err)
	}

	second, err := s.Scans.Create(ctx, "scheduled", 300)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if n, err := s.Scans.CompletedBefore(ctx, second); err != nil || n != 1 {
		t.Fatalf("before second scan = %d (err %v), want 1", n, err)
	}
	// The run's own completion must not count towards its own baseline check.
	if err := s.Scans.Complete(ctx, &ScanRun{ID: second, CompletedAt: &done}); err != nil {
		t.Fatalf("complete second: %v", err)
	}
	if n, err := s.Scans.CompletedBefore(ctx, second); err != nil || n != 1 {
		t.Fatalf("after own completion = %d (err %v), want 1", n, err)
	}
}
