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

	// Compatibility-engine fields (internal/compatibility), all from the primary
	// (first) stream of the relevant type — see Streams for the full list.
	PixelFormat     *string // video pix_fmt, e.g. "yuv420p10le"
	VideoBitDepth   *int    // derived from PixelFormat: 8, 10, or 12
	ColorTransfer   *string // e.g. "smpte2084" (HDR10), "arib-std-b67" (HLG)
	ColorPrimaries  *string
	AudioSampleRate *int
	SubtitleCodec   *string // first subtitle stream, e.g. "hdmv_pgs_subtitle"

	// DolbyVisionProfile/Level come from the primary video stream's
	// side_data_list ("DOVI configuration record" entry), when present.
	// Promoted to real columns (unlike the broader side_data_list dump in
	// StreamInfo.Extra) because docs/COMPATIBILITY PLAN.md §6 already
	// anticipates a profile-specific rule (Apple TV 4K: "Dolby Vision
	// Profile 5 only", per Apple's tech specs) — a concrete near-term
	// consumer, not speculative capture.
	DolbyVisionProfile *int
	DolbyVisionLevel   *int

	// Streams holds every stream ffprobe reported, for callers that need more
	// than the primary-stream summary above (e.g. persisting to media_streams
	// for multi-audio-track / multi-subtitle compatibility checks).
	Streams []StreamInfo

	// FormatExtraJSON is a JSON object of rarely-needed format-level ffprobe
	// fields (format_long_name, nb_streams, probe_score, format tags like
	// encoder/creation_time/title) — nil when none were present. Deliberately
	// captured now, even though nothing reads it yet: every probed field that
	// isn't captured today requires a full library re-probe (see
	// docs/COMPATIBILITY PLAN.md §5 "Backfill") to backfill later, since the
	// scanner skips re-probing files whose (size, mtime) haven't changed.
	// Storing the raw values as a JSON catch-all up front means a *future*
	// feature that wants one of these fields can mine already-stored data
	// instead of forcing every user through another 10+ minute full rescan.
	FormatExtraJSON *string
}

// StreamInfo is one entry from ffprobe's "streams" array, trimmed to the
// fields the compatibility engine and media_streams table care about.
type StreamInfo struct {
	Index              int
	CodecType          string // video | audio | subtitle
	CodecName          string
	Profile            string
	Channels           int
	Language           string
	DispositionDefault bool
	// Extra carries rarely-needed raw fields (pix_fmt, level, bit_rate,
	// codec_tag_string, frame rates, aspect ratios, other disposition flags,
	// side_data_list incl. Dolby Vision/HDR10+ metadata, etc.) for storage in
	// media_streams.extra_json without growing this struct — or the
	// media_streams schema — per new field. See FormatExtraJSON's doc comment
	// for why this is captured broadly now rather than only what
	// internal/compatibility currently uses.
	Extra map[string]any
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

// Inspection is the verification-oriented view of a file: stream counts by
// type, canonical dimensions, and duration. Distinct from Result, which maps
// the primary streams into media_files columns.
type Inspection struct {
	DurationSeconds float64
	Width           int
	Height          int
	VideoStreams    int
	AudioStreams    int
	SubtitleStreams int
}

// Inspect runs ffprobe for verification purposes and returns per-type stream
// counts plus the primary video dimensions and duration. A successful return
// (no error, at least one stream) doubles as the playability check.
func Inspect(ctx context.Context, path string) (*Inspection, error) {
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

	var raw probeOutput
	if jerr := json.Unmarshal(out, &raw); jerr != nil {
		return nil, &ProbeError{Path: path, Msg: fmt.Sprintf("parse JSON: %v", jerr)}
	}

	insp := &Inspection{}
	if d := parsePositiveFloat(raw.Format.Duration); d != nil {
		insp.DurationSeconds = *d
	}
	for _, s := range raw.Streams {
		switch s.CodecType {
		case "video":
			insp.VideoStreams++
			if insp.Width == 0 && s.Width > 0 {
				insp.Width = s.Width
			}
			if insp.Height == 0 && s.Height > 0 {
				insp.Height = s.Height
			}
			if insp.DurationSeconds == 0 {
				if d := parsePositiveFloat(s.Duration); d != nil {
					insp.DurationSeconds = *d
				}
			}
		case "audio":
			insp.AudioStreams++
		case "subtitle":
			insp.SubtitleStreams++
		}
	}

	if len(raw.Streams) == 0 {
		return nil, &ProbeError{Path: path, Msg: "no streams found"}
	}
	return insp, nil
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
	FormatName     string            `json:"format_name"`
	FormatLongName string            `json:"format_long_name"`
	Duration       string            `json:"duration"`
	BitRate        string            `json:"bit_rate"`
	NBStreams      int               `json:"nb_streams"`
	ProbeScore     int               `json:"probe_score"`
	Tags           map[string]string `json:"tags"`
}

type probeStream struct {
	Index              int               `json:"index"`
	CodecType          string            `json:"codec_type"`
	CodecName          string            `json:"codec_name"`
	CodecTagString     string            `json:"codec_tag_string"`
	Profile            string            `json:"profile"`
	Width              int               `json:"width"`
	Height             int               `json:"height"`
	Duration           string            `json:"duration"`
	StartTime          string            `json:"start_time"`
	BitRate            string            `json:"bit_rate"`
	BitsPerRawSample   string            `json:"bits_per_raw_sample"`
	Channels           int               `json:"channels"`
	ChannelLayout      string            `json:"channel_layout"`
	SampleRate         string            `json:"sample_rate"`
	SampleFmt          string            `json:"sample_fmt"`
	PixFmt             string            `json:"pix_fmt"`
	ColorSpace         string            `json:"color_space"`
	ColorTransfer      string            `json:"color_transfer"`
	ColorPrimaries     string            `json:"color_primaries"`
	ColorRange         string            `json:"color_range"`
	FieldOrder         string            `json:"field_order"`
	RFrameRate         string            `json:"r_frame_rate"`
	AvgFrameRate       string            `json:"avg_frame_rate"`
	SampleAspectRatio  string            `json:"sample_aspect_ratio"`
	DisplayAspectRatio string            `json:"display_aspect_ratio"`
	Level              int               `json:"level"`
	Disposition        probeDisposition  `json:"disposition"`
	Tags               map[string]string `json:"tags"`
	// SideDataList carries HDR dynamic metadata (Dolby Vision "DOVI
	// configuration record", HDR10+ dynamic metadata, mastering display
	// colour volume, content light level) — field shapes vary by ffmpeg
	// version and side_data_type, so this is decoded generically rather than
	// into a fixed struct per known type.
	SideDataList []map[string]any `json:"side_data_list"`
}

type probeDisposition struct {
	Default         int `json:"default"`
	Dub             int `json:"dub"`
	Original        int `json:"original"`
	Comment         int `json:"comment"`
	Lyrics          int `json:"lyrics"`
	Karaoke         int `json:"karaoke"`
	Forced          int `json:"forced"`
	HearingImpaired int `json:"hearing_impaired"`
	VisualImpaired  int `json:"visual_impaired"`
	CleanEffects    int `json:"clean_effects"`
	AttachedPic     int `json:"attached_pic"`
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
		if s.PixFmt != "" {
			pf := s.PixFmt
			r.PixelFormat = &pf
			if bd := bitDepthFromPixFmt(pf); bd > 0 {
				r.VideoBitDepth = &bd
			}
		}
		if s.ColorTransfer != "" && s.ColorTransfer != "unknown" {
			ct := s.ColorTransfer
			r.ColorTransfer = &ct
		}
		if s.ColorPrimaries != "" && s.ColorPrimaries != "unknown" {
			cp := s.ColorPrimaries
			r.ColorPrimaries = &cp
		}
		r.DolbyVisionProfile, r.DolbyVisionLevel = dolbyVisionFromSideData(s.SideDataList)
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
		if sr := parsePositiveFloat(s.SampleRate); sr != nil {
			hz := int(*sr)
			r.AudioSampleRate = &hz
		}
		break
	}

	// First subtitle stream.
	for _, s := range raw.Streams {
		if s.CodecType != "subtitle" {
			continue
		}
		codec := s.CodecName
		r.SubtitleCodec = &codec
		break
	}

	r.Streams = make([]StreamInfo, 0, len(raw.Streams))
	for _, s := range raw.Streams {
		si := StreamInfo{
			Index:              s.Index,
			CodecType:          s.CodecType,
			CodecName:          s.CodecName,
			Channels:           s.Channels,
			DispositionDefault: s.Disposition.Default != 0,
		}
		if s.Profile != "" && s.Profile != "unknown" {
			si.Profile = s.Profile
		}
		if lang, ok := s.Tags["language"]; ok {
			si.Language = lang
		}
		si.Extra = streamExtra(&s)
		r.Streams = append(r.Streams, si)
	}

	if len(raw.Format.Tags) > 0 || raw.Format.FormatLongName != "" || raw.Format.NBStreams > 0 || raw.Format.ProbeScore > 0 {
		formatExtra := map[string]any{}
		if raw.Format.FormatLongName != "" {
			formatExtra["format_long_name"] = raw.Format.FormatLongName
		}
		if raw.Format.NBStreams > 0 {
			formatExtra["nb_streams"] = raw.Format.NBStreams
		}
		if raw.Format.ProbeScore > 0 {
			formatExtra["probe_score"] = raw.Format.ProbeScore
		}
		for k, v := range raw.Format.Tags {
			formatExtra["tag_"+strings.ToLower(k)] = v
		}
		if b, err := json.Marshal(formatExtra); err == nil {
			s := string(b)
			r.FormatExtraJSON = &s
		}
	}

	return r
}

// streamExtra collects the raw ffprobe fields not already promoted to a
// StreamInfo/media_streams column into a generic bucket — see
// Result.FormatExtraJSON's doc comment for why this is captured broadly.
func streamExtra(s *probeStream) map[string]any {
	extra := map[string]any{}
	str := func(key, val string) {
		if val != "" && val != "unknown" && val != "N/A" {
			extra[key] = val
		}
	}
	str("pix_fmt", s.PixFmt)
	str("color_transfer", s.ColorTransfer)
	str("color_primaries", s.ColorPrimaries)
	str("color_space", s.ColorSpace)
	str("color_range", s.ColorRange)
	str("codec_tag_string", s.CodecTagString)
	str("bits_per_raw_sample", s.BitsPerRawSample)
	str("sample_rate", s.SampleRate)
	str("sample_fmt", s.SampleFmt)
	str("channel_layout", s.ChannelLayout)
	str("field_order", s.FieldOrder)
	str("r_frame_rate", s.RFrameRate)
	str("avg_frame_rate", s.AvgFrameRate)
	str("sample_aspect_ratio", s.SampleAspectRatio)
	str("display_aspect_ratio", s.DisplayAspectRatio)
	str("bit_rate", s.BitRate)
	str("start_time", s.StartTime)
	if s.Level > 0 {
		extra["level"] = strconv.Itoa(s.Level)
	}
	if title, ok := s.Tags["title"]; ok && title != "" {
		extra["title"] = title
	}
	flag := func(key string, v int) {
		if v != 0 {
			extra[key] = true
		}
	}
	flag("forced", s.Disposition.Forced)
	flag("hearing_impaired", s.Disposition.HearingImpaired)
	flag("visual_impaired", s.Disposition.VisualImpaired)
	flag("comment", s.Disposition.Comment)
	flag("lyrics", s.Disposition.Lyrics)
	flag("karaoke", s.Disposition.Karaoke)
	flag("clean_effects", s.Disposition.CleanEffects)
	flag("attached_pic", s.Disposition.AttachedPic)
	flag("dub", s.Disposition.Dub)
	flag("original", s.Disposition.Original)
	if len(s.SideDataList) > 0 {
		extra["side_data_list"] = s.SideDataList
	}
	if len(extra) == 0 {
		return nil
	}
	return extra
}

// dolbyVisionFromSideData scans a video stream's side_data_list for a "DOVI
// configuration record" entry (ffprobe's name for Dolby Vision's RPU
// metadata block) and extracts dv_profile/dv_level. ffprobe emits these as
// JSON numbers, but side_data entries are decoded generically (map[string]any)
// since side_data_type shapes vary, so values arrive as float64 here.
func dolbyVisionFromSideData(sideData []map[string]any) (profile, level *int) {
	for _, sd := range sideData {
		t, _ := sd["side_data_type"].(string)
		if !strings.Contains(strings.ToLower(t), "dovi") {
			continue
		}
		if v, ok := sd["dv_profile"].(float64); ok {
			p := int(v)
			profile = &p
		}
		if v, ok := sd["dv_level"].(float64); ok {
			l := int(v)
			level = &l
		}
		return profile, level
	}
	return nil, nil
}

// bitDepthFromPixFmt derives the sample bit depth from ffmpeg's pixel format
// name. Formats without an explicit "10le"/"12le"/... suffix (e.g. "yuv420p")
// are standard 8-bit. Returns 0 when the format is unrecognized so callers
// can leave VideoBitDepth unset rather than guess.
func bitDepthFromPixFmt(pixFmt string) int {
	switch {
	case strings.Contains(pixFmt, "12le") || strings.Contains(pixFmt, "12be"):
		return 12
	case strings.Contains(pixFmt, "10le") || strings.Contains(pixFmt, "10be"):
		return 10
	case strings.HasPrefix(pixFmt, "yuv") || strings.HasPrefix(pixFmt, "yuvj") || strings.HasPrefix(pixFmt, "rgb") || strings.HasPrefix(pixFmt, "bgr"):
		return 8
	default:
		return 0
	}
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
