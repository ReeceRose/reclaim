package media

import "testing"

func strptr(s string) *string { return &s }

func TestPredictedSavings_perCodec(t *testing.T) {
	const size = 10_000_000_000 // 10 GB

	tests := []struct {
		name      string
		codec     *string
		hevc      bool
		wantRatio float64
	}{
		{"h264", strptr("h264"), false, 0.60},
		{"mpeg2", strptr("mpeg2video"), false, 0.40},
		{"vp9 efficient", strptr("vp9"), false, 0.90},
		{"case insensitive", strptr("H264"), false, 0.60},
		{"unknown codec uses default", strptr("weirdcodec"), false, defaultHEVCRatio},
		{"nil codec uses default", nil, false, defaultHEVCRatio},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := int64(float64(size) * (1 - tt.wantRatio))
			got := PredictedSavingsBytes(tt.codec, tt.hevc, size)
			if got != want {
				t.Fatalf("savings = %d, want %d", got, want)
			}
		})
	}
}

func TestPredictedSavings_hevcIsZero(t *testing.T) {
	if got := PredictedSavingsBytes(strptr("hevc"), true, 5_000_000); got != 0 {
		t.Fatalf("HEVC savings = %d, want 0", got)
	}
}

func TestPredictedSavings_nonPositiveSize(t *testing.T) {
	if got := PredictedSavingsBytes(strptr("h264"), false, 0); got != 0 {
		t.Fatalf("zero-size savings = %d, want 0", got)
	}
}

func TestPredictedSavings_rankingOrder(t *testing.T) {
	// Same size: an inefficient codec should rank above an efficient one.
	const size = 1_000_000_000
	mpeg2 := PredictedSavingsBytes(strptr("mpeg2video"), false, size)
	h264 := PredictedSavingsBytes(strptr("h264"), false, size)
	vp9 := PredictedSavingsBytes(strptr("vp9"), false, size)

	if !(mpeg2 > h264 && h264 > vp9) {
		t.Fatalf("expected mpeg2(%d) > h264(%d) > vp9(%d)", mpeg2, h264, vp9)
	}
}

func TestRatioFor_source(t *testing.T) {
	if _, src := RatioFor(strptr("h264")); src != RatioSeed {
		t.Fatalf("source = %q, want seed", src)
	}
	if _, src := RatioFor(nil); src != RatioSeed {
		t.Fatalf("nil source = %q, want seed", src)
	}
}
