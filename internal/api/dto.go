package api

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"

	"reclaim/internal/media"
	"reclaim/internal/store"
)

func errorBody(msg string) map[string]string { return map[string]string{"error": msg} }

func badRequest(c *echo.Context, msg string) error {
	return c.JSON(http.StatusBadRequest, errorBody(msg))
}

func serverError(c *echo.Context, err error) error {
	slog.Error("api: internal error", "path", c.Path(), "err", err)
	return c.JSON(http.StatusInternalServerError, errorBody("internal error"))
}

// mediaFileDTO is the wire shape of a media file. Pointers map to JSON null.
type mediaFileDTO struct {
	ID                    int64    `json:"id"`
	Path                  string   `json:"path"`
	LibraryType           string   `json:"library_type"`
	SizeBytes             int64    `json:"size_bytes"`
	Mtime                 int64    `json:"mtime"`
	VideoCodec            *string  `json:"video_codec"`
	VideoCodecProfile     *string  `json:"video_codec_profile"`
	Width                 *int     `json:"width"`
	Height                *int     `json:"height"`
	DurationSeconds       *float64 `json:"duration_seconds"`
	BitrateKbps           *int     `json:"bitrate_kbps"`
	AudioCodec            *string  `json:"audio_codec"`
	AudioChannels         *int     `json:"audio_channels"`
	ContainerFormat       *string  `json:"container_format"`
	IsAlreadyHEVC         bool     `json:"is_already_hevc"`
	PredictedSavingsBytes int64    `json:"predicted_savings_bytes"`
	LastProbedAt          *int64   `json:"last_probed_at"`
	ProbeError            *string  `json:"probe_error"`
	Status                string   `json:"status"`
	CandidateState        string   `json:"candidate_state"`
	PosterPath            *string  `json:"poster_path"`
	BackdropPath          *string  `json:"backdrop_path"`
	// TMDB metadata fields — only populated by handleFileDetail
	Overview    *string  `json:"overview,omitempty"`
	Tagline     *string  `json:"tagline,omitempty"`
	Genres      []string `json:"genres,omitempty"`
	VoteAverage *float64 `json:"vote_average,omitempty"`
	VoteCount   *int64   `json:"vote_count,omitempty"`
	ReleaseYear *int     `json:"release_year,omitempty"`
	RuntimeMins *int     `json:"runtime_mins,omitempty"`
}

func toMediaFileDTO(f *store.MediaFile) mediaFileDTO {
	return toMediaFileDTOWithState(f, "")
}

func toMediaFileDTOWithState(f *store.MediaFile, candidateState string) mediaFileDTO {
	return mediaFileDTO{
		ID:                    f.ID,
		Path:                  f.Path,
		LibraryType:           f.LibraryType,
		SizeBytes:             f.SizeBytes,
		Mtime:                 f.Mtime,
		VideoCodec:            f.VideoCodec,
		VideoCodecProfile:     f.VideoCodecProfile,
		Width:                 f.Width,
		Height:                f.Height,
		DurationSeconds:       f.DurationSeconds,
		BitrateKbps:           f.BitrateKbps,
		AudioCodec:            f.AudioCodec,
		AudioChannels:         f.AudioChannels,
		ContainerFormat:       f.ContainerFormat,
		IsAlreadyHEVC:         f.IsAlreadyHEVC,
		PredictedSavingsBytes: f.PredictedSavingsBytes,
		LastProbedAt:          f.LastProbedAt,
		ProbeError:            f.ProbeError,
		Status:                f.Status,
		CandidateState:        candidateState,
	}
}

type profileDTO struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	CRF       int     `json:"crf"`
	Preset    string  `json:"preset"`
	ExtraArgs *string `json:"extra_args"`
	IsDefault bool    `json:"is_default"`
}

func toProfileDTO(p *store.TranscodeProfile) profileDTO {
	return profileDTO{
		ID:        p.ID,
		Name:      p.Name,
		CRF:       p.CRF,
		Preset:    p.Preset,
		ExtraArgs: p.ExtraArgs,
		IsDefault: p.IsDefault,
	}
}

type jobDTO struct {
	ID                 int64   `json:"id"`
	MediaFileID        int64   `json:"media_file_id"`
	ProfileID          int64   `json:"profile_id"`
	Status             string  `json:"status"`
	QueuedAt           int64   `json:"queued_at"`
	StartedAt          *int64  `json:"started_at"`
	CompletedAt        *int64  `json:"completed_at"`
	OriginalSizeBytes  int64   `json:"original_size_bytes"`
	OutputSizeBytes    *int64  `json:"output_size_bytes"`
	ProgressPercent    float64 `json:"progress_percent"`
	OutputPath         *string `json:"output_path"`
	ErrorMessage       *string `json:"error_message"`
	VerificationResult *string `json:"verification_result"`
	// SourcePath is the original media file path, used by the UI to show the
	// file name. Nil if the media row was deleted after the job ran.
	SourcePath *string `json:"source_path"`
	// QueuePosition is 1-based for queued jobs, 0 otherwise.
	QueuePosition int  `json:"queue_position"`
	Forced        bool `json:"forced"`
	// Snapshot encode settings at queue time.
	EncodePreset    *string `json:"encode_preset"`
	EncodeCRF       *int    `json:"encode_crf"`
	EncodeExtraArgs *string `json:"encode_extra_args"`
	// EstimatedDurationSeconds is populated for queued/running jobs.
	EstimatedDurationSeconds *int64 `json:"estimated_duration_seconds,omitempty"`
	// EncodeDurationSeconds is actual wall-clock time for completed jobs.
	EncodeDurationSeconds *int64 `json:"encode_duration_seconds,omitempty"`
	EstimateSource        string `json:"estimate_source,omitempty"`
	EstimateSampleCount   *int   `json:"estimate_sample_count,omitempty"`
}

func toJobDTO(j *store.TranscodeJob, position int, lookup *media.EncodeRateLookup) jobDTO {
	dto := jobDTO{
		ID:                 j.ID,
		MediaFileID:        j.MediaFileID,
		ProfileID:          j.ProfileID,
		Status:             j.Status,
		QueuedAt:           j.QueuedAt,
		StartedAt:          j.StartedAt,
		CompletedAt:        j.CompletedAt,
		OriginalSizeBytes:  j.OriginalSizeBytes,
		OutputSizeBytes:    j.OutputSizeBytes,
		ProgressPercent:    j.ProgressPercent,
		OutputPath:         j.OutputPath,
		ErrorMessage:       j.ErrorMessage,
		VerificationResult: j.VerificationResult,
		SourcePath:         j.SourcePath,
		QueuePosition:      position,
		Forced:             j.Forced,
		EncodePreset:       j.EncodePreset,
		EncodeCRF:          j.EncodeCRF,
		EncodeExtraArgs:    j.EncodeExtraArgs,
	}

	preset := ""
	if j.EncodePreset != nil {
		preset = *j.EncodePreset
	}
	crf := 0
	if j.EncodeCRF != nil {
		crf = *j.EncodeCRF
	}

	switch j.Status {
	case "queued", "running":
		rate, source, samples := media.ResolveEncodeRate(j.ProfileID, preset, crf, lookup)
		if est := media.PredictedEncodeSeconds(rate, j.DurationSeconds, j.Width, j.Height); est > 0 {
			dto.EstimatedDurationSeconds = &est
			dto.EstimateSource = string(source)
			if samples > 0 {
				dto.EstimateSampleCount = &samples
			}
		}
	case "completed":
		if j.StartedAt != nil && j.CompletedAt != nil && *j.CompletedAt > *j.StartedAt {
			elapsed := *j.CompletedAt - *j.StartedAt
			dto.EncodeDurationSeconds = &elapsed
		}
	}

	return dto
}
