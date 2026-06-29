package media

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var seasonEpisodeRe = regexp.MustCompile(`(?i)[._ \-]s(\d{1,3})[._ \-]?e(\d{1,4})`)
var seasonDirRe = regexp.MustCompile(`(?i)^season[ ._]*(\d{1,3})$`)

// ParseTVInfo derives (series title, season, episode) from a TV file path.
// season/episode are -1 when they can't be determined. The series title is the
// first path segment under tvRoot; season/episode come from the SxxExx token in
// the filename, falling back to a "Season NN" directory for the season.
func ParseTVInfo(path, tvRoot string) (title string, season, episode int) {
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
		season = atoiOrNeg(mm[1])
		episode = atoiOrNeg(mm[2])
		return title, season, episode
	}
	for _, seg := range segs {
		if dm := seasonDirRe.FindStringSubmatch(seg); dm != nil {
			season = atoiOrNeg(dm[1])
		}
	}
	return title, season, episode
}

func atoiOrNeg(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}
