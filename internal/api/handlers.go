package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"reclaim/internal/store"
)

func (s *Server) handleStats(c *echo.Context) error {
	ov, err := s.store.Stats.Overview(c.Request().Context())
	if err != nil {
		return serverError(c, err)
	}

	codecs := make([]map[string]any, 0, len(ov.ByCodec))
	for _, cs := range ov.ByCodec {
		codecs = append(codecs, map[string]any{
			"codec":                   cs.Codec,
			"file_count":              cs.FileCount,
			"total_bytes":             cs.TotalBytes,
			"predicted_savings_bytes": cs.PredictedSavingsBytes,
		})
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

	return c.JSON(http.StatusOK, map[string]any{
		"total_files":             ov.TotalFiles,
		"total_bytes":             ov.TotalBytes,
		"total_recoverable_bytes": ov.TotalRecoverableBytes,
		"by_codec":                codecs,
		"by_resolution":           res,
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
	// Provide the next keyset cursor when the default sort filled the page.
	if q.Sort == store.SortSavingsDesc && len(files) > 0 {
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
// is pushed over the WS hub so the UI can show progress without polling. The
// request context is not used for the scan itself (it would cancel on return).
func (s *Server) triggerScan(c *echo.Context, force bool) error {
	if s.scanner == nil {
		return c.JSON(http.StatusServiceUnavailable, errorBody("scanner unavailable"))
	}
	kind := "incremental"
	if force {
		kind = "full"
	}
	s.hub.Broadcast("scan_started", map[string]any{"kind": kind})
	go func() {
		run, err := s.scanner.Scan(context.Background(), "manual", force)
		if err != nil {
			s.hub.Broadcast("scan_failed", map[string]any{"error": err.Error()})
			return
		}
		s.hub.Broadcast("scan_completed", map[string]any{
			"scan_run_id":   run.ID,
			"files_scanned": run.FilesScanned,
			"files_added":   run.FilesAdded,
			"files_updated": run.FilesUpdated,
			"files_moved":   run.FilesMoved,
			"files_removed": run.FilesRemoved,
			"errors":        run.Errors,
		})
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
