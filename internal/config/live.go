package config

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Live is the runtime-mutable view of the settings an operator can change
// without a restart: the encode window, probe concurrency, and scan interval.
// It is seeded from the env-loaded Config at boot and then owned in memory —
// the scanner and worker read it on each use, so a PUT /api/settings takes
// effect immediately. The API also persists each override (LiveOverrides) to
// the settings row, and main.go restores them over the env seed at boot, so a
// value set in the UI survives a restart; env only supplies the defaults.
type Live struct {
	mu                sync.RWMutex
	timezone          string
	location          *time.Location
	encodeWindowStart time.Duration
	encodeWindowEnd   time.Duration
	scanInterval      time.Duration
	scanAnchor        string
	probeConcurrency  int
	oversizeThreshold float64
	missingRetention  time.Duration
	replaceLookback   time.Duration
}

// NewLive seeds a Live holder from the immutable boot Config.
func NewLive(c *Config) *Live {
	loc := c.Location
	if loc == nil {
		loc = time.UTC
	}
	return &Live{
		timezone:          c.Timezone,
		location:          loc,
		encodeWindowStart: c.EncodeWindowStart,
		encodeWindowEnd:   c.EncodeWindowEnd,
		scanInterval:      c.ScanInterval,
		scanAnchor:        c.ScanAnchor,
		probeConcurrency:  c.ProbeConcurrency,
		oversizeThreshold: c.OversizeThreshold,
		missingRetention:  c.MissingRetention,
		replaceLookback:   c.ReplaceLookback,
	}
}

// Timezone is the IANA name of the zone the encode window and scan anchor are
// evaluated in.
func (l *Live) Timezone() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.timezone
}

// Location is the resolved Timezone. The process clock stays UTC, so every
// wall-clock decision (window, scan anchor) must be made against this.
func (l *Live) Location() *time.Location {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.location == nil {
		return time.UTC // time.Time.In panics on a nil location
	}
	return l.location
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

func (l *Live) ScanAnchor() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.scanAnchor
}

func (l *Live) ProbeConcurrency() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.probeConcurrency
}

// OversizeThreshold is the oversize_ratio at or above which a file is flagged as
// oversized (larger than a well-encoded file of its codec and resolution).
func (l *Live) OversizeThreshold() float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.oversizeThreshold
}

// MissingRetention is how long a file may stay soft-deleted ("missing") before
// the post-scan cleanup hard-deletes its row. Zero means never prune.
func (l *Live) MissingRetention() time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.missingRetention
}

// ReplaceLookback is how long after a file goes missing a newly indexed file
// with the same content identity is still treated as its replacement rather
// than an unrelated arrival. Zero disables replacement matching entirely.
func (l *Live) ReplaceLookback() time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.replaceLookback
}

// LiveOverrides is a partial update to Live, in the API's wire format. A nil
// field is unchanged. It doubles as the persisted form: the settings row stores
// the merged set of every override the operator has made, as JSON.
type LiveOverrides struct {
	Timezone          *string  `json:"timezone,omitempty"`
	EncodeWindowStart *string  `json:"encode_window_start,omitempty"`
	EncodeWindowEnd   *string  `json:"encode_window_end,omitempty"`
	ScanInterval      *string  `json:"scan_interval,omitempty"`
	ScanAnchor        *string  `json:"scan_anchor,omitempty"`
	ProbeConcurrency  *int     `json:"probe_concurrency,omitempty"`
	OversizeThreshold *float64 `json:"oversize_threshold,omitempty"`
	MissingRetention  *string  `json:"missing_retention,omitempty"`
	ReplaceLookback   *string  `json:"replace_lookback,omitempty"`
}

// Empty reports whether o changes nothing.
func (o LiveOverrides) Empty() bool {
	return o == LiveOverrides{}
}

// Merge returns o with every field set in next taking next's value.
func (o LiveOverrides) Merge(next LiveOverrides) LiveOverrides {
	if next.Timezone != nil {
		o.Timezone = next.Timezone
	}
	if next.EncodeWindowStart != nil {
		o.EncodeWindowStart = next.EncodeWindowStart
	}
	if next.EncodeWindowEnd != nil {
		o.EncodeWindowEnd = next.EncodeWindowEnd
	}
	if next.ScanInterval != nil {
		o.ScanInterval = next.ScanInterval
	}
	if next.ScanAnchor != nil {
		o.ScanAnchor = next.ScanAnchor
	}
	if next.ProbeConcurrency != nil {
		o.ProbeConcurrency = next.ProbeConcurrency
	}
	if next.OversizeThreshold != nil {
		o.OversizeThreshold = next.OversizeThreshold
	}
	if next.MissingRetention != nil {
		o.MissingRetention = next.MissingRetention
	}
	if next.ReplaceLookback != nil {
		o.ReplaceLookback = next.ReplaceLookback
	}
	return o
}

// split breaks o into one single-field override per set field, so Restore can
// apply each independently.
func (o LiveOverrides) split() []LiveOverrides {
	var out []LiveOverrides
	add := func(set bool, one LiveOverrides) {
		if set {
			out = append(out, one)
		}
	}
	add(o.Timezone != nil, LiveOverrides{Timezone: o.Timezone})
	add(o.EncodeWindowStart != nil, LiveOverrides{EncodeWindowStart: o.EncodeWindowStart})
	add(o.EncodeWindowEnd != nil, LiveOverrides{EncodeWindowEnd: o.EncodeWindowEnd})
	add(o.ScanInterval != nil, LiveOverrides{ScanInterval: o.ScanInterval})
	add(o.ScanAnchor != nil, LiveOverrides{ScanAnchor: o.ScanAnchor})
	add(o.ProbeConcurrency != nil, LiveOverrides{ProbeConcurrency: o.ProbeConcurrency})
	add(o.OversizeThreshold != nil, LiveOverrides{OversizeThreshold: o.OversizeThreshold})
	add(o.MissingRetention != nil, LiveOverrides{MissingRetention: o.MissingRetention})
	add(o.ReplaceLookback != nil, LiveOverrides{ReplaceLookback: o.ReplaceLookback})
	return out
}

// Restore applies persisted overrides at boot. Unlike Update it is per-field:
// a stored value that no longer validates (a timezone the host's tzdata has
// dropped) falls back to its env seed without discarding the rest. The
// returned error joins every field that was skipped.
func (l *Live) Restore(o LiveOverrides) error {
	var errs []error
	for _, one := range o.split() {
		if err := l.Update(one); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Update applies validated settings. Any field left nil is unchanged. It
// validates the whole set before mutating, so a bad value never leaves the
// holder half-updated.
func (l *Live) Update(o LiveOverrides) error {
	var (
		timezone          = o.Timezone
		encodeStart       = o.EncodeWindowStart
		encodeEnd         = o.EncodeWindowEnd
		scanInterval      = o.ScanInterval
		scanAnchor        = o.ScanAnchor
		probeConcurrency  = o.ProbeConcurrency
		oversizeThreshold = o.OversizeThreshold
		missingRetention  = o.MissingRetention
		replaceLookback   = o.ReplaceLookback
	)
	var (
		start     = l.EncodeWindowStart()
		end       = l.EncodeWindowEnd()
		intvl     = l.ScanInterval()
		anchor    = l.ScanAnchor()
		conc      = l.ProbeConcurrency()
		thresh    = l.OversizeThreshold()
		retention = l.MissingRetention()
		lookback  = l.ReplaceLookback()
		tzName    = l.Timezone()
		loc       = l.Location()
		err       error
	)

	if timezone != nil {
		tzName = strings.TrimSpace(*timezone)
		if loc, err = time.LoadLocation(tzName); err != nil {
			return fmt.Errorf("timezone: must be an IANA name like \"America/New_York\" (got %q)", *timezone)
		}
	}
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
	if scanAnchor != nil {
		if _, err = parseHHMMValue(*scanAnchor); err != nil {
			return fmt.Errorf("scan_anchor: %w", err)
		}
		anchor = *scanAnchor
	}
	if probeConcurrency != nil {
		if *probeConcurrency < 1 {
			return fmt.Errorf("probe_concurrency must be a positive integer")
		}
		conc = *probeConcurrency
	}
	if oversizeThreshold != nil {
		if *oversizeThreshold <= 1 {
			return fmt.Errorf("oversize_threshold must be greater than 1")
		}
		thresh = *oversizeThreshold
	}
	if missingRetention != nil {
		if retention, err = ParseRetentionValue(*missingRetention); err != nil {
			return fmt.Errorf("missing_retention: %w", err)
		}
	}
	if replaceLookback != nil {
		if lookback, err = ParseRetentionValue(*replaceLookback); err != nil {
			return fmt.Errorf("replace_lookback: %w", err)
		}
	}

	l.mu.Lock()
	l.timezone = tzName
	l.location = loc
	l.encodeWindowStart = start
	l.encodeWindowEnd = end
	l.scanInterval = intvl
	l.scanAnchor = anchor
	l.probeConcurrency = conc
	l.oversizeThreshold = thresh
	l.missingRetention = retention
	l.replaceLookback = lookback
	l.mu.Unlock()
	return nil
}

// ParseRetentionValue parses a retention duration from the API, where "0" and
// "" both mean "never prune". Shared with env parsing so a value accepted at
// boot is accepted at runtime.
func ParseRetentionValue(v string) (time.Duration, error) {
	if v == "" || v == "0" {
		return 0, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("must be 0 or a duration like \"720h\" (got %q)", v)
	}
	if d < 0 {
		return 0, fmt.Errorf("must not be negative (got %q)", v)
	}
	return d, nil
}

// WindowState reports whether an encode window is open at now, and how long
// until it next flips. now must already be in the configured location — the
// process clock is UTC, so callers pass time.Now().In(live.Location()).
//
// A zero-length window (start == end) means always-open and never flips, so the
// returned duration is zero. Otherwise the window is half-open [start, end):
// the end minute itself is closed, so a job cannot start on the boundary.
//
// The worker gates on this and the API reports it, so the badge in the UI can
// never disagree with the clock the worker actually pulls jobs on.
func WindowState(now time.Time, start, end time.Duration) (open bool, until time.Duration) {
	if start == end {
		return true, 0
	}
	elapsed := time.Duration(now.Hour())*time.Hour +
		time.Duration(now.Minute())*time.Minute +
		time.Duration(now.Second())*time.Second

	if start < end {
		open = elapsed >= start && elapsed < end
	} else {
		open = elapsed >= start || elapsed < end // wraps midnight
	}

	target := start
	if open {
		target = end
	}
	until = target - elapsed
	if until <= 0 {
		until += 24 * time.Hour
	}
	return open, until
}

// ProjectQueueFinish predicts when the worker will drain its queue, replaying
// its loop against the window: the running job finishes wherever it is, forced
// jobs run back to back regardless of the window, and each remaining job starts
// only while the window is open — a job pulled just before close still runs to
// completion, so a night can overrun its end by up to one job. now must be in
// the window's location, since WindowState reads its wall clock.
func ProjectQueueFinish(now time.Time, start, end, running time.Duration, forced, queued []time.Duration) time.Time {
	t := now.Add(running)
	for _, d := range forced {
		t = t.Add(d)
	}
	for _, d := range queued {
		if open, until := WindowState(t, start, end); !open {
			t = t.Add(until)
		}
		t = t.Add(d)
	}
	return t
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
