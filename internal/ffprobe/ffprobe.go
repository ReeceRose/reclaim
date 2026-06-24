package ffprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ProbeError is returned when ffprobe exits non-zero or its output cannot be parsed.
// Callers store the message in probe_error and continue — never panic.
type ProbeError struct {
	Path string
	Msg  string
}

func (e *ProbeError) Error() string {
	return fmt.Sprintf("ffprobe %q: %s", e.Path, e.Msg)
}

// Result holds the mapped fields from a ffprobe run. All video/audio fields are
// nullable because some files have no audio or missing metadata.
type Result struct {
	VideoCodec        *string
	VideoCodecProfile *string
	Width             *int
	Height            *int
	DurationSeconds   *float64
	BitrateKbps       *int
	AudioCodec        *string
	AudioChannels     *int
	ContainerFormat   *string
	IsAlreadyHEVC     bool
}

// Probe runs ffprobe on path and returns the mapped result.
// The context should include a timeout so a pathological file cannot hang a scan.
func Probe(ctx context.Context, path string) (*Result, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			stderr = ": " + strings.TrimSpace(string(ee.Stderr))
		}
		return nil, &ProbeError{Path: path, Msg: fmt.Sprintf("ffprobe failed%s", stderr)}
	}
	return parse(out, path)
}

// parse unmarshals raw ffprobe JSON and maps it to a Result.
// Exported as an internal helper so tests can drive it directly with fixture files.
func parse(data []byte, path string) (*Result, error) {
	var raw probeOutput
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, &ProbeError{Path: path, Msg: fmt.Sprintf("parse JSON: %v", err)}
	}
	return mapResult(&raw), nil
}

// --- Raw JSON types --------------------------------------------------------

type probeOutput struct {
	Format  probeFormat   `json:"format"`
	Streams []probeStream `json:"streams"`
}

type probeFormat struct {
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
	BitRate    string `json:"bit_rate"`
}

type probeStream struct {
	Index     int    `json:"index"`
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
	Profile   string `json:"profile"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Duration  string `json:"duration"`
	BitRate   string `json:"bit_rate"`
	Channels  int    `json:"channels"`
}

// --- Mapping ---------------------------------------------------------------

func mapResult(raw *probeOutput) *Result {
	r := &Result{}

	if raw.Format.FormatName != "" {
		s := raw.Format.FormatName
		r.ContainerFormat = &s
	}

	// Duration: format first, fall back to first video stream.
	r.DurationSeconds = parsePositiveFloat(raw.Format.Duration)
	if r.DurationSeconds == nil {
		for _, s := range raw.Streams {
			if s.CodecType == "video" {
				r.DurationSeconds = parsePositiveFloat(s.Duration)
				break
			}
		}
	}

	// Bitrate: format first, fall back to first video stream.
	r.BitrateKbps = parseBitrateKbps(raw.Format.BitRate)
	if r.BitrateKbps == nil {
		for _, s := range raw.Streams {
			if s.CodecType == "video" {
				r.BitrateKbps = parseBitrateKbps(s.BitRate)
				break
			}
		}
	}

	// First video stream.
	for _, s := range raw.Streams {
		if s.CodecType != "video" {
			continue
		}
		codec := s.CodecName
		r.VideoCodec = &codec
		r.IsAlreadyHEVC = strings.EqualFold(codec, "hevc") || strings.EqualFold(codec, "h265")
		if s.Profile != "" && s.Profile != "unknown" {
			p := s.Profile
			r.VideoCodecProfile = &p
		}
		if s.Width > 0 {
			w := s.Width
			r.Width = &w
		}
		if s.Height > 0 {
			h := s.Height
			r.Height = &h
		}
		break
	}

	// First audio stream.
	for _, s := range raw.Streams {
		if s.CodecType != "audio" {
			continue
		}
		codec := s.CodecName
		r.AudioCodec = &codec
		if s.Channels > 0 {
			ch := s.Channels
			r.AudioChannels = &ch
		}
		break
	}

	return r
}

// --- Helpers ---------------------------------------------------------------

func parsePositiveFloat(s string) *float64 {
	if s == "" || s == "N/A" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return nil
	}
	return &v
}

func parseBitrateKbps(s string) *int {
	if s == "" || s == "N/A" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return nil
	}
	kbps := int(v / 1000)
	if kbps == 0 {
		return nil
	}
	return &kbps
}
