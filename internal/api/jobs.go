package api

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"reclaim/internal/store"
)

type createJobsRequest struct {
	FileIDs   []int64 `json:"file_ids"`
	ProfileID *int64  `json:"profile_id"`
}

// handleCreateJobs enqueues one job per eligible file and echoes the resolved
// selection back (§9.1) so the UI's confirm step is honest about exactly what
// was queued and what was skipped.
func (s *Server) handleCreateJobs(c echo.Context) error {
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
		if f.Status != "active" {
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
			Status:            "queued",
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
		"profile": toProfileDTO(profile),
		"queued":  queued,
		"skipped": skipped,
	})
}

// handleListJobs returns the combined queue + history, optionally filtered by
// status, with a 1-based queue position attached to queued jobs.
func (s *Server) handleListJobs(c echo.Context) error {
	ctx := c.Request().Context()
	jobs, err := s.store.Jobs.ListAll(ctx)
	if err != nil {
		return serverError(c, err)
	}

	positions := queuePositions(jobs)

	statusFilter := c.QueryParam("status")
	out := make([]jobDTO, 0, len(jobs))
	for i := range jobs {
		if statusFilter != "" && jobs[i].Status != statusFilter {
			continue
		}
		out = append(out, toJobDTO(&jobs[i], positions[jobs[i].ID]))
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

// queuePositions assigns 1-based positions to queued jobs ordered by queue time.
func queuePositions(jobs []store.TranscodeJob) map[int64]int {
	queued := make([]store.TranscodeJob, 0)
	for _, j := range jobs {
		if j.Status == "queued" {
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

// handleCancelJob cancels a queued/running/verifying job. The worker (P6/P7)
// performs the process kill + temp cleanup for a running job; here we flip the
// state so it stops being pulled / gets reconciled.
func (s *Server) handleCancelJob(c echo.Context) error {
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
	case "queued", "running", "verifying":
		if err := s.store.Jobs.UpdateStatus(ctx, id, "cancelled"); err != nil {
			return serverError(c, err)
		}
		s.hub.Broadcast("job_cancelled", map[string]any{"job_id": id})
		return c.JSON(http.StatusOK, map[string]any{"job_id": id, "status": "cancelled"})
	default:
		return c.JSON(http.StatusConflict, errorBody("job is not cancellable in its current state"))
	}
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
