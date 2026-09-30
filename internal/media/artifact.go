package media

import (
	"path/filepath"
	"strings"
)

// The worker's own files beside an original while it encodes and swaps:
// a.mkv → a.mkv.reclaim-tmp.mkv, a.mkv.reclaim-backup.
const (
	TempSuffix   = ".reclaim-tmp"
	BackupSuffix = ".reclaim-backup"
)

// IsReclaimArtifact reports whether path is one of the worker's temp or
// backup files. The temp keeps its original's extension so ffmpeg muxes the
// same container, which makes it look like media to anything matching on
// extension; indexing one would list a half-written encode as a candidate.
func IsReclaimArtifact(path string) bool {
	name := filepath.Base(path)
	return strings.Contains(name, TempSuffix) || strings.HasSuffix(name, BackupSuffix)
}
