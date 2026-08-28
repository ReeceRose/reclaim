package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"

	"reclaim/internal/store"
)

// savingsSummaryDTO is the realized-savings roll-up: what encoding has actually
// reclaimed, as opposed to the predicted figures in library_stats.
// Pointers map to JSON null.
type savingsSummaryDTO struct {
	FilesEncoded  int64 `json:"files_encoded"`
	OriginalBytes int64 `json:"original_bytes"`
	OutputBytes   int64 `json:"output_bytes"`
	BytesSaved    int64 `json:"bytes_saved"`
	EncodeSeconds int64 `json:"encode_seconds_total"`

	CompressionRatio float64 `json:"compression_ratio"`

	FilesEncoded7d  int64 `json:"files_encoded_7d"`
	BytesSaved7d    int64 `json:"bytes_saved_7d"`
	FilesEncoded30d int64 `json:"files_encoded_30d"`
	BytesSaved30d   int64 `json:"bytes_saved_30d"`

	FirstCompletedAt *int64 `json:"first_completed_at"`
	LastCompletedAt  *int64 `json:"last_completed_at"`

	BestSavedBytes int64  `json:"best_saved_bytes"`
	BestPath       string `json:"best_path"`

	SavingsEstimateRatio   *float64 `json:"savings_estimate_ratio"`
	SavingsEstimateSamples int64    `json:"savings_estimate_samples"`
	DurationEstimateRatio  *float64 `json:"duration_estimate_ratio"`
	DurationEstimateSample int64    `json:"duration_estimate_samples"`

	MeanEncodeSeconds  *float64 `json:"mean_encode_seconds"`
	BytesSavedPerHour  *float64 `json:"bytes_saved_per_encode_hour"`
	ProjectedSeconds   *int64   `json:"projected_remaining_encode_seconds"`
	RemainingCandidate int64    `json:"remaining_candidates"`
}

type savingsBucketDTO struct {
	Key              string  `json:"key"`
	FilesEncoded     int64   `json:"files_encoded"`
	OriginalBytes    int64   `json:"original_bytes"`
	OutputBytes      int64   `json:"output_bytes"`
	BytesSaved       int64   `json:"bytes_saved"`
	CompressionRatio float64 `json:"compression_ratio"`
}

type savingsDayDTO struct {
	Day          string `json:"day"`
	FilesEncoded int64  `json:"files_encoded"`
	BytesSaved   int64  `json:"bytes_saved"`
}

type savingsEntryDTO struct {
	JobID         int64   `json:"job_id"`
	MediaFileID   int64   `json:"media_file_id"`
	Path          string  `json:"path"`
	LibraryType   string  `json:"library_type"`
	SourceCodec   *string `json:"source_codec"`
	Width         *int    `json:"width"`
	Height        *int    `json:"height"`
	OriginalBytes int64   `json:"original_size_bytes"`
	OutputBytes   int64   `json:"output_size_bytes"`
	BytesSaved    int64   `json:"bytes_saved"`
	EncodeSeconds *int64  `json:"encode_seconds"`
	CompletedAt   int64   `json:"completed_at"`
}

func ratio(num, den int64) float64 {
	if den <= 0 {
		return 0
	}
	return float64(num) / float64(den)
}

func ratioPtr(num, den int64) *float64 {
	if den <= 0 {
		return nil
	}
	v := float64(num) / float64(den)
	return &v
}

func toSavingsBucketDTOs(in []store.SavingsBucket) []savingsBucketDTO {
	out := make([]savingsBucketDTO, 0, len(in))
	for _, b := range in {
		out = append(out, savingsBucketDTO{
			Key:              b.Key,
			FilesEncoded:     b.FilesEncoded,
			OriginalBytes:    b.OriginalBytes,
			OutputBytes:      b.OutputBytes,
			BytesSaved:       b.BytesSaved,
			CompressionRatio: ratio(b.OutputBytes, b.OriginalBytes),
		})
	}
	return out
}

func toSavingsEntryDTOs(in []store.SavingsEntry) []savingsEntryDTO {
	out := make([]savingsEntryDTO, 0, len(in))
	for _, e := range in {
		out = append(out, savingsEntryDTO{
			JobID:         e.JobID,
			MediaFileID:   e.MediaFileID,
			Path:          e.Path,
			LibraryType:   e.LibraryType,
			SourceCodec:   e.SourceCodec,
			Width:         e.Width,
			Height:        e.Height,
			OriginalBytes: e.OriginalBytes,
			OutputBytes:   e.OutputBytes,
			BytesSaved:    e.BytesSaved,
			EncodeSeconds: e.EncodeSeconds,
			CompletedAt:   e.CompletedAt,
		})
	}
	return out
}

// toSavingsSummaryDTO folds the ledger roll-up together with the outstanding
// candidate count so the projection fields can be derived.
func toSavingsSummaryDTO(s *store.SavingsSummary, remainingCandidates int64) savingsSummaryDTO {
	dto := savingsSummaryDTO{
		FilesEncoded:           s.FilesEncoded,
		OriginalBytes:          s.OriginalBytes,
		OutputBytes:            s.OutputBytes,
		BytesSaved:             s.BytesSaved,
		EncodeSeconds:          s.EncodeSeconds,
		CompressionRatio:       ratio(s.OutputBytes, s.OriginalBytes),
		FilesEncoded7d:         s.FilesEncoded7d,
		BytesSaved7d:           s.BytesSaved7d,
		FilesEncoded30d:        s.FilesEncoded30d,
		BytesSaved30d:          s.BytesSaved30d,
		FirstCompletedAt:       s.FirstCompletedAt,
		LastCompletedAt:        s.LastCompletedAt,
		BestSavedBytes:         s.BestSavedBytes,
		BestPath:               s.BestPath,
		SavingsEstimateRatio:   ratioPtr(s.PredictedActualSum, s.PredictedSavingsSum),
		SavingsEstimateSamples: s.PredictedSamples,
		DurationEstimateRatio:  ratioPtr(s.ActualDurationSum, s.EstimatedDurationSum),
		DurationEstimateSample: s.DurationSamples,
		RemainingCandidate:     remainingCandidates,
	}

	if s.FilesEncoded > 0 && s.EncodeSeconds > 0 {
		mean := float64(s.EncodeSeconds) / float64(s.FilesEncoded)
		dto.MeanEncodeSeconds = &mean

		perHour := float64(s.BytesSaved) / (float64(s.EncodeSeconds) / 3600)
		dto.BytesSavedPerHour = &perHour

		if remainingCandidates > 0 {
			projected := int64(mean * float64(remainingCandidates))
			dto.ProjectedSeconds = &projected
		}
	}
	return dto
}

// handleSavings returns the full realized-savings report: roll-up, breakdowns,
// a daily time series, and the largest and most recent wins.
func (s *Server) handleSavings(c *echo.Context) error {
	ctx := c.Request().Context()

	days := 90
	if raw := c.QueryParam("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 || n > 3650 {
			return badRequest(c, "days must be between 1 and 3650")
		}
		days = n
	}

	now := time.Now()
	summary, err := s.store.Savings.Summary(ctx, now.Unix())
	if err != nil {
		return serverError(c, err)
	}

	ov, err := s.store.Stats.Overview(ctx)
	if err != nil {
		return serverError(c, err)
	}
	remaining := remainingCandidates(ov)

	byCodec, err := s.store.Savings.ByCodec(ctx)
	if err != nil {
		return serverError(c, err)
	}
	byLibrary, err := s.store.Savings.ByLibrary(ctx)
	if err != nil {
		return serverError(c, err)
	}
	byResolution, err := s.store.Savings.ByResolution(ctx)
	if err != nil {
		return serverError(c, err)
	}

	loc := s.live.Location()
	_, offset := now.In(loc).Zone()
	since := now.AddDate(0, 0, -days).Unix()
	daily, err := s.store.Savings.Daily(ctx, since, offset)
	if err != nil {
		return serverError(c, err)
	}

	topWins, err := s.store.Savings.TopWins(ctx, 10)
	if err != nil {
		return serverError(c, err)
	}
	recent, err := s.store.Savings.Recent(ctx, 10)
	if err != nil {
		return serverError(c, err)
	}

	outcomes := map[string]int64{}
	for _, st := range []string{"completed", "failed", "cancelled"} {
		n, err := s.store.Jobs.CountJobs(ctx, []string{st})
		if err != nil {
			return serverError(c, err)
		}
		outcomes[st] = n
	}

	return c.JSON(http.StatusOK, map[string]any{
		"summary":       toSavingsSummaryDTO(summary, remaining),
		"by_codec":      toSavingsBucketDTOs(byCodec),
		"by_library":    toSavingsBucketDTOs(byLibrary),
		"by_resolution": toSavingsBucketDTOs(byResolution),
		"daily":         toSavingsDayDTOs(daily),
		"top_wins":      toSavingsEntryDTOs(topWins),
		"recent":        toSavingsEntryDTOs(recent),
		"job_outcomes":  outcomes,
		"days":          days,
	})
}

func toSavingsDayDTOs(in []store.SavingsDay) []savingsDayDTO {
	out := make([]savingsDayDTO, 0, len(in))
	for _, d := range in {
		out = append(out, savingsDayDTO{Day: d.Day, FilesEncoded: d.FilesEncoded, BytesSaved: d.BytesSaved})
	}
	return out
}

// remainingCandidates counts the active files that are not yet HEVC.
func remainingCandidates(ov *store.LibraryStats) int64 {
	var hevc int64
	for _, c := range ov.ByCodec {
		if c.Codec == "hevc" {
			hevc = c.FileCount
			break
		}
	}
	if n := ov.TotalFiles - hevc; n > 0 {
		return n
	}
	return 0
}
