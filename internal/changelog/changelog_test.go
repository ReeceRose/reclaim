package changelog

import (
	"strings"
	"testing"
)

const sample = "# Changelog\n" +
	"\n" +
	"Preamble that belongs to no release.\n" +
	"\n" +
	"## v1.2.0 — 2026-09-20\n" +
	"\n" +
	"Headline line.\n" +
	"\n" +
	"### What's Changed\n" +
	"- did a thing\n" +
	"\n" +
	"### Docker\n" +
	"\n" +
	"```\n" +
	"## not a heading, it is inside a fence\n" +
	"docker pull ghcr.io/ReeceRose/reclaim:1.2.0\n" +
	"```\n" +
	"\n" +
	"## v1.1.0\n" +
	"\n" +
	"Older release with no date.\n"

func TestParse(t *testing.T) {
	got := Parse(sample)
	if len(got) != 2 {
		t.Fatalf("got %d releases, want 2", len(got))
	}

	first := got[0]
	if first.Tag != "v1.2.0" || first.Version != "1.2.0" || first.Date != "2026-09-20" {
		t.Errorf("first heading parsed as %+v", first)
	}
	if want := "https://github.com/ReeceRose/reclaim/releases/tag/v1.2.0"; first.URL != want {
		t.Errorf("URL = %q, want %q", first.URL, want)
	}
	if !strings.Contains(first.Body, "did a thing") {
		t.Errorf("body missing bullet: %q", first.Body)
	}
	// A `##` line inside a fenced block must not end the release.
	if !strings.Contains(first.Body, "it is inside a fence") {
		t.Errorf("fenced heading ended the release early: %q", first.Body)
	}
	if strings.Contains(first.Body, "Older release") {
		t.Errorf("first release swallowed the second: %q", first.Body)
	}

	second := got[1]
	if second.Tag != "v1.1.0" || second.Date != "" {
		t.Errorf("second heading parsed as %+v", second)
	}
	if strings.Contains(second.Body, "Preamble") {
		t.Errorf("preamble leaked into a release body")
	}
}

func TestParseIgnoresPreamble(t *testing.T) {
	if got := Parse("# Changelog\n\nNothing tagged yet.\n"); len(got) != 0 {
		t.Fatalf("got %d releases from a changelog with no entries", len(got))
	}
}

func TestForRejectsDev(t *testing.T) {
	for _, v := range []string{"dev", "", "  "} {
		if _, ok := For(v); ok {
			t.Errorf("For(%q) returned a release", v)
		}
	}
}

// TestEmbeddedChangelogParses guards the real file: a malformed heading in a
// release would silently drop that entry from the UI.
func TestEmbeddedChangelogParses(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("embedded CHANGELOG.md parsed to zero releases")
	}
	seen := map[string]bool{}
	for _, r := range all {
		if r.Version == "" {
			t.Errorf("release %q has no version", r.Tag)
		}
		if r.Body == "" {
			t.Errorf("release %q has an empty body", r.Tag)
		}
		if seen[r.Version] {
			t.Errorf("duplicate entry for %q", r.Version)
		}
		seen[r.Version] = true
	}
}
