package api

import (
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"reclaim/internal/store"
)

// seasonEpisodeRe matches the common *arr SxxExx token in a filename, e.g.
// "Harbor.Lights.S02E07.1080p.mkv" → season 2, episode 7.
var seasonEpisodeRe = regexp.MustCompile(`(?i)[._ \-]s(\d{1,3})[._ \-]?e(\d{1,4})`)

// seasonDirRe matches a "Season 02" / "Season.2" directory component.
var seasonDirRe = regexp.MustCompile(`(?i)^season[ ._]*(\d{1,3})$`)

// episodeDTO is a media file plus the parsed season/episode it belongs to.
type episodeDTO struct {
	mediaFileDTO
	Season  int  `json:"season"`
	Episode *int `json:"episode"`
}

type seasonSummary struct {
	Season                int     `json:"season"`
	FileCount             int     `json:"file_count"`
	CandidateCount        int     `json:"candidate_count"`
	TotalBytes            int64   `json:"total_bytes"`
	PredictedSavingsBytes int64   `json:"predicted_savings_bytes"`
	EpisodeIDs            []int64 `json:"episode_ids"`
}

type seriesSummary struct {
	Title                 string          `json:"title"`
	LibraryType           string          `json:"library_type"`
	FileCount             int             `json:"file_count"`
	CandidateCount        int             `json:"candidate_count"`
	SeasonCount           int             `json:"season_count"`
	TotalBytes            int64           `json:"total_bytes"`
	PredictedSavingsBytes int64           `json:"predicted_savings_bytes"`
	Seasons               []seasonSummary `json:"seasons"`
}

func parseCandidateFilter(c *echo.Context) store.CandidateFilter {
	return store.CandidateFilter{
		LibraryType:    c.QueryParam("library_type"),
		VideoCodec:     c.QueryParam("video_codec"),
		Height:      c.QueryParam("height"),
		Search:         c.QueryParam("search"),
	}
}

// handleGroupedCandidates returns TV series/season summaries (no episode rows)
// for the "By series" view. Episode details are loaded on demand via
// handleGroupedSeasonEpisodes; movies use the paginated /api/candidates endpoint.
func (s *Server) handleGroupedCandidates(c *echo.Context) error {
	filter := parseCandidateFilter(c)
	if filter.LibraryType == store.LibraryTypeMovies {
		return c.JSON(http.StatusOK, map[string]any{"series": []seriesSummary{}, "total_count": 0})
	}

	limit, offset, err := parseLimitOffset(c, defaultPageLimit, maxPageLimit)
	if err != nil {
		return err
	}

	tvFilter := filter
	tvFilter.LibraryType = store.LibraryTypeTV
	cacheKey := filterCacheKey(tvFilter)
	var all []seriesSummary
	if cached, ok := s.candCache.getGrouped(cacheKey); ok {
		all = cached
	} else {
		built, err := s.buildTVCandidateSummaries(c.Request().Context(), tvFilter)
		if err != nil {
			return badRequest(c, err.Error())
		}
		all = built
		s.candCache.putGrouped(cacheKey, all)
	}

	resp := map[string]any{
		"series":      slicePage(all, offset, limit),
		"total_count": len(all),
	}
	return c.JSON(http.StatusOK, resp)
}

// handleGroupedSeasonEpisodes returns the episode rows for one TV series season.
func (s *Server) handleGroupedSeasonEpisodes(c *echo.Context) error {
	series := strings.TrimSpace(c.QueryParam("series"))
	if series == "" {
		return badRequest(c, "series is required")
	}
	seasonStr := c.QueryParam("season")
	season, err := strconv.Atoi(seasonStr)
	if err != nil {
		return badRequest(c, "season must be an integer")
	}

	filter := parseCandidateFilter(c)
	filter.LibraryType = store.LibraryTypeTV

	limit, offset, err := parseLimitOffset(c, defaultPageLimit, maxPageLimit)
	if err != nil {
		return err
	}

	prefix := filepath.Join(s.tvPath, series)
	if s.tvPath != "" && !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	files, err := s.store.Media.CandidatesUnderPathPrefix(c.Request().Context(), filter, prefix, limit, offset)
	if err != nil {
		return badRequest(c, err.Error())
	}

	episodes := s.buildSeasonEpisodes(files, series, season)
	resp := map[string]any{"episodes": episodes}
	if offset == 0 {
		if total, err := s.store.Media.CountCandidatesUnderPathPrefix(c.Request().Context(), filter, prefix); err == nil {
			resp["total_count"] = total
		}
	}
	return c.JSON(http.StatusOK, resp)
}

// groupTVSummaries aggregates TV candidates into series → season summaries.
// candidates is expected pre-sorted by predicted savings desc; series ordering
// follows total predicted savings. allFiles supplies per-season file totals
// (active episodes only, search-scoped) so the UI can show coverage.
func (s *Server) groupTVSummaries(candidates []store.MediaFile, allFiles []store.MediaFile) []seriesSummary {
	type seasonKey struct {
		title  string
		season int
	}
	seasonFiles := make(map[seasonKey]int)
	seriesFiles := make(map[string]int)
	for i := range allFiles {
		f := &allFiles[i]
		if f.Status != store.MediaStatusActive {
			continue
		}
		title, season, _ := parseTVInfo(f.Path, s.tvPath)
		seasonFiles[seasonKey{title, season}]++
		seriesFiles[title]++
	}

	type seasonAcc struct {
		season int
		ids    []int64
		bytes  int64
		saving int64
	}
	type seriesAcc struct {
		title   string
		lib     string
		seasons map[int]*seasonAcc
		order   []int
		savings int64
	}

	bySeries := make(map[string]*seriesAcc)
	var seriesOrder []string

	for i := range candidates {
		f := &candidates[i]
		title, season, _ := parseTVInfo(f.Path, s.tvPath)
		acc, ok := bySeries[title]
		if !ok {
			acc = &seriesAcc{title: title, lib: f.LibraryType, seasons: map[int]*seasonAcc{}}
			bySeries[title] = acc
			seriesOrder = append(seriesOrder, title)
		}
		acc.savings += f.PredictedSavingsBytes

		sa, ok := acc.seasons[season]
		if !ok {
			sa = &seasonAcc{season: season}
			acc.seasons[season] = sa
			acc.order = append(acc.order, season)
		}
		sa.ids = append(sa.ids, f.ID)
		sa.bytes += f.SizeBytes
		sa.saving += f.PredictedSavingsBytes
	}

	sort.SliceStable(seriesOrder, func(a, b int) bool {
		return bySeries[seriesOrder[a]].savings > bySeries[seriesOrder[b]].savings
	})

	out := make([]seriesSummary, 0, len(seriesOrder))
	for _, title := range seriesOrder {
		acc := bySeries[title]
		sg := seriesSummary{Title: acc.title, LibraryType: acc.lib}

		sort.Ints(acc.order)
		for _, sn := range acc.order {
			sa := acc.seasons[sn]
			fileCount := seasonFiles[seasonKey{acc.title, sn}]
			if fileCount == 0 {
				fileCount = len(sa.ids)
			}
			sg.Seasons = append(sg.Seasons, seasonSummary{
				Season:                sn,
				FileCount:             fileCount,
				CandidateCount:        len(sa.ids),
				TotalBytes:            sa.bytes,
				PredictedSavingsBytes: sa.saving,
				EpisodeIDs:            sa.ids,
			})
			sg.CandidateCount += len(sa.ids)
			sg.TotalBytes += sa.bytes
			sg.PredictedSavingsBytes += sa.saving
		}
		sg.FileCount = seriesFiles[acc.title]
		if sg.FileCount == 0 {
			sg.FileCount = sg.CandidateCount
		}
		sg.SeasonCount = len(sg.Seasons)
		out = append(out, sg)
	}
	return out
}

func (s *Server) buildSeasonEpisodes(files []store.MediaFile, seriesTitle string, season int) []episodeDTO {
	eps := make([]episodeDTO, 0)
	for i := range files {
		f := &files[i]
		title, sn, episode := parseTVInfo(f.Path, s.tvPath)
		if title != seriesTitle || sn != season {
			continue
		}
		ep := episodeDTO{mediaFileDTO: toMediaFileDTOWithState(f, string(store.CandidateStateCandidate)), Season: season}
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
		return ea.PredictedSavingsBytes > eb.PredictedSavingsBytes
	})
	return eps
}

// groupCandidates partitions candidates into TV series groups and flat movies.
// Kept for tests; production uses groupTVSummaries + paginated movie queries.
func (s *Server) groupCandidates(files []store.MediaFile) ([]seriesSummary, []mediaFileDTO) {
	var tv []store.MediaFile
	movies := make([]mediaFileDTO, 0)
	for i := range files {
		f := &files[i]
		if f.LibraryType != store.LibraryTypeTV {
			movies = append(movies, toMediaFileDTOWithState(f, string(store.CandidateStateCandidate)))
			continue
		}
		tv = append(tv, *f)
	}
	return s.groupTVSummaries(tv, nil), movies
}

// parseTVInfo derives (series title, season, episode) from a TV file path.
// season/episode are -1 when they can't be determined. The series title is the
// first path segment under tvRoot; season/episode come from the SxxExx token in
// the filename, falling back to a "Season NN" directory for the season.
func parseTVInfo(path, tvRoot string) (title string, season, episode int) {
	season, episode = -1, -1

	rel := path
	if tvRoot != "" && strings.HasPrefix(path, tvRoot) {
		rel = strings.TrimPrefix(path, tvRoot)
	}
	rel = strings.TrimPrefix(rel, string(filepath.Separator))
	rel = strings.TrimPrefix(rel, "/")
	segs := strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == filepath.Separator })

	if len(segs) > 0 {
		title = segs[0]
	} else {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	base := filepath.Base(path)
	if mm := seasonEpisodeRe.FindStringSubmatch(base); mm != nil {
		season = atoiSafe(mm[1])
		episode = atoiSafe(mm[2])
		return title, season, episode
	}
	for _, seg := range segs {
		if dm := seasonDirRe.FindStringSubmatch(seg); dm != nil {
			season = atoiSafe(dm[1])
		}
	}
	return title, season, episode
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}
