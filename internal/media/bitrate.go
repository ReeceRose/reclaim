package media

import "strings"

// resBand is a coarse resolution class used to pick an expected bitrate. It
// mirrors the resolution buckets the stats and candidate queries use, so a
// file's band here lines up with how it is labelled elsewhere.
type resBand int

const (
	bandUnknown resBand = iota
	bandSD
	bandHD
	bandFHD
	bandQHD
	bandUHD
	band8K
)

// baseExpectedKbps is the whole-file bitrate we'd expect from a well-encoded,
// modern-codec (HEVC/AV1-class) file at each resolution. It is the reference
// baseline; less efficient source codecs are scaled up from here via
// codecBitrateFactor. These are deliberately generous "good quality" targets,
// not floors — the point is to flag files well above them, not merely above.
var baseExpectedKbps = map[resBand]float64{
	bandSD:  1500,
	bandHD:  2500,
	bandFHD: 5000,
	bandQHD: 9000,
	bandUHD: 16000,
	band8K:  50000,
}

// codecBitrateFactor scales the expected bitrate by how much a given source
// codec typically needs to reach the same quality as the HEVC-class reference.
// A file is measured against the ceiling appropriate for its own codec, so a
// normal H.264 file is not flagged for being larger than HEVC would be — only
// files that are bloated relative to what their own codec should need. Because
// efficient codecs get the tightest ceiling, an oversized HEVC file trips the
// flag sooner than an H.264 file at the same absolute bitrate.
var codecBitrateFactor = map[string]float64{
	"hevc":       1.0,
	"h265":       1.0,
	"av1":        1.0,
	"vp9":        1.05,
	"h264":       1.6,
	"avc":        1.6,
	"vp8":        1.7,
	"mpeg4":      2.2,
	"theora":     2.2,
	"vc1":        2.2,
	"wmv3":       2.2,
	"mpeg2video": 2.6,
	"mpeg1video": 2.6,
	"msmpeg4v3":  2.6,
	"msmpeg4v2":  2.6,
	"msmpeg4v1":  2.6,
	"wmv1":       2.6,
	"wmv2":       2.6,
}

// defaultCodecBitrateFactor is used for codecs not in the table. Middling, so an
// unknown codec is neither flagged too eagerly nor let off entirely.
const defaultCodecBitrateFactor = 1.8

// resolutionBand classifies a file by its larger dimension into a band. The
// bounds match resolutionHeightClause in the store so a file's band is
// consistent across the app.
func resolutionBand(width, height *int) resBand {
	w, h := 0, 0
	if width != nil {
		w = *width
	}
	if height != nil {
		h = *height
	}
	switch {
	case w >= 7680 || h >= 4320:
		return band8K
	case w >= 3840 || h >= 2160:
		return bandUHD
	case w >= 2560 || h >= 1440:
		return bandQHD
	case w >= 1920 || h >= 1080:
		return bandFHD
	case w >= 1280 || h >= 720:
		return bandHD
	case w > 0 || h > 0:
		return bandSD
	default:
		return bandUnknown
	}
}

func codecFactor(videoCodec *string) float64 {
	if videoCodec == nil {
		return defaultCodecBitrateFactor
	}
	if f, ok := codecBitrateFactor[strings.ToLower(*videoCodec)]; ok {
		return f
	}
	return defaultCodecBitrateFactor
}

// ExpectedBitrateKbps returns the codec-aware expected whole-file bitrate for a
// file of the given resolution, and ok=false when there is no honest baseline
// (unknown resolution). It is a reference ceiling for a well-encoded file, not a
// prediction of this file's actual bitrate.
func ExpectedBitrateKbps(videoCodec *string, width, height *int) (kbps float64, ok bool) {
	base, ok := baseExpectedKbps[resolutionBand(width, height)]
	if !ok {
		return 0, false
	}
	return base * codecFactor(videoCodec), true
}

// ActualBitrateKbps computes the whole-file (container) bitrate from size and
// duration: size_bytes * 8 / duration / 1000. This is codec-independent and
// always available, unlike the probed stream bitrate, and is exactly the "how
// large is this file for its runtime" signal. ok=false for invalid inputs.
func ActualBitrateKbps(sizeBytes int64, durationSeconds *float64) (kbps float64, ok bool) {
	if sizeBytes <= 0 || durationSeconds == nil || *durationSeconds <= 0 {
		return 0, false
	}
	return float64(sizeBytes) * 8 / *durationSeconds / 1000, true
}

// OversizeRatio returns actual_bitrate / expected_bitrate for a file: how many
// times larger the file is than a well-encoded file of the same codec and
// resolution would be. A ratio at or below 1 is normal; higher means bloated,
// regardless of codec. It returns 0 when the ratio cannot be computed (missing
// duration/size or unknown resolution), which callers treat as "not oversized".
func OversizeRatio(videoCodec *string, width, height *int, sizeBytes int64, durationSeconds *float64) float64 {
	actual, ok := ActualBitrateKbps(sizeBytes, durationSeconds)
	if !ok {
		return 0
	}
	expected, ok := ExpectedBitrateKbps(videoCodec, width, height)
	if !ok || expected <= 0 {
		return 0
	}
	return actual / expected
}
