package store

import (
	"context"
	"testing"
)

func TestClockFormat_defaultsTo12h(t *testing.T) {
	s := openTestStore(t)

	if got := s.Settings.ClockFormat(context.Background()); got != DefaultClockFormat {
		t.Errorf("clock format on fresh DB = %q, want %q", got, DefaultClockFormat)
	}
}

func TestSetClockFormat_roundTrips(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.Settings.SetClockFormat(ctx, ClockFormat24h); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := s.Settings.ClockFormat(ctx); got != ClockFormat24h {
		t.Errorf("clock format = %q, want %q", got, ClockFormat24h)
	}

	if err := s.Settings.SetClockFormat(ctx, DefaultClockFormat); err != nil {
		t.Fatalf("set back: %v", err)
	}
	if got := s.Settings.ClockFormat(ctx); got != DefaultClockFormat {
		t.Errorf("clock format = %q, want %q", got, DefaultClockFormat)
	}
}

func TestSetClockFormat_rejectsUnknown(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.Settings.SetClockFormat(ctx, "military"); err == nil {
		t.Fatal("expected error for unknown clock format")
	}
	if got := s.Settings.ClockFormat(ctx); got != DefaultClockFormat {
		t.Errorf("clock format after rejected write = %q, want %q", got, DefaultClockFormat)
	}
}

// A value written directly to the column outside SetClockFormat's validation
// must not reach the UI as-is.
func TestClockFormat_fallsBackOnGarbage(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.w.ExecContext(ctx,
		"UPDATE settings SET clock_format = 'sundial' WHERE id = 1",
	); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if got := s.Settings.ClockFormat(ctx); got != DefaultClockFormat {
		t.Errorf("clock format = %q, want %q", got, DefaultClockFormat)
	}
}

// Databases that predate migration 00013 (or where goose recorded it without
// applying it) are repaired at boot rather than failing every settings write.
func TestEnsureClockFormat_addsMissingColumn(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.w.ExecContext(ctx, "ALTER TABLE settings DROP COLUMN clock_format"); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	if err := s.ensureClockFormat(ctx); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if err := s.Settings.SetClockFormat(ctx, ClockFormat24h); err != nil {
		t.Fatalf("set after repair: %v", err)
	}
	if got := s.Settings.ClockFormat(ctx); got != ClockFormat24h {
		t.Errorf("clock format = %q, want %q", got, ClockFormat24h)
	}
}
