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

func TestResolveEncodeRate_cascade(t *testing.T) {
	lookup := &EncodeRateLookup{
		ByProfileID: map[int64]LearnedEncodeRate{1: {Rate: 1.1, SampleCount: 3}},
		ByPresetCRF: map[string]LearnedEncodeRate{"medium:26": {Rate: 1.5, SampleCount: 5}},
		ByPreset:    map[string]LearnedEncodeRate{"medium": {Rate: 1.8, SampleCount: 5}},
		Global:      &LearnedEncodeRate{Rate: 2.0, SampleCount: 10},
	}

	t.Run("profile hit", func(t *testing.T) {
		rate, src, n := ResolveEncodeRate(1, "medium", 26, lookup)
		if rate != 1.1 || src != EncodeRateLearnedProfile || n != 3 {
			t.Fatalf("got rate=%v src=%q n=%d", rate, src, n)
		}
	})
	t.Run("preset crf hit", func(t *testing.T) {
		rate, src, n := ResolveEncodeRate(99, "medium", 26, lookup)
		if rate != 1.5 || src != EncodeRateLearnedPresetCRF || n != 5 {
			t.Fatalf("got rate=%v src=%q n=%d", rate, src, n)
		}
	})
	t.Run("preset hit", func(t *testing.T) {
		rate, src, n := ResolveEncodeRate(99, "medium", 22, lookup)
		if rate != 1.8 || src != EncodeRateLearnedPreset || n != 5 {
			t.Fatalf("got rate=%v src=%q n=%d", rate, src, n)
		}
	})
	t.Run("global hit", func(t *testing.T) {
		rate, src, n := ResolveEncodeRate(99, "slow", 22, lookup)
		if rate != 2.0 || src != EncodeRateLearnedGlobal || n != 10 {
			t.Fatalf("got rate=%v src=%q n=%d", rate, src, n)
		}
	})
	t.Run("seed fallback", func(t *testing.T) {
		rate, src, n := ResolveEncodeRate(99, "slow", 22, nil)
		if rate != SeedEncodeRate("slow") || src != EncodeRateSeed || n != 0 {
			t.Fatalf("got rate=%v src=%q n=%d", rate, src, n)
		}
	})
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
