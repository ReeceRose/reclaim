package api

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"reclaim/internal/compatibility"
	"reclaim/internal/config"
)

// validClientProfiles are the built-in compatibility client profile IDs,
// sourced from internal/compatibility.BuiltinProfiles() so there's a single
// source of truth for what's selectable.
var validClientProfiles = func() map[string]bool {
	m := make(map[string]bool)
	for _, p := range compatibility.BuiltinProfiles() {
		m[p.ID] = true
	}
	return m
}()

func (s *Server) handleGetSettings(c *echo.Context) error {
	defaultProfile, err := s.store.Settings.DefaultClientProfile(c.Request().Context())
	if err != nil {
		return serverError(c, err)
	}

	resp := map[string]any{
		"encode_window_start":    config.FormatHHMM(s.live.EncodeWindowStart()),
		"encode_window_end":      config.FormatHHMM(s.live.EncodeWindowEnd()),
		"scan_interval":          s.live.ScanInterval().String(),
		"scan_anchor":            s.live.ScanAnchor(),
		"probe_concurrency":      s.live.ProbeConcurrency(),
		"movies_path":            s.moviesPath,
		"tv_path":                s.tvPath,
		"tmdb_configured":        s.tmdbKey != "",
		"default_client_profile": defaultProfile,
	}
	return c.JSON(http.StatusOK, resp)
}

type settingsRequest struct {
	EncodeWindowStart    *string `json:"encode_window_start"`
	EncodeWindowEnd      *string `json:"encode_window_end"`
	ScanInterval         *string `json:"scan_interval"`
	ScanAnchor           *string `json:"scan_anchor"`
	ProbeConcurrency     *int    `json:"probe_concurrency"`
	DefaultClientProfile *string `json:"default_client_profile"`
}

func (s *Server) handlePutSettings(c *echo.Context) error {
	var req settingsRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid JSON body")
	}
	if err := s.live.Update(req.EncodeWindowStart, req.EncodeWindowEnd, req.ScanInterval, req.ScanAnchor, req.ProbeConcurrency); err != nil {
		return badRequest(c, err.Error())
	}
	if req.DefaultClientProfile != nil {
		if !validClientProfiles[*req.DefaultClientProfile] {
			return badRequest(c, "unknown client_profile")
		}
		if err := s.store.Settings.SetDefaultClientProfile(c.Request().Context(), *req.DefaultClientProfile); err != nil {
			return serverError(c, err)
		}
	}
	return s.handleGetSettings(c)
}
