package media

import "testing"

func TestParseTVInfo(t *testing.T) {
	tvRoot := "/media/tv"
	cases := []struct {
		path        string
		wantTitle   string
		wantSeason  int
		wantEpisode int
	}{
		{"/media/tv/Harbor Lights/Season 02/Harbor.Lights.S02E07.1080p.WEB.mkv", "Harbor Lights", 2, 7},
		// A bare token is a real naming scheme; the boundary before it may be
		// the start of the filename rather than a separator.
		{"/media/tv/Harbor Lights/Season 02/S02E07.mkv", "Harbor Lights", 2, 7},
		// ...but it still has to be a boundary, not a substring of a word.
		{"/media/tv/Harbor Lights/Season 02/Episodes02e07.mkv", "Harbor Lights", 2, -1},
		{"/media/tv/Lantern Bay/Season 01/Lantern.Bay.s01e10.mkv", "Lantern Bay", 1, 10},
		{"/media/tv/Ridgeline/Season 03/Ridgeline 3x04.mkv", "Ridgeline", 3, -1}, // no SxxExx token; season from dir
		{"/media/tv/Some Show/specials/clip.mkv", "Some Show", -1, -1},
	}
	for _, c := range cases {
		title, season, episode := ParseTVInfo(c.path, tvRoot)
		if title != c.wantTitle || season != c.wantSeason || episode != c.wantEpisode {
			t.Errorf("ParseTVInfo(%q) = (%q, %d, %d), want (%q, %d, %d)",
				c.path, title, season, episode, c.wantTitle, c.wantSeason, c.wantEpisode)
		}
	}
}
