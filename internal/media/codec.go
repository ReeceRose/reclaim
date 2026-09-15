package media

import (
	"slices"
	"strconv"
	"strings"
)

// efficientCodecs are the source codecs Reclaim never re-encodes, whatever the
// target. Moving between them trades a generation of quality loss for a small
// or negative size change: an AV1 file sent through x265 grows, and an HEVC
// file sent through SVT-AV1 gains perhaps a fifth at a visible cost.
var efficientCodecs = map[string]struct{}{
	"hevc": {},
	"h265": {},
	"av1":  {},
	"vvc":  {},
	"h266": {},
}

// IsEfficientCodec reports whether a probed video codec is already a modern,
// efficient codec that is not worth re-encoding.
func IsEfficientCodec(codec *string) bool {
	if codec == nil {
		return false
	}
	_, ok := efficientCodecs[strings.ToLower(strings.TrimSpace(*codec))]
	return ok
}

// TargetCodec is an output codec a transcode profile encodes to. Its value is
// the ffprobe codec_name the encoded stream reports, which is what
// verification compares against.
type TargetCodec string

const (
	TargetHEVC TargetCodec = "hevc"
	TargetAV1  TargetCodec = "av1"
)

// DefaultTargetCodec is what a profile with no recorded codec encodes to —
// every profile created before AV1 support existed.
const DefaultTargetCodec = TargetHEVC

// Encoder describes how Reclaim drives one target codec through ffmpeg: which
// encoder to invoke and the CRF and preset vocabulary that encoder accepts.
type Encoder struct {
	Codec         TargetCodec
	Label         string
	FFmpegEncoder string
	CRFMin        int
	CRFMax        int
	DefaultCRF    int
	Presets       []string
	DefaultPreset string
}

var encoders = []Encoder{
	{
		Codec:         TargetHEVC,
		Label:         "HEVC",
		FFmpegEncoder: "libx265",
		CRFMin:        0,
		CRFMax:        51,
		DefaultCRF:    26,
		Presets: []string{
			"ultrafast", "superfast", "veryfast", "faster", "fast",
			"medium", "slow", "slower", "veryslow", "placebo",
		},
		DefaultPreset: "medium",
	},
	{
		Codec:         TargetAV1,
		Label:         "AV1",
		FFmpegEncoder: "libsvtav1",
		CRFMin:        0,
		CRFMax:        63,
		DefaultCRF:    30,
		Presets:       svtAV1Presets(),
		DefaultPreset: "6",
	},
}

// svtAV1Presets lists SVT-AV1's numeric presets, slowest first. Presets below
// zero exist but are research-grade and far too slow for a library.
func svtAV1Presets() []string {
	out := make([]string, 0, 14)
	for i := 0; i <= 13; i++ {
		out = append(out, strconv.Itoa(i))
	}
	return out
}

// Encoders returns every supported target encoder in display order.
func Encoders() []Encoder {
	out := make([]Encoder, len(encoders))
	for i, e := range encoders {
		e.Presets = slices.Clone(e.Presets)
		out[i] = e
	}
	return out
}

// TargetCodecs returns every supported target codec in display order.
func TargetCodecs() []TargetCodec {
	out := make([]TargetCodec, len(encoders))
	for i, e := range encoders {
		out[i] = e.Codec
	}
	return out
}

// NormalizeTargetCodec maps a stored or submitted codec to its canonical form.
// An empty value is the default; an unrecognised one is returned lowercased so
// EncoderFor can reject it rather than silently encoding to something else.
func NormalizeTargetCodec(codec string) TargetCodec {
	c := strings.ToLower(strings.TrimSpace(codec))
	switch c {
	case "":
		return DefaultTargetCodec
	case "h265", "x265", "libx265":
		return TargetHEVC
	case "svtav1", "libsvtav1":
		return TargetAV1
	}
	return TargetCodec(c)
}

// EncoderFor returns the encoder for a target codec.
func EncoderFor(codec TargetCodec) (Encoder, bool) {
	for _, e := range encoders {
		if e.Codec == codec {
			e.Presets = slices.Clone(e.Presets)
			return e, true
		}
	}
	return Encoder{}, false
}

// ValidPreset reports whether preset is one this encoder accepts.
func (e Encoder) ValidPreset(preset string) bool {
	return slices.Contains(e.Presets, strings.ToLower(strings.TrimSpace(preset)))
}

// ValidCRF reports whether crf is inside this encoder's quality range.
func (e Encoder) ValidCRF(crf int) bool {
	return crf >= e.CRFMin && crf <= e.CRFMax
}
