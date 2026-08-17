// Package notify batches newly-discovered re-encode candidates into a single
// notification. A season import drops dozens of files into the library at once,
// and one line per file — in the events feed or, worse, in a chat client — is
// noise. So additions are collected and only announced once the library has been
// quiet for the configured delay.
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"reclaim/internal/store"
)

// ErrNoWebhook is returned by SendTest when no webhook URL is configured.
var ErrNoWebhook = errors.New("no webhook URL configured")

// Broadcaster is the WS hub slice the notifier pushes the events feed entry to.
// Satisfied by api.Hub; an interface so the notifier is testable without a hub.
type Broadcaster interface {
	Broadcast(event string, data any)
}

const (
	// tickInterval is how often the pending batch is checked against its delay.
	// The check is O(1) when nothing is pending, so this only bounds how late a
	// batch can be — never how much work is done.
	tickInterval = 15 * time.Second

	// maxWaitFactor caps how long a batch can be held open by a continuous
	// trickle of additions. Without it, a library import that adds a file every
	// few minutes for hours would never satisfy the quiet period.
	maxWaitFactor = 4

	// maxTitlesPerFlush is how many separate notifications one flush may send
	// before they are collapsed into a single rollup. Per-title notifications
	// are the point of this package, but a bulk import that lands 200 movies at
	// once must not become 200 messages.
	maxTitlesPerFlush = 10

	// webhookSpacing separates consecutive webhook posts. Chat services rate-limit
	// per webhook — Discord allows 5 requests per 2 seconds — and a flush of ten
	// titles would otherwise arrive as one burst and be throttled.
	webhookSpacing = 500 * time.Millisecond

	webhookTimeout = 15 * time.Second
)

// Notifier collects the IDs of newly-inserted media rows and announces the ones
// that are still re-encode candidates once the batch settles.
type Notifier struct {
	store  *store.Store
	hub    Broadcaster
	client *http.Client
	now    func() time.Time

	mu      sync.Mutex
	pending map[int64]struct{}
	first   time.Time // when the oldest pending addition arrived
	last    time.Time // when the newest pending addition arrived
}

// New builds a Notifier. Call SetBroadcaster to wire the hub (built later in
// main.go), then Run to start the batching loop.
func New(st *store.Store) *Notifier {
	return &Notifier{
		store:   st,
		client:  &http.Client{Timeout: webhookTimeout},
		now:     time.Now,
		pending: make(map[int64]struct{}),
	}
}

// SetBroadcaster wires the hub after construction. Must be called before Run.
func (n *Notifier) SetBroadcaster(b Broadcaster) { n.hub = b }

// Add queues media row IDs for the next batch. Whether a row is actually worth
// announcing is decided at send time, not here — see Notifier.send.
func (n *Notifier) Add(ids ...int64) {
	if len(ids) == 0 {
		return
	}
	now := n.now()

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.pending) == 0 {
		n.first = now
	}
	for _, id := range ids {
		n.pending[id] = struct{}{}
	}
	n.last = now
}

// Discard drops IDs from the pending batch. The scanner calls it for a row that
// turned out to be a reconciliation rather than an arrival — the surviving half
// of a supersede is the same content in a new container, not a new file.
func (n *Notifier) Discard(ids ...int64) {
	if len(ids) == 0 {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, id := range ids {
		delete(n.pending, id)
	}
}

// Pending reports how many additions are waiting to be announced.
func (n *Notifier) Pending() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.pending)
}

// Run drives the batching loop until ctx is cancelled.
func (n *Notifier) Run(ctx context.Context) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.step(ctx)
		}
	}
}

// step flushes the pending batch if it has settled. Settings are read every tick
// so a change to the delay (or to the enabled flag) applies without a restart.
func (n *Notifier) step(ctx context.Context) {
	cfg := n.store.Settings.Notify(ctx)
	if !cfg.Enabled {
		// Drop what's queued rather than holding it: re-enabling notifications
		// should not replay everything that landed while they were off.
		n.take()
		return
	}
	ids := n.due(cfg, n.now())
	if len(ids) == 0 {
		return
	}
	if err := n.send(ctx, cfg, ids); err != nil {
		slog.Error("notify: send batch", "count", len(ids), "err", err)
	}
}

// Flush sends whatever is pending immediately, ignoring the delay. Used by tests
// and by shutdown paths that would otherwise drop a settled batch.
func (n *Notifier) Flush(ctx context.Context) error {
	cfg := n.store.Settings.Notify(ctx)
	if !cfg.Enabled {
		n.take()
		return nil
	}
	ids := n.take()
	if len(ids) == 0 {
		return nil
	}
	return n.send(ctx, cfg, ids)
}

// due returns the batch to send, or nil while it is still settling. A batch goes
// out once no addition has arrived for the configured delay, or once it has been
// open for maxWaitFactor times that delay — whichever comes first.
func (n *Notifier) due(cfg store.NotifySettings, now time.Time) []int64 {
	delay := time.Duration(cfg.DelaySeconds) * time.Second

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.pending) == 0 {
		return nil
	}
	quiet := now.Sub(n.last) >= delay
	overdue := now.Sub(n.first) >= maxWaitFactor*delay
	if !quiet && !overdue {
		return nil
	}
	return n.takeLocked()
}

func (n *Notifier) take() []int64 {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.takeLocked()
}

func (n *Notifier) takeLocked() []int64 {
	if len(n.pending) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(n.pending))
	for id := range n.pending {
		ids = append(ids, id)
	}
	n.pending = make(map[int64]struct{})
	n.first = time.Time{}
	n.last = time.Time{}
	return ids
}

// send re-reads the batch against the candidate query before announcing it. IDs
// are collected as rows are inserted, and a lot can happen during the delay: a
// renamed file's duplicate row is deleted, a file can be queued or re-encoded,
// or it can vanish again. Only what is still a candidate is worth a notification.
//
// A failed webhook is not retried — the batch has already been taken, and the
// events feed carries the same information permanently. Retrying a chat message
// nobody missed is not worth holding the batch open for.
func (n *Notifier) send(ctx context.Context, cfg store.NotifySettings, ids []int64) error {
	files, err := n.store.Media.CandidatesByID(ctx, ids)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	// One notification per title, so a season of Severance never shares a message
	// with an unrelated movie. Past the cap that would be more noise than signal,
	// and the whole flush collapses into a single rollup instead.
	batches := Split(files)
	if len(batches) > maxTitlesPerFlush {
		slog.Info("notify: rolling up a bulk arrival",
			"titles", len(batches), "files", len(files), "cap", maxTitlesPerFlush)
		batches = []Summary{RollUp(batches)}
	}

	occurredAt := n.now().Unix()
	var firstErr error

	for i, batch := range batches {
		slog.Info("notify: new re-encode candidates",
			"title", batch.Title, "count", batch.Count, "savings_bytes", batch.SavingsBytes)

		n.recordEvent(ctx, batch, occurredAt)

		if cfg.WebhookURL == "" {
			continue
		}
		if i > 0 {
			if err := sleep(ctx, webhookSpacing); err != nil {
				return err
			}
		}
		// One failing receiver must not cost the remaining titles their webhook,
		// so the error is held and the loop continues.
		if err := postWebhook(ctx, n.client, cfg, batch, occurredAt); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// sleep waits for d, or returns early when ctx is cancelled (shutdown).
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// recordEvent writes one batch to the audit feed and pushes it to connected
// clients, which is what lights up the notification bell. A failure here is
// logged rather than returned: the webhook still deserves its chance to fire.
func (n *Notifier) recordEvent(ctx context.Context, summary Summary, occurredAt int64) {
	message := summary.Message()
	meta, err := json.Marshal(summary)
	if err != nil {
		slog.Error("notify: marshal event metadata", "err", err)
		return
	}

	eventID, err := n.store.Events.Insert(ctx, store.EventCandidatesAdded, store.SeverityInfo, message, string(meta))
	if err != nil {
		slog.Error("notify: insert event", "err", err)
		return
	}
	if n.hub == nil {
		return
	}

	var decoded any
	_ = json.Unmarshal(meta, &decoded)
	n.hub.Broadcast("event_created", map[string]any{
		"id":         eventID,
		"type":       store.EventCandidatesAdded,
		"severity":   store.SeverityInfo,
		"message":    message,
		"created_at": occurredAt,
		"metadata":   decoded,
	})
}

// SendTest posts a sample batch so the operator can verify a webhook from the
// Settings page. url and format override the stored settings, so a URL can be
// checked before it is saved. Errors are returned verbatim — this is the one
// path where a delivery failure has somewhere to be shown.
func (n *Notifier) SendTest(ctx context.Context, rawURL, format string) error {
	cfg := n.store.Settings.Notify(ctx)
	if rawURL != "" {
		cfg.WebhookURL = rawURL
	}
	if format != "" {
		cfg.WebhookFormat = format
	}
	if err := store.ValidateWebhookURL(cfg.WebhookURL); err != nil {
		return err
	}
	if cfg.WebhookURL == "" {
		return ErrNoWebhook
	}
	if !store.ValidWebhookFormat(cfg.WebhookFormat) {
		cfg.WebhookFormat = store.WebhookFormatJSON
	}

	return postWebhook(ctx, n.client, cfg, sampleSummary(), n.now().Unix())
}

// sampleSummary is the stand-in batch SendTest delivers, shaped like a real
// per-title notification so the operator sees what a real one will look like.
func sampleSummary() Summary {
	return Summary{
		Title:        "Reclaim test — Example Show",
		LibraryType:  store.LibraryTypeTV,
		Count:        3,
		Titles:       1,
		SizeBytes:    9 * 1024 * 1024 * 1024,
		SavingsBytes: 3600 * 1024 * 1024,
		Seasons: []Season{{
			Number:       1,
			Count:        3,
			SizeBytes:    9 * 1024 * 1024 * 1024,
			SavingsBytes: 3600 * 1024 * 1024,
		}},
	}
}
