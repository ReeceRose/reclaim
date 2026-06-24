package config

import (
	"fmt"
	"sync"
	"time"
)

// Live is the runtime-mutable view of the settings that the spec lets an
// operator change without a restart (§11): the encode window, probe concurrency,
// and scan interval. It is seeded from the env-loaded Config at boot and then
// owned in memory — the scanner and (P6) worker read it on each use, so a
// PUT /api/settings takes effect immediately. Overrides are intentionally not
// persisted to the DB: the settings table stays auth-only (§8), and a restart
// re-seeds from env.
type Live struct {
	mu                sync.RWMutex
	encodeWindowStart time.Duration
	encodeWindowEnd   time.Duration
	scanInterval      time.Duration
	probeConcurrency  int
}

// NewLive seeds a Live holder from the immutable boot Config.
func NewLive(c *Config) *Live {
	return &Live{
		encodeWindowStart: c.EncodeWindowStart,
		encodeWindowEnd:   c.EncodeWindowEnd,
		scanInterval:      c.ScanInterval,
		probeConcurrency:  c.ProbeConcurrency,
	}
}

func (l *Live) EncodeWindowStart() time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.encodeWindowStart
}

func (l *Live) EncodeWindowEnd() time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.encodeWindowEnd
}

func (l *Live) ScanInterval() time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.scanInterval
}

func (l *Live) ProbeConcurrency() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.probeConcurrency
}

// Update applies validated settings. Any field left nil is unchanged. It
// validates the whole set before mutating, so a bad value never leaves the
// holder half-updated.
func (l *Live) Update(encodeStart, encodeEnd *string, scanInterval *string, probeConcurrency *int) error {
	var (
		start = l.EncodeWindowStart()
		end   = l.EncodeWindowEnd()
		intvl = l.ScanInterval()
		conc  = l.ProbeConcurrency()
		err   error
	)

	if encodeStart != nil {
		if start, err = parseHHMMValue(*encodeStart); err != nil {
			return fmt.Errorf("encode_window_start: %w", err)
		}
	}
	if encodeEnd != nil {
		if end, err = parseHHMMValue(*encodeEnd); err != nil {
			return fmt.Errorf("encode_window_end: %w", err)
		}
	}
	if scanInterval != nil {
		if intvl, err = time.ParseDuration(*scanInterval); err != nil {
			return fmt.Errorf("scan_interval: %w", err)
		}
		if intvl <= 0 {
			return fmt.Errorf("scan_interval must be positive")
		}
	}
	if probeConcurrency != nil {
		if *probeConcurrency < 1 {
			return fmt.Errorf("probe_concurrency must be a positive integer")
		}
		conc = *probeConcurrency
	}

	l.mu.Lock()
	l.encodeWindowStart = start
	l.encodeWindowEnd = end
	l.scanInterval = intvl
	l.probeConcurrency = conc
	l.mu.Unlock()
	return nil
}

// FormatHHMM renders a since-midnight duration back to "HH:MM" for the API.
func FormatHHMM(d time.Duration) string {
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	return fmt.Sprintf("%02d:%02d", h, m)
}

func parseHHMMValue(v string) (time.Duration, error) {
	var h, m int
	if _, err := fmt.Sscanf(v, "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("must be HH:MM (got %q)", v)
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute, nil
}
