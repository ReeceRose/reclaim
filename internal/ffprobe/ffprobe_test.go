package ffprobe

import (
	"os"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    Result
	}{
		{
			name:    "h264 mp4 with audio",
			fixture: "testdata/h264_mp4.json",
			want: Result{
				VideoCodec:        ptr("h264"),
				VideoCodecProfile: ptr("High"),
				Width:             ptr(1920),
				Height:            ptr(1080),
				DurationSeconds:   ptr(5400.123),
				BitrateKbps:       ptr(8123),
				AudioCodec:        ptr("aac"),
				AudioChannels:     ptr(2),
				ContainerFormat:   ptr("mov,mp4,m4a,3gp,3g2,mj2"),
				IsAlreadyHEVC:     false,
			},
		},
		{
			name:    "hevc mkv — is_already_hevc true",
			fixture: "testdata/hevc_mkv.json",
			want: Result{
				VideoCodec:        ptr("hevc"),
				VideoCodecProfile: ptr("Main"),
				Width:             ptr(3840),
				Height:            ptr(2160),
				DurationSeconds:   ptr(7200.0),
				BitrateKbps:       ptr(15234),
				AudioCodec:        ptr("dts"),
				AudioChannels:     ptr(6),
				ContainerFormat:   ptr("matroska,webm"),
				IsAlreadyHEVC:     true,
			},
		},
		{
			name:    "mpeg2 — duration and bitrate from format only",
			fixture: "testdata/mpeg2_mpeg.json",
			want: Result{
				VideoCodec:        ptr("mpeg2video"),
				VideoCodecProfile: ptr("Main"),
				Width:             ptr(720),
				Height:            ptr(480),
				DurationSeconds:   ptr(1800.0),
				BitrateKbps:       ptr(5000),
				AudioCodec:        ptr("mp2"),
				AudioChannels:     ptr(2),
				ContainerFormat:   ptr("mpeg"),
				IsAlreadyHEVC:     false,
			},
		},
		{
			name:    "no audio stream",
			fixture: "testdata/no_audio.json",
			want: Result{
				VideoCodec:        ptr("h264"),
				VideoCodecProfile: ptr("High"),
				Width:             ptr(1280),
				Height:            ptr(720),
				DurationSeconds:   ptr(120.0),
				BitrateKbps:       ptr(3001),
				AudioCodec:        nil,
				AudioChannels:     nil,
				ContainerFormat:   ptr("mov,mp4,m4a,3gp,3g2,mj2"),
				IsAlreadyHEVC:     false,
			},
		},
		{
			name:    "duration falls back to video stream",
			fixture: "testdata/duration_in_stream.json",
			want: Result{
				VideoCodec:        ptr("h264"),
				VideoCodecProfile: ptr("High"),
				Width:             ptr(1280),
				Height:            ptr(720),
				DurationSeconds:   ptr(900.0),
				BitrateKbps:       ptr(4012),
				AudioCodec:        nil,
				AudioChannels:     nil,
				ContainerFormat:   ptr("mov,mp4,m4a,3gp,3g2,mj2"),
				IsAlreadyHEVC:     false,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.fixture)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			got, err := parse(data, tc.fixture)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			assertResult(t, got, &tc.want)
		})
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := parse([]byte("not json at all"), "fake.mp4")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	var pe *ProbeError
	if !isProbeError(err, &pe) {
		t.Fatalf("expected *ProbeError, got %T: %v", err, err)
	}
	if pe.Path != "fake.mp4" {
		t.Errorf("ProbeError.Path = %q, want %q", pe.Path, "fake.mp4")
	}
}

// isProbeError checks if err is a *ProbeError and sets out if so.
func isProbeError(err error, out **ProbeError) bool {
	pe, ok := err.(*ProbeError)
	if ok {
		*out = pe
	}
	return ok
}

// assertResult compares two *Result structs field by field.
func assertResult(t *testing.T, got, want *Result) {
	t.Helper()
	assertPtrString(t, "VideoCodec", got.VideoCodec, want.VideoCodec)
	assertPtrString(t, "VideoCodecProfile", got.VideoCodecProfile, want.VideoCodecProfile)
	assertPtrInt(t, "Width", got.Width, want.Width)
	assertPtrInt(t, "Height", got.Height, want.Height)
	assertPtrInt(t, "BitrateKbps", got.BitrateKbps, want.BitrateKbps)
	assertPtrString(t, "AudioCodec", got.AudioCodec, want.AudioCodec)
	assertPtrInt(t, "AudioChannels", got.AudioChannels, want.AudioChannels)
	assertPtrString(t, "ContainerFormat", got.ContainerFormat, want.ContainerFormat)
	if got.IsAlreadyHEVC != want.IsAlreadyHEVC {
		t.Errorf("IsAlreadyHEVC = %v, want %v", got.IsAlreadyHEVC, want.IsAlreadyHEVC)
	}
	// DurationSeconds: compare within 0.001s tolerance
	if got.DurationSeconds == nil && want.DurationSeconds != nil {
		t.Errorf("DurationSeconds = nil, want %v", *want.DurationSeconds)
	} else if got.DurationSeconds != nil && want.DurationSeconds == nil {
		t.Errorf("DurationSeconds = %v, want nil", *got.DurationSeconds)
	} else if got.DurationSeconds != nil && want.DurationSeconds != nil {
		diff := *got.DurationSeconds - *want.DurationSeconds
		if diff < -0.001 || diff > 0.001 {
			t.Errorf("DurationSeconds = %v, want %v", *got.DurationSeconds, *want.DurationSeconds)
		}
	}
}

func assertPtrString(t *testing.T, field string, got, want *string) {
	t.Helper()
	if got == nil && want == nil {
		return
	}
	if got == nil {
		t.Errorf("%s = nil, want %q", field, *want)
		return
	}
	if want == nil {
		t.Errorf("%s = %q, want nil", field, *got)
		return
	}
	if *got != *want {
		t.Errorf("%s = %q, want %q", field, *got, *want)
	}
}

func assertPtrInt(t *testing.T, field string, got, want *int) {
	t.Helper()
	if got == nil && want == nil {
		return
	}
	if got == nil {
		t.Errorf("%s = nil, want %d", field, *want)
		return
	}
	if want == nil {
		t.Errorf("%s = %d, want nil", field, *got)
		return
	}
	if *got != *want {
		t.Errorf("%s = %d, want %d", field, *got, *want)
	}
}
