package notify

import (
	"strings"
	"testing"

	"reclaim/internal/store"
)

func tvFile(path, series string, season int, savings int64) store.MediaFile {
	return store.MediaFile{
		Path:                  path,
		LibraryType:           store.LibraryTypeTV,
		SizeBytes:             savings * 2,
		PredictedSavingsBytes: savings,
		SeriesTitle:           &series,
		SeasonNumber:          &season,
	}
}

func movieFile(path string, savings int64) store.MediaFile {
	return store.MediaFile{
		Path:                  path,
		LibraryType:           store.LibraryTypeMovies,
		SizeBytes:             savings * 2,
		PredictedSavingsBytes: savings,
	}
}

// The core rule: a notification covers one title. Two shows arriving together
// are two notifications, never one mixed message.
func TestSplit_oneSummaryPerTitle(t *testing.T) {
	files := []store.MediaFile{
		tvFile("/tv/Severance/Season 3/S03E01.mkv", "Severance", 3, 100),
		tvFile("/tv/Severance/Season 3/S03E02.mkv", "Severance", 3, 100),
		tvFile("/tv/The Wire/Season 1/S01E01.mkv", "The Wire", 1, 500),
		movieFile("/movies/Dune (2021)/Dune (2021).mkv", 300),
	}

	got := Split(files)

	if len(got) != 3 {
		t.Fatalf("got %d summaries, want 3 (one per title): %+v", len(got), got)
	}
	// Ranked by savings: The Wire (500), Dune (300), Severance (200).
	want := []string{"The Wire", "Dune (2021)", "Severance"}
	for i, title := range want {
		if got[i].Title != title {
			t.Errorf("summary %d title = %q, want %q", i, got[i].Title, title)
		}
		if got[i].Titles != 1 {
			t.Errorf("summary %d covers %d titles, want 1", i, got[i].Titles)
		}
	}
	if got[2].Count != 2 {
		t.Errorf("Severance count = %d, want both episodes", got[2].Count)
	}
}

// Every season of one show belongs to that show's notification — importing a
// whole series is one message, not one per season.
func TestSplit_keepsSeasonsOfOneShowTogether(t *testing.T) {
	files := []store.MediaFile{
		tvFile("/tv/Severance/Season 1/S01E01.mkv", "Severance", 1, 100),
		tvFile("/tv/Severance/Season 2/S02E01.mkv", "Severance", 2, 100),
		tvFile("/tv/Severance/Season 2/S02E02.mkv", "Severance", 2, 100),
	}

	got := Split(files)

	if len(got) != 1 {
		t.Fatalf("got %d summaries, want 1 for a single show: %+v", len(got), got)
	}
	if got[0].Count != 3 {
		t.Errorf("count = %d, want 3", got[0].Count)
	}
	if len(got[0].Seasons) != 2 {
		t.Fatalf("seasons = %+v, want 2", got[0].Seasons)
	}
	// Ascending, with their own counts.
	if got[0].Seasons[0].Number != 1 || got[0].Seasons[1].Number != 2 {
		t.Errorf("seasons = %+v, want 1 then 2", got[0].Seasons)
	}
	if got[0].Seasons[1].Count != 2 {
		t.Errorf("season 2 count = %d, want 2", got[0].Seasons[1].Count)
	}
}

// Each movie stands alone: two unrelated films are never one notification.
func TestSplit_moviesDoNotShareABatch(t *testing.T) {
	files := []store.MediaFile{
		movieFile("/movies/Dune (2021)/Dune (2021).mkv", 300),
		movieFile("/movies/Inception (2010)/Inception (2010).mkv", 200),
	}

	got := Split(files)

	if len(got) != 2 {
		t.Fatalf("got %d summaries, want 2: %+v", len(got), got)
	}
	for _, s := range got {
		if len(s.Seasons) != 0 {
			t.Errorf("%q carries seasons %+v, want none", s.Title, s.Seasons)
		}
	}
}

// A TV file the parser couldn't attribute becomes its own title rather than
// silently joining an unrelated show's batch.
func TestSplit_unparsedSeriesStandsAlone(t *testing.T) {
	files := []store.MediaFile{
		tvFile("/tv/Severance/Season 1/S01E01.mkv", "Severance", 1, 100),
		{Path: "/tv/loose/Unknown.Episode.mkv", LibraryType: store.LibraryTypeTV},
	}

	got := Split(files)

	if len(got) != 2 {
		t.Fatalf("got %d summaries, want 2: %+v", len(got), got)
	}
	if got[1].Title != "Unknown.Episode" {
		t.Errorf("title = %q, want the filename stem", got[1].Title)
	}
}

func TestMessage_perTitle(t *testing.T) {
	cases := []struct {
		name  string
		files []store.MediaFile
		want  string
	}{
		{
			name:  "single episode names its season",
			files: []store.MediaFile{tvFile("/tv/S/Season 3/S03E01.mkv", "Severance", 3, 1024*1024*600)},
			want:  "Severance · Season 3 — 1 new re-encode candidate · 600.0 MB recoverable",
		},
		{
			name: "one season folds into the title",
			files: []store.MediaFile{
				tvFile("/tv/S/Season 3/S03E01.mkv", "Severance", 3, 1024*1024*512),
				tvFile("/tv/S/Season 3/S03E02.mkv", "Severance", 3, 1024*1024*512),
			},
			want: "Severance · Season 3 — 2 new re-encode candidates · 1.0 GB recoverable",
		},
		{
			name: "several seasons become a scope clause",
			files: []store.MediaFile{
				tvFile("/tv/S/Season 1/S01E01.mkv", "Severance", 1, 1024*1024*512),
				tvFile("/tv/S/Season 2/S02E01.mkv", "Severance", 2, 1024*1024*512),
			},
			want: "Severance — 2 new re-encode candidates across 2 seasons · 1.0 GB recoverable",
		},
		{
			name:  "a movie has no season",
			files: []store.MediaFile{movieFile("/movies/Dune (2021)/Dune (2021).mkv", 1024*1024*1024*3)},
			want:  "Dune (2021) — 1 new re-encode candidate · 3.0 GB recoverable",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Split(tc.files)
			if len(got) != 1 {
				t.Fatalf("got %d summaries, want 1", len(got))
			}
			if msg := got[0].Message(); msg != tc.want {
				t.Errorf("message =\n %q\nwant\n %q", msg, tc.want)
			}
		})
	}
}

// A season fold must not mislead: with some files unattributed to that season,
// the title cannot claim to be scoped to it.
func TestMessage_doesNotFoldPartialSeason(t *testing.T) {
	files := []store.MediaFile{
		tvFile("/tv/S/Season 3/S03E01.mkv", "Severance", 3, 100),
		{Path: "/tv/S/extra.mkv", LibraryType: store.LibraryTypeTV, SeriesTitle: strPtr("Severance")},
	}

	got := Split(files)
	if len(got) != 1 {
		t.Fatalf("got %d summaries, want 1", len(got))
	}
	if msg := got[0].Message(); strings.Contains(msg, "Season 3") {
		t.Errorf("message = %q, must not claim the batch is all Season 3", msg)
	}
}

func TestDetails_seasonsOnlyWhenMoreThanOne(t *testing.T) {
	oneSeason := Split([]store.MediaFile{
		tvFile("/tv/S/Season 3/S03E01.mkv", "Severance", 3, 100),
		tvFile("/tv/S/Season 3/S03E02.mkv", "Severance", 3, 100),
	})[0]
	if got := oneSeason.Details(); len(got) != 0 {
		t.Errorf("details = %v, want none (the message already says it)", got)
	}

	twoSeasons := Split([]store.MediaFile{
		tvFile("/tv/S/Season 1/S01E01.mkv", "Severance", 1, 100),
		tvFile("/tv/S/Season 2/S02E01.mkv", "Severance", 2, 100),
	})[0]
	got := twoSeasons.Details()
	if len(got) != 2 || !strings.HasPrefix(got[0], "Season 1 —") {
		t.Errorf("details = %v, want a line per season", got)
	}
}

func TestRollUp_collapsesManyTitles(t *testing.T) {
	var files []store.MediaFile
	for i := range 12 {
		name := string(rune('A' + i))
		files = append(files, movieFile("/movies/"+name+"/"+name+".mkv", int64(1024*1024*(100-i))))
	}

	rolled := RollUp(Split(files))

	if !rolled.IsRollup() {
		t.Fatal("want a rollup (no single title)")
	}
	if rolled.Count != 12 || rolled.Titles != 12 {
		t.Errorf("count/titles = %d/%d, want 12/12", rolled.Count, rolled.Titles)
	}
	if len(rolled.Rollup) != maxRollupTitles {
		t.Errorf("listed %d titles, want them capped at %d", len(rolled.Rollup), maxRollupTitles)
	}
	if msg := rolled.Message(); !strings.Contains(msg, "12 new re-encode candidates across 12 titles") {
		t.Errorf("message = %q", msg)
	}
	if got := rolled.Details(); got[len(got)-1] != "…and 2 more" {
		t.Errorf("details tail = %q, want the dropped titles acknowledged", got[len(got)-1])
	}
}

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		0:                      "0 B",
		512:                    "512 B",
		1024:                   "1.0 KB",
		1536:                   "1.5 KB",
		5 * 1024 * 1024:        "5.0 MB",
		3 * 1024 * 1024 * 1024: "3.0 GB",
	}
	for in, want := range cases {
		if got := FormatBytes(in); got != want {
			t.Errorf("FormatBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func strPtr(s string) *string { return &s }
