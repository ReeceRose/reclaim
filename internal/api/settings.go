package api

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"reclaim/internal/config"
)

func (s *Server) handleGetSettings(c *echo.Context) error {
	ctx := c.Request().Context()
	resp := map[string]any{
		"encode_window_start": config.FormatHHMM(s.live.EncodeWindowStart()),
		"encode_window_end":   config.FormatHHMM(s.live.EncodeWindowEnd()),
		"scan_interval":       s.live.ScanInterval().String(),
		"scan_anchor":         s.live.ScanAnchor(),
		"probe_concurrency":   s.live.ProbeConcurrency(),
		"movies_path":         s.moviesPath,
		"tv_path":             s.tvPath,
		"tmdb_api_key":        nil,
	}
	if s.store != nil {
		if key, err := s.store.Settings.GetTMDBKey(ctx); err == nil {
			resp["tmdb_api_key"] = key
		}
	}
	return c.JSON(http.StatusOK, resp)
}

type settingsRequest struct {
	EncodeWindowStart *string `json:"encode_window_start"`
	EncodeWindowEnd   *string `json:"encode_window_end"`
	ScanInterval      *string `json:"scan_interval"`
	ScanAnchor        *string `json:"scan_anchor"`
	ProbeConcurrency  *int    `json:"probe_concurrency"`
	TMDBKey           *string `json:"tmdb_api_key"`
}

func (s *Server) handlePutSettings(c *echo.Context) error {
	var req settingsRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid JSON body")
	}
	if err := s.live.Update(req.EncodeWindowStart, req.EncodeWindowEnd, req.ScanInterval, req.ScanAnchor, req.ProbeConcurrency); err != nil {
		return badRequest(c, err.Error())
	}
	if req.TMDBKey != nil && s.store != nil {
		if err := s.store.Settings.SetTMDBKey(c.Request().Context(), *req.TMDBKey); err != nil {
			return serverError(c, err)
		}
		if s.metaFetcher != nil {
			s.metaFetcher.Trigger()
		}
	}
	return s.handleGetSettings(c)
}
