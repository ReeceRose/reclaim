// Package ffmpeg is the typed encode wrapper: it builds the libx265 command from
// a transcode profile, runs it to a temp file, parses live progress, and cancels
// cleanly by killing the whole process group.
package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Options describes one encode. CRF/preset come from the chosen profile, never
// hardcoded. ExtraArgs is the profile's advanced-flag escape hatch and is
// inserted verbatim after the stream-handling flags.
type Options struct {
	InputPath  string
	OutputPath string
	CRF        int
	Preset     string
	ExtraArgs  []string
	// DurationSeconds is the known source duration from the probe, used to turn
	// ffmpeg's out_time into a percent. Zero means "unknown" → no percent emitted.
	DurationSeconds float64
}

// ProgressFunc receives a clamped 0–100 percent on each ffmpeg progress tick.
type ProgressFunc func(percent float64)

// EncodeError wraps a non-zero ffmpeg exit with captured stderr context.
type EncodeError struct {
	Path string
	Msg  string
}

func (e *EncodeError) Error() string {
	return fmt.Sprintf("ffmpeg %q: %s", e.Path, e.Msg)
}

// Encode runs libx265 against opts.InputPath, writing to opts.OutputPath. Video
// is re-encoded; audio and subtitles are copied untouched so nothing is
// silently dropped on remux. Progress is parsed from `-progress pipe:1` and
// reported via onProgress (may be nil). Cancelling ctx kills the whole ffmpeg
// process group, not just the parent — ffmpeg can spawn children.
func Encode(ctx context.Context, opts Options, onProgress ProgressFunc) error {
	args := []string{
		"-nostdin",
		"-y",
		"-i", opts.InputPath,
		"-c:v", "libx265",
		"-crf", strconv.Itoa(opts.CRF),
		"-preset", opts.Preset,
		"-c:a", "copy",
		"-c:s", "copy",
	}
	args = append(args, opts.ExtraArgs...)
	// Progress on stdout, no noisy stats on stderr; output last.
	args = append(args, "-progress", "pipe:1", "-nostats", opts.OutputPath)

	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	setProcessGroup(cmd)

	// Override the default ctx-cancel (which signals only the parent pid) to kill
	// the whole process group — ffmpeg can spawn children. WaitDelay gives it a
	// moment to die before the I/O pipes are force-closed.
	cmd.Cancel = func() error { return killProcessGroup(cmd) }
	cmd.WaitDelay = 5 * time.Second

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	parseProgress(stdout, opts.DurationSeconds, onProgress)

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		} else {
			msg = lastLine(msg)
		}
		return &EncodeError{Path: opts.InputPath, Msg: msg}
	}
	return nil
}

// parseProgress reads ffmpeg's key=value progress stream and reports a clamped
// percent on each block. Percent is derived from the known source duration
// rather than trusting ffmpeg's own ETA.
func parseProgress(r interface{ Read([]byte) (int, error) }, duration float64, onProgress ProgressFunc) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		if onProgress == nil || duration <= 0 {
			continue
		}

		var outSeconds float64
		switch key {
		case "out_time_us", "out_time_ms":
			// out_time_us is microseconds; out_time_ms is, despite the name,
			// also microseconds in practice — both are scaled the same way.
			if us, err := strconv.ParseInt(val, 10, 64); err == nil && us >= 0 {
				outSeconds = float64(us) / 1_000_000
			} else {
				continue
			}
		case "out_time":
			s, err := parseFFTime(val)
			if err != nil {
				continue
			}
			outSeconds = s
		default:
			continue
		}

		pct := outSeconds / duration * 100
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		onProgress(pct)
	}
}

// parseFFTime parses ffmpeg's HH:MM:SS.micro out_time format into seconds.
func parseFFTime(v string) (float64, error) {
	parts := strings.Split(v, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("bad time %q", v)
	}
	h, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}
	m, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, err
	}
	s, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, err
	}
	return h*3600 + m*60 + s, nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
