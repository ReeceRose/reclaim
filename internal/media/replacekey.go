package media

import (
	"fmt"
	"strings"
	"unicode"
)

// Library type values ReplaceKey understands. Declared here rather than
// imported from store because store depends on this package, not the reverse.
const (
	libraryTV     = "tv"
	libraryMovies = "movies"
)

// ReplaceKey derives a content identity for a media file: what the file *is*,
// independent of which release happens to provide it. Two paths that share a
// key are two copies of the same episode or the same movie, so a vanished row
// and a freshly indexed one that agree on it are a delete-and-redownload pair
// rather than an unrelated loss and arrival.
//
// It returns "" when identity cannot be established — a TV file with no SxxExx
// token, an unrecognised library type. Callers must read the empty string as
// "never matches anything", not as a key: two files that both failed to parse
// are not thereby the same file.
func ReplaceKey(path, libraryType, tvRoot, moviesRoot string) string {
	switch libraryType {
	case libraryTV:
		title, season, episode := ParseTVInfo(path, tvRoot)
		title = normalizeKeyPart(title)
		if title == "" || season < 0 || episode < 0 {
			return ""
		}
		return fmt.Sprintf("tv|%s|s%03d|e%04d", title, season, episode)
	case libraryMovies:
		key := movieIdentity(path, moviesRoot)
		if key == "" {
			return ""
		}
		return "movie|" + key
	}
	return ""
}

// movieIdentity reduces a movie file to title + year whenever a release year is
// present. Everything past the year is release metadata by construction, so
// "Inception (2010)", "Inception (2010) [Bluray-1080p]", and the folderless
// "Inception.2010.2160p.WEB.x265-GRP.mkv" all key on "inception2010" — which is
// what lets a redownload change resolution, source, and release group and still
// be recognised as the same movie.
//
// The last year wins, so a title carrying one of its own keeps it: "2001 A
// Space Odyssey (1968)" truncates at 1968, not at 2001.
//
// Without a year there is nothing dependable to truncate at — a flat library
// naming files "Inception.1080p.BluRay-GRP.mkv" offers no way to tell where the
// title stops and the tags start — so the whole segment is used. Two releases
// of such a movie then simply fail to match, which is the safe direction: a
// guess here would fold two unrelated films into one row.
func movieIdentity(path, moviesRoot string) string {
	seg := MovieKey(path, moviesRoot)
	if end := lastYearEnd(seg); end > 0 {
		return normalizeKeyPart(seg[:end])
	}
	return normalizeKeyPart(seg)
}

// lastYearEnd returns the offset just past the last release year in s, or -1
// when there is none.
//
// A year is a run of exactly four digits in 1900–2099. Requiring the run to be
// exactly four digits long is what keeps "x264" and "h265" out; requiring the
// 19/20 prefix is what keeps "1080p" and "2160p" out. Scanning run by run
// rather than by regex is deliberate: consecutive years ("Blade.Runner.2049.
// 2017.2160p") share the delimiter a pattern would have to consume, so a
// regex sweep silently drops the second one — the real year.
func lastYearEnd(s string) int {
	end := -1
	for i := 0; i < len(s); {
		if !isDigit(s[i]) {
			i++
			continue
		}
		j := i
		for j < len(s) && isDigit(s[j]) {
			j++
		}
		if j-i == 4 && (s[i:i+2] == "19" || s[i:i+2] == "20") {
			end = j
		}
		i = j
	}
	return end
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// normalizeKeyPart reduces a title to letters and digits, lowercased. A
// redownload can rewrite separators and casing ("Breaking Bad" vs
// "Breaking.Bad") without being a different show, while a year kept by
// movieIdentity survives and keeps remakes apart.
func normalizeKeyPart(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
