package ffmpeg

import (
	"math"
	"strings"
	"testing"
)

func TestEncodeArgsMapsAllInputStreams(t *testing.T) {
	args := encodeArgs(Options{
		InputPath:  "in.mkv",
		OutputPath: "out.mkv",
		CRF:        22,
		Preset:     "medium",
	})

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-map 0") {
		t.Fatalf("encode args missing -map 0: %q", joined)
	}
	// -map must come immediately after the input so every stream is selected.
	if i := indexOf(args, "-i"); i < 0 {
		t.Fatal("missing -i")
	} else if args[i+2] != "-map" || args[i+3] != "0" {
		t.Fatalf("-map 0 not placed after input: %v", args)
	}
}

func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

func TestParseFFTime(t *testing.T) {
	cases := map[string]float64{
		"00:00:00.000000": 0,
		"00:00:05.500000": 5.5,
		"00:01:30.000000": 90,
		"01:00:00.000000": 3600,
	}
	for in, want := range cases {
		got, err := parseFFTime(in)
		if err != nil {
			t.Fatalf("parseFFTime(%q): %v", in, err)
		}
		if math.Abs(got-want) > 1e-6 {
			t.Errorf("parseFFTime(%q) = %v, want %v", in, got, want)
		}
	}

	if _, err := parseFFTime("bogus"); err == nil {
		t.Error("expected error for malformed time")
	}
}

func TestParseProgress(t *testing.T) {
	// duration = 10s. out_time at 5s → 50%, then end at 10s → 100%.
	stream := strings.Join([]string{
		"frame=10",
		"out_time=00:00:05.000000",
		"progress=continue",
		"out_time=00:00:10.000000",
		"progress=end",
		"",
	}, "\n")

	var pcts []float64
	parseProgress(strings.NewReader(stream), 10, func(p float64) {
		pcts = append(pcts, p)
	})

	if len(pcts) != 2 {
		t.Fatalf("got %d progress ticks, want 2 (%v)", len(pcts), pcts)
	}
	if math.Abs(pcts[0]-50) > 0.01 {
		t.Errorf("first tick = %v, want 50", pcts[0])
	}
	if math.Abs(pcts[1]-100) > 0.01 {
		t.Errorf("second tick = %v, want 100", pcts[1])
	}
}

func TestParseProgressClampsAndIgnoresUnknownDuration(t *testing.T) {
	// out_time beyond duration clamps to 100.
	var pcts []float64
	parseProgress(strings.NewReader("out_time=00:00:99.000000\n"), 10, func(p float64) {
		pcts = append(pcts, p)
	})
	if len(pcts) != 1 || pcts[0] != 100 {
		t.Fatalf("clamp: got %v, want [100]", pcts)
	}

	// Unknown duration → no ticks emitted.
	called := false
	parseProgress(strings.NewReader("out_time=00:00:05.000000\n"), 0, func(float64) { called = true })
	if called {
		t.Error("expected no progress ticks when duration is unknown")
	}
}

func TestParseProgressMicroseconds(t *testing.T) {
	var got float64
	parseProgress(strings.NewReader("out_time_us=2500000\n"), 5, func(p float64) { got = p })
	if math.Abs(got-50) > 0.01 {
		t.Errorf("out_time_us tick = %v, want 50", got)
	}
}
