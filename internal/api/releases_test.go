package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"reclaim/internal/version"
)

func getReleases(t *testing.T, h http.Handler) releasesResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/releases", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/releases = %d, want 200", rec.Code)
	}
	var resp releasesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp
}

func TestListReleasesServesEmbeddedChangelog(t *testing.T) {
	_, h, _, _ := newTestServer(t, true)
	resp := getReleases(t, h)

	if len(resp.Releases) == 0 {
		t.Fatal("no releases returned")
	}
	if resp.RepoURL != version.RepoURL {
		t.Errorf("repo_url = %q, want %q", resp.RepoURL, version.RepoURL)
	}
	first := resp.Releases[0]
	if first.Tag == "" || first.Body == "" || first.URL == "" {
		t.Errorf("incomplete release entry: %+v", first)
	}
}

// A dev build has no changelog entry, so it must never claim there is something
// new to read — otherwise every `make dev` run would nag.
func TestListReleasesNoWhatsNewOnDevBuild(t *testing.T) {
	_, h, _, _ := newTestServer(t, true)
	if version.IsRelease() {
		t.Skip("test binary was built with a release version")
	}
	if resp := getReleases(t, h); resp.WhatsNew {
		t.Error("whats_new is true on a dev build")
	}
}

func TestAckReleaseStampsVersion(t *testing.T) {
	_, h, st, _ := newTestServer(t, true)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/releases/seen", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST /api/releases/seen = %d, want 204", rec.Code)
	}
	if got := st.Settings.LastSeenVersion(context.Background()); got != version.Version {
		t.Errorf("last_seen_version = %q, want %q", got, version.Version)
	}
}

// A fresh install (setup not yet complete, nothing acknowledged) is stamped so
// its first login does not open on a changelog it never upgraded through.
func TestSeedLastSeenVersionSkipsConfiguredInstance(t *testing.T) {
	_, _, st, _ := newTestServer(t, true)
	ctx := context.Background()

	if err := st.Settings.SeedLastSeenVersion(ctx, "9.9.9"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if got := st.Settings.LastSeenVersion(ctx); got != "9.9.9" {
		t.Fatalf("fresh install not stamped: %q", got)
	}

	// Already stamped — a later boot must not overwrite it.
	if err := st.Settings.SeedLastSeenVersion(ctx, "9.9.10"); err != nil {
		t.Fatalf("reseed: %v", err)
	}
	if got := st.Settings.LastSeenVersion(ctx); got != "9.9.9" {
		t.Errorf("stamp overwritten on a later boot: %q", got)
	}
}

func TestSeedLastSeenVersionLeavesExistingInstallUnstamped(t *testing.T) {
	_, _, st, _ := newTestServer(t, true)
	ctx := context.Background()
	completeSetup(t, st)

	if err := st.Settings.SeedLastSeenVersion(ctx, "9.9.9"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if got := st.Settings.LastSeenVersion(ctx); got != "" {
		t.Errorf("configured install was stamped %q; it should be offered the notes", got)
	}
}
