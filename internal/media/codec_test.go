package media

import "testing"

func TestIsEfficientCodec(t *testing.T) {
	for _, c := range []string{"hevc", "H265", "av1", " AV1 ", "vvc"} {
		if !IsEfficientCodec(strptr(c)) {
			t.Errorf("%q should be efficient", c)
		}
	}
	for _, c := range []string{"h264", "vp9", "mpeg4", ""} {
		if IsEfficientCodec(strptr(c)) {
			t.Errorf("%q should not be efficient", c)
		}
	}
	if IsEfficientCodec(nil) {
		t.Error("nil codec should not be efficient")
	}
}

func TestNormalizeTargetCodec(t *testing.T) {
	tests := map[string]TargetCodec{
		"":          TargetHEVC,
		"hevc":      TargetHEVC,
		"H265":      TargetHEVC,
		"libx265":   TargetHEVC,
		"AV1":       TargetAV1,
		"libsvtav1": TargetAV1,
		"vp9":       "vp9",
	}
	for in, want := range tests {
		if got := NormalizeTargetCodec(in); got != want {
			t.Errorf("NormalizeTargetCodec(%q) = %q, want %q", in, got, want)
		}
	}
	if _, ok := EncoderFor(NormalizeTargetCodec("vp9")); ok {
		t.Error("vp9 is not a supported target and must not resolve to an encoder")
	}
}

func TestEncoderVocabulary(t *testing.T) {
	hevc, ok := EncoderFor(TargetHEVC)
	if !ok || hevc.FFmpegEncoder != "libx265" {
		t.Fatalf("hevc encoder = %+v, %v", hevc, ok)
	}
	if !hevc.ValidPreset("medium") || !hevc.ValidPreset(" Slow ") || hevc.ValidPreset("6") {
		t.Error("hevc preset validation")
	}
	if !hevc.ValidCRF(51) || hevc.ValidCRF(52) || hevc.ValidCRF(-1) {
		t.Error("hevc crf range")
	}

	av1, ok := EncoderFor(TargetAV1)
	if !ok || av1.FFmpegEncoder != "libsvtav1" {
		t.Fatalf("av1 encoder = %+v, %v", av1, ok)
	}
	if !av1.ValidPreset("0") || !av1.ValidPreset("13") || av1.ValidPreset("14") || av1.ValidPreset("medium") {
		t.Error("av1 preset validation")
	}
	if !av1.ValidCRF(63) || av1.ValidCRF(64) {
		t.Error("av1 crf range")
	}
	if !av1.ValidPreset(av1.DefaultPreset) || !av1.ValidCRF(av1.DefaultCRF) {
		t.Error("av1 defaults must pass their own validation")
	}
}

func TestEncodersReturnsCopies(t *testing.T) {
	Encoders()[0].Presets[0] = "mutated"
	if enc, _ := EncoderFor(TargetHEVC); enc.Presets[0] == "mutated" {
		t.Fatal("callers must not be able to mutate the registry")
	}
}
