package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	MoviesPath       string
	TVPath           string
	DBPath           string
	EncodeWindowStart time.Duration // minutes since midnight
	EncodeWindowEnd   time.Duration
	ScanInterval     time.Duration
	ScanAnchor       string
	ProbeConcurrency int
	DisableAuth      bool
	ResetAuth        bool
}

func Load() (*Config, error) {
	c := &Config{}
	var errs []error

	c.MoviesPath = requireEnv("MOVIES_PATH", &errs)
	c.TVPath = requireEnv("TV_PATH", &errs)
	c.DBPath = requireEnv("DB_PATH", &errs)

	c.EncodeWindowStart = parseHHMM("ENCODE_WINDOW_START", "00:00", &errs)
	c.EncodeWindowEnd = parseHHMM("ENCODE_WINDOW_END", "06:00", &errs)

	c.ScanInterval = parseDuration("SCAN_INTERVAL", "24h", &errs)
	c.ScanAnchor = parseHHMMString("SCAN_ANCHOR", "00:00", &errs)
	c.ProbeConcurrency = parseInt("PROBE_CONCURRENCY", "4", &errs)

	c.DisableAuth = os.Getenv("DISABLE_AUTH") == "true"
	c.ResetAuth = os.Getenv("RESET_AUTH") == "true"

	if len(errs) > 0 {
		return nil, fmt.Errorf("config errors: %v", errs)
	}
	return c, nil
}

func requireEnv(key string, errs *[]error) string {
	v := os.Getenv(key)
	if v == "" {
		*errs = append(*errs, fmt.Errorf("%s must not be empty", key))
	}
	return v
}

func parseHHMMString(key, def string, errs *[]error) string {
	v := os.Getenv(key)
	if v == "" {
		v = def
	}
	if _, err := parseHHMMValue(v); err != nil {
		*errs = append(*errs, fmt.Errorf("%s %w", key, err))
		return def
	}
	return v
}

func parseHHMM(key, def string, errs *[]error) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		v = def
	}
	d, err := parseHHMMValue(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s %w", key, err))
		return 0
	}
	return d
}

func parseDuration(key, def string, errs *[]error) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		v = def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s must be a valid duration (got %q)", key, v))
	}
	return d
}

func parseInt(key, def string, errs *[]error) int {
	v := os.Getenv(key)
	if v == "" {
		v = def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		*errs = append(*errs, fmt.Errorf("%s must be a positive integer (got %q)", key, v))
	}
	return n
}
