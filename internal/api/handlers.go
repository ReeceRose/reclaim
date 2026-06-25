package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"reclaim/internal/media"
	"reclaim/internal/scanner"
	"reclaim/internal/store"
)

func (s *Server) handleStats(c *echo.Context) error {
	ctx := c.Request().Context()
	ov, err := s.store.Stats.Overview(ctx)
	if err != nil {
		return serverError(c, err)
	}

	learnedMap, err := s.store.Jobs.LearnedRatios(ctx, store.LearnedRatioMinSamples)
	if err != nil {
		return serverError(c, err)
	}

	codecs := make([]map[string]any, 0, len(ov.ByCodec))
	for _, cs := range ov.ByCodec {
		entry := map[string]any{
			"codec":                   cs.Codec,
			"file_count":              cs.FileCount,
			"total_bytes":             cs.TotalBytes,
			"predicted_savings_bytes": cs.PredictedSavingsBytes,
			"ratio_source":            string(media.RatioSeed),
		}
		if lr, ok := learnedMap[cs.Codec]; ok {
			entry["ratio_source"] = string(media.RatioLearned)
			entry["learned_sample_count"] = lr.SampleCount
		}
		codecs = append(codecs, entry)
	}
	res := make([]map[string]any, 0, len(ov.ByResolution))
	for _, rs := range ov.ByResolution {
		res = append(res, map[string]any{
			"band":                    rs.Band,
			"file_count":              rs.FileCount,
			"total_bytes":             rs.TotalBytes,
			"predicted_savings_bytes": rs.PredictedSavingsBytes,
		})
	}
	libs := make([]map[string]any, 0, len(ov.ByLibrary))
	for _, ls := range ov.ByLibrary {
		libs = append(libs, map[string]any{
			"library_type":            ls.LibraryType,
			"file_count":              ls.FileCount,
			"total_bytes":             ls.TotalBytes,
			"predicted_savings_bytes": ls.PredictedSavingsBytes,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"total_files":             ov.TotalFiles,
		"total_bytes":             ov.TotalBytes,
		"total_recoverable_bytes": ov.TotalRecoverableBytes,
		"by_codec":                codecs,
		"by_resolution":           res,
		"by_library":              libs,
	})
}

// handleCandidates returns one page of ranked candidates. The default sort uses
// keyset pagination on (predicted_savings_bytes, id); other sorts use offset.
func (s *Server) handleCandidates(c *echo.Context) error {
	q := store.CandidateQuery{
		Sort: store.CandidateSort(defaultStr(c.QueryParam("sort"), string(store.SortSavingsDesc))),
		Filter: store.CandidateFilter{
			LibraryType:    c.QueryParam("library_type"),
			VideoCodec:     c.QueryParam("video_codec"),
			ResolutionBand: c.QueryParam("resolution_band"),
			Search:         c.QueryParam("search"),
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
	as := c.QueryParam("after_savings")
	ai := c.QueryParam("after_id")
	if as != "" && ai != "" {
		asv, err1 := strconv.ParseInt(as, 10, 64)
		aiv, err2 := strconv.ParseInt(ai, 10, 64)
		if err1 != nil || err2 != nil {
			return badRequest(c, "after_savings and after_id must be integers")
		}
		q.AfterSavings = &asv
		q.AfterID = &aiv
	} else if as != "" || ai != "" {
		return badRequest(c, "after_savings and after_id must be provided together")
	}

	files, err := s.store.Media.Candidates(c.Request().Context(), q)
	if err != nil {
		return badRequest(c, err.Error())
	}

	items := make([]mediaFileDTO, 0, len(files))
	for i := range files {
		items = append(items, toMediaFileDTO(&files[i]))
	}

	resp := map[string]any{"items": items}
	// Total count on the first page helps the UI show "N files" vs "N+ files".
	if q.Sort == store.SortSavingsDesc && q.AfterSavings == nil && q.AfterID == nil && q.Offset == 0 {
		if total, err := s.store.Media.CountCandidates(c.Request().Context(), q.Filter); err == nil {
			resp["total_count"] = total
		}
	}
	// Provide the next keyset cursor only when the default sort returned a full
	// page. A partial (or empty) page means we've reached the end of the list.
	if q.Sort == store.SortSavingsDesc && q.Limit > 0 && len(files) == q.Limit {
		last := files[len(files)-1]
		resp["next_cursor"] = map[string]any{
			"after_savings": last.PredictedSavingsBytes,
			"after_id":      last.ID,
		}
	}
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) handleFileDetail(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return badRequest(c, "invalid file id")
	}
	f, err := s.store.Media.GetByID(c.Request().Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		return c.JSON(http.StatusNotFound, errorBody("file not found"))
	}
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, toMediaFileDTO(f))
}

func (s *Server) handleScan(c *echo.Context) error {
	return s.triggerScan(c, false)
}

func (s *Server) handleFullScan(c *echo.Context) error {
	return s.triggerScan(c, true)
}

// triggerScan kicks off a scan in the background and returns 202. Scan lifecycle
// is pushed over the WS hub by the scanner itself so startup and manual scans
// share the same path. The request context is not used for the scan itself (it
// would cancel on return).
func (s *Server) triggerScan(c *echo.Context, force bool) error {
	if s.scanner == nil {
		return c.JSON(http.StatusServiceUnavailable, errorBody("scanner unavailable"))
	}
	kind := scanner.ScanKind(force)
	go func() {
		_, _ = s.scanner.Scan(context.Background(), scanner.TriggerManual, force)
	}()
	return c.JSON(http.StatusAccepted, map[string]any{"started": true, "kind": kind})
}

// handleDryRun projects total savings for a set or filter, queuing nothing.
func (s *Server) handleDryRun(c *echo.Context) error {
	ids, err := parseIDList(c.QueryParam("ids"))
	if err != nil {
		return badRequest(c, "ids must be a comma-separated list of integers")
	}
	filter := store.CandidateFilter{
		LibraryType:    c.QueryParam("library_type"),
		VideoCodec:     c.QueryParam("video_codec"),
		ResolutionBand: c.QueryParam("resolution_band"),
		Search:         c.QueryParam("search"),
	}
	res, err := s.store.Media.DryRunSavings(c.Request().Context(), ids, filter)
	if err != nil {
		return badRequest(c, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{
		"file_count":              res.FileCount,
		"total_bytes":             res.TotalBytes,
		"predicted_savings_bytes": res.PredictedSavingsBytes,
	})
}

func parseIDList(v string) ([]int64, error) {
	if strings.TrimSpace(v) == "" {
		return nil, nil
	}
	parts := strings.Split(v, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
