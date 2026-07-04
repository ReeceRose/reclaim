// Package version holds build-time identifiers set via -ldflags -X. Defaults
// apply to `go run`/`go build` without ldflags (i.e. local dev).
package version

var (
	Version = "dev"
	Commit  = "unknown"
)
