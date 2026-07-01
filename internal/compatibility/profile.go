package compatibility

// ClientProfile is a named ruleset representing a playback device or app —
// not an encode profile (CRF/preset, see internal/store TranscodeProfile).
type ClientProfile struct {
	ID          string // "apple_tv_4k", "nvidia_shield", "plex_web", "generic_hevc"
	Name        string
	Description string
	Rules       Rules
}

// AppleTV4K models modern Apple TV 4K hardware (3rd gen tvOS 17/18) via
// Plex/Jellyfin. Sourcing: docs/COMPATIBILITY PLAN.md §6 "Apple TV 4K via
// Plex" — Apple's own tech specs for the hardware ceiling, plus two
// independently reproduced GitHub reports (Nov 2025 / Jul 2025) and Plex
// forum threads showing the *default* tvOS client profile rejects
// hevc+mkv+http without a hand-installed custom server profile, and Apple's
// own Dolby Atmos docs confirming tvOS has no audio bitstream passthrough at
// all (it always decodes internally to LPCM or its own Dolby MAT re-encode).
var AppleTV4K = ClientProfile{
	ID:   "apple_tv_4k",
	Name: "Apple TV 4K",
	Description: "Modern Apple TV 4K hardware (tvOS 17/18) via Plex/Jellyfin. " +
		"HEVC 10-bit HDR decodes natively; MKV and lossless/spatial audio " +
		"formats depend on the server's client profile, not tvOS hardware.",
	Rules: Rules{
		// Hard: video codec is a real hardware decode ceiling.
		VideoCodecs:     []string{"h264", "hevc"},
		MaxHEVCBitDepth: 10, // modern hardware supports 10-bit HDR
		// Hard: MP4/MOV/M4V direct-play reliably. MKV is deliberately NOT
		// Hard — reproducible reports as recent as Nov 2025 show the
		// *default* tvOS profile still rejects hevc+mkv+http without a
		// hand-installed custom server-side XML profile, on current PMS/tvOS.
		Containers:         []string{"mp4", "mov", "m4v"},
		ContainersAdvisory: []string{"mkv"},
		// Advisory: AAC/AC3/EAC3 decode natively (AC3/EAC3 also carry Atmos
		// metadata via Apple's Dolby MAT mechanism). DTS/DTS-HD/TrueHD are
		// Advisory because Apple TV has NO bitstream passthrough at all — it
		// always decodes internally to LPCM, losing lossless/spatial audio
		// metadata, and whether PMS even grants Direct Play to these codecs
		// on this profile in the first place is unconfirmed from an
		// authoritative source.
		AudioCodecs:         []string{"aac", "ac3", "eac3"},
		AudioCodecsAdvisory: []string{"dts", "dts-hd", "truehd"},
		MaxAudioChannels:    6, // AC3/EAC3 5.1 ok natively
	},
}

// NvidiaShield models the NVIDIA Shield TV / Shield TV Pro (Kodi/Jellyfin
// Android TV app) via Plex/Jellyfin. Sourcing: docs/COMPATIBILITY PLAN.md §6
// "Nvidia Shield" — NVIDIA's own current spec page (broadest video decode of
// the 4 profiles, incl. hardware MPEG-2, and explicit "(pass-through)" for
// TrueHD/DTS-HD/DTS:X; no AV1 on any Shield model), corroborated by a
// Jellyfin Android TV maintainer confirming no AV1 hardware decode even on
// the 2019 Shield Pro.
var NvidiaShield = ClientProfile{
	ID:   "nvidia_shield",
	Name: "NVIDIA Shield",
	Description: "NVIDIA Shield TV / Shield TV Pro. Broadest video codec " +
		"support of the 4 profiles, incl. hardware MPEG-2 decode and real " +
		"HDMI bitstream passthrough for lossless/spatial audio. No AV1 on " +
		"any Shield model.",
	Rules: Rules{
		// Hard: broadest hardware decode of the 4 profiles, incl. MPEG-2.
		// AV1 deliberately excluded.
		VideoCodecs:     []string{"h264", "hevc", "mpeg2video", "vp8", "vp9"},
		MaxHEVCBitDepth: 10,
		Containers:      []string{"mkv", "mp4", "mov", "m2ts", "mpegts", "webm", "avi", "asf"},
		// Advisory, for a different reason than Apple TV: NVIDIA's own spec
		// explicitly says "(pass-through)" for these — real HDMI bitstream
		// passthrough, not internal decode. Confidence here is "depends on
		// your downstream AVR," not "may not even attempt it."
		AudioCodecs:         []string{"aac", "ac3", "eac3", "mp3", "flac", "pcm"},
		AudioCodecsAdvisory: []string{"truehd", "dts-hd", "dtsx"},
		MaxAudioChannels:    8, // 7.1 passthrough over HDMI
	},
}

// PlexWeb models Plex's default browser client profile (Plex Web App in
// Chrome/Edge/etc via plex.tv/web), not the browser's raw decode capability.
// Sourcing: docs/COMPATIBILITY PLAN.md §6 "Plex Web Player" — Plex's own
// support docs ("MP4 files with H.264 video and AAC audio" direct-play by
// default), corroborated by a 2026-dated Plex forum thread showing Plex Web
// still fails on HEVC source content today even on HEVC-hardware-decode
// browsers, because Plex's default server-side browser profile doesn't grant
// HEVC direct play — only a hand-edited server-side XML profile changes that,
// which Reclaim can't detect.
var PlexWeb = ClientProfile{
	ID:   "plex_web",
	Name: "Plex Web / Browser",
	Description: "Plex Web App in a browser (plex.tv/web). Conservative by " +
		"design: H.264 + AAC in MP4 only, 8-bit, \u2264 6 audio channels. A " +
		"browser tab has no HDMI/eARC passthrough path, so unlike the other " +
		"3 profiles there is no audio hedge here.",
	Rules: Rules{
		// Hard: Plex's default browser client profile, not the browser's
		// actual decode capability — HEVC support requires a hand-edited
		// server-side XML profile Reclaim cannot detect. Note: Plex's 2025
		// "HEVC hardware transcoding" feature is PMS choosing to *output*
		// HEVC during a transcode — unrelated to whether a source HEVC file
		// direct-plays here.
		VideoCodecs:     []string{"h264"},
		MaxHEVCBitDepth: 0, // n/a — HEVC not in the allowlist at all
		Containers:      []string{"mp4"},
		// Hard, not Advisory: a browser tab has no HDMI/eARC passthrough
		// path, so there's no "might work with the right AVR" hedge here.
		AudioCodecs:      []string{"aac"},
		MaxAudioChannels: 6,
	},
}

// GenericHEVC is intentionally synthetic, not a single certified device — it
// models Kodi and Jellyfin Media Player (native desktop) specifically, the
// two clients with the cleanest, most uniform HEVC 8/10-bit + MKV support in
// Jellyfin's own codec matrix. It deliberately does NOT claim to cover
// Android TV (10-bit is device-dependent), Roku (HEVC only on 4K devices), or
// the Android mobile client (currently has a broken HEVC capability report).
// Sourcing: docs/COMPATIBILITY PLAN.md §6 "Jellyfin / generic_hevc" — use as
// a loose reference point, not a device-accurate prediction; UI copy must
// say "generic" explicitly.
var GenericHEVC = ClientProfile{
	ID:   "generic_hevc",
	Name: "Generic HEVC client",
	Description: "Synthetic baseline modeling Kodi / Jellyfin Media Player " +
		"(native desktop) specifically — the cleanest, most uniform HEVC " +
		"8/10-bit + MKV support in Jellyfin's own codec matrix. A loose " +
		"reference point, not a device-accurate prediction.",
	Rules: Rules{
		VideoCodecs:     []string{"h264", "hevc"},
		MaxHEVCBitDepth: 10,
		// Hard: both container families direct-play cleanly on Kodi/JMP —
		// no MKV penalty here, unlike apple_tv_4k/plex_web.
		Containers: []string{"mp4", "mov", "m4v", "mkv"},
		// Hard: Kodi/JMP decode all of these in software or via the host's
		// audio stack without a passthrough-dependent hedge, so — unlike
		// apple_tv_4k/nvidia_shield — nothing here is Advisory.
		AudioCodecs:      []string{"aac", "ac3", "eac3", "dts", "truehd", "flac", "mp3", "pcm"},
		MaxAudioChannels: 8,
	},
}

// BuiltinProfiles returns all v1 built-in client profiles in the fixed
// display order used across the API and settings validation.
func BuiltinProfiles() []ClientProfile {
	return []ClientProfile{AppleTV4K, NvidiaShield, PlexWeb, GenericHEVC}
}

// Profile looks up a built-in profile by ID.
func Profile(id string) (ClientProfile, bool) {
	for _, p := range BuiltinProfiles() {
		if p.ID == id {
			return p, true
		}
	}
	return ClientProfile{}, false
}

// DefaultProfileID is the silent default (no first-visit prompt) per
// docs/COMPATIBILITY PLAN.md §15 Q2, and matches migration
// 00010_default_client_profile.sql's column default.
const DefaultProfileID = "apple_tv_4k"
