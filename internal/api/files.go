package api

import (
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"reclaim/internal/store"
)

type librarySeasonSummary struct {
	Season                int     `json:"season"`
	FileCount             int     `json:"file_count"`
	EligibleCount         int     `json:"eligible_count"`
	TotalBytes            int64   `json:"total_bytes"`
	PredictedSavingsBytes int64   `json:"predicted_savings_bytes"`
	EpisodeIDs            []int64 `json:"episode_ids"`
	EligibleIDs           []int64 `json:"eligible_ids"`
}

type librarySeriesSummary struct {
	Title                 string                 `json:"title"`
	LibraryType           string                 `json:"library_type"`
	FileCount             int                    `json:"file_count"`
	EligibleCount         int                    `json:"eligible_count"`
	SeasonCount           int                    `json:"season_count"`
	TotalBytes            int64                  `json:"total_bytes"`
	PredictedSavingsBytes int64                  `json:"predicted_savings_bytes"`
	Seasons               []librarySeasonSummary `json:"seasons"`
}

func parseFileFilter(c *echo.Context) store.FileFilter {
	return store.FileFilter{
		LibraryType:    c.QueryParam("library_type"),
		VideoCodec:     c.QueryParam("video_codec"),
		Height:      c.QueryParam("height"),
		Search:         c.QueryParam("search"),
		Status:         c.QueryParam("status"),
		CandidateState: c.QueryParam("candidate_state"),
	}
}

func (s *Server) handleFiles(c *echo.Context) error {
	q := store.FileQuery{
		Sort:   store.FileSort(defaultStr(c.QueryParam("sort"), string(store.SortPathAsc))),
		Filter: parseFileFilter(c),
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

	files, err := s.store.Media.Files(c.Request().Context(), q)
	if err != nil {
		return badRequest(c, err.Error())
	}
	states, err := s.store.Media.CandidateStates(c.Request().Context(), files)
	if err != nil {
		return serverError(c, err)
	}

	items := make([]mediaFileDTO, 0, len(files))
	for i := range files {
		items = append(items, toMediaFileDTOWithState(&files[i], string(states[files[i].ID])))
	}

	resp := map[string]any{"items": items}
	if q.Offset == 0 {
		if total, err := s.store.Media.CountFiles(c.Request().Context(), q.Filter); err == nil {
			resp["total_count"] = total
		}
	}
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) handleGroupedFiles(c *echo.Context) error {
	filter := parseFileFilter(c)
	if filter.LibraryType == store.LibraryTypeMovies {
		return c.JSON(http.StatusOK, map[string]any{"series": []librarySeriesSummary{}, "total_count": 0})
	}

	limit, offset, err := parseLimitOffset(c, defaultPageLimit, maxPageLimit)
	if err != nil {
		return err
	}

	tvFilter := filter
	tvFilter.LibraryType = store.LibraryTypeTV
	acc := newLibraryTVAccumulator()
	if err := s.scanTVLibraryFiles(c.Request().Context(), tvFilter, func(files []store.MediaFile, states map[int64]store.CandidateState) error {
		acc.add(files, states, s.tvPath)
		return nil
	}); err != nil {
		return serverError(c, err)
	}

	all := acc.summaries()
	resp := map[string]any{
		"series":      slicePage(all, offset, limit),
		"total_count": len(all),
	}
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) handleGroupedFileEpisodes(c *echo.Context) error {
	series := strings.TrimSpace(c.QueryParam("series"))
	if series == "" {
		return badRequest(c, "series is required")
	}
	seasonStr := c.QueryParam("season")
	season, err := strconv.Atoi(seasonStr)
	if err != nil {
		return badRequest(c, "season must be an integer")
	}

	filter := parseFileFilter(c)
	filter.LibraryType = store.LibraryTypeTV

	limit, offset, err := parseLimitOffset(c, defaultPageLimit, maxPageLimit)
	if err != nil {
		return err
	}

	prefix := filepath.Join(s.tvPath, series)
	if s.tvPath != "" && !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	files, err := s.store.Media.FilesUnderPathPrefix(c.Request().Context(), store.PathPrefixQuery{
		Filter: filter,
		Prefix: prefix,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return badRequest(c, err.Error())
	}
	states, err := s.store.Media.CandidateStates(c.Request().Context(), files)
	if err != nil {
		return serverError(c, err)
	}

	resp := map[string]any{"episodes": s.buildLibrarySeasonEpisodes(files, states, series, season)}
	if offset == 0 {
		if total, err := s.store.Media.CountFilesUnderPathPrefix(c.Request().Context(), filter, prefix); err == nil {
			resp["total_count"] = total
		}
	}
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) groupLibraryTVSummaries(files []store.MediaFile, states map[int64]store.CandidateState) []librarySeriesSummary {
	type seasonAcc struct {
		season      int
		ids         []int64
		eligibleIDs []int64
		bytes       int64
		saving      int64
		eligible    int
	}
	type seriesAcc struct {
		title   string
		lib     string
		seasons map[int]*seasonAcc
		order   []int
		bytes   int64
	}

	bySeries := make(map[string]*seriesAcc)
	var seriesOrder []string

	for i := range files {
		f := &files[i]
		title, season, _ := parseTVInfo(f.Path, s.tvPath)
		acc, ok := bySeries[title]
		if !ok {
			acc = &seriesAcc{title: title, lib: f.LibraryType, seasons: map[int]*seasonAcc{}}
			bySeries[title] = acc
			seriesOrder = append(seriesOrder, title)
		}
		acc.bytes += f.SizeBytes

		sa, ok := acc.seasons[season]
		if !ok {
			sa = &seasonAcc{season: season}
			acc.seasons[season] = sa
			acc.order = append(acc.order, season)
		}
		sa.ids = append(sa.ids, f.ID)
		sa.bytes += f.SizeBytes
		if states[f.ID] == store.CandidateStateCandidate {
			sa.eligible++
			sa.eligibleIDs = append(sa.eligibleIDs, f.ID)
			sa.saving += f.PredictedSavingsBytes
		}
	}

	sortByTitle := func(a, b int) bool { return seriesOrder[a] < seriesOrder[b] }
	sort.SliceStable(seriesOrder, sortByTitle)

	out := make([]librarySeriesSummary, 0, len(seriesOrder))
	for _, title := range seriesOrder {
		acc := bySeries[title]
		sg := librarySeriesSummary{Title: acc.title, LibraryType: acc.lib}

		sort.Ints(acc.order)
		for _, sn := range acc.order {
			sa := acc.seasons[sn]
			sg.Seasons = append(sg.Seasons, librarySeasonSummary{
				Season:                sn,
				FileCount:             len(sa.ids),
				EligibleCount:         sa.eligible,
				TotalBytes:            sa.bytes,
				PredictedSavingsBytes: sa.saving,
				EpisodeIDs:            sa.ids,
				EligibleIDs:           sa.eligibleIDs,
			})
			sg.FileCount += len(sa.ids)
			sg.EligibleCount += sa.eligible
			sg.TotalBytes += sa.bytes
			sg.PredictedSavingsBytes += sa.saving
		}
		sg.SeasonCount = len(sg.Seasons)
		out = append(out, sg)
	}
	return out
}

func (s *Server) buildLibrarySeasonEpisodes(files []store.MediaFile, states map[int64]store.CandidateState, seriesTitle string, season int) []episodeDTO {
	eps := make([]episodeDTO, 0)
	for i := range files {
		f := &files[i]
		title, sn, episode := parseTVInfo(f.Path, s.tvPath)
		if title != seriesTitle || sn != season {
			continue
		}
		ep := episodeDTO{mediaFileDTO: toMediaFileDTOWithState(f, string(states[f.ID])), Season: season}
		if episode >= 0 {
			e := episode
			ep.Episode = &e
		}
		eps = append(eps, ep)
	}
	sort.SliceStable(eps, func(a, b int) bool {
		ea, eb := eps[a], eps[b]
		if (ea.Episode == nil) != (eb.Episode == nil) {
			return ea.Episode != nil
		}
		if ea.Episode != nil && eb.Episode != nil && *ea.Episode != *eb.Episode {
			return *ea.Episode < *eb.Episode
		}
		return ea.Path < eb.Path
	})
	return eps
}
