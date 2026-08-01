package config

import (
	"os"
	"testing"
	"time"
)

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"MOVIES_PATH":         "/media/movies",
		"TV_PATH":             "/media/tv",
		"DB_PATH":             "/data/reclaim.db",
		"ENCODE_WINDOW_START": "00:00",
		"ENCODE_WINDOW_END":   "06:00",
		"SCAN_INTERVAL":       "24h",
		"PROBE_CONCURRENCY":   "4",
	}
}

func TestLoad_valid(t *testing.T) {
	setEnv(t, validEnv())
	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ProbeConcurrency != 4 {
		t.Errorf("concurrency = %d, want 4", c.ProbeConcurrency)
	}
	if c.ScanInterval != 24*time.Hour {
		t.Errorf("scan interval = %v, want 24h", c.ScanInterval)
	}
	if c.EncodeWindowEnd != 6*time.Hour {
		t.Errorf("window end = %v, want 6h", c.EncodeWindowEnd)
	}
}

func TestLoad_missingPath(t *testing.T) {
	setEnv(t, validEnv())
	os.Unsetenv("MOVIES_PATH")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing MOVIES_PATH")
	}
}

func TestLoad_badWindow(t *testing.T) {
	env := validEnv()
	env["ENCODE_WINDOW_END"] = "25:00"
	setEnv(t, env)
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for bad ENCODE_WINDOW_END")
	}
}

func TestLoad_badDuration(t *testing.T) {
	env := validEnv()
	env["SCAN_INTERVAL"] = "notaduration"
	setEnv(t, env)
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for bad SCAN_INTERVAL")
	}
}

func TestLoad_missingRetention(t *testing.T) {
	tests := []struct {
		value   string
		want    time.Duration
		wantErr bool
	}{
		{value: "", want: 0},
		{value: "0", want: 0},
		{value: "720h", want: 720 * time.Hour},
		{value: "-24h", wantErr: true},
		{value: "30 days", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			env := validEnv()
			if tt.value != "" {
				env["MISSING_RETENTION"] = tt.value
			}
			setEnv(t, env)

			c, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for MISSING_RETENTION=%q", tt.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.MissingRetention != tt.want {
				t.Errorf("retention = %v, want %v", c.MissingRetention, tt.want)
			}
		})
	}
}

func TestLive_updateMissingRetention(t *testing.T) {
	live := NewLive(&Config{ScanInterval: time.Hour, ProbeConcurrency: 1, OversizeThreshold: 2})
	if live.MissingRetention() != 0 {
		t.Fatalf("default retention = %v, want 0 (off)", live.MissingRetention())
	}

	month := "720h"
	if err := live.Update(nil, nil, nil, nil, nil, nil, &month, nil); err != nil {
		t.Fatalf("update: %v", err)
	}
	if live.MissingRetention() != 720*time.Hour {
		t.Errorf("retention = %v, want 720h", live.MissingRetention())
	}

	// A rejected value must leave the holder untouched.
	bad := "-1h"
	if err := live.Update(nil, nil, nil, nil, nil, nil, &bad, nil); err == nil {
		t.Fatal("expected error for negative retention")
	}
	if live.MissingRetention() != 720*time.Hour {
		t.Errorf("retention after failed update = %v, want 720h", live.MissingRetention())
	}

	off := "0"
	if err := live.Update(nil, nil, nil, nil, nil, nil, &off, nil); err != nil {
		t.Fatalf("update to off: %v", err)
	}
	if live.MissingRetention() != 0 {
		t.Errorf("retention = %v, want 0", live.MissingRetention())
	}
}

func TestLoad_badConcurrency(t *testing.T) {
	env := validEnv()
	env["PROBE_CONCURRENCY"] = "0"
	setEnv(t, env)
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for PROBE_CONCURRENCY=0")
	}
}

func TestLoad_timezone(t *testing.T) {
	cases := []struct {
		name     string
		timezone string
		tz       string
		want     string
		wantErr  bool
	}{
		{name: "defaults to UTC", want: "UTC"},
		{name: "TIMEZONE wins", timezone: "America/New_York", tz: "Europe/Berlin", want: "America/New_York"},
		{name: "falls back to TZ", tz: "Europe/Berlin", want: "Europe/Berlin"},
		// A NAS template field picks up trailing whitespace easily; untrimmed it
		// fails to load and silently drops the whole deployment back to UTC.
		{name: "trims surrounding space", tz: "America/New_York ", want: "America/New_York"},
		{name: "trims TIMEZONE too", timezone: " America/New_York", want: "America/New_York"},
		{name: "bad TIMEZONE is fatal", timezone: "Mars/Olympus", wantErr: true},
		// TZ is not the app's variable to reject — warn and fall back instead.
		{name: "bad TZ falls back to UTC", tz: "Mars/Olympus", want: "UTC"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := validEnv()
			env["TIMEZONE"] = c.timezone
			env["TZ"] = c.tz
			setEnv(t, env)

			got, err := Load()
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Timezone != c.want {
				t.Errorf("timezone = %q, want %q", got.Timezone, c.want)
			}
			if got.Location == nil || got.Location.String() != c.want {
				t.Errorf("location = %v, want %q", got.Location, c.want)
			}
		})
	}
}

func TestLiveUpdate_timezone(t *testing.T) {
	live := NewLive(&Config{Timezone: "UTC", Location: time.UTC})

	tz := "America/New_York"
	if err := live.Update(nil, nil, nil, nil, nil, nil, nil, &tz); err != nil {
		t.Fatalf("update: %v", err)
	}
	if live.Timezone() != tz || live.Location().String() != tz {
		t.Errorf("timezone = %q / %v, want %q", live.Timezone(), live.Location(), tz)
	}

	bad := "Mars/Olympus"
	if err := live.Update(nil, nil, nil, nil, nil, nil, nil, &bad); err == nil {
		t.Fatal("expected error for unknown zone")
	}
	if live.Timezone() != tz {
		t.Errorf("timezone after failed update = %q, want %q", live.Timezone(), tz)
	}
}

func TestWindowState(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	at := func(loc *time.Location, h, m int) time.Time {
		return time.Date(2026, 1, 1, h, m, 0, 0, loc)
	}
	cases := []struct {
		name       string
		now        time.Time
		start, end time.Duration
		wantOpen   bool
		wantUntil  time.Duration
	}{
		{"closed before start", at(time.UTC, 8, 0), 9 * time.Hour, 17 * time.Hour, false, time.Hour},
		{"open counts to end", at(time.UTC, 12, 0), 9 * time.Hour, 17 * time.Hour, true, 5 * time.Hour},
		{"end boundary is closed", at(time.UTC, 17, 0), 9 * time.Hour, 17 * time.Hour, false, 16 * time.Hour},
		{"overnight before midnight", at(time.UTC, 23, 24), 0, 6 * time.Hour, false, 36 * time.Minute},
		{"overnight after midnight", at(time.UTC, 2, 0), 0, 6 * time.Hour, true, 4 * time.Hour},
		{"always open never flips", at(time.UTC, 2, 0), 3 * time.Hour, 3 * time.Hour, true, 0},
		// Same instant as "overnight before midnight" above, read in New York:
		// the zone, not the underlying instant, decides.
		{"zone decides", at(newYork, 23, 24), 0, 6 * time.Hour, false, 36 * time.Minute},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			open, until := WindowState(c.now, c.start, c.end)
			if open != c.wantOpen {
				t.Errorf("open = %v, want %v", open, c.wantOpen)
			}
			if until != c.wantUntil {
				t.Errorf("until = %v, want %v", until, c.wantUntil)
			}
		})
	}
}
