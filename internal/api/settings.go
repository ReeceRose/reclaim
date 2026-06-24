package api

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"reclaim/internal/config"
)

// handleGetSettings returns the live runtime knobs plus the read-only mount
// paths (set via env).
func (s *Server) handleGetSettings(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"encode_window_start": config.FormatHHMM(s.live.EncodeWindowStart()),
		"encode_window_end":   config.FormatHHMM(s.live.EncodeWindowEnd()),
		"scan_interval":       s.live.ScanInterval().String(),
		"scan_anchor":         s.live.ScanAnchor(),
		"probe_concurrency":   s.live.ProbeConcurrency(),
		"movies_path":         s.moviesPath,
		"tv_path":             s.tvPath,
	})
}

type settingsRequest struct {
	EncodeWindowStart *string `json:"encode_window_start"`
	EncodeWindowEnd   *string `json:"encode_window_end"`
	ScanInterval      *string `json:"scan_interval"`
	ScanAnchor        *string `json:"scan_anchor"`
	ProbeConcurrency  *int    `json:"probe_concurrency"`
}

// handlePutSettings applies runtime-mutable settings without a restart. The
// scanner and worker read the live holder on each use, so changes take effect
// immediately. Mount paths are read-only and ignored here.
func (s *Server) handlePutSettings(c *echo.Context) error {
	var req settingsRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid JSON body")
	}
	if err := s.live.Update(req.EncodeWindowStart, req.EncodeWindowEnd, req.ScanInterval, req.ScanAnchor, req.ProbeConcurrency); err != nil {
		return badRequest(c, err.Error())
	}
	return s.handleGetSettings(c)
}
