package startup

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
)

// CheckBinaries verifies ffmpeg and ffprobe are present and executable,
// then logs their versions.
func CheckBinaries() error {
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		path, err := exec.LookPath(bin)
		if err != nil {
			return fmt.Errorf("%s not found in PATH: %w", bin, err)
		}
		out, err := exec.Command(path, "-version").Output()
		if err != nil {
			return fmt.Errorf("%s -version failed: %w", bin, err)
		}
		// Log only the first line (contains the version string)
		first := firstLine(out)
		slog.Info("binary ready", "bin", bin, "version", first)
	}
	return nil
}

// CheckMounts verifies each media mount exists and is readable.
func CheckMounts(paths ...string) error {
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("media mount not accessible %q: %w", p, err)
		}
		f, err := os.Open(p)
		if err != nil {
			return fmt.Errorf("media mount not readable %q: %w", p, err)
		}
		f.Close()
	}
	return nil
}

func firstLine(b []byte) string {
	for i, c := range b {
		if c == '\n' {
			return string(b[:i])
		}
	}
	return string(b)
}
