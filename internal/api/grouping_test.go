package api

import (
	"testing"

	"reclaim/internal/store"
)

func TestParseTVInfo(t *testing.T) {
	tvRoot := "/media/tv"
	cases := []struct {
		path        string
		wantTitle   string
		wantSeason  int
		wantEpisode int
	}{
		{"/media/tv/Harbor Lights/Season 02/Harbor.Lights.S02E07.1080p.WEB.mkv", "Harbor Lights", 2, 7},
		{"/media/tv/Lantern Bay/Season 01/Lantern.Bay.s01e10.mkv", "Lantern Bay", 1, 10},
		{"/media/tv/Ridgeline/Season 03/Ridgeline 3x04.mkv", "Ridgeline", 3, -1}, // no SxxExx token; season from dir
		{"/media/tv/Some Show/specials/clip.mkv", "Some Show", -1, -1},
	}
	for _, c := range cases {
		title, season, episode := parseTVInfo(c.path, tvRoot)
		if title != c.wantTitle || season != c.wantSeason || episode != c.wantEpisode {
			t.Errorf("parseTVInfo(%q) = (%q, %d, %d), want (%q, %d, %d)",
				c.path, title, season, episode, c.wantTitle, c.wantSeason, c.wantEpisode)
		}
	}
}

func TestGroupTVSummaries(t *testing.T) {
	s := &Server{tvPath: "/media/tv", moviesPath: "/media/movies"}
	mk := func(id int64, lib, path string, size, savings int64) store.MediaFile {
		return store.MediaFile{
			ID: id, LibraryType: lib, Path: path,
			SizeBytes: size, PredictedSavingsBytes: savings, Status: "active",
		}
	}
	files := []store.MediaFile{
		mk(1, "tv", "/media/tv/Harbor Lights/Season 01/Harbor.Lights.S01E01.mkv", 100, 40),
		mk(2, "tv", "/media/tv/Harbor Lights/Season 01/Harbor.Lights.S01E02.mkv", 100, 40),
		mk(3, "tv", "/media/tv/Harbor Lights/Season 02/Harbor.Lights.S02E01.mkv", 100, 40),
	}
	series := s.groupTVSummaries(files, nil)

	if len(series) != 1 {
		t.Fatalf("want 1 series, got %d", len(series))
	}
	hl := series[0]
	if hl.Title != "Harbor Lights" {
		t.Fatalf("want Harbor Lights, got %q", hl.Title)
	}
	if hl.CandidateCount != 3 {
		t.Errorf("want 3 candidate episodes, got %d", hl.CandidateCount)
	}
	if hl.FileCount != 3 {
		t.Errorf("want 3 files, got %d", hl.FileCount)
	}
	if hl.SeasonCount != 2 {
		t.Errorf("want 2 seasons, got %d", hl.SeasonCount)
	}
	if hl.PredictedSavingsBytes != 120 {
		t.Errorf("want 120 savings, got %d", hl.PredictedSavingsBytes)
	}
	if len(hl.Seasons) != 2 || hl.Seasons[0].Season != 1 || hl.Seasons[0].CandidateCount != 2 {
		t.Errorf("unexpected season layout: %+v", hl.Seasons)
	}
	if hl.Seasons[0].FileCount != 2 {
		t.Errorf("want 2 files in season 1, got %d", hl.Seasons[0].FileCount)
	}
	if len(hl.Seasons[0].EpisodeIDs) != 2 || hl.Seasons[0].EpisodeIDs[0] != 1 {
		t.Errorf("unexpected episode ids: %+v", hl.Seasons[0].EpisodeIDs)
	}
}

func TestGroupTVSummariesFileCoverage(t *testing.T) {
	s := &Server{tvPath: "/media/tv"}
	mk := func(id int64, path string) store.MediaFile {
		return store.MediaFile{
			ID: id, LibraryType: "tv", Path: path,
			SizeBytes: 100, PredictedSavingsBytes: 40, Status: store.MediaStatusActive,
		}
	}
	candidates := []store.MediaFile{
		mk(1, "/media/tv/Harbor Lights/Season 01/Harbor.Lights.S01E01.mkv"),
		mk(2, "/media/tv/Harbor Lights/Season 01/Harbor.Lights.S01E02.mkv"),
	}
	allFiles := append(candidates,
		store.MediaFile{
			ID: 3, LibraryType: "tv", Path: "/media/tv/Harbor Lights/Season 01/Harbor.Lights.S01E03.mkv",
			SizeBytes: 100, Status: store.MediaStatusActive, IsAlreadyHEVC: true,
		},
	)
	series := s.groupTVSummaries(candidates, allFiles)
	if len(series) != 1 {
		t.Fatalf("want 1 series, got %d", len(series))
	}
	se := series[0].Seasons[0]
	if se.FileCount != 3 {
		t.Errorf("want 3 files in season, got %d", se.FileCount)
	}
	if se.CandidateCount != 2 {
		t.Errorf("want 2 candidates in season, got %d", se.CandidateCount)
	}
}

func TestBuildSeasonEpisodes(t *testing.T) {
	s := &Server{tvPath: "/media/tv"}
	mk := func(id int64, path string, savings int64) store.MediaFile {
		return store.MediaFile{
			ID: id, LibraryType: "tv", Path: path,
			SizeBytes: 100, PredictedSavingsBytes: savings, Status: "active",
		}
	}
	files := []store.MediaFile{
		mk(1, "/media/tv/Harbor Lights/Season 01/Harbor.Lights.S01E02.mkv", 40),
		mk(2, "/media/tv/Harbor Lights/Season 01/Harbor.Lights.S01E01.mkv", 50),
		mk(3, "/media/tv/Harbor Lights/Season 02/Harbor.Lights.S02E01.mkv", 30),
	}
	eps := s.buildSeasonEpisodes(files, "Harbor Lights", 1)
	if len(eps) != 2 {
		t.Fatalf("want 2 episodes, got %d", len(eps))
	}
	if eps[0].ID != 2 || eps[0].Episode == nil || *eps[0].Episode != 1 {
		t.Errorf("want ep 1 first, got %+v", eps[0])
	}
}

func TestGroupCandidates(t *testing.T) {
	s := &Server{tvPath: "/media/tv", moviesPath: "/media/movies"}
	mk := func(id int64, lib, path string, size, savings int64) store.MediaFile {
		return store.MediaFile{
			ID: id, LibraryType: lib, Path: path,
			SizeBytes: size, PredictedSavingsBytes: savings, Status: "active",
		}
	}
	files := []store.MediaFile{
		mk(1, "tv", "/media/tv/Harbor Lights/Season 01/Harbor.Lights.S01E01.mkv", 100, 40),
		mk(4, "movie", "/media/movies/Glasshouse (2011)/Glasshouse.2011.mkv", 200, 90),
	}
	series, movies := s.groupCandidates(files)

	if len(movies) != 1 || movies[0].ID != 4 {
		t.Fatalf("want 1 movie (id 4), got %+v", movies)
	}
	if len(series) != 1 || series[0].Title != "Harbor Lights" {
		t.Fatalf("unexpected series: %+v", series)
	}
}
