package media

import "testing"

func TestReplaceKey(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		libraryType string
		want        string
	}{
		{
			name:        "tv episode",
			path:        "/tv/Breaking Bad/Season 1/Breaking.Bad.S01E01.1080p.WEB-DL.x264-GRP.mkv",
			libraryType: "tv",
			want:        "tv|breakingbad|s001|e0001",
		},
		{
			// The whole point: a redownload keeps the show, season, and episode
			// but changes resolution, codec, and release group.
			name:        "same episode from another release group",
			path:        "/tv/Breaking Bad/Season 1/Breaking Bad - S01E01 - 2160p HEVC-OTHR.mkv",
			libraryType: "tv",
			want:        "tv|breakingbad|s001|e0001",
		},
		{
			name:        "tv without an episode token is unidentifiable",
			path:        "/tv/Breaking Bad/Season 1/behind the scenes.mkv",
			libraryType: "tv",
			want:        "",
		},
		{
			name:        "movie folder carries the year",
			path:        "/movies/Inception (2010)/Inception.2010.1080p.mkv",
			libraryType: "movies",
			want:        "movie|inception2010",
		},
		{
			name:        "remake is a different movie",
			path:        "/movies/Inception (2020)/Inception.2020.1080p.mkv",
			libraryType: "movies",
			want:        "movie|inception2020",
		},
		{
			// A folderless movie library is the case the folder-name key used
			// to miss entirely: every release token sat in the key, so two
			// releases of one film never agreed.
			name:        "flat movie library keys on title and year",
			path:        "/movies/Inception.2010.2160p.WEB.x265-GRP.mkv",
			libraryType: "movies",
			want:        "movie|inception2010",
		},
		{
			name:        "flat and foldered layouts agree",
			path:        "/movies/Inception.2010.1080p.BluRay.x264-OTHR.mkv",
			libraryType: "movies",
			want:        "movie|inception2010",
		},
		{
			// Quality tags appended to the folder by a *arr rename must not
			// make it a different movie.
			name:        "quality tags after the year are dropped",
			path:        "/movies/Inception (2010) [Bluray-1080p]/Inception.mkv",
			libraryType: "movies",
			want:        "movie|inception2010",
		},
		{
			// The last year wins, so a title that is itself a year survives.
			name:        "title containing a year keeps it",
			path:        "/movies/2001.A.Space.Odyssey.1968.1080p.mkv",
			libraryType: "movies",
			want:        "movie|2001aspaceodyssey1968",
		},
		{
			// Two adjacent year-shaped tokens: the release year is the second,
			// and it is the one that must terminate the key.
			name:        "numeric title next to the release year",
			path:        "/movies/Blade.Runner.2049.2017.2160p.mkv",
			libraryType: "movies",
			want:        "movie|bladerunner20492017",
		},
		{
			// 2160 and 1080 are year-shaped but out of range; 264 is too short.
			name:        "resolution and codec tokens are not years",
			path:        "/movies/Some Film (1999)/Some.Film.2160p.x264.mkv",
			libraryType: "movies",
			want:        "movie|somefilm1999",
		},
		{
			// No year to truncate at, so the whole folder is the identity —
			// unchanged from before, and still stable for a foldered library.
			name:        "folder without a year falls back to the whole name",
			path:        "/movies/The Godfather/The.Godfather.mkv",
			libraryType: "movies",
			want:        "movie|thegodfather",
		},
		{
			name:        "unknown library type",
			path:        "/other/thing.mkv",
			libraryType: "unknown",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReplaceKey(tt.path, tt.libraryType, "/tv", "/movies")
			if got != tt.want {
				t.Errorf("ReplaceKey(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// Two files that both fail to parse must not thereby be the same file.
func TestReplaceKey_emptyKeysDoNotCollide(t *testing.T) {
	a := ReplaceKey("/tv/Show/Season 1/extra one.mkv", "tv", "/tv", "/movies")
	b := ReplaceKey("/tv/Show/Season 1/extra two.mkv", "tv", "/tv", "/movies")
	if a != "" || b != "" {
		t.Fatalf("expected both unidentifiable, got %q and %q", a, b)
	}
}
