package compatibility

import (
	"fmt"
	"strings"
)

// EvalInput is the subset of probed file metadata Evaluate needs. Callers
// (the scanner, using ffprobe.Result; later the store, using persisted
// media_streams rows) map their own types into this one — see the package
// doc comment for why this package can't import either directly.
type EvalInput struct {
	// ContainerFormat is the raw ffprobe format_name, e.g. "matroska,webm"
	// or "mov,mp4,m4a,3gp,3g2,mj2". ffmpeg's demuxer names are coarser than
	// the container concept client profiles care about (the MP4/MOV/M4V
	// family shares one demuxer name), so matching happens against the
	// whole containerFamilies set, not an exact string.
	ContainerFormat string
	// ColorTransfer is the primary video stream's color_transfer (e.g.
	// "smpte2084" for HDR10 PQ, "arib-std-b67" for HLG). Empty means SDR or
	// unknown.
	ColorTransfer string
	// DolbyVisionProfile is set when ffprobe reports a Dolby Vision
	// configuration record on the primary video stream.
	DolbyVisionProfile *int
	Streams            []StreamInfo
}

// StreamInfo is one ffprobe stream, trimmed to what rule evaluation needs.
type StreamInfo struct {
	Index     int
	CodecType string // video | audio | subtitle
	CodecName string
	Profile   string // e.g. "Main 10", "DTS-HD MA" — disambiguates codec variants codec_name alone can't
	BitDepth  int    // video only, derived from pix_fmt; 0 = unknown
	Channels  int    // audio only
}

// Evaluate predicts direct-play compatibility of a file against profile.
// v1 models a single primary video stream and a single primary audio stream
// (the first of each in ffprobe's stream order) — the same simplification
// internal/ffprobe/internal/store already make when denormalizing
// VideoCodec/AudioCodec onto media_files. Subtitles are the exception: every
// subtitle stream is checked for image-based formats, since a PGS track
// buried behind a first "normal" subtitle track would otherwise be missed
// (the reason media_streams stores all streams, not just the first — see
// docs/COMPATIBILITY PLAN.md §5).
func Evaluate(input EvalInput, profile ClientProfile) Verdict {
	var video, audio *StreamInfo
	for i := range input.Streams {
		st := &input.Streams[i]
		switch st.CodecType {
		case "video":
			if video == nil {
				video = st
			}
		case "audio":
			if audio == nil {
				audio = st
			}
		}
	}

	if video == nil {
		// No video stream to evaluate (e.g. audio-only file). Upstream
		// query filters already exclude these from the compatibility list
		// (video_codec IS NOT NULL, §8), but stay defensive rather than
		// fabricate a verdict.
		return Verdict{DirectPlayPredicted: true, RecommendedAction: ActionNone}
	}

	var reasons []Reason

	reasons = append(reasons, evaluateVideo(*video, profile)...)
	reasons = append(reasons, evaluateHDR(input, *video, profile)...)
	reasons = append(reasons, evaluateContainer(input.ContainerFormat, profile)...)
	if audio != nil {
		reasons = append(reasons, evaluateAudio(*audio, profile)...)
	}
	reasons = append(reasons, evaluateSubtitles(input.Streams)...)

	score := 0
	for _, r := range reasons {
		score += weight(r.Code, r.Severity)
	}
	if score > 100 {
		score = 100
	}

	return Verdict{
		DirectPlayPredicted: len(reasons) == 0,
		RiskScore:           score,
		Reasons:             reasons,
		RecommendedAction:   recommendedAction(reasons, profile),
	}
}

func evaluateVideo(video StreamInfo, profile ClientProfile) []Reason {
	codec := strings.ToLower(video.CodecName)
	idx := video.Index

	if !contains(profile.Rules.VideoCodecs, codec) {
		return []Reason{{
			Code:     "video_codec_" + codec,
			Severity: Hard,
			Stream:   &idx,
			Message:  fmt.Sprintf("%s video is not supported on %s", strings.ToUpper(codec), profile.Name),
		}}
	}

	if codec == "hevc" && profile.Rules.MaxHEVCBitDepth > 0 && video.BitDepth > profile.Rules.MaxHEVCBitDepth {
		return []Reason{{
			Code:     fmt.Sprintf("hevc_%dbit", video.BitDepth),
			Severity: Hard,
			Stream:   &idx,
			Message: fmt.Sprintf("HEVC %d-bit exceeds %s's %d-bit cap",
				video.BitDepth, profile.Name, profile.Rules.MaxHEVCBitDepth),
		}}
	}

	return nil
}

func evaluateHDR(input EvalInput, video StreamInfo, profile ClientProfile) []Reason {
	if profile.Rules.SupportsHDR {
		return nil
	}
	kind, ok := hdrKind(input)
	if !ok {
		return nil
	}
	idx := video.Index
	return []Reason{{
		Code:     "hdr_" + kind,
		Severity: Hard,
		Stream:   &idx,
		Message:  hdrMessage(profile.Name, kind),
	}}
}

func hdrKind(input EvalInput) (string, bool) {
	if input.DolbyVisionProfile != nil {
		return "dolby_vision", true
	}
	switch strings.ToLower(strings.TrimSpace(input.ColorTransfer)) {
	case "smpte2084":
		return "hdr10", true
	case "arib-std-b67":
		return "hlg", true
	}
	return "", false
}

func hdrMessage(profileName, kind string) string {
	switch kind {
	case "dolby_vision":
		return fmt.Sprintf("Dolby Vision is not supported on %s (SDR-only client profile)", profileName)
	case "hdr10":
		return fmt.Sprintf("HDR10 (PQ transfer) is not supported on %s (SDR-only client profile)", profileName)
	case "hlg":
		return fmt.Sprintf("HLG HDR is not supported on %s (SDR-only client profile)", profileName)
	default:
		return fmt.Sprintf("HDR content is not supported on %s", profileName)
	}
}

// containerFamilies maps ffprobe's format_name (a demuxer alias list, often
// coarser than the container concept a client profile cares about) to every
// container ID it could plausibly represent. The QuickTime/MP4 family (mov,
// mp4, m4v) all share one demuxer name, so membership must be checked as a
// set intersection, not an exact string match.
var containerFamilies = map[string][]string{
	"mov,mp4,m4a,3gp,3g2,mj2": {"mp4", "mov", "m4v"},
	"matroska,webm":           {"mkv", "webm"},
	"mpegts":                  {"mpegts", "m2ts"},
	"mpeg":                    {"m2ts", "mpegts"},
}

func containerCandidates(formatName string) []string {
	f := strings.ToLower(strings.TrimSpace(formatName))
	if fam, ok := containerFamilies[f]; ok {
		return fam
	}
	if f == "" {
		return nil
	}
	return []string{f}
}

func evaluateContainer(formatName string, profile ClientProfile) []Reason {
	candidates := containerCandidates(formatName)
	if len(candidates) == 0 {
		return nil // unknown container, nothing to flag
	}

	for _, cand := range candidates {
		if contains(profile.Rules.Containers, cand) {
			return nil // Hard-allowed
		}
	}
	for _, cand := range candidates {
		if contains(profile.Rules.ContainersAdvisory, cand) {
			return []Reason{{
				Code:     "container_" + cand,
				Severity: Advisory,
				Message:  fmt.Sprintf("%s container support on %s depends on the server's client profile, not hardware", strings.ToUpper(cand), profile.Name),
			}}
		}
	}

	label := candidates[0]
	return []Reason{{
		Code:     "container_" + label,
		Severity: Hard,
		Message:  fmt.Sprintf("%s container is not supported on %s", strings.ToUpper(label), profile.Name),
	}}
}

// audioCodecKey maps an ffprobe (codec_name, profile) pair to the codec ID
// used in Rules.AudioCodecs/AudioCodecsAdvisory. This exists because ffprobe
// reports DTS-HD MA / DTS:X with codec_name "dts" and the variant only in
// the profile string, and multi-channel PCM under several codec_name
// variants (pcm_s16le, pcm_bluray, ...).
func audioCodecKey(codecName, profile string) string {
	name := strings.ToLower(codecName)
	prof := strings.ToLower(profile)
	switch {
	case name == "dts" && (strings.Contains(prof, "dts:x") || strings.Contains(prof, "dts-x") || strings.Contains(prof, "dtsx")):
		return "dtsx"
	case name == "dts" && strings.Contains(prof, "dts-hd"):
		return "dts-hd"
	case strings.HasPrefix(name, "pcm"):
		return "pcm"
	default:
		return name
	}
}

func evaluateAudio(audio StreamInfo, profile ClientProfile) []Reason {
	key := audioCodecKey(audio.CodecName, audio.Profile)
	idx := audio.Index
	var reasons []Reason

	switch {
	case contains(profile.Rules.AudioCodecs, key):
		// Hard-allowed, no reason.
	case contains(profile.Rules.AudioCodecsAdvisory, key):
		reasons = append(reasons, Reason{
			Code:     "audio_" + key,
			Severity: Advisory,
			Stream:   &idx,
			Message:  audioAdvisoryMessage(profile.ID, key),
		})
	default:
		reasons = append(reasons, Reason{
			Code:     "audio_" + key,
			Severity: Hard,
			Stream:   &idx,
			Message:  fmt.Sprintf("%s audio is not supported on %s", strings.ToUpper(key), profile.Name),
		})
	}

	if profile.Rules.MaxAudioChannels > 0 && audio.Channels > profile.Rules.MaxAudioChannels {
		reasons = append(reasons, Reason{
			Code:     "audio_channels_exceeded",
			Severity: Hard,
			Stream:   &idx,
			Message: fmt.Sprintf("%d audio channels exceeds %s's %d-channel cap",
				audio.Channels, profile.Name, profile.Rules.MaxAudioChannels),
		})
	}

	return reasons
}

// audioAdvisoryMessage hedges per-device, per the corrected sourcing in
// docs/COMPATIBILITY PLAN.md §6 correction #2: Apple TV has no bitstream
// passthrough at all (always decodes internally to LPCM), which is a
// categorically different mechanism from NVIDIA Shield's real HDMI
// passthrough — the two profiles must not share hedge copy.
func audioAdvisoryMessage(profileID, key string) string {
	upper := strings.ToUpper(key)
	switch profileID {
	case "apple_tv_4k":
		return fmt.Sprintf("%s audio \u2014 Apple TV has no bitstream passthrough; it decodes this internally to LPCM, likely losing lossless/spatial metadata", upper)
	case "nvidia_shield":
		return fmt.Sprintf("%s audio \u2014 passes through over HDMI on Shield, but reaching your speakers intact depends on a passthrough-capable AVR/receiver", upper)
	default:
		return fmt.Sprintf("%s audio may require an audio-only transcode depending on your setup", upper)
	}
}

// pgsSubtitleCodecs are image/bitmap subtitle formats that force subtitle
// burn-in (a full video transcode) on most clients regardless of profile —
// see docs/COMPATIBILITY PLAN.md §6 "Subtitles". This check is deliberately
// not part of Rules: it isn't profile-specific.
var pgsSubtitleCodecs = map[string]bool{
	"hdmv_pgs_subtitle": true,
	"dvd_subtitle":      true,
	"dvdsub":            true,
	"dvb_subtitle":      true,
	"dvbsub":            true,
	"xsub":              true,
}

func evaluateSubtitles(streams []StreamInfo) []Reason {
	for _, st := range streams {
		if st.CodecType != "subtitle" {
			continue
		}
		if !pgsSubtitleCodecs[strings.ToLower(st.CodecName)] {
			continue
		}
		idx := st.Index
		return []Reason{{
			Code:     "subtitle_pgs",
			Severity: Hard,
			Stream:   &idx,
			Message: fmt.Sprintf("Image-based subtitles (%s) force subtitle burn-in \u2014 a full video transcode \u2014 on most clients regardless of profile",
				st.CodecName),
		}}
	}
	return nil
}

// weight is how much one reason contributes to RiskScore. Hard reasons carry
// more weight than Advisory ones in every category — see docs/COMPATIBILITY
// PLAN.md §6's RiskScore definition — but the exact numbers are this
// package's judgment call; the doc pins the qualitative ordering, not the
// scale.
func weight(code string, sev Severity) int {
	switch {
	case strings.HasPrefix(code, "video_codec_"):
		return 45
	case strings.HasPrefix(code, "hevc_"):
		return 40
	case strings.HasPrefix(code, "hdr_"):
		return 38
	case code == "subtitle_pgs":
		return 40
	case code == "audio_channels_exceeded":
		return 25
	case strings.HasPrefix(code, "container_"):
		if sev == Hard {
			return 35
		}
		return 12
	case strings.HasPrefix(code, "audio_"):
		if sev == Hard {
			return 30
		}
		return 10
	default:
		return 10
	}
}

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
