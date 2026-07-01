package backfill

import (
	"context"

	"reclaim/internal/scanner"
	"reclaim/internal/store"
)

const (
	// TaskCompatibilityProbe backfills pixel_format, media_streams, and
	// media_compatibility for files indexed before the compatibility engine
	// shipped. Requires a genuine full re-probe — see docs/COMPATIBILITY
	// PLAN.md §5.
	TaskCompatibilityProbe = "compatibility_probe"
)

// DefaultTasks returns the ordered list of backfill tasks. Order matters:
// earlier tasks run before later ones.
func DefaultTasks() []Task {
	return []Task{
		compatibilityProbeTask{},
	}
}

type compatibilityProbeTask struct{}

func (compatibilityProbeTask) Key() string    { return TaskCompatibilityProbe }
func (compatibilityProbeTask) Label() string  { return "Compatibility evaluation" }
func (compatibilityProbeTask) Action() Action { return ActionFullScan }

func (compatibilityProbeTask) Needed(ctx context.Context, s *store.Store) (bool, error) {
	return s.Media.NeedsCompatibilityBackfill(ctx)
}

// BackfillTrigger is the scan_runs.trigger value for coordinator-driven scans.
const BackfillTrigger = scanner.TriggerBackfill
