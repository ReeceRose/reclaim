package media

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var movieYearRe = regexp.MustCompile(`^(.*?)\s*\((\d{4})\)\s*$`)

func ParseMovieInfo(path, moviesRoot string) (title string, year int) {
	name := movieFolderName(path, moviesRoot)
	if m := movieYearRe.FindStringSubmatch(name); m != nil {
		year, _ = strconv.Atoi(m[2])
		title = strings.TrimSpace(m[1])
		return
	}
	title = name
	return
}

func MovieKey(path, moviesRoot string) string {
	return movieFolderName(path, moviesRoot)
}

func movieFolderName(path, moviesRoot string) string {
	rel := path
	if moviesRoot != "" && strings.HasPrefix(path, moviesRoot) {
		rel = strings.TrimPrefix(path, moviesRoot)
	}
	rel = strings.TrimPrefix(rel, string(filepath.Separator))
	rel = strings.TrimPrefix(rel, "/")
	segs := strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == filepath.Separator })
	if len(segs) > 0 {
		return segs[0]
	}
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}
