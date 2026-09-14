package media

import (
	"path/filepath"
	"strings"
)

func MetadataKey(path, libraryType, tvRoot, moviesRoot string) string {
	switch libraryType {
	case libraryTV:
		title, _, _ := ParseTVInfo(path, tvRoot)
		return title
	case libraryMovies:
		return MovieKey(path, moviesRoot)
	}
	return ""
}

func ParsedReleaseYear(path, libraryType, moviesRoot string) string {
	if libraryType != libraryMovies {
		return ""
	}
	if y := lastYear(MovieKey(path, moviesRoot)); y != "" {
		return y
	}
	base := filepath.Base(path)
	return lastYear(strings.TrimSuffix(base, filepath.Ext(base)))
}

func lastYear(s string) string {
	end := lastYearEnd(s)
	if end < 0 {
		return ""
	}
	return s[end-4 : end]
}
