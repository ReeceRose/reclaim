package scanner

import (
	"context"
	"sync"
)

// probeGate bounds concurrent storage-touching probes. Capacity is read from
// capFn on each acquire so PUT /api/settings takes effect without a restart.
type probeGate struct {
	mu       sync.Mutex
	capFn    func() int
	inFlight int
	waiters  []chan struct{}
}

func newProbeGate(capFn func() int) *probeGate {
	return &probeGate{capFn: capFn}
}

func (g *probeGate) capacity() int {
	if g.capFn == nil {
		return 1
	}
	c := g.capFn()
	if c < 1 {
		return 1
	}
	return c
}

func (g *probeGate) acquire(ctx context.Context) error {
	g.mu.Lock()
	for {
		if g.inFlight < g.capacity() {
			g.inFlight++
			g.mu.Unlock()
			return nil
		}
		ch := make(chan struct{})
		g.waiters = append(g.waiters, ch)
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			g.mu.Lock()
			g.removeWaiter(ch)
			g.mu.Unlock()
			return ctx.Err()
		case <-ch:
			g.mu.Lock()
		}
	}
}

func (g *probeGate) removeWaiter(ch chan struct{}) {
	for i, w := range g.waiters {
		if w == ch {
			g.waiters = append(g.waiters[:i], g.waiters[i+1:]...)
			return
		}
	}
}

func (g *probeGate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.inFlight > 0 {
		g.inFlight--
	}
	for len(g.waiters) > 0 && g.inFlight < g.capacity() {
		ch := g.waiters[0]
		g.waiters = g.waiters[1:]
		close(ch)
	}
}
