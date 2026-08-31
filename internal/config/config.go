package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	MoviesPath        string
	TVPath            string
	DBPath            string
	TMDBKey           string
	Timezone          string         // IANA name the window and anchor are evaluated in
	Location          *time.Location // resolved Timezone
	EncodeWindowStart time.Duration  // minutes since midnight
	EncodeWindowEnd   time.Duration
	ScanInterval      time.Duration
	ScanAnchor        string
	ProbeConcurrency  int
	OversizeThreshold float64
	MissingRetention  time.Duration // 0 = never prune missing rows
	ReplaceLookback   time.Duration // how far back a missing file may be reclaimed by a redownload; 0 = off
	DisableAuth       bool
	ResetAuth         bool
}

func Load() (*Config, error) {
	c := &Config{}
	var errs []error

	c.MoviesPath = requireEnv("MOVIES_PATH", &errs)
	c.TVPath = requireEnv("TV_PATH", &errs)
	c.DBPath = requireEnv("DB_PATH", &errs)

	c.Timezone, c.Location = parseTimezone(&errs)
	c.EncodeWindowStart = parseHHMM("ENCODE_WINDOW_START", "00:00", &errs)
	c.EncodeWindowEnd = parseHHMM("ENCODE_WINDOW_END", "06:00", &errs)

	c.ScanInterval = parseDuration("SCAN_INTERVAL", "24h", &errs)
	c.ScanAnchor = parseHHMMString("SCAN_ANCHOR", "00:00", &errs)
	c.ProbeConcurrency = parseInt("PROBE_CONCURRENCY", "4", &errs)
	c.OversizeThreshold = parseFloat("OVERSIZE_THRESHOLD", "2.0", &errs)
	c.MissingRetention = parseRetention("MISSING_RETENTION", "0", &errs)
	c.ReplaceLookback = parseRetention("REPLACE_LOOKBACK", "720h", &errs)

	c.TMDBKey = os.Getenv("TMDB_API_KEY")

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

// parseTimezone resolves the zone the encode window and scan anchor are
// evaluated in, independent of the process clock (which stays UTC). TIMEZONE
// wins; TZ is honoured as a fallback so a deployment that already relies on the
// container timezone keeps its behaviour. Both are trimmed — a trailing space,
// which a NAS template field picks up easily, otherwise makes LoadLocation fail
// and silently shifts the window by the whole UTC offset. An unusable TIMEZONE
// is fatal because the operator set it for this purpose; an unusable TZ only
// warns, since the app does not own that variable.
func parseTimezone(errs *[]error) (string, *time.Location) {
	if v := strings.TrimSpace(os.Getenv("TIMEZONE")); v != "" {
		loc, err := time.LoadLocation(v)
		if err != nil {
			*errs = append(*errs, fmt.Errorf("TIMEZONE must be an IANA name like \"America/New_York\" (got %q)", v))
			return "UTC", time.UTC
		}
		return v, loc
	}
	if v := strings.TrimSpace(os.Getenv("TZ")); v != "" {
		loc, err := time.LoadLocation(v)
		if err != nil {
			slog.Warn("TZ is not a loadable IANA timezone — falling back to UTC for the encode window; set TIMEZONE to be explicit",
				"tz", v, "err", err)
			return "UTC", time.UTC
		}
		return v, loc
	}
	return "UTC", time.UTC
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

// parseRetention parses a Go duration where "0" means the feature is off.
func parseRetention(key, def string, errs *[]error) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		v = def
	}
	d, err := ParseRetentionValue(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s %w", key, err))
		return 0
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

func parseFloat(key, def string, errs *[]error) float64 {
	v := os.Getenv(key)
	if v == "" {
		v = def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 1 {
		*errs = append(*errs, fmt.Errorf("%s must be a number greater than 1 (got %q)", key, v))
	}
	return f
}
