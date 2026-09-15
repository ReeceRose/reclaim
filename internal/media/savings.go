package media

import "strings"

// RatioSource labels where an expected output/original ratio came from, so the
// UI can be honest that a savings figure is an estimate rather than a
// measurement.
type RatioSource string

const (
	// RatioSeed is a conservative rule-of-thumb constant shipped with the build.
	RatioSeed RatioSource = "seed"
	// RatioLearned is derived from this instance's own completed jobs. The
	// stats handler surfaces it for codecs with enough learned samples.
	RatioLearned RatioSource = "learned"
)

// seedRatios maps a target codec, then a source video codec, to the estimated
// ratio of output_size / original_size when re-encoding that source to that
// target. Lower means more savings. These are intentionally conservative
// rule-of-thumb seeds: it is better to under-promise savings than to
// over-promise them.
//
// The AV1 column sits roughly a fifth below HEVC's, which is the size SVT-AV1
// typically saves over x265 at matched quality. Efficient sources (HEVC, AV1)
// are absent: they are never candidates, and RatioFor prices them at 1.0.
var seedRatios = map[TargetCodec]map[string]float64{
	TargetHEVC: {
		"mpeg1video": 0.40,
		"mpeg2video": 0.40,
		"msmpeg4v3":  0.50,
		"msmpeg4v2":  0.50,
		"msmpeg4v1":  0.50,
		"wmv1":       0.50,
		"wmv2":       0.50,
		"wmv3":       0.50,
		"vc1":        0.50,
		"mpeg4":      0.55,
		"theora":     0.55,
		"h264":       0.60,
		"avc":        0.60,
		"vp8":        0.65,
		"vp9":        0.90,
	},
	TargetAV1: {
		"mpeg1video": 0.32,
		"mpeg2video": 0.32,
		"msmpeg4v3":  0.40,
		"msmpeg4v2":  0.40,
		"msmpeg4v1":  0.40,
		"wmv1":       0.40,
		"wmv2":       0.40,
		"wmv3":       0.40,
		"vc1":        0.40,
		"mpeg4":      0.44,
		"theora":     0.44,
		"h264":       0.48,
		"avc":        0.48,
		"vp8":        0.52,
		"vp9":        0.72,
	},
}

// defaultRatios is used for source codecs not present in a target's seed
// table. Conservative (modest savings) so an unknown codec never inflates
// predicted recoverable space.
var defaultRatios = map[TargetCodec]float64{
	TargetHEVC: 0.70,
	TargetAV1:  0.56,
}

// RatioFor returns the expected output/original ratio for re-encoding a source
// codec to target, plus the source of that figure. Efficient sources are 1.0
// (nothing to gain); a nil or unknown codec, or an unknown target, falls back
// to the conservative default. The source is always RatioSeed.
func RatioFor(target TargetCodec, videoCodec *string) (ratio float64, source RatioSource) {
	if IsEfficientCodec(videoCodec) {
		return 1.0, RatioSeed
	}
	def, ok := defaultRatios[target]
	if !ok {
		def = defaultRatios[DefaultTargetCodec]
	}
	if videoCodec == nil {
		return def, RatioSeed
	}
	if r, ok := seedRatios[target][strings.ToLower(*videoCodec)]; ok {
		return r, RatioSeed
	}
	return def, RatioSeed
}

// PredictedSavingsBytes estimates how many bytes a re-encode to target would
// reclaim: size_bytes * (1 - ratio[target][codec]). Files already in an
// efficient codec have nothing to gain and return 0. The result is clamped to
// be non-negative. It is an estimate for ranking, never a guarantee.
func PredictedSavingsBytes(target TargetCodec, videoCodec *string, isEfficient bool, sizeBytes int64) int64 {
	ratio, _ := RatioFor(target, videoCodec)
	return SavingsForRatio(ratio, isEfficient, sizeBytes)
}

// SavingsForRatio is PredictedSavingsBytes for a caller that already knows the
// ratio, such as one learned from this instance's completed encodes.
func SavingsForRatio(ratio float64, isEfficient bool, sizeBytes int64) int64 {
	if isEfficient || sizeBytes <= 0 {
		return 0
	}
	saved := int64(float64(sizeBytes) * (1 - ratio))
	if saved < 0 {
		return 0
	}
	return saved
}
