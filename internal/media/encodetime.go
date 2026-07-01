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
	EncodeRateLearnedGlobal    EncodeRateSource = "learned_global"
	EncodeRateSeed             EncodeRateSource = "seed"
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

// seedEncodeRates maps x265 preset to wall seconds per 1080p-equivalent source
// second. Values are conservative (README upper bounds) so estimates err long.
var seedEncodeRates = map[string]float64{
	"ultrafast": 0.13,
	"superfast": 0.15,
	"veryfast":  0.20,
	"faster":    0.25,
	"fast":      0.50,
	"medium":    2.0,
	"slow":      4.0,
	"slower":    6.0,
	"veryslow":  8.0,
}

const defaultSeedEncodeRate = 2.0

// LearnedEncodeRate is a normalized encode speed derived from completed jobs.
type LearnedEncodeRate struct {
	Rate        float64 // wall-sec / 1080p-equiv source-sec
	SampleCount int
}

// EncodeRateLookup holds learned rates at each fallback tier.
type EncodeRateLookup struct {
	ByProfileID map[int64]LearnedEncodeRate
	ByPresetCRF map[string]LearnedEncodeRate // key: "medium:26"
	ByPreset    map[string]LearnedEncodeRate
	Global      *LearnedEncodeRate
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

// PresetCRFKey builds the preset+crf bucket key.
func PresetCRFKey(preset string, crf int) string {
	return fmt.Sprintf("%s:%d", strings.ToLower(preset), crf)
}

// SeedEncodeRate returns the shipped conservative rate for a preset.
func SeedEncodeRate(preset string) float64 {
	if r, ok := seedEncodeRates[strings.ToLower(preset)]; ok {
		return r
	}
	return defaultSeedEncodeRate
}

// ResolveEncodeRate picks the best available rate using the profile-first cascade.
func ResolveEncodeRate(profileID int64, preset string, crf int, lookup *EncodeRateLookup) (rate float64, source EncodeRateSource, sampleCount int) {
	if lookup != nil {
		if lr, ok := lookup.ByProfileID[profileID]; ok {
			return lr.Rate, EncodeRateLearnedProfile, lr.SampleCount
		}
		if lr, ok := lookup.ByPresetCRF[PresetCRFKey(preset, crf)]; ok {
			return lr.Rate, EncodeRateLearnedPresetCRF, lr.SampleCount
		}
		if lr, ok := lookup.ByPreset[strings.ToLower(preset)]; ok {
			return lr.Rate, EncodeRateLearnedPreset, lr.SampleCount
		}
		if lookup.Global != nil {
			return lookup.Global.Rate, EncodeRateLearnedGlobal, lookup.Global.SampleCount
		}
	}
	return SeedEncodeRate(preset), EncodeRateSeed, 0
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
