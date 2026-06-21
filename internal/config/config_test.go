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
		"MOVIES_PATH":          "/media/movies",
		"TV_PATH":              "/media/tv",
		"DB_PATH":              "/data/reclaim.db",
		"ENCODE_WINDOW_START":  "00:00",
		"ENCODE_WINDOW_END":    "06:00",
		"SCAN_INTERVAL":        "24h",
		"PROBE_CONCURRENCY":    "4",
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

func TestLoad_badConcurrency(t *testing.T) {
	env := validEnv()
	env["PROBE_CONCURRENCY"] = "0"
	setEnv(t, env)
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for PROBE_CONCURRENCY=0")
	}
}
