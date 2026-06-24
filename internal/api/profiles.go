package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"reclaim/internal/store"
)

type profileRequest struct {
	Name      string  `json:"name"`
	CRF       int     `json:"crf"`
	Preset    string  `json:"preset"`
	ExtraArgs *string `json:"extra_args"`
	IsDefault bool    `json:"is_default"`
}

func (r profileRequest) validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("name must not be empty")
	}
	if r.CRF < 0 || r.CRF > 51 {
		return errors.New("crf must be between 0 and 51")
	}
	if strings.TrimSpace(r.Preset) == "" {
		return errors.New("preset must not be empty")
	}
	return nil
}

func (s *Server) handleListProfiles(c echo.Context) error {
	profiles, err := s.store.Profiles.List(c.Request().Context())
	if err != nil {
		return serverError(c, err)
	}
	out := make([]profileDTO, 0, len(profiles))
	for i := range profiles {
		out = append(out, toProfileDTO(&profiles[i]))
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleCreateProfile(c echo.Context) error {
	var req profileRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid JSON body")
	}
	if err := req.validate(); err != nil {
		return badRequest(c, err.Error())
	}
	p := &store.TranscodeProfile{
		Name:      req.Name,
		CRF:       req.CRF,
		Preset:    req.Preset,
		ExtraArgs: req.ExtraArgs,
		IsDefault: req.IsDefault,
	}
	id, err := s.store.Profiles.Create(c.Request().Context(), p)
	if err != nil {
		return serverError(c, err)
	}
	p.ID = id
	return c.JSON(http.StatusCreated, toProfileDTO(p))
}

func (s *Server) handleUpdateProfile(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return badRequest(c, "invalid profile id")
	}
	var req profileRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid JSON body")
	}
	if err := req.validate(); err != nil {
		return badRequest(c, err.Error())
	}
	if _, err := s.store.Profiles.GetByID(c.Request().Context(), id); errors.Is(err, store.ErrNotFound) {
		return c.JSON(http.StatusNotFound, errorBody("profile not found"))
	} else if err != nil {
		return serverError(c, err)
	}
	p := &store.TranscodeProfile{
		ID:        id,
		Name:      req.Name,
		CRF:       req.CRF,
		Preset:    req.Preset,
		ExtraArgs: req.ExtraArgs,
		IsDefault: req.IsDefault,
	}
	if err := s.store.Profiles.Update(c.Request().Context(), p); err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, toProfileDTO(p))
}

func (s *Server) handleDeleteProfile(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return badRequest(c, "invalid profile id")
	}
	if err := s.store.Profiles.Delete(c.Request().Context(), id); err != nil {
		return serverError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
