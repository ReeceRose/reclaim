package media

import "testing"

func strptr(s string) *string { return &s }

func TestPredictedSavings_perCodec(t *testing.T) {
	const size = 10_000_000_000 // 10 GB

	tests := []struct {
		name      string
		target    TargetCodec
		codec     *string
		wantRatio float64
	}{
		{"h264", TargetHEVC, strptr("h264"), 0.60},
		{"mpeg2", TargetHEVC, strptr("mpeg2video"), 0.40},
		{"vp9 efficient", TargetHEVC, strptr("vp9"), 0.90},
		{"case insensitive", TargetHEVC, strptr("H264"), 0.60},
		{"unknown codec uses default", TargetHEVC, strptr("weirdcodec"), defaultRatios[TargetHEVC]},
		{"nil codec uses default", TargetHEVC, nil, defaultRatios[TargetHEVC]},
		{"av1 h264", TargetAV1, strptr("h264"), 0.48},
		{"av1 vp9", TargetAV1, strptr("vp9"), 0.72},
		{"av1 unknown codec uses its default", TargetAV1, strptr("weirdcodec"), defaultRatios[TargetAV1]},
		{"unknown target uses the default target", "vp10", strptr("weirdcodec"), defaultRatios[DefaultTargetCodec]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := int64(float64(size) * (1 - tt.wantRatio))
			got := PredictedSavingsBytes(tt.target, tt.codec, false, size)
			if got != want {
				t.Fatalf("savings = %d, want %d", got, want)
			}
		})
	}
}

func TestPredictedSavings_efficientIsZero(t *testing.T) {
	for _, target := range TargetCodecs() {
		for _, codec := range []string{"hevc", "h265", "av1"} {
			if got := PredictedSavingsBytes(target, strptr(codec), true, 5_000_000); got != 0 {
				t.Fatalf("%s → %s savings = %d, want 0", codec, target, got)
			}
			// Even a row whose flag was never set must not promise savings:
			// the ratio itself says there is nothing to gain.
			if got := PredictedSavingsBytes(target, strptr(codec), false, 5_000_000); got != 0 {
				t.Fatalf("%s → %s unflagged savings = %d, want 0", codec, target, got)
			}
		}
	}
}

func TestPredictedSavings_nonPositiveSize(t *testing.T) {
	if got := PredictedSavingsBytes(TargetHEVC, strptr("h264"), false, 0); got != 0 {
		t.Fatalf("zero-size savings = %d, want 0", got)
	}
}

func TestPredictedSavings_rankingOrder(t *testing.T) {
	// Same size: an inefficient codec should rank above an efficient one.
	const size = 1_000_000_000
	for _, target := range TargetCodecs() {
		mpeg2 := PredictedSavingsBytes(target, strptr("mpeg2video"), false, size)
		h264 := PredictedSavingsBytes(target, strptr("h264"), false, size)
		vp9 := PredictedSavingsBytes(target, strptr("vp9"), false, size)

		if !(mpeg2 > h264 && h264 > vp9) {
			t.Fatalf("%s: expected mpeg2(%d) > h264(%d) > vp9(%d)", target, mpeg2, h264, vp9)
		}
	}
}

func TestPredictedSavings_av1ReclaimsMoreThanHEVC(t *testing.T) {
	const size = 1_000_000_000
	for codec := range seedRatios[TargetHEVC] {
		hevc := PredictedSavingsBytes(TargetHEVC, strptr(codec), false, size)
		av1 := PredictedSavingsBytes(TargetAV1, strptr(codec), false, size)
		if av1 <= hevc {
			t.Errorf("%s: AV1 savings %d should exceed HEVC savings %d", codec, av1, hevc)
		}
	}
}

func TestRatioFor_source(t *testing.T) {
	if _, src := RatioFor(TargetHEVC, strptr("h264")); src != RatioSeed {
		t.Fatalf("source = %q, want seed", src)
	}
	if _, src := RatioFor(TargetAV1, nil); src != RatioSeed {
		t.Fatalf("nil source = %q, want seed", src)
	}
}
