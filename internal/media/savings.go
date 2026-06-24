package media

import "strings"

// RatioSource labels where an expected_hevc_ratio came from, so the UI can be
// honest that a savings figure is an estimate rather than a measurement.
type RatioSource string

const (
	// RatioSeed is a conservative rule-of-thumb constant shipped with the build.
	RatioSeed RatioSource = "seed"
	// RatioLearned is derived from this instance's own completed jobs (P9, §10.2).
	// Not produced in v1 — reserved so the API contract is stable.
	RatioLearned RatioSource = "learned"
)

// seedHEVCRatios maps a source video codec to the estimated ratio of
// output_size / original_size when re-encoding it to HEVC. Lower means more
// savings. These are intentionally conservative rule-of-thumb seeds (§10.2):
// it is better to under-promise savings than to over-promise them.
//
// This table is the single place the seeds live. P9 (§10.2) will start
// overriding individual entries with the observed mean output/original ratio
// per source codec once there is enough completed-job history to beat a seed;
// at that point RatioFor should consult learned values first and fall back here.
var seedHEVCRatios = map[string]float64{
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
	"av1":        1.00,
	// HEVC sources are excluded from candidates entirely (§10.1); a 1.0 ratio
	// here means "no savings" for the rare path that still asks.
	"hevc": 1.00,
	"h265": 1.00,
}

// defaultHEVCRatio is used for codecs not present in the seed table. Conservative
// (modest savings) so an unknown codec never inflates predicted recoverable space.
const defaultHEVCRatio = 0.70

// RatioFor returns the expected output/original ratio for a source codec plus
// the source of that figure. A nil or unknown codec falls back to the default.
// In v1 the source is always RatioSeed.
func RatioFor(videoCodec *string) (ratio float64, source RatioSource) {
	if videoCodec == nil {
		return defaultHEVCRatio, RatioSeed
	}
	if r, ok := seedHEVCRatios[strings.ToLower(*videoCodec)]; ok {
		return r, RatioSeed
	}
	return defaultHEVCRatio, RatioSeed
}

// PredictedSavingsBytes estimates how many bytes a re-encode to HEVC would
// reclaim: size_bytes * (1 - expected_hevc_ratio[codec]) (§10.2). Files already
// in HEVC have nothing to gain and return 0. The result is clamped to be
// non-negative. It is an estimate for ranking, never a guarantee.
func PredictedSavingsBytes(videoCodec *string, isAlreadyHEVC bool, sizeBytes int64) int64 {
	if isAlreadyHEVC || sizeBytes <= 0 {
		return 0
	}
	ratio, _ := RatioFor(videoCodec)
	saved := int64(float64(sizeBytes) * (1 - ratio))
	if saved < 0 {
		return 0
	}
	return saved
}
