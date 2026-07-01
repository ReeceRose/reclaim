package api

import (
	"net/http"

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
		"movies_path":         s.moviesPath,
		"tv_path":             s.tvPath,
		"tmdb_configured":     s.tmdbKey != "",
	}
	return c.JSON(http.StatusOK, resp)
}

type settingsRequest struct {
	EncodeWindowStart *string `json:"encode_window_start"`
	EncodeWindowEnd   *string `json:"encode_window_end"`
	ScanInterval      *string `json:"scan_interval"`
	ScanAnchor        *string `json:"scan_anchor"`
	ProbeConcurrency  *int    `json:"probe_concurrency"`
}

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
