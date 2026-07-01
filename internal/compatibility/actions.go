package compatibility

import "strings"

// recommendedAction picks one remediation for a Verdict from its Reasons.
// Priority (most foundational fix first): a video-codec/bit-depth problem
// dominates everything else (fixing it via re-encode also changes the
// container the worker writes, incidentally addressing many container
// issues too — see docs/COMPATIBILITY PLAN.md §11); subtitle burn-in has no
// automatable job type in this plan (Phase 3's job types are reencode,
// remux, audio_transcode — none of them can burn in subtitles), so it's
// manual; container and audio issues map to their Phase 3 job types.
//
// Advisory-only reasons still drive a recommendation here (not just Hard
// ones) — an Advisory reason means "might transcode, can't confirm," and the
// fix that would remove the ambiguity is the same job either way; RiskScore,
// not RecommendedAction, is where the Hard/Advisory distinction shows up.
func recommendedAction(reasons []Reason, profile ClientProfile) Action {
	if len(reasons) == 0 {
		return ActionNone
	}

	var hasVideo, hasHDR, hasSubtitle, hasContainer, hasAudio bool
	for _, r := range reasons {
		switch {
		case strings.HasPrefix(r.Code, "video_codec_") || strings.HasPrefix(r.Code, "hevc_"):
			hasVideo = true
		case strings.HasPrefix(r.Code, "hdr_"):
			hasHDR = true
		case r.Code == "subtitle_pgs":
			hasSubtitle = true
		case strings.HasPrefix(r.Code, "container_"):
			hasContainer = true
		case strings.HasPrefix(r.Code, "audio_"):
			hasAudio = true
		}
	}

	switch {
	case hasVideo:
		// Re-encoding to HEVC only fixes the video-codec problem if this
		// profile can actually play HEVC (e.g. it does nothing for
		// plex_web, which is H.264-only) — Reclaim's worker has no other
		// video-target job type (see §1 non-goals: libx265 only).
		if contains(profile.Rules.VideoCodecs, "hevc") {
			return ActionReencodeHEVC
		}
		return ActionManual
	case hasHDR:
		// Tone-mapping / HDR→SDR is not automatable in v1 (Phase 1.5).
		return ActionManual
	case hasSubtitle:
		return ActionManual
	case hasContainer:
		return ActionRemux
	case hasAudio:
		return ActionAudioTranscode
	default:
		return ActionNone
	}
}
