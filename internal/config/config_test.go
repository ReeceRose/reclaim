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
	if err := live.Update(nil, nil, nil, nil, nil, nil, &month); err != nil {
		t.Fatalf("update: %v", err)
	}
	if live.MissingRetention() != 720*time.Hour {
		t.Errorf("retention = %v, want 720h", live.MissingRetention())
	}

	// A rejected value must leave the holder untouched.
	bad := "-1h"
	if err := live.Update(nil, nil, nil, nil, nil, nil, &bad); err == nil {
		t.Fatal("expected error for negative retention")
	}
	if live.MissingRetention() != 720*time.Hour {
		t.Errorf("retention after failed update = %v, want 720h", live.MissingRetention())
	}

	off := "0"
	if err := live.Update(nil, nil, nil, nil, nil, nil, &off); err != nil {
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
