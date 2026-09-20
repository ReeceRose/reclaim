// Package version holds build-time identifiers set via -ldflags -X. Defaults
// apply to `go run`/`go build` without ldflags (i.e. local dev).
package version

var (
	Version = "dev"
	Commit  = "unknown"
)

// RepoURL is the canonical source repository. Used to link release notes back
// to their GitHub Release; no request is ever made to it from the binary.
const RepoURL = "https://github.com/ReeceRose/reclaim"

// IsRelease reports whether this build came from a tag rather than a local or
// untagged build. Release-notes surfaces stay hidden otherwise, since a "dev"
// build has no changelog entry to show.
func IsRelease() bool { return Version != "" && Version != "dev" }
