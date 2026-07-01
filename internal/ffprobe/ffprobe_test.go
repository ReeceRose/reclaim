package ffprobe

import (
	"encoding/json"
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

func TestParse_compatFields(t *testing.T) {
	data, err := os.ReadFile("testdata/hevc_10bit_hdr_pgs.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	got, err := parse(data, "testdata/hevc_10bit_hdr_pgs.json")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	assertPtrString(t, "PixelFormat", got.PixelFormat, ptr("yuv420p10le"))
	assertPtrInt(t, "VideoBitDepth", got.VideoBitDepth, ptr(10))
	assertPtrString(t, "ColorTransfer", got.ColorTransfer, ptr("smpte2084"))
	assertPtrString(t, "ColorPrimaries", got.ColorPrimaries, ptr("bt2020"))
	assertPtrInt(t, "AudioSampleRate", got.AudioSampleRate, ptr(48000))
	assertPtrString(t, "SubtitleCodec", got.SubtitleCodec, ptr("hdmv_pgs_subtitle"))

	if len(got.Streams) != 3 {
		t.Fatalf("Streams len = %d, want 3", len(got.Streams))
	}

	video := got.Streams[0]
	if video.CodecType != "video" || video.CodecName != "hevc" || video.Profile != "Main 10" {
		t.Errorf("video stream = %+v", video)
	}
	if !video.DispositionDefault {
		t.Error("video stream DispositionDefault = false, want true")
	}
	if video.Extra["level"] != "150" || video.Extra["pix_fmt"] != "yuv420p10le" {
		t.Errorf("video stream Extra = %+v", video.Extra)
	}

	audio := got.Streams[1]
	if audio.CodecType != "audio" || audio.CodecName != "truehd" || audio.Channels != 8 {
		t.Errorf("audio stream = %+v", audio)
	}
	if audio.Language != "eng" {
		t.Errorf("audio Language = %q, want %q", audio.Language, "eng")
	}

	subtitle := got.Streams[2]
	if subtitle.CodecType != "subtitle" || subtitle.CodecName != "hdmv_pgs_subtitle" {
		t.Errorf("subtitle stream = %+v", subtitle)
	}
	if subtitle.DispositionDefault {
		t.Error("subtitle stream DispositionDefault = true, want false")
	}
}

func TestParse_dolbyVisionAndExpandedExtra(t *testing.T) {
	data, err := os.ReadFile("testdata/dolby_vision.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	got, err := parse(data, "testdata/dolby_vision.json")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	assertPtrInt(t, "DolbyVisionProfile", got.DolbyVisionProfile, ptr(5))
	assertPtrInt(t, "DolbyVisionLevel", got.DolbyVisionLevel, ptr(6))

	if len(got.Streams) != 3 {
		t.Fatalf("Streams len = %d, want 3", len(got.Streams))
	}

	video := got.Streams[0]
	if video.Extra["codec_tag_string"] != "hvc1" {
		t.Errorf("video Extra[codec_tag_string] = %v, want hvc1", video.Extra["codec_tag_string"])
	}
	if video.Extra["color_space"] != "bt2020nc" {
		t.Errorf("video Extra[color_space] = %v, want bt2020nc", video.Extra["color_space"])
	}
	if video.Extra["r_frame_rate"] != "24000/1001" {
		t.Errorf("video Extra[r_frame_rate] = %v, want 24000/1001", video.Extra["r_frame_rate"])
	}
	if video.Extra["display_aspect_ratio"] != "16:9" {
		t.Errorf("video Extra[display_aspect_ratio] = %v, want 16:9", video.Extra["display_aspect_ratio"])
	}
	sideData, ok := video.Extra["side_data_list"].([]map[string]any)
	if !ok || len(sideData) != 1 {
		t.Fatalf("video Extra[side_data_list] = %+v, want 1 entry", video.Extra["side_data_list"])
	}
	if sideData[0]["side_data_type"] != "DOVI configuration record" {
		t.Errorf("side_data_list[0] = %+v", sideData[0])
	}

	audio := got.Streams[1]
	if audio.Extra["channel_layout"] != "5.1" {
		t.Errorf("audio Extra[channel_layout] = %v, want 5.1", audio.Extra["channel_layout"])
	}
	if audio.Extra["title"] != "Surround 5.1" {
		t.Errorf("audio Extra[title] = %v, want %q", audio.Extra["title"], "Surround 5.1")
	}

	subtitle := got.Streams[2]
	if subtitle.Extra["forced"] != true {
		t.Errorf("subtitle Extra[forced] = %v, want true", subtitle.Extra["forced"])
	}

	if got.FormatExtraJSON == nil {
		t.Fatal("FormatExtraJSON = nil, want non-nil")
	}
	var formatExtra map[string]any
	if err := json.Unmarshal([]byte(*got.FormatExtraJSON), &formatExtra); err != nil {
		t.Fatalf("unmarshal FormatExtraJSON: %v", err)
	}
	if formatExtra["format_long_name"] != "QuickTime / MOV" {
		t.Errorf("format_long_name = %v, want %q", formatExtra["format_long_name"], "QuickTime / MOV")
	}
	if formatExtra["nb_streams"] != float64(3) {
		t.Errorf("nb_streams = %v, want 3", formatExtra["nb_streams"])
	}
	if formatExtra["tag_encoder"] != "HandBrake 1.7.0" {
		t.Errorf("tag_encoder = %v, want %q", formatExtra["tag_encoder"], "HandBrake 1.7.0")
	}
}

func TestBitDepthFromPixFmt(t *testing.T) {
	tests := map[string]int{
		"yuv420p":     8,
		"yuv420p10le": 10,
		"yuv422p10le": 10,
		"yuv420p12le": 12,
		"rgb24":       8,
		"":            0,
		"nv12":        0,
	}
	for pixFmt, want := range tests {
		if got := bitDepthFromPixFmt(pixFmt); got != want {
			t.Errorf("bitDepthFromPixFmt(%q) = %d, want %d", pixFmt, got, want)
		}
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
