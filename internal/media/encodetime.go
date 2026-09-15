package media

import (
	"fmt"
	"math"
	"strings"
)

// EncodeRateSource labels where an encode-duration estimate came from.
type EncodeRateSource string

const (
	EncodeRateLearnedProfile   EncodeRateSource = "learned_profile"
	EncodeRateLearnedPresetCRF EncodeRateSource = "learned_preset_crf"
	EncodeRateLearnedPreset    EncodeRateSource = "learned_preset"
	// EncodeRateLearnedGlobal is every completed encode to the same target
	// codec. Rates are never pooled across codecs: SVT-AV1 and x265 run at
	// unrelated speeds, so a global figure would describe neither.
	EncodeRateLearnedGlobal EncodeRateSource = "learned_global"
	EncodeRateSeed          EncodeRateSource = "seed"
)

const (
	refWidth  = 1920
	refHeight = 1080

	pixelFactorMin = 0.25
	pixelFactorMax = 16.0

	encodeRateOutlierMin = 0.02
	encodeRateOutlierMax = 25.0

	encodeRateClampMin = 0.05
	encodeRateClampMax = 20.0
)

// seedEncodeRates maps a target codec, then its encoder preset, to wall seconds
// per 1080p-equivalent source second. Values are conservative upper bounds for
// a modest multi-core CPU so estimates err long. SVT-AV1's presets are numeric,
// 0 slowest to 13 fastest.
var seedEncodeRates = map[TargetCodec]map[string]float64{
	TargetHEVC: {
		"ultrafast": 0.13,
		"superfast": 0.15,
		"veryfast":  0.20,
		"faster":    0.25,
		"fast":      0.50,
		"medium":    2.0,
		"slow":      4.0,
		"slower":    6.0,
		"veryslow":  8.0,
		"placebo":   16.0,
	},
	TargetAV1: {
		"0":  20.0,
		"1":  14.0,
		"2":  10.0,
		"3":  6.0,
		"4":  4.0,
		"5":  2.5,
		"6":  1.6,
		"7":  1.1,
		"8":  0.70,
		"9":  0.45,
		"10": 0.30,
		"11": 0.22,
		"12": 0.16,
		"13": 0.13,
	},
}

var defaultSeedEncodeRates = map[TargetCodec]float64{
	TargetHEVC: 2.0,
	TargetAV1:  1.6,
}

// LearnedEncodeRate is a normalized encode speed derived from completed jobs.
type LearnedEncodeRate struct {
	Rate        float64 // wall-sec / 1080p-equiv source-sec
	SampleCount int
}

// EncodeRateLookup holds learned rates at each fallback tier. Every key carries
// the target codec, so a profile switched from HEVC to AV1 starts learning
// afresh instead of inheriting x265's timings.
type EncodeRateLookup struct {
	ByProfile   map[string]LearnedEncodeRate // key: ProfileRateKey → "12:av1"
	ByPresetCRF map[string]LearnedEncodeRate // key: PresetCRFKey → "hevc:medium:26"
	ByPreset    map[string]LearnedEncodeRate // key: PresetKey → "hevc:medium"
	ByCodec     map[string]LearnedEncodeRate // key: target codec → "av1"
}

// PixelFactor scales source duration by resolution relative to 1080p.
func PixelFactor(width, height *int) float64 {
	w, h := refWidth, refHeight
	if width != nil && *width > 0 {
		w = *width
	}
	if height != nil && *height > 0 {
		h = *height
	}
	pf := float64(w*h) / float64(refWidth*refHeight)
	if pf < pixelFactorMin {
		return pixelFactorMin
	}
	if pf > pixelFactorMax {
		return pixelFactorMax
	}
	return pf
}

// NormalizedEncodeRate converts wall-clock encode time into a resolution-
// normalized rate. Returns ok=false for invalid inputs or outlier rates.
func NormalizedEncodeRate(elapsedSeconds int64, durationSeconds float64, width, height *int) (rate float64, ok bool) {
	if elapsedSeconds <= 0 || durationSeconds <= 0 {
		return 0, false
	}
	pf := PixelFactor(width, height)
	effective := durationSeconds * pf
	if effective <= 0 {
		return 0, false
	}
	rate = float64(elapsedSeconds) / effective
	if rate < encodeRateOutlierMin || rate > encodeRateOutlierMax {
		return 0, false
	}
	return rate, true
}

// ClampEncodeRate bounds an aggregate rate to a sane range.
func ClampEncodeRate(rate float64) float64 {
	if rate < encodeRateClampMin {
		return encodeRateClampMin
	}
	if rate > encodeRateClampMax {
		return encodeRateClampMax
	}
	return rate
}

// ProfileRateKey builds the profile bucket key.
func ProfileRateKey(profileID int64, codec TargetCodec) string {
	return fmt.Sprintf("%d:%s", profileID, codec)
}

// PresetCRFKey builds the codec+preset+crf bucket key.
func PresetCRFKey(codec TargetCodec, preset string, crf int) string {
	return fmt.Sprintf("%s:%s:%d", codec, strings.ToLower(preset), crf)
}

// PresetKey builds the codec+preset bucket key.
func PresetKey(codec TargetCodec, preset string) string {
	return fmt.Sprintf("%s:%s", codec, strings.ToLower(preset))
}

// SeedEncodeRate returns the shipped conservative rate for a codec's preset.
func SeedEncodeRate(codec TargetCodec, preset string) float64 {
	if r, ok := seedEncodeRates[codec][strings.ToLower(preset)]; ok {
		return r
	}
	if r, ok := defaultSeedEncodeRates[codec]; ok {
		return r
	}
	return defaultSeedEncodeRates[DefaultTargetCodec]
}

// ResolveEncodeRate picks the best available rate using the profile-first cascade.
func ResolveEncodeRate(profileID int64, codec TargetCodec, preset string, crf int, lookup *EncodeRateLookup) (rate float64, source EncodeRateSource, sampleCount int) {
	if lookup != nil {
		if lr, ok := lookup.ByProfile[ProfileRateKey(profileID, codec)]; ok {
			return lr.Rate, EncodeRateLearnedProfile, lr.SampleCount
		}
		if lr, ok := lookup.ByPresetCRF[PresetCRFKey(codec, preset, crf)]; ok {
			return lr.Rate, EncodeRateLearnedPresetCRF, lr.SampleCount
		}
		if lr, ok := lookup.ByPreset[PresetKey(codec, preset)]; ok {
			return lr.Rate, EncodeRateLearnedPreset, lr.SampleCount
		}
		if lr, ok := lookup.ByCodec[string(codec)]; ok {
			return lr.Rate, EncodeRateLearnedGlobal, lr.SampleCount
		}
	}
	return SeedEncodeRate(codec, preset), EncodeRateSeed, 0
}

// PredictedEncodeSeconds estimates wall-clock encode time from a normalized rate.
func PredictedEncodeSeconds(rate float64, durationSeconds *float64, width, height *int) int64 {
	if durationSeconds == nil || *durationSeconds <= 0 || rate <= 0 {
		return 0
	}
	sec := rate * *durationSeconds * PixelFactor(width, height)
	if sec <= 0 {
		return 0
	}
	return int64(math.Round(sec))
}
