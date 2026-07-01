package compatibility

import (
	"testing"
)

// video/audio/subtitle are tiny builders so each test case reads as "what's
// in the file", not ffprobe JSON boilerplate.

func video(codec string, bitDepth int) StreamInfo {
	return StreamInfo{Index: 0, CodecType: "video", CodecName: codec, BitDepth: bitDepth}
}

func audio(index int, codec, profile string, channels int) StreamInfo {
	return StreamInfo{Index: index, CodecType: "audio", CodecName: codec, Profile: profile, Channels: channels}
}

func subtitle(index int, codec string) StreamInfo {
	return StreamInfo{Index: index, CodecType: "subtitle", CodecName: codec}
}

func hasReason(reasons []Reason, code string) bool {
	for _, r := range reasons {
		if r.Code == code {
			return true
		}
	}
	return false
}

func hasSeverity(reasons []Reason, code string, sev Severity) bool {
	for _, r := range reasons {
		if r.Code == code {
			return r.Severity == sev
		}
	}
	return false
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name    string
		input   EvalInput
		profile ClientProfile

		wantDirectPlay bool
		wantAction     Action
		wantHardCodes  []string // reasons that must be present with Hard severity
		wantAdvisory   []string // reasons that must be present with Advisory severity
		wantNoHard     bool     // asserts zero Hard reasons even if Advisory ones exist
	}{
		{
			// Sourced by docs/COMPATIBILITY PLAN.md §6 "Plex format support
			// (general)": MP4 + H.264 + AAC direct-plays everywhere.
			name: "h264 aac mp4 direct plays on apple tv",
			input: EvalInput{
				ContainerFormat: "mov,mp4,m4a,3gp,3g2,mj2",
				Streams:         []StreamInfo{video("h264", 8), audio(1, "aac", "LC", 2)},
			},
			profile:        AppleTV4K,
			wantDirectPlay: true,
			wantAction:     ActionNone,
		},
		{
			name: "hevc 8bit ac3 5.1 mp4 direct plays on apple tv",
			input: EvalInput{
				ContainerFormat: "mov,mp4,m4a,3gp,3g2,mj2",
				Streams:         []StreamInfo{video("hevc", 8), audio(1, "ac3", "", 6)},
			},
			profile:        AppleTV4K,
			wantDirectPlay: true,
			wantAction:     ActionNone,
		},
		{
			// Corrected sourcing (§6 correction #1/#2): MKV is Advisory, not
			// Hard, on Apple TV, and DTS-HD MA is Advisory (no bitstream
			// passthrough at all, but the server-side allowlist is
			// unconfirmed) — so this file should NOT show as a confident
			// direct-play failure (no Hard reasons), just a hedged one.
			// Both a container and an audio reason fire here; recommended
			// action priority (see actions.go) picks container -> remux.
			name: "hevc 10bit dts-hd mkv on apple tv is advisory-only, not a hard fail",
			input: EvalInput{
				ContainerFormat: "matroska,webm",
				Streams:         []StreamInfo{video("hevc", 10), audio(1, "dts", "DTS-HD MA", 6)},
			},
			profile:        AppleTV4K,
			wantDirectPlay: false,
			wantAction:     ActionRemux,
			wantAdvisory:   []string{"container_mkv", "audio_dts-hd"},
			wantNoHard:     true,
		},
		{
			// Sourced by §6 "Apple TV 4K via Plex" (Apple's own tech specs):
			// VC-1/MPEG-2 are not in the Apple TV 4K hardware decode ceiling.
			name: "mpeg2 fails hard on apple tv, recommends hevc re-encode",
			input: EvalInput{
				ContainerFormat: "mov,mp4,m4a,3gp,3g2,mj2",
				Streams:         []StreamInfo{video("mpeg2video", 8), audio(1, "aac", "", 2)},
			},
			profile:        AppleTV4K,
			wantDirectPlay: false,
			wantAction:     ActionReencodeHEVC,
			wantHardCodes:  []string{"video_codec_mpeg2video"},
		},
		{
			// Sourced by §6 "Nvidia Shield" (NVIDIA's own spec page + Plex's
			// NVIDIA Shield limitations doc): hardware MPEG-2 decode.
			name: "mpeg2 direct plays on nvidia shield (hardware decode)",
			input: EvalInput{
				ContainerFormat: "mpegts",
				Streams:         []StreamInfo{video("mpeg2video", 8), audio(1, "ac3", "", 6)},
			},
			profile:        NvidiaShield,
			wantDirectPlay: true,
			wantAction:     ActionNone,
		},
		{
			// Sourced by §6 "Nvidia Shield": TrueHD is real HDMI
			// pass-through per NVIDIA's own spec, so this is Advisory
			// ("depends on your AVR"), not a confident failure.
			name: "truehd 7.1 mkv is advisory-only on nvidia shield",
			input: EvalInput{
				ContainerFormat: "matroska,webm",
				Streams:         []StreamInfo{video("hevc", 10), audio(1, "truehd", "Dolby TrueHD + Dolby Atmos", 8)},
			},
			profile:        NvidiaShield,
			wantDirectPlay: false,
			wantAction:     ActionAudioTranscode,
			wantAdvisory:   []string{"audio_truehd"},
			wantNoHard:     true,
		},
		{
			// Sourced by §6 "Plex Web Player": Plex's default browser
			// profile doesn't grant HEVC direct play even on browsers that
			// hardware-decode HEVC. No hevc in VideoCodecs means re-encoding
			// to HEVC would not fix this profile, so the action is manual,
			// not reencode_hevc.
			name: "hevc fails hard on plex web, no automatable fix",
			input: EvalInput{
				ContainerFormat: "matroska,webm",
				Streams:         []StreamInfo{video("hevc", 8), audio(1, "aac", "", 2)},
			},
			profile:        PlexWeb,
			wantDirectPlay: false,
			wantAction:     ActionManual,
			wantHardCodes:  []string{"video_codec_hevc", "container_mkv"},
		},
		{
			name: "8 channel ac3 exceeds plex web's stereo/5.1 cap",
			input: EvalInput{
				ContainerFormat: "mov,mp4,m4a,3gp,3g2,mj2",
				Streams:         []StreamInfo{video("h264", 8), audio(1, "ac3", "", 8)},
			},
			profile:        PlexWeb,
			wantDirectPlay: false,
			wantAction:     ActionAudioTranscode,
			wantHardCodes:  []string{"audio_channels_exceeded"},
		},
		{
			// Sourced by §6 "Jellyfin / generic_hevc": Kodi/JMP direct-play
			// HEVC 8/10-bit in MKV cleanly, no container penalty.
			name: "hevc 10bit ac3 mkv direct plays on generic hevc client",
			input: EvalInput{
				ContainerFormat: "matroska,webm",
				Streams:         []StreamInfo{video("hevc", 10), audio(1, "ac3", "", 6)},
			},
			profile:        GenericHEVC,
			wantDirectPlay: true,
			wantAction:     ActionNone,
		},
		{
			// Sourced by §6 "Subtitles": PGS forces burn-in regardless of
			// profile — even a profile that's otherwise fully compatible.
			name: "pgs subtitles force a hard fail even on an otherwise-compatible file",
			input: EvalInput{
				ContainerFormat: "mov,mp4,m4a,3gp,3g2,mj2",
				Streams: []StreamInfo{
					video("h264", 8),
					audio(1, "aac", "", 2),
					subtitle(2, "hdmv_pgs_subtitle"),
				},
			},
			profile:        AppleTV4K,
			wantDirectPlay: false,
			wantAction:     ActionManual,
			wantHardCodes:  []string{"subtitle_pgs"},
		},
		{
			// PGS subtitle buried behind a non-subtitle-track index still
			// must be caught — this is the whole reason media_streams
			// stores every stream, not just the first subtitle track.
			name: "pgs subtitle beyond the first subtitle stream is still caught",
			input: EvalInput{
				ContainerFormat: "matroska,webm",
				Streams: []StreamInfo{
					video("hevc", 8),
					audio(1, "aac", "", 2),
					subtitle(2, "subrip"),
					subtitle(3, "hdmv_pgs_subtitle"),
				},
			},
			profile:       NvidiaShield,
			wantAction:    ActionManual,
			wantHardCodes: []string{"subtitle_pgs"},
		},
		{
			name: "no video stream returns a clean verdict rather than panicking",
			input: EvalInput{
				ContainerFormat: "mp3",
				Streams:         []StreamInfo{audio(0, "mp3", "", 2)},
			},
			profile:        PlexWeb,
			wantDirectPlay: true,
			wantAction:     ActionNone,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := Evaluate(tc.input, tc.profile)

			if v.DirectPlayPredicted != tc.wantDirectPlay && (tc.wantDirectPlay || len(tc.wantHardCodes) > 0 || len(tc.wantAdvisory) > 0) {
				t.Errorf("DirectPlayPredicted = %v, want %v (reasons: %+v)", v.DirectPlayPredicted, tc.wantDirectPlay, v.Reasons)
			}
			if v.RecommendedAction != tc.wantAction {
				t.Errorf("RecommendedAction = %q, want %q", v.RecommendedAction, tc.wantAction)
			}
			for _, code := range tc.wantHardCodes {
				if !hasSeverity(v.Reasons, code, Hard) {
					t.Errorf("expected Hard reason %q, got reasons: %+v", code, v.Reasons)
				}
			}
			for _, code := range tc.wantAdvisory {
				if !hasSeverity(v.Reasons, code, Advisory) {
					t.Errorf("expected Advisory reason %q, got reasons: %+v", code, v.Reasons)
				}
			}
			if tc.wantNoHard {
				for _, r := range v.Reasons {
					if r.Severity == Hard {
						t.Errorf("expected no Hard reasons, got %q (Hard)", r.Code)
					}
				}
			}
			if !tc.wantDirectPlay && len(v.Reasons) == 0 {
				t.Errorf("wantDirectPlay=false but no reasons were produced")
			}
		})
	}
}

func TestBuiltinProfiles(t *testing.T) {
	profiles := BuiltinProfiles()
	if len(profiles) != 4 {
		t.Fatalf("expected 4 built-in profiles, got %d", len(profiles))
	}

	wantIDs := map[string]bool{"apple_tv_4k": true, "nvidia_shield": true, "plex_web": true, "generic_hevc": true}
	for _, p := range profiles {
		if !wantIDs[p.ID] {
			t.Errorf("unexpected profile ID %q", p.ID)
		}
		delete(wantIDs, p.ID)
		if p.Name == "" {
			t.Errorf("profile %q missing Name", p.ID)
		}
	}
	if len(wantIDs) != 0 {
		t.Errorf("missing profiles: %v", wantIDs)
	}

	if _, ok := Profile("apple_tv_4k"); !ok {
		t.Error("Profile(\"apple_tv_4k\") should be found")
	}
	if _, ok := Profile("does_not_exist"); ok {
		t.Error("Profile(\"does_not_exist\") should not be found")
	}
}

func TestWeightOrdering(t *testing.T) {
	// The doc's qualitative ordering (§6): Hard reasons must always weigh
	// at least as much as Advisory reasons of the same reason family.
	if weight("container_mkv", Hard) <= weight("container_mkv", Advisory) {
		t.Error("Hard container weight should exceed Advisory container weight")
	}
	if weight("audio_dts", Hard) <= weight("audio_dts", Advisory) {
		t.Error("Hard audio weight should exceed Advisory audio weight")
	}
}
