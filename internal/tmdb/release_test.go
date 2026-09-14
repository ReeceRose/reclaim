package tmdb

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFirstTheatricalDate(t *testing.T) {
	var rd releaseDates
	if err := json.Unmarshal([]byte(`{"results": [
		{"iso_3166_1": "US", "release_dates": [
			{"type": 1, "release_date": "2010-07-08T00:00:00.000Z"},
			{"type": 3, "release_date": "2010-07-16T00:00:00.000Z"},
			{"type": 4, "release_date": "2010-12-07T00:00:00.000Z"}
		]},
		{"iso_3166_1": "GB", "release_dates": [
			{"type": 3, "release_date": "2010-07-16T00:00:00.000Z"},
			{"type": 2, "release_date": "2010-07-13T00:00:00.000Z"}
		]}
	]}`), &rd); err != nil {
		t.Fatal(err)
	}

	if got := firstTheatricalDate(rd, "2010-07-15"); got != "2010-07-13" {
		t.Errorf("earliest theatrical = %q, want 2010-07-13 (the premiere and digital release do not count)", got)
	}
	if got := firstTheatricalDate(releaseDates{}, "2021-10-22"); got != "2021-10-22" {
		t.Errorf("no theatrical release = %q, want the primary release date", got)
	}
	if got := firstTheatricalDate(releaseDates{}, ""); got != "" {
		t.Errorf("no dates at all = %q, want empty", got)
	}
}

func TestParseSeasonAirDates(t *testing.T) {
	block := json.RawMessage(`{"episodes": [
		{"episode_number": 1, "air_date": "2008-01-20"},
		{"episode_number": 2, "air_date": ""},
		{"episode_number": 3, "air_date": null}
	]}`)
	got, err := parseSeasonAirDates(block, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []EpisodeAirDate{{Season: 1, Episode: 1, AirDate: "2008-01-20"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	missing, err := parseSeasonAirDates(nil, 9)
	if err != nil || len(missing) != 0 {
		t.Errorf("a season TMDB does not have = %+v, %v; want no episodes and no error", missing, err)
	}
}
