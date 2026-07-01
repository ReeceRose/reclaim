// Package compatibility predicts whether a media file will direct-play on a
// given client profile (Apple TV 4K, NVIDIA Shield, Plex Web, a generic HEVC
// client) instead of forcing Plex/Jellyfin/Emby to transcode it. Everything
// here is derived from ffprobe metadata only — never Plex/Jellyfin telemetry
// (see docs/COMPATIBILITY PLAN.md §1 non-goals) — so every verdict is a
// prediction, not a guarantee, and callers must keep that framing in
// user-facing copy (§10 "Copy guidelines").
//
// Named "compatibility" rather than "directplay" deliberately: direct-play
// prediction is the first consumer, not the package's ceiling — future
// checks unrelated to direct-play (e.g. corruption/health checks) could
// reasonably live here too.
//
// This package is deliberately free of dependencies on internal/store and
// internal/ffprobe: internal/store will depend on this package (to persist
// Verdicts in media_compatibility), so this package cannot depend back on
// internal/store without an import cycle. Callers map their own probe types
// into EvalInput.
package compatibility

// Severity reflects confidence, not just impact. Hard reasons are client
// hardware/software ceilings that are stable regardless of user settings.
// Advisory reasons depend on things this package can't probe (server
// version, AVR/receiver passthrough capability, user-toggled settings) and
// contribute less to RiskScore — see docs/COMPATIBILITY PLAN.md §6.
type Severity int

const (
	Hard Severity = iota
	Advisory
)

func (s Severity) String() string {
	switch s {
	case Hard:
		return "hard"
	case Advisory:
		return "advisory"
	default:
		return "unknown"
	}
}

// Action is the recommended remediation for a failing verdict. It names the
// job type that would address the issue, not a promise that Reclaim can
// queue it yet — only "reencode_hevc" is queueable pre-Phase-3.
type Action string

const (
	ActionNone           Action = "none"
	ActionReencodeHEVC   Action = "reencode_hevc"
	ActionRemux          Action = "remux"
	ActionAudioTranscode Action = "audio_transcode"
	ActionManual         Action = "manual"
)

// Reason is one specific, human-readable cause contributing to a Verdict.
type Reason struct {
	Code     string // e.g. "audio_dts", "container_mkv", "hevc_10bit"
	Severity Severity
	Stream   *int // ffprobe stream index, when the reason is stream-specific
	Message  string
}

// Verdict is the outcome of evaluating one file against one ClientProfile.
type Verdict struct {
	DirectPlayPredicted bool
	RiskScore           int // 0 = likely direct play, 100 = certain transcode
	Reasons             []Reason
	RecommendedAction   Action
}

// Rules is the versioned bundle of allow/deny checks for one client. Fields
// with an "Advisory" counterpart split hard hardware ceilings from
// passthrough-/server-profile-dependent behavior Reclaim can't confirm from
// probe data alone — see docs/COMPATIBILITY PLAN.md §6.
type Rules struct {
	// VideoCodecs is the Hard allowlist of ffprobe codec_name values (e.g.
	// "h264", "hevc", "mpeg2video") the client can decode at all. There is
	// no VideoCodecsAdvisory: video codec support is a hardware decode
	// ceiling, not something that varies with settings or receivers.
	VideoCodecs []string
	// MaxHEVCBitDepth caps HEVC bit depth (8/10/12) when "hevc" is in
	// VideoCodecs. 0 means HEVC isn't allowed at all (see PlexWeb), in which
	// case VideoCodecs already carries the failure and this is unused.
	MaxHEVCBitDepth int

	// Containers is the Hard allowlist of container IDs ("mp4", "mov",
	// "m4v", "mkv", "webm", "mpegts", "m2ts", "avi", "asf").
	Containers []string
	// ContainersAdvisory lists containers that often work but aren't a
	// stable hardware fact (e.g. MKV+HEVC on Apple TV depends on the
	// server's client profile, not tvOS hardware — see §6 sourcing).
	ContainersAdvisory []string

	// AudioCodecs is the Hard allowlist: codecs with no known passthrough
	// or reliable internal-decode path on this client.
	AudioCodecs []string
	// AudioCodecsAdvisory lists codecs whose fate depends on downstream
	// AVR/receiver passthrough capability (NVIDIA Shield) or on an
	// unconfirmed server-side allowlist (Apple TV) — never a hardware
	// ceiling. See §6 audio caveat.
	AudioCodecsAdvisory []string
	// MaxAudioChannels is Hard: channel counts above this always fail on
	// clients with no passthrough path (e.g. plex_web); on
	// passthrough-capable clients it reflects the HDMI/eARC channel cap.
	MaxAudioChannels int
}
