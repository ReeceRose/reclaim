// Package web embeds the built Next.js static export so the single Reclaim
// binary can serve the frontend without any external files.
package web

import (
	"embed"
	"io/fs"
)

// dist holds the static export produced by `pnpm run build` (output: 'export').
// A committed placeholder index.html keeps `go build` working before the
// frontend has been built; a real build overwrites it.
//
//go:embed all:out
var dist embed.FS

// FS returns the embedded frontend rooted at the export directory.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "out")
	if err != nil {
		panic(err)
	}
	return sub
}
