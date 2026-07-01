package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"

	"reclaim/internal/compatibility"
	"reclaim/internal/store"
)

// compatibilityProfileDTO is the wire shape of a built-in client profile.
type compatibilityProfileDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// compatibilityReasonDTO is the wire shape of one compatibility.Reason.
type compatibilityReasonDTO struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Stream   *int   `json:"stream,omitempty"`
	Message  string `json:"message"`
}

// compatibilityDTO is the wire shape of one file's verdict for one client
// profile — docs/COMPATIBILITY PLAN.md §8 response shape.
type compatibilityDTO struct {
	ClientProfile       string                   `json:"client_profile"`
	DirectPlayPredicted bool                     `json:"direct_play_predicted"`
	RiskScore           int                      `json:"risk_score"`
	Reasons             []compatibilityReasonDTO `json:"reasons"`
	RecommendedAction   string                   `json:"recommended_action"`
}

func toCompatibilityDTO(r *store.CompatibilityRow) compatibilityDTO {
	reasons := make([]compatibilityReasonDTO, 0, len(r.Reasons))
	for _, rr := range r.Reasons {
		reasons = append(reasons, compatibilityReasonDTO{
			Code: rr.Code, Severity: rr.Severity, Stream: rr.Stream, Message: rr.Message,
		})
	}
	return compatibilityDTO{
		ClientProfile:       r.ClientProfile,
		DirectPlayPredicted: r.DirectPlayPredicted,
		RiskScore:           r.RiskScore,
		Reasons:             reasons,
		RecommendedAction:   r.RecommendedAction,
	}
}

// compatibilityItemDTO is one row of GET /api/compatibility: a media file
// plus its verdict for the requested client profile.
type compatibilityItemDTO struct {
	mediaFileDTO
	Compatibility compatibilityDTO `json:"compatibility"`
}

func toCompatibilityItemDTO(r *store.CompatibilityRow) compatibilityItemDTO {
	return compatibilityItemDTO{
		mediaFileDTO:  toMediaFileDTO(&r.MediaFile),
		Compatibility: toCompatibilityDTO(r),
	}
}

// streamDTO is the wire shape of one media_streams row.
type streamDTO struct {
	Index     int     `json:"index"`
	CodecType string  `json:"codec_type"`
	CodecName *string `json:"codec_name"`
	Profile   *string `json:"profile"`
	Channels  *int    `json:"channels"`
	Language  *string `json:"language"`
	IsDefault bool    `json:"is_default"`
}

func toStreamDTO(s *store.MediaStream) streamDTO {
	return streamDTO{
		Index:     s.StreamIndex,
		CodecType: s.CodecType,
		CodecName: s.CodecName,
		Profile:   s.Profile,
		Channels:  s.Channels,
		Language:  s.Language,
		IsDefault: s.DispositionDefault,
	}
}

// handleCompatibilityProfiles lists the built-in client profiles (§8 GET
// /api/compatibility/profiles). Static and cheap — no store access needed.
func (s *Server) handleCompatibilityProfiles(c *echo.Context) error {
	profiles := compatibility.BuiltinProfiles()
	out := make([]compatibilityProfileDTO, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, compatibilityProfileDTO{ID: p.ID, Name: p.Name, Description: p.Description})
	}
	return c.JSON(http.StatusOK, map[string]any{"profiles": out})
}

// resolveClientProfile reads ?client_profile=, falling back to the settings
// default when omitted (§4 "Settings integration"), and validates it's a
// known built-in profile.
func (s *Server) resolveClientProfile(c *echo.Context) (string, error) {
	id := c.QueryParam("client_profile")
	if id == "" {
		def, err := s.store.Settings.DefaultClientProfile(c.Request().Context())
		if err != nil {
			return "", err
		}
		id = def
	}
	if _, ok := compatibility.Profile(id); !ok {
		return "", fmt.Errorf("unknown client_profile %q", id)
	}
	return id, nil
}

// handleCompatibility returns one page of the "Direct play" list for one
// client profile (§8 GET /api/compatibility). Mirrors handleCandidates'
// shape: keyset pagination on the default sort (risk_desc), offset
// pagination otherwise.
func (s *Server) handleCompatibility(c *echo.Context) error {
	profileID, err := s.resolveClientProfile(c)
	if err != nil {
		return badRequest(c, err.Error())
	}

	q := store.CompatibilityQuery{
		Sort: store.CompatibilitySort(defaultStr(c.QueryParam("sort"), string(store.CompatibilitySortRiskDesc))),
		Filter: store.CompatibilityFilter{
			ClientProfile: profileID,
			LibraryType:   c.QueryParam("library_type"),
			VideoCodec:    c.QueryParam("video_codec"),
			Height:        c.QueryParam("height"),
			Search:        c.QueryParam("search"),
			Reason:        c.QueryParam("reason"),
			DirectPlay:    c.QueryParam("direct_play"),
		},
	}

	if v := c.QueryParam("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return badRequest(c, "limit must be a positive integer")
		}
		q.Limit = n
	}
	if v := c.QueryParam("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return badRequest(c, "offset must be a non-negative integer")
		}
		q.Offset = n
	}

	// Keyset cursor (default sort only).
	ar := c.QueryParam("after_risk")
	ai := c.QueryParam("after_id")
	if ar != "" && ai != "" {
		arv, err1 := strconv.Atoi(ar)
		aiv, err2 := strconv.ParseInt(ai, 10, 64)
		if err1 != nil || err2 != nil {
			return badRequest(c, "after_risk and after_id must be integers")
		}
		q.AfterRisk = &arv
		q.AfterID = &aiv
	} else if ar != "" || ai != "" {
		return badRequest(c, "after_risk and after_id must be provided together")
	}

	ctx := c.Request().Context()
	rows, err := s.store.Media.CompatibilityList(ctx, q)
	if err != nil {
		return badRequest(c, err.Error())
	}

	items := make([]compatibilityItemDTO, 0, len(rows))
	for i := range rows {
		items = append(items, toCompatibilityItemDTO(&rows[i]))
	}

	resp := map[string]any{"items": items}
	// Total count on the first page helps the UI show "N files" vs "N+ files".
	if q.Sort == store.CompatibilitySortRiskDesc && q.AfterRisk == nil && q.AfterID == nil && q.Offset == 0 {
		if total, err := s.store.Media.CountCompatibility(ctx, q.Filter); err == nil {
			resp["total_count"] = total
		}
	}
	// Provide the next keyset cursor only when the default sort returned a
	// full page — a partial (or empty) page means we've reached the end.
	if q.Sort == store.CompatibilitySortRiskDesc && q.Limit > 0 && len(rows) == q.Limit {
		last := rows[len(rows)-1]
		resp["next_cursor"] = map[string]any{
			"after_risk": last.RiskScore,
			"after_id":   last.ID,
		}
	}
	return c.JSON(http.StatusOK, resp)
}

// handleCompatibilityStats returns the compatibility overview for one
// client profile (§8 GET /api/compatibility/stats).
func (s *Server) handleCompatibilityStats(c *echo.Context) error {
	profileID, err := s.resolveClientProfile(c)
	if err != nil {
		return badRequest(c, err.Error())
	}

	stats, err := s.store.Media.CompatibilityStats(c.Request().Context(), profileID)
	if err != nil {
		return serverError(c, err)
	}

	byReason := make([]map[string]any, 0, len(stats.ByReason))
	for _, r := range stats.ByReason {
		byReason = append(byReason, map[string]any{"code": r.Code, "file_count": r.FileCount})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"client_profile":        stats.ClientProfile,
		"total_files":           stats.TotalFiles,
		"direct_play_count":     stats.DirectPlayCount,
		"transcode_risk_count":  stats.TranscodeRiskCount,
		"savings_overlap_count": stats.SavingsOverlapCount,
		"by_reason":             byReason,
	})
}
