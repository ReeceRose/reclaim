package api

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"reclaim/internal/config"
	ijobs "reclaim/internal/jobs"
	"reclaim/internal/media"
	"reclaim/internal/store"
)

type createJobsRequest struct {
	FileIDs   []int64 `json:"file_ids"`
	ProfileID *int64  `json:"profile_id"`
}

// handleCreateJobs enqueues one job per eligible file and echoes the resolved
// selection back so the UI's confirm step is honest about exactly what was
// queued and what was skipped.
func (s *Server) handleCreateJobs(c *echo.Context) error {
	ctx := c.Request().Context()
	var req createJobsRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid JSON body")
	}
	if len(req.FileIDs) == 0 {
		return badRequest(c, "file_ids must not be empty")
	}

	// Resolve the profile: explicit id, else the default.
	var profile *store.TranscodeProfile
	var err error
	if req.ProfileID != nil {
		profile, err = s.store.Profiles.GetByID(ctx, *req.ProfileID)
		if errors.Is(err, store.ErrNotFound) {
			return badRequest(c, "profile not found")
		}
	} else {
		profile, err = s.store.Profiles.GetDefault(ctx)
		if errors.Is(err, store.ErrNotFound) {
			return badRequest(c, "no default profile configured; specify profile_id")
		}
	}
	if err != nil {
		return serverError(c, err)
	}
	target := media.NormalizeTargetCodec(profile.Codec)
	enc, ok := media.EncoderFor(target)
	if !ok {
		return badRequest(c, fmt.Sprintf("profile %q has an unsupported codec %q", profile.Name, profile.Codec))
	}
	if !s.encoderAvailable(target) {
		return badRequest(c, encoderUnavailableMsg(enc))
	}
	encodeCodec := string(target)

	type queuedItem struct {
		JobID       int64  `json:"job_id"`
		MediaFileID int64  `json:"media_file_id"`
		Path        string `json:"path"`
	}
	type skippedItem struct {
		MediaFileID int64  `json:"media_file_id"`
		Reason      string `json:"reason"`
	}

	queued := make([]queuedItem, 0, len(req.FileIDs))
	skipped := make([]skippedItem, 0)
	now := time.Now().Unix()

	rateLookup, err := s.store.Jobs.LearnedEncodeRates(ctx)
	if err != nil {
		return serverError(c, err)
	}

	for _, fid := range dedupeIDs(req.FileIDs) {
		f, err := s.store.Media.GetByID(ctx, fid)
		if errors.Is(err, store.ErrNotFound) {
			skipped = append(skipped, skippedItem{fid, "file not found"})
			continue
		}
		if err != nil {
			return serverError(c, err)
		}
		if f.Status != store.MediaStatusActive {
			skipped = append(skipped, skippedItem{fid, "file is not active"})
			continue
		}
		if f.IsEfficientCodec {
			skipped = append(skipped, skippedItem{fid, "file is already in an efficient codec"})
			continue
		}
		blocked, err := s.store.Jobs.HasBlockingJob(ctx, fid)
		if err != nil {
			return serverError(c, err)
		}
		if blocked {
			skipped = append(skipped, skippedItem{fid, "file already has an active or completed job"})
			continue
		}

		predictedSavings := s.store.SavingsModel.PredictFor(target, f.VideoCodec, f.IsEfficientCodec, f.SizeBytes)
		rate, _, _ := media.ResolveEncodeRate(profile.ID, target, profile.Preset, profile.CRF, rateLookup)
		var initialEstimate *int64
		if est := media.PredictedEncodeSeconds(rate, f.DurationSeconds, f.Width, f.Height); est > 0 {
			initialEstimate = &est
		}

		jobID, err := s.store.Jobs.Create(ctx, &store.TranscodeJob{
			MediaFileID:                     fid,
			ProfileID:                       profile.ID,
			Status:                          string(ijobs.StatusQueued),
			QueuedAt:                        now,
			OriginalSizeBytes:               f.SizeBytes,
			EncodeCodec:                     &encodeCodec,
			EncodePreset:                    &profile.Preset,
			EncodeCRF:                       &profile.CRF,
			EncodeExtraArgs:                 profile.ExtraArgs,
			PredictedSavingsBytes:           &predictedSavings,
			InitialEstimatedDurationSeconds: initialEstimate,
		})
		if err != nil {
			return serverError(c, err)
		}
		queued = append(queued, queuedItem{JobID: jobID, MediaFileID: fid, Path: f.Path})
	}

	if len(queued) > 0 {
		s.hub.Broadcast("jobs_queued", map[string]any{
			"count":      len(queued),
			"profile_id": profile.ID,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"profile":                  toProfileDTO(profile),
		string(ijobs.StatusQueued): queued,
		"skipped":                  skipped,
	})
}

// parseStatusFilter splits a comma-separated status query param (e.g.
// "completed,failed") into its parts, dropping blanks.
func parseStatusFilter(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// includesStatus reports whether statuses contains target, or is unfiltered
// (which matches everything).
func includesStatus(statuses []string, target string) bool {
	return len(statuses) == 0 || slices.Contains(statuses, target)
}

// handleListJobs returns one page of jobs, optionally filtered by status,
// with a 1-based queue position attached to queued jobs. Pass
// `order=recent` to sort by completion time (for history views) instead of
// the default oldest-queued-first order (for queue views).
func (s *Server) handleListJobs(c *echo.Context) error {
	ctx := c.Request().Context()

	limit, offset, err := parseLimitOffset(c, defaultPageLimit, maxPageLimit)
	if err != nil {
		return err
	}

	statuses := parseStatusFilter(c.QueryParam("status"))
	orderBy := ""
	if c.QueryParam("order") == "recent" {
		orderBy = "recent"
	}

	jobs, err := s.store.Jobs.ListWithPath(ctx, store.JobListQuery{
		Statuses: statuses,
		OrderBy:  orderBy,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return serverError(c, err)
	}

	positions, err := s.store.Jobs.QueuedPositions(ctx)
	if err != nil {
		return serverError(c, err)
	}

	lookup, err := s.store.Jobs.LearnedEncodeRates(ctx)
	if err != nil {
		return serverError(c, err)
	}

	out := make([]jobDTO, 0, len(jobs))
	for i := range jobs {
		out = append(out, toJobDTO(&jobs[i], positions[jobs[i].ID], lookup))
	}

	// The summary blocks describe the whole filtered set rather than this
	// request's page, and are returned on every page: numbered pagination needs
	// total_count to know how many pages there are, and the header totals must
	// not vanish when the user steps off page 1.
	resp := map[string]any{"items": out}
	if total, err := s.store.Jobs.CountJobs(ctx, statuses); err == nil {
		resp["total_count"] = total
	}

	if includesStatus(statuses, string(ijobs.StatusQueued)) {
		queueJobs, err := s.store.Jobs.ListWithPath(ctx, store.JobListQuery{
			Statuses: []string{string(ijobs.StatusQueued), string(ijobs.StatusRunning)},
			NoLimit:  true,
		})
		if err == nil {
			var queueTotalEstimated int64
			var queuedCount, queuedOriginal, queuedPredictedSavings int64
			var runningLeft time.Duration
			var forcedDurations, queuedDurations []time.Duration
			for i := range queueJobs {
				dto := toJobDTO(&queueJobs[i], positions[queueJobs[i].ID], lookup)
				switch queueJobs[i].Status {
				case string(ijobs.StatusQueued):
					queuedCount++
					// Byte totals are queued-only, matching queued_count. The
					// running job's bytes are already partly on disk as the temp
					// output, so counting them as outstanding would overstate.
					queuedOriginal += queueJobs[i].OriginalSizeBytes
					if dto.PredictedSavingsBytes != nil {
						queuedPredictedSavings += *dto.PredictedSavingsBytes
					}
					if dto.EstimatedDurationSeconds != nil {
						queueTotalEstimated += *dto.EstimatedDurationSeconds
						d := time.Duration(*dto.EstimatedDurationSeconds) * time.Second
						if queueJobs[i].Forced {
							forcedDurations = append(forcedDurations, d)
						} else {
							queuedDurations = append(queuedDurations, d)
						}
					}
				case string(ijobs.StatusRunning):
					if dto.EstimatedDurationSeconds != nil {
						remaining := *dto.EstimatedDurationSeconds
						if queueJobs[i].StartedAt != nil {
							elapsed := time.Now().Unix() - *queueJobs[i].StartedAt
							remaining -= elapsed
							if remaining < 0 {
								remaining = 0
							}
						}
						queueTotalEstimated += remaining
						runningLeft += time.Duration(remaining) * time.Second
					}
				}
			}
			if queueTotalEstimated > 0 {
				resp["queue_total_estimated_seconds"] = queueTotalEstimated
				finish := config.ProjectQueueFinish(
					time.Now().In(s.live.Location()),
					s.live.EncodeWindowStart(), s.live.EncodeWindowEnd(),
					runningLeft, forcedDurations, queuedDurations,
				)
				resp["queue_estimated_finish_at"] = finish.Unix()
			}
			if queuedCount > 0 {
				resp["queued_count"] = queuedCount
				resp["queue_total_original_bytes"] = queuedOriginal
				resp["queue_total_predicted_savings_bytes"] = queuedPredictedSavings
			}
		}
	}

	if includesStatus(statuses, string(ijobs.StatusCompleted)) ||
		includesStatus(statuses, string(ijobs.StatusFailed)) {
		if h, err := s.store.Jobs.HistorySummary(ctx, statuses); err == nil {
			resp["history"] = historySummaryDTO{
				CompletedCount:    h.CompletedCount,
				FailedCount:       h.FailedCount,
				CancelledCount:    h.CancelledCount,
				OriginalSizeBytes: h.OriginalSizeBytes,
				OutputSizeBytes:   h.OutputSizeBytes,
				BytesSaved:        h.OriginalSizeBytes - h.OutputSizeBytes,
				EncodeSeconds:     h.EncodeSeconds,
			}
		}
	}
	return c.JSON(http.StatusOK, resp)
}

// queuePositions assigns 1-based positions to queued jobs ordered by queue time.
func queuePositions(jobs []store.TranscodeJob) map[int64]int {
	queued := make([]store.TranscodeJob, 0)
	for _, j := range jobs {
		if j.Status == string(ijobs.StatusQueued) {
			queued = append(queued, j)
		}
	}
	sort.Slice(queued, func(a, b int) bool {
		if queued[a].QueuedAt != queued[b].QueuedAt {
			return queued[a].QueuedAt < queued[b].QueuedAt
		}
		return queued[a].ID < queued[b].ID
	})
	pos := make(map[int64]int, len(queued))
	for i, j := range queued {
		pos[j.ID] = i + 1
	}
	return pos
}

// handleCancelJob cancels a queued/running/verifying job. The worker performs
// the process kill + temp cleanup for a running job; here we flip the state so
// it stops being pulled / gets reconciled.
func (s *Server) handleCancelJob(c *echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return badRequest(c, "invalid job id")
	}
	job, err := s.store.Jobs.GetByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return c.JSON(http.StatusNotFound, errorBody("job not found"))
	}
	if err != nil {
		return serverError(c, err)
	}
	switch job.Status {
	case string(ijobs.StatusRunning), string(ijobs.StatusVerifying):
		// Hand off to the worker: it kills the ffmpeg process group, removes the
		// temp, leaves the original untouched, and flips the state to cancelled.
		// If the worker isn't actually running it (e.g. across a restart before
		// reconcile), fall through to a direct state flip.
		if s.canceller != nil && s.canceller.Cancel(id) {
			return c.JSON(http.StatusOK, map[string]any{"job_id": id, "status": "cancelling"})
		}
		eventID, err := s.store.CancelJob(ctx, id, time.Now().Unix(), `{"job_id":`+strconv.FormatInt(id, 10)+`}`)
		if err != nil {
			return serverError(c, err)
		}
		s.hub.Broadcast("job_cancelled", map[string]any{"job_id": id})
		s.hub.Broadcast("event_created", apiEventPayload(eventID, store.EventJobCancelled, store.SeverityInfo, "Job cancelled", id))
		return c.JSON(http.StatusOK, map[string]any{"job_id": id, "status": "cancelled"})
	case string(ijobs.StatusQueued):
		// A queued job is just dropped. The guarded transition also wins the race
		// against the worker claiming it at the same moment.
		eventID, err := s.store.CancelJob(ctx, id, time.Now().Unix(), `{"job_id":`+strconv.FormatInt(id, 10)+`}`)
		if err != nil {
			return serverError(c, err)
		}
		s.hub.Broadcast("job_cancelled", map[string]any{"job_id": id})
		s.hub.Broadcast("event_created", apiEventPayload(eventID, store.EventJobCancelled, store.SeverityInfo, "Job cancelled", id))
		return c.JSON(http.StatusOK, map[string]any{"job_id": id, "status": "cancelled"})
	default:
		return c.JSON(http.StatusConflict, errorBody("job is not cancellable in its current state"))
	}
}

// handleForceJob marks a queued job as forced so the worker runs it immediately,
// bypassing the encode window.
func (s *Server) handleForceJob(c *echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return badRequest(c, "invalid job id")
	}
	if err := s.store.Jobs.Force(ctx, id); errors.Is(err, store.ErrNotFound) {
		return c.JSON(http.StatusNotFound, errorBody("job not found"))
	} else if errors.Is(err, store.ErrIllegalTransition) {
		return c.JSON(http.StatusConflict, errorBody("job is not in the queued state"))
	} else if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"job_id": id, "forced": true})
}

// handleDeleteJob hides a completed/failed/cancelled job from the history
// list. The row is kept (not deleted) so it still counts toward learned
// compression ratios and the completed-job dedupe guard. Queued/running/
// verifying jobs must be cancelled first.
func (s *Server) handleDeleteJob(c *echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return badRequest(c, "invalid job id")
	}
	if err := s.store.Jobs.Dismiss(ctx, id, time.Now().Unix()); errors.Is(err, store.ErrNotFound) {
		return c.JSON(http.StatusNotFound, errorBody("job not found"))
	} else if errors.Is(err, store.ErrIllegalTransition) {
		return c.JSON(http.StatusConflict, errorBody("job must be cancelled before it can be deleted"))
	} else if err != nil {
		return serverError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func dedupeIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
