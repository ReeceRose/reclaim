package media

import "testing"

func TestMetadataKey(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		libraryType string
		want        string
	}{
		{"tv uses the series folder", "/tv/Breaking Bad/Season 1/S01E01.mkv", "tv", "Breaking Bad"},
		{"movie uses its folder", "/movies/Inception (2010)/Inception.2010.1080p.mkv", "movies", "Inception (2010)"},
		{"flat movie uses the file name", "/movies/Inception.2010.1080p.mkv", "movies", "Inception.2010.1080p.mkv"},
		{"unknown library type has no key", "/other/file.mkv", "other", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MetadataKey(tt.path, tt.libraryType, "/tv", "/movies"); got != tt.want {
				t.Errorf("MetadataKey(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestParsedReleaseYear(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		libraryType string
		want        string
	}{
		{"folder year", "/movies/Inception (2010)/movie.mkv", "movies", "2010"},
		{"folderless release name", "/movies/Inception.2010.2160p.WEB.x265-GRP.mkv", "movies", "2010"},
		{"title carrying its own year", "/movies/2001 A Space Odyssey (1968)/a.mkv", "movies", "1968"},
		{"yearless folder falls back to the file name", "/movies/Inception/Inception.2010.1080p.mkv", "movies", "2010"},
		{"resolution and codec are not years", "/movies/Inception/Inception.1080p.x264.mkv", "movies", ""},
		{"tv never parses a year", "/tv/Show (2019)/Season 1/S01E01.mkv", "tv", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParsedReleaseYear(tt.path, tt.libraryType, "/movies"); got != tt.want {
				t.Errorf("ParsedReleaseYear(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
