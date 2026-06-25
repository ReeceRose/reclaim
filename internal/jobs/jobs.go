// Package jobs holds the transcode-job state machine. It is deliberately pure
// (no DB, no I/O) so the legal-transition rules can be unit-tested in isolation
// and shared by the worker and the store's guarded UPDATEs.
package jobs

// Status is a transcode job's lifecycle state.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusVerifying Status = "verifying"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// transitions is the full legal state graph:
//
//	queued    → running, cancelled
//	running   → verifying, failed, cancelled
//	verifying → completed, failed, cancelled
//
// completed / failed / cancelled are terminal.
var transitions = map[Status]map[Status]bool{
	StatusQueued: {
		StatusRunning:   true,
		StatusCancelled: true,
	},
	StatusRunning: {
		StatusVerifying: true,
		StatusFailed:    true,
		StatusCancelled: true,
	},
	StatusVerifying: {
		StatusCompleted: true,
		StatusFailed:    true,
		StatusCancelled: true,
	},
	StatusCompleted: {},
	StatusFailed:    {},
	StatusCancelled: {},
}

// CanTransition reports whether moving from → to is legal. It is the single
// source of truth for the lifecycle, enforced by the store's guarded UPDATEs.
func CanTransition(from, to Status) bool {
	return transitions[from][to]
}
