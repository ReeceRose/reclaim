package scanner

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	"reclaim/internal/config"
	"reclaim/internal/ffprobe"
	"reclaim/internal/media"
	"reclaim/internal/store"
)

// Broadcaster is the WS hub slice the scanner pushes scan + event notifications
// to. Satisfied by api.Hub; an interface so the scanner is testable without a
// real hub.
type Broadcaster interface {
	Broadcast(event string, data any)
	ScanStarted(kind string)
	ScanCompleted(data map[string]any)
	ScanFailed(errMsg string)
}

// CandidatesInvalidator is notified when media or job state changes affect the
// candidate list. Satisfied by api.Server.
type CandidatesInvalidator interface {
	InvalidateCandidates()
}

// Scan trigger values passed to Scan and recorded in scan_runs.
const (
	TriggerStartup   = "startup"
	TriggerScheduled = "scheduled"
	TriggerManual    = "manual"
)

// Scan kinds sent in scan_started WS payloads and POST /api/scan responses.
const (
	ScanKindIncremental = "incremental"
	ScanKindFull        = "full"
)

// ErrScanInProgress is returned when a scan is requested while another is active.
var ErrScanInProgress = errors.New("scan already in progress")

const scanProgressMinInterval = time.Second

// ScanKind returns the wire kind for a scan given whether it is forced (full).
func ScanKind(force bool) string {
	if force {
		return ScanKindFull
	}
	return ScanKindIncremental
}

var mediaExtensions = map[string]struct{}{
	".mkv": {}, ".mp4": {}, ".avi": {}, ".m4v": {},
	".ts": {}, ".wmv": {}, ".mov": {}, ".flv": {},
	".webm": {}, ".m2ts": {}, ".mpg": {}, ".mpeg": {},
}

func isMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := mediaExtensions[ext]
	return ok
}

// ProbeFunc is the probe function used by the scanner. Defaults to ffprobe.Probe;
// injectable for testing.
type ProbeFunc func(ctx context.Context, path string) (*ffprobe.Result, error)

// Scanner indexes media files, maintains the DB, and drives the fsnotify watcher.
type Scanner struct {
	store      *store.Store
	roots      map[string]string     // mountPath -> libraryType
	hub        Broadcaster           // nil-safe; set via SetBroadcaster
	invalidate CandidatesInvalidator // nil-safe; set via SetCandidateInvalidator

	probeFunc    ProbeFunc
	scanInterval time.Duration
	scanAnchor   string

	// scanIntervalFn / scanAnchorFn return the current scheduled-rescan interval
	// and anchor time. They default to the boot values but can be backed by the
	// live config so a settings change reschedules without a restart.
	scanIntervalFn func() time.Duration
	scanAnchorFn   func() string

	// probe bounds concurrent ffprobe subprocesses across every probe entry point
	// (walk, fsnotify watcher, scheduled/manual scans). Capacity comes from the
	// live config when wired via WithLiveConfig. The limit exists to spare the
	// NAS/storage, not Go — goroutines fan out freely and queue here before
	// touching disk.
	probe *probeGate

	// scanMu / scanRunning serialize Scan and StartScan so startup, scheduled,
	// and manual triggers cannot overlap.
	scanMu      sync.Mutex
	scanRunning bool

	// fsnotify watcher and per-path debounce timers
	watcher        *fsnotify.Watcher
	debounceDur    time.Duration // for create/modify events
	debounceRemDur time.Duration // for remove/rename events (slightly longer)
	timerMu        sync.Mutex
	probeTimers    map[string]*time.Timer
	removeTimers   map[string]*time.Timer
}

// Option is a functional option for New.
type Option func(*Scanner)

// WithProbeFunc overrides the probe function (use in tests to avoid ffprobe dependency).
func WithProbeFunc(fn ProbeFunc) Option {
	return func(s *Scanner) { s.probeFunc = fn }
}

// SetBroadcaster wires the hub after construction. Used in main.go where the
// API server (and its hub) is created after the scanner.
func (s *Scanner) SetBroadcaster(b Broadcaster) { s.hub = b }

// SetCandidateInvalidator wires cache invalidation after construction.
func (s *Scanner) SetCandidateInvalidator(inv CandidatesInvalidator) { s.invalidate = inv }

func (s *Scanner) invalidateCandidates() {
	if s.invalidate != nil {
		s.invalidate.InvalidateCandidates()
	}
}

func (s *Scanner) tvRoot() string {
	for path, lt := range s.roots {
		if lt == store.LibraryTypeTV {
			return path
		}
	}
	return ""
}

// WithDebounceDur overrides the debounce window (use in tests to avoid 30s waits).
// The remove debounce is set to dur+5s so creates settle before vanished checks run.
func WithDebounceDur(dur time.Duration) Option {
	return func(s *Scanner) {
		s.debounceDur = dur
		s.debounceRemDur = dur + 5*time.Second
	}
}

// liveScanConfig is the subset of config.Live the scanner reads. Declared here
// rather than importing config.Live directly so the option stays test-friendly.
type liveScanConfig interface {
	ScanInterval() time.Duration
	ScanAnchor() string
	ProbeConcurrency() int
}

// WithLiveConfig backs the scheduled-rescan interval, anchor, and probe
// concurrency with the live config so PUT /api/settings takes effect without
// a restart.
func WithLiveConfig(live liveScanConfig) Option {
	return func(s *Scanner) {
		s.scanIntervalFn = live.ScanInterval
		s.scanAnchorFn = live.ScanAnchor
		s.probe.capFn = live.ProbeConcurrency
	}
}

// durationUntilNext returns the wait time until the next clock-aligned scan.
// The interval is anchored to anchorHHMM (e.g. "00:00") so scans recur at
// predictable wall-clock times regardless of when the container started.
func durationUntilNext(now time.Time, interval time.Duration, anchorHHMM string) time.Duration {
	var h, m int
	fmt.Sscanf(anchorHHMM, "%d:%d", &h, &m) //nolint:errcheck — validated on write
	anchorToday := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	elapsed := now.Sub(anchorToday) % interval
	if elapsed < 0 {
		elapsed += interval
	}
	remaining := interval - elapsed
	if remaining == 0 {
		remaining = interval
	}
	return remaining
}

// New creates a Scanner. Call Start to activate the watcher and scheduled rescan.
func New(st *store.Store, cfg *config.Config, opts ...Option) (*Scanner, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	bootProbeConcurrency := cfg.ProbeConcurrency
	s := &Scanner{
		store: st,
		roots: map[string]string{
			cfg.MoviesPath: store.LibraryTypeMovies,
			cfg.TVPath:     store.LibraryTypeTV,
		},
		probeFunc:    ffprobe.Probe,
		probe:        newProbeGate(func() int { return bootProbeConcurrency }),
		scanInterval: cfg.ScanInterval,
		scanAnchor:   cfg.ScanAnchor,
		watcher:      w,
		debounceDur:  30 * time.Second,
		debounceRemDur: 35 * time.Second,
		probeTimers:  make(map[string]*time.Timer),
		removeTimers: make(map[string]*time.Timer),
	}
	for _, o := range opts {
		o(s)
	}
	if s.scanIntervalFn == nil {
		s.scanIntervalFn = func() time.Duration { return s.scanInterval }
	}
	if s.scanAnchorFn == nil {
		s.scanAnchorFn = func() string { return s.scanAnchor }
	}
	return s, nil
}

func (s *Scanner) tryBeginScan() bool {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	if s.scanRunning {
		return false
	}
	s.scanRunning = true
	return true
}

func (s *Scanner) endScan() {
	s.scanMu.Lock()
	s.scanRunning = false
	s.scanMu.Unlock()
}

// Scan runs one complete diff scan. trigger describes the initiating action
// ("startup", "scheduled", "manual"). force skips the (size, mtime) equality
// check and re-probes every file. Returns ErrScanInProgress if another scan
// is already running.
func (s *Scanner) Scan(ctx context.Context, trigger string, force bool) (*store.ScanRun, error) {
	if !s.tryBeginScan() {
		return nil, ErrScanInProgress
	}
	defer s.endScan()
	return s.scanWithLifecycle(ctx, trigger, force)
}

// StartScan begins a scan in the background. Returns ErrScanInProgress without
// starting when a scan is already active. Used by the API so 409 can be
// returned before the handler exits.
func (s *Scanner) StartScan(ctx context.Context, trigger string, force bool) error {
	if !s.tryBeginScan() {
		return ErrScanInProgress
	}
	go func() {
		defer s.endScan()
		if _, err := s.scanWithLifecycle(ctx, trigger, force); err != nil {
			slog.Error("scanner: scan failed", "trigger", trigger, "err", err)
		}
	}()
	return nil
}

func (s *Scanner) scanWithLifecycle(ctx context.Context, trigger string, force bool) (*store.ScanRun, error) {
	kind := ScanKind(force)
	if s.hub != nil {
		s.hub.ScanStarted(kind)
	}

	run, err := s.scan(ctx, trigger, force)

	if s.hub != nil {
		if err != nil {
			s.hub.ScanFailed(err.Error())
		} else {
			s.hub.ScanCompleted(map[string]any{
				"scan_run_id":   run.ID,
				"files_scanned": run.FilesScanned,
				"files_added":   run.FilesAdded,
				"files_updated": run.FilesUpdated,
				"files_moved":   run.FilesMoved,
				"files_removed": run.FilesRemoved,
				"errors":        run.Errors,
			})
		}
	}
	return run, err
}

func (s *Scanner) scan(ctx context.Context, trigger string, force bool) (*store.ScanRun, error) {
	startedAt := time.Now().Unix()
	runID, err := s.store.Scans.Create(ctx, trigger, startedAt)
	if err != nil {
		return nil, err
	}

	known, err := s.store.Media.ActiveFileSummaries(ctx)
	if err != nil {
		return nil, err
	}

	var (
		seen, scanned, added, updated, moved, removed, errs int64
	)

	var progressMu sync.Mutex
	var lastProgress time.Time
	emitProgress := func(flush bool) {
		if s.hub == nil {
			return
		}
		now := time.Now()
		progressMu.Lock()
		if !flush && !lastProgress.IsZero() && now.Sub(lastProgress) < scanProgressMinInterval {
			progressMu.Unlock()
			return
		}
		lastProgress = now
		progressMu.Unlock()

		filesScanned := atomic.LoadInt64(&scanned)
		filesAdded := atomic.LoadInt64(&added)
		filesUpdated := atomic.LoadInt64(&updated)
		filesRemoved := atomic.LoadInt64(&removed)
		filesMoved := atomic.LoadInt64(&moved)
		errorCount := atomic.LoadInt64(&errs)
		s.hub.Broadcast("scan_progress", map[string]any{
			"scan_run_id":     runID,
			"kind":            ScanKind(force),
			"trigger":         trigger,
			"started_at":      startedAt,
			"files_seen":      atomic.LoadInt64(&seen),
			"files_processed": filesScanned + filesAdded + filesUpdated + errorCount,
			"files_scanned":   filesScanned,
			"files_added":     filesAdded,
			"files_updated":   filesUpdated,
			"files_moved":     filesMoved,
			"files_removed":   filesRemoved,
			"errors":          errorCount,
		})
	}

	// seenPaths is written only in the sequential WalkDir callback, so no mutex needed.
	seenPaths := make(map[string]struct{})

	// newlyInserted maps fingerprint -> {id, path} for files inserted this scan;
	// written by probe goroutines → protected by newMu.
	type newInsert struct {
		id   int64
		path string
	}
	newlyInserted := make(map[string]newInsert)
	var newMu sync.Mutex

	type probeWork struct {
		path  string
		lt    string
		size  int64
		mtime int64
		rec   *store.FileSummary
	}

	workerN := s.probe.capacity()
	tasks := make(chan probeWork, workerN*2)
	var poolWg sync.WaitGroup
	for range workerN {
		poolWg.Add(1)
		go func() {
			defer poolWg.Done()
			for task := range tasks {
				fp, newID, isNew := s.probeAndStore(ctx, task.path, task.lt, task.size, task.mtime, task.rec)
				if newID < 0 {
					atomic.AddInt64(&errs, 1)
					emitProgress(false)
					continue
				}
				if isNew {
					atomic.AddInt64(&added, 1)
					if fp != "" {
						newMu.Lock()
						newlyInserted[fp] = newInsert{id: newID, path: task.path}
						newMu.Unlock()
					}
				} else {
					atomic.AddInt64(&updated, 1)
				}
				emitProgress(false)
			}
		}()
	}

	for root, libraryType := range s.roots {
		lt := libraryType // capture
		werr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				slog.Warn("scanner: walk error", "path", path, "err", err)
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !isMediaFile(path) {
				return nil
			}
			atomic.AddInt64(&seen, 1)

			info, err := d.Info()
			if err != nil {
				slog.Warn("scanner: stat error", "path", path, "err", err)
				atomic.AddInt64(&errs, 1)
				emitProgress(false)
				return nil
			}

			seenPaths[path] = struct{}{}
			size := info.Size()
			mtime := info.ModTime().Unix()
			rec := known[path]

			if !force && rec != nil && rec.SizeBytes == size && rec.Mtime == mtime {
				atomic.AddInt64(&scanned, 1)
				emitProgress(false)
				return nil
			}

			select {
			case tasks <- probeWork{path: path, lt: lt, size: size, mtime: mtime, rec: rec}:
			case <-ctx.Done():
				return ctx.Err()
			}

			return nil
		})
		if werr != nil {
			slog.Error("scanner: walk failed", "root", root, "err", werr)
			atomic.AddInt64(&errs, 1)
			emitProgress(false)
		}
	}

	close(tasks)
	poolWg.Wait()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Rename detection: vanished paths.
	// All inserts are done; GetByFingerprintOtherThan will find the new row if
	// the file was renamed.
	for path, rec := range known {
		if _, seen := seenPaths[path]; seen {
			continue
		}

		// Without a fingerprint we can't distinguish a rename from a delete.
		if rec.Fingerprint == "" {
			if merr := s.store.Media.MarkMissing(ctx, rec.ID); merr != nil {
				slog.Error("scanner: mark missing (no fingerprint)", "path", path, "err", merr)
				atomic.AddInt64(&errs, 1)
			} else {
				atomic.AddInt64(&removed, 1)
			}
			emitProgress(false)
			continue
		}

		ni, wasInserted := newlyInserted[rec.Fingerprint]
		if wasInserted {
			if merr := s.store.Media.RecordMove(ctx, rec.ID, ni.id, ni.path); merr != nil {
				slog.Error("scanner: record move", "from", path, "to", ni.path, "err", merr)
				atomic.AddInt64(&errs, 1)
			} else {
				atomic.AddInt64(&moved, 1)
			}
			emitProgress(false)
			continue
		}

		// Fall back to DB lookup in case the new path was already present from a
		// previous scan (e.g. watcher probed it before this diff ran).
		newFile, ferr := s.store.Media.GetByFingerprintOtherThan(ctx, rec.Fingerprint, rec.ID)
		if ferr == nil {
			if merr := s.store.Media.RecordMove(ctx, rec.ID, newFile.ID, newFile.Path); merr != nil {
				slog.Error("scanner: record move (db lookup)", "from", path, "to", newFile.Path, "err", merr)
				atomic.AddInt64(&errs, 1)
			} else {
				atomic.AddInt64(&moved, 1)
			}
			emitProgress(false)
			continue
		}

		if merr := s.store.Media.MarkMissing(ctx, rec.ID); merr != nil {
			slog.Error("scanner: mark missing", "path", path, "err", merr)
			atomic.AddInt64(&errs, 1)
		} else {
			atomic.AddInt64(&removed, 1)
		}
		emitProgress(false)
	}

	// Reconcile incrementally-maintained library_stats against the source of
	// truth after force rescans (which re-probe everything) and after scheduled
	// rescans (drift guard: incremental deltas can accumulate errors over time).
	if force || trigger == TriggerScheduled {
		if err := s.store.Stats.Recompute(ctx); err != nil {
			slog.Error("scanner: stats recompute", "err", err)
			atomic.AddInt64(&errs, 1)
		}
	}
	emitProgress(true)

	now := time.Now().Unix()
	run := &store.ScanRun{
		ID:           runID,
		Trigger:      trigger,
		StartedAt:    startedAt,
		CompletedAt:  &now,
		FilesScanned: int(atomic.LoadInt64(&scanned)),
		FilesAdded:   int(atomic.LoadInt64(&added)),
		FilesUpdated: int(atomic.LoadInt64(&updated)),
		FilesMoved:   int(atomic.LoadInt64(&moved)),
		FilesRemoved: int(atomic.LoadInt64(&removed)),
		Errors:       int(atomic.LoadInt64(&errs)),
	}
	if err := s.store.Scans.Complete(ctx, run); err != nil {
		slog.Error("scanner: complete scan_run", "id", runID, "err", err)
	}

	severity := store.SeverityInfo
	if run.Errors > 0 {
		severity = store.SeverityError
	}
	scanMsg := scanEventMessage(run)
	scanMeta := scanJsonMeta(map[string]any{
		"scan_run_id":   run.ID,
		"files_scanned": run.FilesScanned,
		"files_added":   run.FilesAdded,
		"files_updated": run.FilesUpdated,
		"files_moved":   run.FilesMoved,
		"files_removed": run.FilesRemoved,
		"errors":        run.Errors,
		"trigger":       run.Trigger,
	})
	eventID, err := s.store.Events.Insert(ctx, store.EventScanCompleted, severity, scanMsg, scanMeta)
	if err != nil {
		slog.Error("scanner: scan event", "err", err)
	} else if s.hub != nil {
		s.hub.Broadcast("event_created", scanEventBroadcast(eventID, severity, scanMsg, scanMeta))
	}

	slog.Info("scan complete",
		"trigger", trigger,
		"scanned", run.FilesScanned,
		"added", run.FilesAdded,
		"updated", run.FilesUpdated,
		"moved", run.FilesMoved,
		"removed", run.FilesRemoved,
		"errors", run.Errors,
	)
	s.invalidateCandidates()
	return run, nil
}

// probeAndStore probes path and inserts or updates the DB row.
// Returns the fingerprint, the row ID (or -1 on error), and whether it was a new insert.
func (s *Scanner) probeAndStore(
	ctx context.Context,
	path, libraryType string,
	size, mtime int64,
	existing *store.FileSummary,
) (fp string, rowID int64, isNew bool) {
	// Hold a semaphore slot for the whole probe: both ffprobe and the fingerprint
	// read below hit storage, so bounding the pair is what actually protects the
	// NAS. The DB write is already serialized by the single-writer pool, so
	// keeping it inside the slot is harmless and keeps this the one chokepoint
	// every entry point passes through.
	if err := s.probe.acquire(ctx); err != nil {
		return "", -1, false
	}
	defer s.probe.release()

	result, err := s.probeFunc(ctx, path)

	fp, ferr := media.Fingerprint(path)
	if ferr != nil {
		slog.Warn("scanner: fingerprint error", "path", path, "err", ferr)
	}

	var probeErr *string
	if err != nil {
		msg := err.Error()
		probeErr = &msg
		slog.Warn("scanner: probe error", "path", path, "err", err)
	}

	f := &store.MediaFile{
		Path:        path,
		LibraryType: libraryType,
		SizeBytes:   size,
		Mtime:       mtime,
		Fingerprint: fp,
		ProbeError:  probeErr,
		Status:      store.MediaStatusActive,
	}

	if libraryType == store.LibraryTypeTV {
		title, season, _ := media.ParseTVInfo(path, s.tvRoot())
		if title != "" {
			f.SeriesTitle = &title
		}
		if season >= 0 {
			f.SeasonNumber = &season
		}
	}

	if result != nil {
		now := time.Now().Unix()
		f.VideoCodec = result.VideoCodec
		f.VideoCodecProfile = result.VideoCodecProfile
		f.Width = result.Width
		f.Height = result.Height
		f.DurationSeconds = result.DurationSeconds
		f.BitrateKbps = result.BitrateKbps
		f.AudioCodec = result.AudioCodec
		f.AudioChannels = result.AudioChannels
		f.ContainerFormat = result.ContainerFormat
		f.IsAlreadyHEVC = result.IsAlreadyHEVC
		f.LastProbedAt = &now
		// Compute the savings estimate at probe time and store it so the
		// candidate ranking and dashboard never have to recompute it per query.
		f.PredictedSavingsBytes = media.PredictedSavingsBytes(
			result.VideoCodec, result.IsAlreadyHEVC, size,
		)
	}

	if existing == nil {
		id, err := s.store.Media.Insert(ctx, f)
		if err != nil {
			// A row may already exist at this path with a non-active status —
			// the watcher marks a file "missing" when it briefly vanishes, and
			// ActiveFileSummaries only loads active rows, so a resurrected file
			// looks new and collides with the UNIQUE(path) constraint. Reconcile
			// by updating the existing row in place (which also reactivates it).
			if prior, gerr := s.store.Media.GetByPath(ctx, path); gerr == nil {
				f.ID = prior.ID
				if uerr := s.store.Media.UpdateProbe(ctx, f); uerr != nil {
					slog.Error("scanner: update probe (resurrected)", "path", path, "err", uerr)
					return fp, -1, false
				}
				return fp, prior.ID, false
			}
			slog.Error("scanner: insert", "path", path, "err", err)
			return fp, -1, true
		}
		return fp, id, true
	}

	f.ID = existing.ID
	if err := s.store.Media.UpdateProbe(ctx, f); err != nil {
		slog.Error("scanner: update probe", "path", path, "err", err)
		return fp, -1, false
	}
	return fp, existing.ID, false
}

// Start launches the fsnotify watcher and scheduled rescan loops. Blocks until
// ctx is cancelled.
func (s *Scanner) Start(ctx context.Context) {
	s.logNetworkMountWarning()

	for root := range s.roots {
		if err := s.addDirWatch(root); err != nil {
			slog.Warn("scanner: failed to add root watch", "path", root, "err", err)
		}
	}

	// A resettable timer (rather than a fixed ticker) lets each scheduled rescan
	// pick up a live SCAN_INTERVAL change applied via PUT /api/settings.
	timer := time.NewTimer(durationUntilNext(time.Now(), s.scanIntervalFn(), s.scanAnchorFn()))
	defer timer.Stop()

	// Initial scan on startup. Backfill series metadata first so the browse
	// page shows all TV shows immediately, even for files not yet re-probed.
	go func() {
		if err := s.store.Media.BackfillSeriesMeta(ctx, s.tvRoot()); err != nil {
			slog.Warn("scanner: series meta backfill failed", "err", err)
		}
		if _, err := s.Scan(ctx, TriggerStartup, false); err != nil {
			if errors.Is(err, ErrScanInProgress) {
				slog.Debug("scanner: startup scan skipped, another scan in progress")
			} else {
				slog.Error("scanner: startup scan failed", "err", err)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			_ = s.watcher.Close()
			return
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			s.handleWatchEvent(ctx, event)
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("scanner: watcher error", "err", err)
		case <-timer.C:
			go func() {
				if _, err := s.Scan(ctx, TriggerScheduled, false); err != nil {
					if errors.Is(err, ErrScanInProgress) {
						slog.Debug("scanner: scheduled scan skipped, another scan in progress")
					} else {
						slog.Error("scanner: scheduled scan failed", "err", err)
					}
				}
			}()
			timer.Reset(durationUntilNext(time.Now(), s.scanIntervalFn(), s.scanAnchorFn()))
		}
	}
}

// handleWatchEvent classifies the event and queues the appropriate debounced action.
func (s *Scanner) handleWatchEvent(ctx context.Context, event fsnotify.Event) {
	path := event.Name

	// New directory: add recursive watch.
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			if err := s.addDirWatch(path); err != nil {
				slog.Warn("scanner: add new dir watch", "path", path, "err", err)
			}
			return
		}
	}

	if !isMediaFile(path) {
		return
	}

	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		s.debounceRemoveEvent(ctx, path)
	} else {
		s.debounceProbeEvent(ctx, path)
	}
}

func (s *Scanner) debounceProbeEvent(ctx context.Context, path string) {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	if t, ok := s.probeTimers[path]; ok {
		t.Stop()
	}
	s.probeTimers[path] = time.AfterFunc(s.debounceDur, func() {
		s.timerMu.Lock()
		delete(s.probeTimers, path)
		s.timerMu.Unlock()
		s.probeSingleFile(ctx, path)
	})
}

func (s *Scanner) debounceRemoveEvent(ctx context.Context, path string) {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	if t, ok := s.removeTimers[path]; ok {
		t.Stop()
	}
	s.removeTimers[path] = time.AfterFunc(s.debounceRemDur, func() {
		s.timerMu.Lock()
		delete(s.removeTimers, path)
		s.timerMu.Unlock()
		s.checkVanishedFile(ctx, path)
	})
}

// probeSingleFile probes one file and inserts/updates the DB row.
// Called from the watcher debounce timer goroutine.
func (s *Scanner) probeSingleFile(ctx context.Context, path string) {
	info, err := os.Stat(path)
	if err != nil {
		// File disappeared between the event and the timer firing.
		s.checkVanishedFile(ctx, path)
		return
	}

	existing, _ := s.store.Media.GetByPath(ctx, path)
	var rec *store.FileSummary
	if existing != nil {
		rec = &store.FileSummary{
			ID:          existing.ID,
			SizeBytes:   existing.SizeBytes,
			Mtime:       existing.Mtime,
			Fingerprint: existing.Fingerprint,
		}
	}

	lt := s.libraryTypeFor(path)
	if _, id, _ := s.probeAndStore(ctx, path, lt, info.Size(), info.ModTime().Unix(), rec); id >= 0 {
		s.invalidateCandidates()
	}
}

// checkVanishedFile handles a file that no longer exists on disk: if another
// active row has the same fingerprint the file was renamed; otherwise missing.
func (s *Scanner) checkVanishedFile(ctx context.Context, path string) {
	f, err := s.store.Media.GetByPath(ctx, path)
	if err != nil {
		return // not in DB
	}
	if f.Status != store.MediaStatusActive {
		return
	}
	if f.Fingerprint == "" {
		if err := s.store.Media.MarkMissing(ctx, f.ID); err == nil {
			s.invalidateCandidates()
		}
		return
	}

	newFile, err := s.store.Media.GetByFingerprintOtherThan(ctx, f.Fingerprint, f.ID)
	if err == nil {
		if merr := s.store.Media.RecordMove(ctx, f.ID, newFile.ID, newFile.Path); merr != nil {
			slog.Error("scanner: watcher record move", "from", path, "to", newFile.Path, "err", merr)
		} else {
			s.invalidateCandidates()
		}
		return
	}

	if merr := s.store.Media.MarkMissing(ctx, f.ID); merr != nil {
		slog.Error("scanner: watcher mark missing", "path", path, "err", merr)
	} else {
		s.invalidateCandidates()
	}
}

// libraryTypeFor returns the library type of the root that contains path.
func (s *Scanner) libraryTypeFor(path string) string {
	for root, lt := range s.roots {
		if strings.HasPrefix(path, root+string(os.PathSeparator)) || path == root {
			return lt
		}
	}
	return "unknown"
}

// addDirWatch adds path and all its subdirectories to the fsnotify watcher.
func (s *Scanner) addDirWatch(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable dirs
		}
		if !d.IsDir() {
			return nil
		}
		if werr := s.watcher.Add(path); werr != nil {
			slog.Warn("scanner: add watch", "path", path, "err", werr)
		}
		return nil
	})
}

// logNetworkMountWarning reads /proc/mounts and warns if any media root appears
// to be on an NFS or SMB mount where inotify events may not fire reliably.
func (s *Scanner) logNetworkMountWarning() {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return // not Linux or not readable
	}
	defer f.Close()

	networkFS := map[string]bool{
		"nfs": true, "nfs4": true, "cifs": true, "smbfs": true, "smb2": true,
	}

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		mountPoint, fsType := fields[1], fields[2]
		if !networkFS[fsType] {
			continue
		}
		for root := range s.roots {
			if strings.HasPrefix(root, mountPoint) {
				slog.Warn("scanner: media root is on a network filesystem — inotify events may not fire reliably; rely on the scheduled rescan",
					"root", root, "fstype", fsType, "mountpoint", mountPoint)
			}
		}
	}
}

func scanTriggerLabel(trigger string) string {
	switch trigger {
	case TriggerManual:
		return "Manual"
	case TriggerScheduled:
		return "Scheduled"
	case TriggerStartup:
		return "Startup"
	default:
		return trigger
	}
}

func scanEventMessage(run *store.ScanRun) string {
	label := scanTriggerLabel(run.Trigger)
	totalChanges := run.FilesAdded + run.FilesUpdated + run.FilesMoved + run.FilesRemoved
	if totalChanges == 0 && run.Errors == 0 {
		return fmt.Sprintf("%s scan: no changes", label)
	}
	msg := fmt.Sprintf("%s scan: %d added, %d updated, %d moved, %d removed",
		label, run.FilesAdded, run.FilesUpdated, run.FilesMoved, run.FilesRemoved)
	if run.Errors > 0 {
		msg += fmt.Sprintf(", %d errors", run.Errors)
	}
	return msg
}

// scanJsonMeta serializes v to a JSON string for events.metadata.
func scanJsonMeta(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// scanEventBroadcast builds the WS payload for an event_created scan event.
func scanEventBroadcast(id int64, severity, message, meta string) map[string]any {
	m := map[string]any{
		"id":         id,
		"type":       store.EventScanCompleted,
		"severity":   severity,
		"message":    message,
		"created_at": time.Now().Unix(),
		"metadata":   nil,
	}
	if meta != "" {
		var v any
		if err := json.Unmarshal([]byte(meta), &v); err == nil {
			m["metadata"] = v
		}
	}
	return m
}
