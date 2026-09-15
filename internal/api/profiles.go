package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"reclaim/internal/media"
	"reclaim/internal/store"
)

type profileRequest struct {
	Name      string  `json:"name"`
	Codec     string  `json:"codec"`
	CRF       int     `json:"crf"`
	Preset    string  `json:"preset"`
	ExtraArgs *string `json:"extra_args"`
	IsDefault bool    `json:"is_default"`
}

// validate checks the request against its target encoder's CRF range and
// preset vocabulary and returns that encoder. An omitted codec is HEVC, which
// is what every client written before AV1 support sends.
func (r profileRequest) validate() (media.Encoder, error) {
	if strings.TrimSpace(r.Name) == "" {
		return media.Encoder{}, errors.New("name must not be empty")
	}
	enc, ok := media.EncoderFor(media.NormalizeTargetCodec(r.Codec))
	if !ok {
		codecs := make([]string, 0, len(media.TargetCodecs()))
		for _, c := range media.TargetCodecs() {
			codecs = append(codecs, string(c))
		}
		return media.Encoder{}, fmt.Errorf("codec must be one of: %s", strings.Join(codecs, ", "))
	}
	if !enc.ValidCRF(r.CRF) {
		return media.Encoder{}, fmt.Errorf("crf must be between %d and %d", enc.CRFMin, enc.CRFMax)
	}
	if strings.TrimSpace(r.Preset) == "" {
		return media.Encoder{}, errors.New("preset must not be empty")
	}
	if !enc.ValidPreset(r.Preset) {
		return media.Encoder{}, fmt.Errorf("preset %q is not a %s preset; use one of: %s",
			r.Preset, enc.FFmpegEncoder, strings.Join(enc.Presets, ", "))
	}
	return enc, nil
}

func (r profileRequest) toProfile(id int64, enc media.Encoder) *store.TranscodeProfile {
	return &store.TranscodeProfile{
		ID:        id,
		Name:      r.Name,
		Codec:     string(enc.Codec),
		CRF:       r.CRF,
		Preset:    strings.ToLower(strings.TrimSpace(r.Preset)),
		ExtraArgs: r.ExtraArgs,
		IsDefault: r.IsDefault,
	}
}

// encoderAvailable reports whether the host ffmpeg can encode to codec.
func (s *Server) encoderAvailable(codec media.TargetCodec) bool {
	if s.encoders == nil {
		return true
	}
	return s.encoders[codec]
}

func encoderUnavailableMsg(enc media.Encoder) string {
	return fmt.Sprintf("this ffmpeg build has no %s encoder, so %s profiles cannot run", enc.FFmpegEncoder, enc.Label)
}

// refreshSavingsModel reprices the library after a profile change: the default
// profile's codec is the target every stored prediction is priced against. A
// failure only leaves predictions stale until the next refresh, so it is
// logged rather than failing the request that already succeeded.
func (s *Server) refreshSavingsModel(ctx context.Context) {
	if _, err := s.store.SavingsModel.Refresh(context.WithoutCancel(ctx)); err != nil {
		slog.Warn("api: savings model refresh", "err", err)
	}
}

func (s *Server) handleListProfiles(c *echo.Context) error {
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

func (s *Server) handleCreateProfile(c *echo.Context) error {
	var req profileRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid JSON body")
	}
	enc, err := req.validate()
	if err != nil {
		return badRequest(c, err.Error())
	}
	if !s.encoderAvailable(enc.Codec) {
		return badRequest(c, encoderUnavailableMsg(enc))
	}
	p := req.toProfile(0, enc)
	id, err := s.store.Profiles.Create(c.Request().Context(), p)
	if err != nil {
		return serverError(c, err)
	}
	p.ID = id
	s.refreshSavingsModel(c.Request().Context())
	return c.JSON(http.StatusCreated, toProfileDTO(p))
}

func (s *Server) handleUpdateProfile(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return badRequest(c, "invalid profile id")
	}
	var req profileRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid JSON body")
	}
	enc, err := req.validate()
	if err != nil {
		return badRequest(c, err.Error())
	}
	if !s.encoderAvailable(enc.Codec) {
		return badRequest(c, encoderUnavailableMsg(enc))
	}
	if _, err := s.store.Profiles.GetByID(c.Request().Context(), id); errors.Is(err, store.ErrNotFound) {
		return c.JSON(http.StatusNotFound, errorBody("profile not found"))
	} else if err != nil {
		return serverError(c, err)
	}
	p := req.toProfile(id, enc)
	if err := s.store.Profiles.Update(c.Request().Context(), p); err != nil {
		return serverError(c, err)
	}
	s.refreshSavingsModel(c.Request().Context())
	return c.JSON(http.StatusOK, toProfileDTO(p))
}

func (s *Server) handleDeleteProfile(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return badRequest(c, "invalid profile id")
	}
	if err := s.store.Profiles.Delete(c.Request().Context(), id); err != nil {
		return serverError(c, err)
	}
	s.refreshSavingsModel(c.Request().Context())
	return c.NoContent(http.StatusNoContent)
}

type encoderDTO struct {
	Codec         string   `json:"codec"`
	Label         string   `json:"label"`
	Encoder       string   `json:"encoder"`
	Available     bool     `json:"available"`
	CRFMin        int      `json:"crf_min"`
	CRFMax        int      `json:"crf_max"`
	DefaultCRF    int      `json:"default_crf"`
	Presets       []string `json:"presets"`
	DefaultPreset string   `json:"default_preset"`
}

// handleListEncoders describes every target codec a profile can encode to,
// whether this host's ffmpeg can run it, and the codec the library's savings
// predictions are currently priced against.
func (s *Server) handleListEncoders(c *echo.Context) error {
	encs := media.Encoders()
	items := make([]encoderDTO, 0, len(encs))
	for _, e := range encs {
		items = append(items, encoderDTO{
			Codec:         string(e.Codec),
			Label:         e.Label,
			Encoder:       e.FFmpegEncoder,
			Available:     s.encoderAvailable(e.Codec),
			CRFMin:        e.CRFMin,
			CRFMax:        e.CRFMax,
			DefaultCRF:    e.DefaultCRF,
			Presets:       e.Presets,
			DefaultPreset: e.DefaultPreset,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"items":                items,
		"savings_target_codec": string(s.store.SavingsModel.Target()),
	})
}
