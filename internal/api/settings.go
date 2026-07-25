package api

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"reclaim/internal/config"
)

func (s *Server) handleGetSettings(c *echo.Context) error {
	resp := map[string]any{
		"encode_window_start": config.FormatHHMM(s.live.EncodeWindowStart()),
		"encode_window_end":   config.FormatHHMM(s.live.EncodeWindowEnd()),
		"scan_interval":       s.live.ScanInterval().String(),
		"scan_anchor":         s.live.ScanAnchor(),
		"probe_concurrency":   s.live.ProbeConcurrency(),
		"oversize_threshold":  s.live.OversizeThreshold(),
		"missing_retention":   formatRetention(s.live.MissingRetention()),
		"movies_path":         s.moviesPath,
		"tv_path":             s.tvPath,
		"tmdb_configured":     s.tmdbKey != "",
	}
	if s.store != nil {
		missing, err := s.store.Media.MissingOverview(c.Request().Context())
		if err != nil {
			return serverError(c, err)
		}
		resp["missing_files"] = missing
	}
	return c.JSON(http.StatusOK, resp)
}

// formatRetention renders the retention duration for the API. Zero becomes "0"
// rather than Go's "0s" so the frontend can compare against a plain "0".
func formatRetention(d time.Duration) string {
	if d <= 0 {
		return "0"
	}
	return d.String()
}

type settingsRequest struct {
	EncodeWindowStart *string  `json:"encode_window_start"`
	EncodeWindowEnd   *string  `json:"encode_window_end"`
	ScanInterval      *string  `json:"scan_interval"`
	ScanAnchor        *string  `json:"scan_anchor"`
	ProbeConcurrency  *int     `json:"probe_concurrency"`
	OversizeThreshold *float64 `json:"oversize_threshold"`
	MissingRetention  *string  `json:"missing_retention"`
}

func (s *Server) handlePutSettings(c *echo.Context) error {
	var req settingsRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid JSON body")
	}
	if err := s.live.Update(
		req.EncodeWindowStart, req.EncodeWindowEnd, req.ScanInterval, req.ScanAnchor,
		req.ProbeConcurrency, req.OversizeThreshold, req.MissingRetention,
	); err != nil {
		return badRequest(c, err.Error())
	}
	return s.handleGetSettings(c)
}

// handlePruneMissing hard-deletes soft-deleted media rows on demand, ignoring
// the retention period — the manual counterpart to the post-scan cleanup. Files
// with a live job are left behind, so the deleted count can be lower than the
// count the settings panel showed.
func (s *Server) handlePruneMissing(c *echo.Context) error {
	ctx := c.Request().Context()

	deleted, err := s.store.Media.PruneMissing(ctx, 0)
	if err != nil {
		return serverError(c, err)
	}
	if deleted > 0 {
		s.recordPruneEvent(ctx, deleted, "manual purge")
	}

	remaining, err := s.store.Media.MissingOverview(ctx)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"deleted":       deleted,
		"missing_files": remaining,
	})
}
