package backfill

import (
	"context"

	"reclaim/internal/store"
)

// Action describes what a backfill task does when it runs.
type Action string

const (
	// ActionFullScan re-probes every file (scanner force=true). Used when
	// stored probe data is missing fields that only ffprobe can supply.
	ActionFullScan Action = "full_scan"
	// ActionInline runs synchronously in-process (SQL recompute, etc.).
	ActionInline Action = "inline"
)

// Task is one upgrade/backfill step the coordinator may run automatically.
// Register tasks in DefaultTasks; add new ones there for future features.
type Task interface {
	Key() string
	Label() string
	Action() Action
	Needed(ctx context.Context, s *store.Store) (bool, error)
}

// FullScanRunner starts a background scan. Satisfied by scanner.Scanner.
type FullScanRunner interface {
	StartScan(ctx context.Context, trigger string, force bool) error
}

// TaskStatus is the wire shape of one task for GET /api/backfill.
type TaskStatus struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Action  Action `json:"action"`
	Needed  bool   `json:"needed"`
	Running bool   `json:"running"`
}
