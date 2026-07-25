package media

import (
	"math"
	"testing"
)

func ptrS(s string) *string    { return &s }
func ptrI(i int) *int          { return &i }
func ptrF(f float64) *float64  { return &f }
func approx(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestActualBitrateKbps(t *testing.T) {
	// 1 GiB over 1000s = 1073741824*8/1000/1000 ≈ 8589.9 kbps.
	kbps, ok := ActualBitrateKbps(1<<30, ptrF(1000))
	if !ok || !approx(kbps, 8589.93) {
		t.Fatalf("got %.2f ok=%v, want ≈8589.93", kbps, ok)
	}

	for _, tc := range []struct {
		size int64
		dur  *float64
	}{
		{0, ptrF(100)},
		{100, nil},
		{100, ptrF(0)},
		{100, ptrF(-5)},
	} {
		if _, ok := ActualBitrateKbps(tc.size, tc.dur); ok {
			t.Fatalf("expected ok=false for size=%d dur=%v", tc.size, tc.dur)
		}
	}
}

func TestExpectedBitrateKbps_codecAware(t *testing.T) {
	// Same 1080p resolution: HEVC held to a tighter ceiling than H.264.
	hevc, ok := ExpectedBitrateKbps(ptrS("hevc"), ptrI(1920), ptrI(1080))
	if !ok {
		t.Fatal("hevc: ok=false")
	}
	h264, ok := ExpectedBitrateKbps(ptrS("h264"), ptrI(1920), ptrI(1080))
	if !ok {
		t.Fatal("h264: ok=false")
	}
	if !(h264 > hevc) {
		t.Fatalf("expected h264 ceiling (%.0f) > hevc ceiling (%.0f)", h264, hevc)
	}

	// Unknown resolution has no baseline.
	if _, ok := ExpectedBitrateKbps(ptrS("h264"), nil, nil); ok {
		t.Fatal("unknown resolution: want ok=false")
	}
}

func TestOversizeRatio(t *testing.T) {
	// A 1080p HEVC file at ~10 Mbps against a 5000 kbps HEVC ceiling ≈ 2x.
	// size chosen so actual ≈ 10000 kbps over 1000s: 10000*1000*1000/8 bytes.
	size := int64(10000 * 1000 * 1000 / 8)
	got := OversizeRatio(ptrS("hevc"), ptrI(1920), ptrI(1080), size, ptrF(1000))
	if !approx(got, 2.0) {
		t.Fatalf("hevc oversize ratio = %.3f, want ≈2.0", got)
	}

	// The same absolute bitrate as H.264 trips a lower ratio (looser ceiling).
	h264 := OversizeRatio(ptrS("h264"), ptrI(1920), ptrI(1080), size, ptrF(1000))
	if !(h264 < got) {
		t.Fatalf("h264 ratio (%.3f) should be < hevc ratio (%.3f) at equal bitrate", h264, got)
	}

	// Not computable → 0.
	if r := OversizeRatio(ptrS("hevc"), nil, nil, size, ptrF(1000)); r != 0 {
		t.Fatalf("unknown resolution ratio = %.3f, want 0", r)
	}
	if r := OversizeRatio(ptrS("hevc"), ptrI(1920), ptrI(1080), size, nil); r != 0 {
		t.Fatalf("no-duration ratio = %.3f, want 0", r)
	}
}
