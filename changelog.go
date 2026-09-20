// Package reclaim embeds repo-root assets the binary needs at runtime. It
// exists only because go:embed cannot reach outside its own package directory
// and CHANGELOG.md belongs at the repository root, where GitHub surfaces it.
// Parsing lives in internal/changelog.
package reclaim

import _ "embed"

//go:embed CHANGELOG.md
var changelogMarkdown string

// ChangelogMarkdown returns the release notes compiled into this binary. The
// file is written by scripts/release.sh on the commit each tag points at, so
// the notes always describe the build serving them.
func ChangelogMarkdown() string { return changelogMarkdown }
