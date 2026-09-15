package startup

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"reclaim/internal/media"
)

// DetectEncoders reports, for every target codec Reclaim can encode to, whether
// the ffmpeg on PATH was built with that codec's encoder. A distro ffmpeg may
// ship libx265 without libsvtav1 (or neither), and finding that out at queue
// time is far kinder than at 1 AM in the middle of the encode window.
func DetectEncoders(ctx context.Context) (map[media.TargetCodec]bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-encoders").Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg -encoders failed: %w", err)
	}
	built := parseEncoders(out)

	avail := make(map[media.TargetCodec]bool)
	for _, e := range media.Encoders() {
		ok := built[e.FFmpegEncoder]
		avail[e.Codec] = ok
		if ok {
			slog.Info("encoder ready", "codec", e.Codec, "encoder", e.FFmpegEncoder)
		} else {
			slog.Warn("encoder unavailable; profiles targeting it cannot be used",
				"codec", e.Codec, "encoder", e.FFmpegEncoder)
		}
	}
	return avail, nil
}

// parseEncoders extracts encoder names from `ffmpeg -encoders` output. Each
// encoder line is a six-character capability field ("V....D") followed by the
// name; the legend above the "------" separator is skipped.
func parseEncoders(out []byte) map[string]bool {
	names := make(map[string]bool)
	sc := bufio.NewScanner(bytes.NewReader(out))
	listing := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !listing {
			listing = strings.HasPrefix(line, "---")
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || len(fields[0]) != 6 {
			continue
		}
		names[fields[1]] = true
	}
	return names
}
