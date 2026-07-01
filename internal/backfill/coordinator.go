package backfill

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"reclaim/internal/scanner"
	"reclaim/internal/store"
)

// Coordinator detects pending backfill tasks and runs them sequentially.
// Full-scan tasks defer to the scanner; the coordinator resumes when a scan
// completes (including after a conflict with startup/scheduled scans).
type Coordinator struct {
	store   *store.Store
	scanner FullScanRunner
	tasks   []Task

	mu         sync.Mutex
	runningKey string // task key waiting on an in-flight backfill scan

	processCh chan struct{}
}

func NewCoordinator(s *store.Store, scan FullScanRunner, tasks []Task) *Coordinator {
	if tasks == nil {
		tasks = DefaultTasks()
	}
	return &Coordinator{
		store:     s,
		scanner:   scan,
		tasks:     tasks,
		processCh: make(chan struct{}, 1),
	}
}

// Start launches the processing loop. Call once from main after the scanner
// is wired.
func (c *Coordinator) Start(ctx context.Context) {
	go c.loop(ctx)
	c.scheduleProcess()
}

// OnScanCompleted should be called whenever any scan finishes (success or
// failure) so queued backfills can run after startup/scheduled scans.
func (c *Coordinator) OnScanCompleted() {
	c.mu.Lock()
	c.runningKey = ""
	c.mu.Unlock()
	c.scheduleProcess()
}

// Status reports each registered task's needed/running state.
func (c *Coordinator) Status(ctx context.Context) ([]TaskStatus, error) {
	c.mu.Lock()
	runningKey := c.runningKey
	c.mu.Unlock()

	out := make([]TaskStatus, 0, len(c.tasks))
	for _, t := range c.tasks {
		needed, err := t.Needed(ctx, c.store)
		if err != nil {
			return nil, err
		}
		out = append(out, TaskStatus{
			Key:     t.Key(),
			Label:   t.Label(),
			Action:  t.Action(),
			Needed:  needed,
			Running: t.Key() == runningKey,
		})
	}
	return out, nil
}

func (c *Coordinator) scheduleProcess() {
	select {
	case c.processCh <- struct{}{}:
	default:
	}
}

func (c *Coordinator) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.processCh:
			c.processNext(ctx)
		}
	}
}

func (c *Coordinator) processNext(ctx context.Context) {
	c.mu.Lock()
	if c.runningKey != "" {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	for _, task := range c.tasks {
		needed, err := task.Needed(ctx, c.store)
		if err != nil {
			slog.Error("backfill: check needed", "task", task.Key(), "err", err)
			continue
		}
		if !needed {
			continue
		}

		switch task.Action() {
		case ActionFullScan:
			c.mu.Lock()
			if c.runningKey != "" {
				c.mu.Unlock()
				return
			}
			c.runningKey = task.Key()
			c.mu.Unlock()

			if err := c.scanner.StartScan(ctx, BackfillTrigger, true); err != nil {
				c.mu.Lock()
				c.runningKey = ""
				c.mu.Unlock()
				if errors.Is(err, scanner.ErrScanInProgress) {
					return
				}
				slog.Error("backfill: start full scan", "task", task.Key(), "err", err)
				continue
			}
			slog.Info("backfill: started full scan", "task", task.Key())
			return

		case ActionInline:
			slog.Warn("backfill: inline task not implemented in coordinator", "task", task.Key())

		default:
			slog.Warn("backfill: unknown action", "task", task.Key(), "action", task.Action())
		}
	}
}
