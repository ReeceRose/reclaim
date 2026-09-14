package store

import (
	"context"
	"reflect"
	"testing"
)

func strp(s string) *string { return &s }
func intp(v int) *int       { return &v }

func insertReleaseFile(t *testing.T, s *Store, f *MediaFile) int64 {
	t.Helper()
	if f.Status == "" {
		f.Status = MediaStatusActive
	}
	if f.Fingerprint == "" {
		f.Fingerprint = "fp-" + f.Path
	}
	codec := "h264"
	f.VideoCodec = &codec
	id, err := s.Media.Insert(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestFiles_releaseSort(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.Metadata.Upsert(ctx, &MediaMetadata{Key: "Inception (2010)", MediaType: "movie", ReleaseDate: strp("2010-07-16"), FetchedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.Metadata.Upsert(ctx, &MediaMetadata{Key: "Unmatched (2005)", MediaType: "movie", ReleaseDate: strp("1999-01-01"), FetchedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.Metadata.SetNoMatch(ctx, "Unmatched (2005)", "movie"); err != nil {
		t.Fatal(err)
	}
	if err := s.Metadata.RecordSeasonAirDates(ctx, "Show", []int{1}, []EpisodeAirDate{{Season: 1, Episode: 1, AirDate: "2011-01-01"}}, 1); err != nil {
		t.Fatal(err)
	}

	tmdbMovie := insertReleaseFile(t, s, &MediaFile{Path: "/movies/Inception (2010)/a.mkv", LibraryType: LibraryTypeMovies, MetadataKey: "Inception (2010)", ParsedReleaseDate: strp("2010")})
	parsedOnly := insertReleaseFile(t, s, &MediaFile{Path: "/movies/Parsed (2012)/b.mkv", LibraryType: LibraryTypeMovies, MetadataKey: "Parsed (2012)", ParsedReleaseDate: strp("2012")})
	unknown := insertReleaseFile(t, s, &MediaFile{Path: "/movies/Unknown/c.mkv", LibraryType: LibraryTypeMovies, MetadataKey: "Unknown"})
	noMatch := insertReleaseFile(t, s, &MediaFile{Path: "/movies/Unmatched (2005)/d.mkv", LibraryType: LibraryTypeMovies, MetadataKey: "Unmatched (2005)", ParsedReleaseDate: strp("2005")})
	episode := insertReleaseFile(t, s, &MediaFile{Path: "/tv/Show/Season 1/S01E01.mkv", LibraryType: LibraryTypeTV, SeriesTitle: strp("Show"), SeasonNumber: intp(1), EpisodeNumber: intp(1), MetadataKey: "Show"})
	undated := insertReleaseFile(t, s, &MediaFile{Path: "/tv/Show/Season 1/S01E02.mkv", LibraryType: LibraryTypeTV, SeriesTitle: strp("Show"), SeasonNumber: intp(1), EpisodeNumber: intp(2), MetadataKey: "Show"})

	ids := func(files []MediaFile) []int64 {
		out := make([]int64, len(files))
		for i, f := range files {
			out[i] = f.ID
		}
		return out
	}

	desc, err := s.Media.Files(ctx, FileQuery{Sort: FileSortReleaseDesc})
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{parsedOnly, episode, tmdbMovie, noMatch, unknown, undated}; !reflect.DeepEqual(ids(desc), want) {
		t.Errorf("release_desc = %v, want %v", ids(desc), want)
	}

	asc, err := s.Media.Files(ctx, FileQuery{Sort: FileSortReleaseAsc})
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{noMatch, tmdbMovie, episode, parsedOnly, unknown, undated}; !reflect.DeepEqual(ids(asc), want) {
		t.Errorf("release_asc = %v, want %v", ids(asc), want)
	}

	if got := desc[1].ReleaseDate; got == nil || *got != "2011-01-01" {
		t.Errorf("episode release date = %v, want 2011-01-01", got)
	}

	cands, err := s.Media.Candidates(ctx, CandidateQuery{Sort: SortReleaseDesc})
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 6 || cands[0].ID != parsedOnly {
		t.Errorf("candidates release_desc = %v, want %d first of 6", ids(cands), parsedOnly)
	}
}

func TestPendingSeasonAirDates(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	tmdbID := int64(1396)
	if err := s.Metadata.Upsert(ctx, &MediaMetadata{Key: "Show", MediaType: "tv", TMDBID: &tmdbID, FetchedAt: 1}); err != nil {
		t.Fatal(err)
	}
	for _, ep := range []struct {
		path   string
		season int
	}{
		{"/tv/Show/Season 1/S01E01.mkv", 1},
		{"/tv/Show/Season 2/S02E01.mkv", 2},
		{"/tv/Show/Season 3/S03E01.mkv", 3},
	} {
		insertReleaseFile(t, s, &MediaFile{Path: ep.path, LibraryType: LibraryTypeTV, SeriesTitle: strp("Show"), SeasonNumber: intp(ep.season), MetadataKey: "Show"})
	}
	insertReleaseFile(t, s, &MediaFile{Path: "/tv/Unmatched/Season 1/S01E01.mkv", LibraryType: LibraryTypeTV, SeriesTitle: strp("Unmatched"), SeasonNumber: intp(1), MetadataKey: "Unmatched"})

	if err := s.Metadata.RecordSeasonAirDates(ctx, "Show", []int{1, 2}, []EpisodeAirDate{{Season: 1, Episode: 1, AirDate: "2008-01-20"}}, 1); err != nil {
		t.Fatal(err)
	}

	pending, err := s.Metadata.PendingSeasonAirDates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []PendingSeasons{{SeriesKey: "Show", TMDBID: tmdbID, Seasons: []int{3}}}
	if !reflect.DeepEqual(pending, want) {
		t.Errorf("pending = %+v, want %+v", pending, want)
	}

	seasons, err := s.Media.SeriesSeasons(ctx, "Show")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seasons, []int{1, 2, 3}) {
		t.Errorf("series seasons = %v, want [1 2 3]", seasons)
	}

	if err := s.Metadata.SetNoMatch(ctx, "Show", "tv"); err != nil {
		t.Fatal(err)
	}
	var dates, fetches int
	if err := s.Media.r.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM episode_air_dates), (SELECT COUNT(*) FROM season_air_date_fetches)").Scan(&dates, &fetches); err != nil {
		t.Fatal(err)
	}
	if dates != 0 || fetches != 0 {
		t.Errorf("after no match: %d air dates and %d fetch markers left, want none", dates, fetches)
	}
}

func TestRecordMove_takesReleaseIdentityFromDestination(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	keep := insertReleaseFile(t, s, &MediaFile{Path: "/movies/Old Name/a.mkv", LibraryType: LibraryTypeMovies, MetadataKey: "Old Name", Fingerprint: "same"})
	merge := insertReleaseFile(t, s, &MediaFile{Path: "/movies/Inception (2010)/a.mkv", LibraryType: LibraryTypeMovies, MetadataKey: "Inception (2010)", ParsedReleaseDate: strp("2010"), Fingerprint: "same-dup"})

	if err := s.Media.RecordMove(ctx, keep, merge, "/movies/Inception (2010)/a.mkv", "movie|inception2010"); err != nil {
		t.Fatal(err)
	}
	f, err := s.Media.GetByID(ctx, keep)
	if err != nil {
		t.Fatal(err)
	}
	if f.MetadataKey != "Inception (2010)" || f.ReleaseDate == nil || *f.ReleaseDate != "2010" {
		t.Errorf("moved row metadata_key=%q release_date=%v, want the destination's identity", f.MetadataKey, f.ReleaseDate)
	}
}

func TestBackfillReleaseIdentity(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	movie := insertReleaseFile(t, s, &MediaFile{Path: "/movies/Dune (2021)/Dune.mkv", LibraryType: LibraryTypeMovies})
	episode := insertReleaseFile(t, s, &MediaFile{Path: "/tv/Severance/Season 1/Severance.S01E03.mkv", LibraryType: LibraryTypeTV})
	if _, err := s.Media.w.ExecContext(ctx, "UPDATE media_files SET metadata_key = NULL"); err != nil {
		t.Fatal(err)
	}

	if err := s.Media.BackfillReleaseIdentity(ctx, "/tv", "/movies"); err != nil {
		t.Fatal(err)
	}

	m, err := s.Media.GetByID(ctx, movie)
	if err != nil {
		t.Fatal(err)
	}
	if m.MetadataKey != "Dune (2021)" || m.ParsedReleaseDate == nil || *m.ParsedReleaseDate != "2021" {
		t.Errorf("movie key=%q parsed=%v, want Dune (2021) / 2021", m.MetadataKey, m.ParsedReleaseDate)
	}
	e, err := s.Media.GetByID(ctx, episode)
	if err != nil {
		t.Fatal(err)
	}
	if e.MetadataKey != "Severance" || e.EpisodeNumber == nil || *e.EpisodeNumber != 3 || e.ParsedReleaseDate != nil {
		t.Errorf("episode key=%q episode=%v parsed=%v, want Severance / 3 / nil", e.MetadataKey, e.EpisodeNumber, e.ParsedReleaseDate)
	}
}
