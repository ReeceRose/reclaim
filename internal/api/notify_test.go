package api

import (
	"context"
	"net/http"
	"testing"

	"reclaim/internal/store"
)

// fakeTestNotifier records the SendTest arguments and returns a canned result.
type fakeTestNotifier struct {
	err       error
	gotURL    string
	gotFormat string
	callCount int
}

func (f *fakeTestNotifier) SendTest(_ context.Context, url, format string) error {
	f.callCount++
	f.gotURL = url
	f.gotFormat = format
	return f.err
}

func TestGetSettings_reportsNotifyDefaults(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)

	w := doReq(h, http.MethodGet, "/api/settings", nil, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}

	body := decodeBody(t, w)
	if body["notify_enabled"] != true {
		t.Errorf("notify_enabled = %v, want true", body["notify_enabled"])
	}
	if body["notify_delay_seconds"] != float64(store.DefaultNotifyDelaySeconds) {
		t.Errorf("notify_delay_seconds = %v, want %d", body["notify_delay_seconds"], store.DefaultNotifyDelaySeconds)
	}
	if body["notify_webhook_url"] != "" {
		t.Errorf("notify_webhook_url = %v, want empty", body["notify_webhook_url"])
	}
	if body["notify_webhook_format"] != store.WebhookFormatJSON {
		t.Errorf("notify_webhook_format = %v, want %q", body["notify_webhook_format"], store.WebhookFormatJSON)
	}
}

func TestPutSettings_persistsNotifyFields(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)

	w := doReq(h, http.MethodPut, "/api/settings", map[string]any{
		"notify_delay_seconds":  60,
		"notify_webhook_url":    "https://example.test/hook",
		"notify_webhook_format": store.WebhookFormatDiscord,
	}, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}

	body := decodeBody(t, w)
	if body["notify_webhook_url"] != "https://example.test/hook" {
		t.Errorf("response url = %v", body["notify_webhook_url"])
	}
	// Untouched fields keep their stored value.
	if body["notify_enabled"] != true {
		t.Errorf("notify_enabled = %v, want it left alone", body["notify_enabled"])
	}

	stored := st.Settings.Notify(context.Background())
	if stored.DelaySeconds != 60 || stored.WebhookFormat != store.WebhookFormatDiscord {
		t.Errorf("stored = %+v, want delay 60 and the discord format", stored)
	}
}

// A PUT that only changes the encode window must not rewrite the notify columns.
func TestPutSettings_leavesNotifyAloneWhenUntouched(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)

	want := store.NotifySettings{
		Enabled: false, DelaySeconds: 120,
		WebhookURL: "https://example.test/hook", WebhookFormat: store.WebhookFormatNtfy,
	}
	if err := st.Settings.SetNotify(context.Background(), want); err != nil {
		t.Fatalf("seed: %v", err)
	}

	w := doReq(h, http.MethodPut, "/api/settings", map[string]any{
		"encode_window_start": "01:00",
	}, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}

	if got := st.Settings.Notify(context.Background()); got != want {
		t.Errorf("notify settings = %+v, want them unchanged (%+v)", got, want)
	}
}

func TestPutSettings_rejectsBadWebhook(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)

	w := doReq(h, http.MethodPut, "/api/settings", map[string]any{
		"notify_webhook_url": "not-a-url",
	}, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", w.Code, w.Body.String())
	}
	if got := st.Settings.Notify(context.Background()).WebhookURL; got != "" {
		t.Errorf("stored url = %q after a rejected write, want empty", got)
	}
}

func TestNotifyTest_passesOverridesThrough(t *testing.T) {
	srv, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)

	fake := &fakeTestNotifier{}
	srv.notifier = fake

	w := doReq(h, http.MethodPost, "/api/settings/notify-test", map[string]any{
		"notify_webhook_url":    "https://example.test/hook",
		"notify_webhook_format": store.WebhookFormatSlack,
	}, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if fake.callCount != 1 {
		t.Fatalf("SendTest called %d times, want 1", fake.callCount)
	}
	if fake.gotURL != "https://example.test/hook" || fake.gotFormat != store.WebhookFormatSlack {
		t.Errorf("SendTest(%q, %q), want the body values", fake.gotURL, fake.gotFormat)
	}
}

// A delivery failure is the one webhook error with somewhere to be shown, so it
// has to reach the caller rather than only the log.
func TestNotifyTest_surfacesDeliveryError(t *testing.T) {
	srv, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	srv.notifier = &fakeTestNotifier{err: errNotifierUnavailable}

	w := doReq(h, http.MethodPost, "/api/settings/notify-test", map[string]any{}, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body == "" {
		t.Error("want the receiver's error in the response body")
	}
}
