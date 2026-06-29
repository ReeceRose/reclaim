package api

import (
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"reclaim/internal/media"
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
	PosterPath            *string                `json:"poster_path"`
	BackdropPath          *string                `json:"backdrop_path"`
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

	ctx := c.Request().Context()
	files, err := s.store.Media.Files(ctx, q)
	if err != nil {
		return badRequest(c, err.Error())
	}
	states, err := s.store.Media.CandidateStates(ctx, files)
	if err != nil {
		return serverError(c, err)
	}

	items := make([]mediaFileDTO, 0, len(files))
	for i := range files {
		items = append(items, toMediaFileDTOWithState(&files[i], string(states[files[i].ID])))
	}

	if q.Filter.LibraryType == store.LibraryTypeMovies && s.store.Metadata != nil {
		keys := make([]string, len(files))
		for i := range files {
			keys[i] = media.MovieKey(files[i].Path, s.moviesPath)
		}
		if metaMap, err := s.store.Metadata.GetBatch(ctx, keys); err == nil {
			for i := range items {
				if m, ok := metaMap[keys[i]]; ok {
					items[i].PosterPath = m.PosterPath
					items[i].BackdropPath = m.BackdropPath
				}
			}
		}
	}

	resp := map[string]any{"items": items}
	if q.Offset == 0 {
		if total, err := s.store.Media.CountFiles(ctx, q.Filter); err == nil {
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

	ctx := c.Request().Context()
	rows, err := s.store.Media.TVSeriesGroups(ctx, filter.Search, limit, offset)
	if err != nil {
		return serverError(c, err)
	}
	total, err := s.store.Media.CountTVSeries(ctx, filter.Search)
	if err != nil {
		return serverError(c, err)
	}

	series := make([]librarySeriesSummary, 0, len(rows))
	for _, r := range rows {
		series = append(series, librarySeriesSummary{
			Title:                 r.Title,
			LibraryType:           store.LibraryTypeTV,
			FileCount:             r.FileCount,
			EligibleCount:         r.EligibleCount,
			SeasonCount:           r.SeasonCount,
			TotalBytes:            r.TotalBytes,
			PredictedSavingsBytes: r.PredictedSavingsBytes,
		})
	}

	if s.store.Metadata != nil && len(series) > 0 {
		titles := make([]string, len(series))
		for i := range series {
			titles[i] = series[i].Title
		}
		if metaMap, err := s.store.Metadata.GetBatch(ctx, titles); err == nil {
			for i := range series {
				if m, ok := metaMap[series[i].Title]; ok {
					series[i].PosterPath = m.PosterPath
					series[i].BackdropPath = m.BackdropPath
				}
			}
		}
	}


	return c.JSON(http.StatusOK, map[string]any{
		"series":      series,
		"total_count": total,
	})
}

func (s *Server) handleGroupedFileSeasons(c *echo.Context) error {
	series := strings.TrimSpace(c.QueryParam("series"))
	if series == "" {
		return badRequest(c, "series is required")
	}
	seasons, err := s.store.Media.TVShowSeasons(c.Request().Context(), series)
	if err != nil {
		return serverError(c, err)
	}
	type seasonDTO struct {
		Season                int   `json:"season"`
		FileCount             int   `json:"file_count"`
		EligibleCount         int   `json:"eligible_count"`
		TotalBytes            int64 `json:"total_bytes"`
		PredictedSavingsBytes int64 `json:"predicted_savings_bytes"`
	}
	out := make([]seasonDTO, 0, len(seasons))
	for _, s := range seasons {
		out = append(out, seasonDTO{
			Season:                s.Season,
			FileCount:             s.FileCount,
			EligibleCount:         s.EligibleCount,
			TotalBytes:            s.TotalBytes,
			PredictedSavingsBytes: s.PredictedSavingsBytes,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"seasons": out})
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


func (s *Server) buildLibrarySeasonEpisodes(files []store.MediaFile, states map[int64]store.CandidateState, seriesTitle string, season int) []episodeDTO {
	eps := make([]episodeDTO, 0)
	for i := range files {
		f := &files[i]
		title, sn, episode := media.ParseTVInfo(f.Path, s.tvPath)
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
