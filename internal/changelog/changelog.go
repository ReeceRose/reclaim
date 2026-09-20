// Package changelog parses the embedded CHANGELOG.md into per-release entries
// so the API can serve the notes for the running build without reaching out to
// GitHub. The file is the release source of truth: scripts/release.sh writes an
// entry, commits it, tags that commit, and publishes the same text as the
// GitHub Release.
package changelog

import (
	"regexp"
	"strings"
	"sync"

	"reclaim"
	"reclaim/internal/version"
)

// Release is one entry in the changelog.
type Release struct {
	// Tag is the git tag as written, e.g. "v0.0.42".
	Tag string
	// Version is Tag without the leading "v", matching version.Version.
	Version string
	// Date is the release date as "2006-01-02", empty if the heading omitted it.
	Date string
	// Body is the entry's Markdown, headings included, with the version
	// heading itself removed.
	Body string
	// URL points at the GitHub Release for this tag.
	URL string
}

// headingRe matches a version heading: "## v0.0.42 — 2026-09-20". The em dash
// and date are optional. Body headings are written at `###` or deeper by
// release.sh precisely so they cannot match here.
var headingRe = regexp.MustCompile(`^## +(v?\d+\.\d+\.\d+[^\s]*)\s*(?:[—–-]\s*(\d{4}-\d{2}-\d{2}))?\s*$`)

// fenceRe matches a fenced code block delimiter. Headings inside a fence are
// literal text, so boundary detection has to skip them.
var fenceRe = regexp.MustCompile("^\\s*```")

// Parse splits changelog Markdown into releases, newest first — the order they
// appear in the file. Any preamble before the first version heading is ignored.
func Parse(md string) []Release {
	var out []Release
	var cur *Release
	var body []string
	inFence := false

	flush := func() {
		if cur == nil {
			return
		}
		cur.Body = strings.Trim(strings.Join(body, "\n"), "\n")
		out = append(out, *cur)
		cur, body = nil, nil
	}

	for _, line := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		if fenceRe.MatchString(line) {
			inFence = !inFence
		}
		if m := headingRe.FindStringSubmatch(line); m != nil && !inFence {
			flush()
			tag := m[1]
			cur = &Release{
				Tag:     tag,
				Version: strings.TrimPrefix(tag, "v"),
				Date:    m[2],
				URL:     version.RepoURL + "/releases/tag/" + tag,
			}
			continue
		}
		if cur != nil {
			body = append(body, line)
		}
	}
	flush()
	return out
}

var (
	once   sync.Once
	loaded []Release
)

// All returns the parsed embedded changelog, newest first. Parsed once.
func All() []Release {
	once.Do(func() { loaded = Parse(reclaim.ChangelogMarkdown()) })
	return loaded
}

// For returns the entry matching a version string, with or without the leading
// "v". Reports false for "dev" builds and for any version the changelog on this
// build predates.
func For(v string) (Release, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" || v == "dev" {
		return Release{}, false
	}
	for _, r := range All() {
		if r.Version == v {
			return r, true
		}
	}
	return Release{}, false
}
