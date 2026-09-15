package media

import "testing"

func TestPredictedEncodeSeconds_1080p(t *testing.T) {
	dur := 3600.0
	w, h := 1920, 1080
	got := PredictedEncodeSeconds(2.0, &dur, &w, &h)
	if got != 7200 {
		t.Fatalf("1080p 1hr @ rate 2.0 = %d, want 7200", got)
	}
}

func TestPredictedEncodeSeconds_4k(t *testing.T) {
	dur := 3600.0
	w, h := 3840, 2160
	got := PredictedEncodeSeconds(2.0, &dur, &w, &h)
	// 4K pixel factor = 4× 1080p → ~4× longer encode
	if got < 28000 || got > 29000 {
		t.Fatalf("4K 1hr @ rate 2.0 = %d, want ~28800", got)
	}
}

func TestPredictedEncodeSeconds_unknownDuration(t *testing.T) {
	if got := PredictedEncodeSeconds(2.0, nil, nil, nil); got != 0 {
		t.Fatalf("nil duration = %d, want 0", got)
	}
}

func hevcLookup() *EncodeRateLookup {
	return &EncodeRateLookup{
		ByProfile:   map[string]LearnedEncodeRate{ProfileRateKey(1, TargetHEVC): {Rate: 1.1, SampleCount: 3}},
		ByPresetCRF: map[string]LearnedEncodeRate{PresetCRFKey(TargetHEVC, "medium", 26): {Rate: 1.5, SampleCount: 5}},
		ByPreset:    map[string]LearnedEncodeRate{PresetKey(TargetHEVC, "medium"): {Rate: 1.8, SampleCount: 5}},
		ByCodec:     map[string]LearnedEncodeRate{string(TargetHEVC): {Rate: 2.0, SampleCount: 10}},
	}
}

func TestResolveEncodeRate_cascade(t *testing.T) {
	lookup := hevcLookup()

	t.Run("profile hit", func(t *testing.T) {
		rate, src, n := ResolveEncodeRate(1, TargetHEVC, "medium", 26, lookup)
		if rate != 1.1 || src != EncodeRateLearnedProfile || n != 3 {
			t.Fatalf("got rate=%v src=%q n=%d", rate, src, n)
		}
	})
	t.Run("preset crf hit", func(t *testing.T) {
		rate, src, n := ResolveEncodeRate(99, TargetHEVC, "medium", 26, lookup)
		if rate != 1.5 || src != EncodeRateLearnedPresetCRF || n != 5 {
			t.Fatalf("got rate=%v src=%q n=%d", rate, src, n)
		}
	})
	t.Run("preset hit", func(t *testing.T) {
		rate, src, n := ResolveEncodeRate(99, TargetHEVC, "medium", 22, lookup)
		if rate != 1.8 || src != EncodeRateLearnedPreset || n != 5 {
			t.Fatalf("got rate=%v src=%q n=%d", rate, src, n)
		}
	})
	t.Run("codec hit", func(t *testing.T) {
		rate, src, n := ResolveEncodeRate(99, TargetHEVC, "slow", 22, lookup)
		if rate != 2.0 || src != EncodeRateLearnedGlobal || n != 10 {
			t.Fatalf("got rate=%v src=%q n=%d", rate, src, n)
		}
	})
	t.Run("seed fallback", func(t *testing.T) {
		rate, src, n := ResolveEncodeRate(99, TargetHEVC, "slow", 22, nil)
		if rate != SeedEncodeRate(TargetHEVC, "slow") || src != EncodeRateSeed || n != 0 {
			t.Fatalf("got rate=%v src=%q n=%d", rate, src, n)
		}
	})
}

// A profile switched from HEVC to AV1 keeps its id, but x265's timings say
// nothing about SVT-AV1's, so no learned tier may answer for the other codec.
func TestResolveEncodeRate_neverCrossesCodecs(t *testing.T) {
	rate, src, n := ResolveEncodeRate(1, TargetAV1, "medium", 26, hevcLookup())
	if src != EncodeRateSeed || n != 0 || rate != SeedEncodeRate(TargetAV1, "medium") {
		t.Fatalf("got rate=%v src=%q n=%d, want the AV1 seed", rate, src, n)
	}
}

func TestSeedEncodeRate_av1Presets(t *testing.T) {
	if slow, fast := SeedEncodeRate(TargetAV1, "4"), SeedEncodeRate(TargetAV1, "8"); slow <= fast {
		t.Fatalf("preset 4 (%v) should be slower than preset 8 (%v)", slow, fast)
	}
	if got := SeedEncodeRate(TargetAV1, "bogus"); got != defaultSeedEncodeRates[TargetAV1] {
		t.Fatalf("unknown AV1 preset = %v, want AV1 default %v", got, defaultSeedEncodeRates[TargetAV1])
	}
	if got := SeedEncodeRate("vp10", "medium"); got != defaultSeedEncodeRates[DefaultTargetCodec] {
		t.Fatalf("unknown codec = %v, want default %v", got, defaultSeedEncodeRates[DefaultTargetCodec])
	}
}

func TestNormalizedEncodeRate_outlier(t *testing.T) {
	dur := 7200.0 // 2 hours
	w, h := 1920, 1080
	// 10s encode of a 2hr file is ~720× faster than realtime → outlier
	if _, ok := NormalizedEncodeRate(10, dur, &w, &h); ok {
		t.Fatal("expected 10s/2hr encode to be excluded as outlier")
	}
}

func TestPixelFactor_clamps(t *testing.T) {
	tinyW, tinyH := 320, 240
	if pf := PixelFactor(&tinyW, &tinyH); pf != pixelFactorMin {
		t.Fatalf("tiny pf = %v, want min %v", pf, pixelFactorMin)
	}
	hugeW, hugeH := 7680, 4320
	if pf := PixelFactor(&hugeW, &hugeH); pf != pixelFactorMax {
		t.Fatalf("8K pf = %v, want max %v", pf, pixelFactorMax)
	}
}
