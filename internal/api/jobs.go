package api

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"

	ijobs "reclaim/internal/jobs"
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
		if f.IsAlreadyHEVC {
			skipped = append(skipped, skippedItem{fid, "file is already HEVC"})
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

		jobID, err := s.store.Jobs.Create(ctx, &store.TranscodeJob{
			MediaFileID:       fid,
			ProfileID:         profile.ID,
			Status:            string(ijobs.StatusQueued),
			QueuedAt:          now,
			OriginalSizeBytes: f.SizeBytes,
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

// handleListJobs returns the combined queue + history, optionally filtered by
// status, with a 1-based queue position attached to queued jobs.
func (s *Server) handleListJobs(c *echo.Context) error {
	ctx := c.Request().Context()

	limit, offset, err := parseLimitOffset(c, defaultPageLimit, maxPageLimit)
	if err != nil {
		return err
	}

	statusFilter := c.QueryParam("status")
	jobs, err := s.store.Jobs.ListWithPath(ctx, store.JobListQuery{
		Status: statusFilter,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return serverError(c, err)
	}

	positions, err := s.store.Jobs.QueuedPositions(ctx)
	if err != nil {
		return serverError(c, err)
	}

	out := make([]jobDTO, 0, len(jobs))
	for i := range jobs {
		out = append(out, toJobDTO(&jobs[i], positions[jobs[i].ID]))
	}

	resp := map[string]any{"items": out}
	if offset == 0 {
		if total, err := s.store.Jobs.CountJobs(ctx, statusFilter); err == nil {
			resp["total_count"] = total
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
