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

type seasonGroup struct {
	Season                int          `json:"season"`
	CandidateCount        int          `json:"candidate_count"`
	TotalBytes            int64        `json:"total_bytes"`
	PredictedSavingsBytes int64        `json:"predicted_savings_bytes"`
	Episodes              []episodeDTO `json:"episodes"`
}

type seriesGroup struct {
	Title                 string        `json:"title"`
	LibraryType           string        `json:"library_type"`
	CandidateCount        int           `json:"candidate_count"`
	SeasonCount           int           `json:"season_count"`
	TotalBytes            int64         `json:"total_bytes"`
	PredictedSavingsBytes int64         `json:"predicted_savings_bytes"`
	Seasons               []seasonGroup `json:"seasons"`
}

// handleGroupedCandidates returns the candidate set grouped by TV series →
// season → episode, with movies listed flat. It backs the "By series" view.
// Aggregation happens server-side over the whole candidate set (filtered, but
// not paginated) so the UI can render counts/savings per series instantly.
func (s *Server) handleGroupedCandidates(c *echo.Context) error {
	filter := store.CandidateFilter{
		LibraryType:    c.QueryParam("library_type"),
		VideoCodec:     c.QueryParam("video_codec"),
		ResolutionBand: c.QueryParam("resolution_band"),
		Search:         c.QueryParam("search"),
	}

	files, err := s.store.Media.AllCandidates(c.Request().Context(), filter)
	if err != nil {
		return badRequest(c, err.Error())
	}

	series, movies := s.groupCandidates(files)
	return c.JSON(http.StatusOK, map[string]any{
		"series": series,
		"movies": movies,
	})
}

// groupCandidates partitions candidates into TV series groups and flat movies.
// files is expected pre-sorted by predicted savings desc; that ordering carries
// through to movies and to episodes-within-season as a savings tiebreak.
func (s *Server) groupCandidates(files []store.MediaFile) ([]seriesGroup, []mediaFileDTO) {
	type seasonAcc struct {
		season int
		eps    []episodeDTO
	}
	type seriesAcc struct {
		title   string
		lib     string
		seasons map[int]*seasonAcc
		order   []int // season numbers in first-seen order
		savings int64
	}

	bySeries := make(map[string]*seriesAcc)
	var seriesOrder []string
	movies := make([]mediaFileDTO, 0)

	for i := range files {
		f := &files[i]
		if f.LibraryType != store.LibraryTypeTV {
			movies = append(movies, toMediaFileDTO(f))
			continue
		}

		title, season, episode := parseTVInfo(f.Path, s.tvPath)
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
		ep := episodeDTO{mediaFileDTO: toMediaFileDTO(f), Season: season}
		if episode >= 0 {
			e := episode
			ep.Episode = &e
		}
		sa.eps = append(sa.eps, ep)
	}

	// Order series by total predicted savings desc.
	sort.SliceStable(seriesOrder, func(a, b int) bool {
		return bySeries[seriesOrder[a]].savings > bySeries[seriesOrder[b]].savings
	})

	out := make([]seriesGroup, 0, len(seriesOrder))
	for _, title := range seriesOrder {
		acc := bySeries[title]
		sg := seriesGroup{Title: acc.title, LibraryType: acc.lib}

		sort.Ints(acc.order)
		for _, sn := range acc.order {
			sa := acc.seasons[sn]
			sort.SliceStable(sa.eps, func(a, b int) bool {
				ea, eb := sa.eps[a], sa.eps[b]
				if (ea.Episode == nil) != (eb.Episode == nil) {
					return ea.Episode != nil // numbered episodes first
				}
				if ea.Episode != nil && eb.Episode != nil && *ea.Episode != *eb.Episode {
					return *ea.Episode < *eb.Episode
				}
				return ea.PredictedSavingsBytes > eb.PredictedSavingsBytes
			})
			season := seasonGroup{Season: sn, Episodes: sa.eps}
			for _, ep := range sa.eps {
				season.CandidateCount++
				season.TotalBytes += ep.SizeBytes
				season.PredictedSavingsBytes += ep.PredictedSavingsBytes
			}
			sg.Seasons = append(sg.Seasons, season)
			sg.CandidateCount += season.CandidateCount
			sg.TotalBytes += season.TotalBytes
			sg.PredictedSavingsBytes += season.PredictedSavingsBytes
		}
		sg.SeasonCount = len(sg.Seasons)
		out = append(out, sg)
	}
	return out, movies
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
