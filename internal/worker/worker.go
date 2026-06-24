// Package worker executes transcode jobs: it pulls queued work only inside the
// encode window, runs ffmpeg to a temp file, verifies the output before touching
// the original, atomically swaps it in, and cleans up orphaned temp/backup files
// left by a crash.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"reclaim/internal/ffmpeg"
	"reclaim/internal/ffprobe"
	"reclaim/internal/jobs"
	"reclaim/internal/media"
	"reclaim/internal/store"
)

const (
	tmpSuffix    = ".reclaim-tmp"
	backupSuffix = ".reclaim-backup"

	// durationToleranceSeconds is the ±window for the duration match.
	durationToleranceSeconds = 1.0
)

// Broadcaster is the WS hub slice the worker pushes live progress to. Satisfied
// by api.Hub; an interface so the worker is testable without a real hub.
type Broadcaster interface {
	Broadcast(event string, data any)
}

// liveWindow is the runtime-mutable encode window the worker reads on each pull
// so a PUT /api/settings change takes effect without a restart. Satisfied by
// config.Live.
type liveWindow interface {
	EncodeWindowStart() time.Duration
	EncodeWindowEnd() time.Duration
}

// EncodeFunc runs one encode; defaults to ffmpeg.Encode, injectable for tests.
type EncodeFunc func(ctx context.Context, opts ffmpeg.Options, onProgress ffmpeg.ProgressFunc) error

// InspectFunc inspects a file for verification; defaults to ffprobe.Inspect.
type InspectFunc func(ctx context.Context, path string) (*ffprobe.Inspection, error)

// Worker is the single-worker encode loop.
type Worker struct {
	store  *store.Store
	window liveWindow
	hub    Broadcaster
	roots  []string

	encode  EncodeFunc
	inspect InspectFunc
	clock   func() time.Time

	pollInterval     time.Duration
	orphanInterval   time.Duration
	progressDBPeriod time.Duration

	mu     sync.Mutex
	active map[int64]*activeJob // running jobs keyed by job id
}

// activeJob tracks an in-flight encode so a cancel can target it and the orphan
// sweep can avoid deleting a temp that's still being written.
type activeJob struct {
	cancel       context.CancelFunc
	tempPath     string
	userCanceled bool
}

// Option configures a Worker.
type Option func(*Worker)

func WithEncodeFunc(fn EncodeFunc) Option     { return func(w *Worker) { w.encode = fn } }
func WithInspectFunc(fn InspectFunc) Option   { return func(w *Worker) { w.inspect = fn } }
func WithClock(fn func() time.Time) Option    { return func(w *Worker) { w.clock = fn } }
func WithPollInterval(d time.Duration) Option { return func(w *Worker) { w.pollInterval = d } }
func WithOrphanInterval(d time.Duration) Option {
	return func(w *Worker) { w.orphanInterval = d }
}
func WithProgressDBPeriod(d time.Duration) Option {
	return func(w *Worker) { w.progressDBPeriod = d }
}

// New builds a Worker. roots are the media mount roots swept for orphans.
func New(st *store.Store, window liveWindow, hub Broadcaster, roots []string, opts ...Option) *Worker {
	w := &Worker{
		store:            st,
		window:           window,
		hub:              hub,
		roots:            roots,
		encode:           ffmpeg.Encode,
		inspect:          ffprobe.Inspect,
		clock:            time.Now,
		pollInterval:     5 * time.Second,
		orphanInterval:   time.Hour,
		progressDBPeriod: time.Second,
		active:           make(map[int64]*activeJob),
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// Run drives the worker until ctx is cancelled. On entry it reconciles any jobs
// left in flight by a crash and sweeps orphaned files, then loops: inside the
// encode window it pulls and runs the oldest queued job; otherwise it waits.
func (w *Worker) Run(ctx context.Context) {
	w.reconcileInterrupted(ctx)
	w.sweepOrphans(ctx)

	poll := time.NewTicker(w.pollInterval)
	defer poll.Stop()
	orphan := time.NewTicker(w.orphanInterval)
	defer orphan.Stop()

	for {
		// Always drain forced jobs regardless of the encode window.
		for {
			job, err := w.store.Jobs.ClaimNextForcedQueued(ctx, w.clock().Unix())
			if errors.Is(err, store.ErrNotFound) {
				break
			}
			if err != nil {
				slog.Error("worker: claim forced job", "err", err)
				break
			}
			w.processJob(ctx, job)
			if ctx.Err() != nil {
				return
			}
		}

		// Drain the queue while inside the window, one job at a time.
		for w.withinWindow() {
			job, err := w.store.Jobs.ClaimNextQueued(ctx, w.clock().Unix())
			if errors.Is(err, store.ErrNotFound) {
				break // queue empty
			}
			if err != nil {
				slog.Error("worker: claim job", "err", err)
				break
			}
			w.processJob(ctx, job)
			if ctx.Err() != nil {
				return
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-poll.C:
		case <-orphan.C:
			w.sweepOrphans(ctx)
		}
	}
}

// Cancel requests cancellation of a running job. It returns true if the job was
// actively running (its ffmpeg killed); false means the worker isn't running it
// (the caller should cancel a merely-queued job via the DB itself).
func (w *Worker) Cancel(jobID int64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	aj, ok := w.active[jobID]
	if !ok {
		return false
	}
	aj.userCanceled = true
	aj.cancel()
	return true
}

// withinWindow reports whether the encode window is currently open. The window
// is read live so a settings change applies immediately. A zero-length window
// (start == end) is treated as always-open.
func (w *Worker) withinWindow() bool {
	start := w.window.EncodeWindowStart()
	end := w.window.EncodeWindowEnd()
	if start == end {
		return true
	}
	now := w.clock()
	mins := time.Duration(now.Hour())*time.Hour + time.Duration(now.Minute())*time.Minute
	if start < end {
		return mins >= start && mins < end
	}
	// Overnight window wraps midnight.
	return mins >= start || mins < end
}

// processJob runs one job end-to-end: encode → verify → replace. The job is
// already in `running` (ClaimNextQueued set it). A running job is never
// interrupted by the window closing — only new pulls are gated.
func (w *Worker) processJob(ctx context.Context, job *store.TranscodeJob) {
	file, err := w.store.Media.GetByID(ctx, job.MediaFileID)
	if err != nil {
		w.failJob(ctx, job.ID, "media file not found: "+err.Error(), nil)
		return
	}
	profile, err := w.store.Profiles.GetByID(ctx, job.ProfileID)
	if err != nil {
		w.failJob(ctx, job.ID, "profile not found: "+err.Error(), nil)
		return
	}

	tmpPath := tempPathFor(file.Path)
	if err := w.store.Jobs.SetOutputPath(ctx, job.ID, tmpPath); err != nil {
		slog.Error("worker: set output path", "job", job.ID, "err", err)
	}

	encCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	aj := &activeJob{cancel: cancel, tempPath: tmpPath}
	w.mu.Lock()
	w.active[job.ID] = aj
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		delete(w.active, job.ID)
		w.mu.Unlock()
	}()

	w.hub.Broadcast("job_started", map[string]any{"job_id": job.ID, "media_file_id": file.ID})

	var duration float64
	if file.DurationSeconds != nil {
		duration = *file.DurationSeconds
	}
	var extra []string
	if profile.ExtraArgs != nil && strings.TrimSpace(*profile.ExtraArgs) != "" {
		extra = strings.Fields(*profile.ExtraArgs)
	}

	lastDB := time.Time{}
	encErr := w.encode(encCtx, ffmpeg.Options{
		InputPath:       file.Path,
		OutputPath:      tmpPath,
		CRF:             profile.CRF,
		Preset:          profile.Preset,
		ExtraArgs:       extra,
		DurationSeconds: duration,
	}, func(pct float64) {
		// One decimal place is plenty for a progress bar and keeps the WS payload
		// and the persisted REAL tidy (no 17-digit float noise).
		pct = math.Round(pct*10) / 10
		w.hub.Broadcast("job_progress", map[string]any{"job_id": job.ID, "percent": pct})
		if now := w.clock(); now.Sub(lastDB) >= w.progressDBPeriod {
			lastDB = now
			if err := w.store.Jobs.UpdateProgress(context.WithoutCancel(encCtx), job.ID, pct); err != nil {
				slog.Warn("worker: persist progress", "job", job.ID, "err", err)
			}
		}
	})

	w.mu.Lock()
	userCanceled := aj.userCanceled
	w.mu.Unlock()

	if encErr != nil {
		switch {
		case userCanceled:
			w.cancelJob(job.ID, tmpPath)
		case ctx.Err() != nil:
			// Worker is shutting down: leave the job running for the next boot's
			// reconcile sweep rather than marking it failed.
			slog.Warn("worker: shutdown mid-encode; leaving for reconcile", "job", job.ID)
		default:
			removeIfExists(tmpPath)
			w.failJob(ctx, job.ID, "encode failed: "+encErr.Error(), nil)
		}
		return
	}

	// Encode succeeded → verify before touching the original.
	if err := w.store.Jobs.Transition(ctx, job.ID, string(jobs.StatusRunning), string(jobs.StatusVerifying)); err != nil {
		slog.Error("worker: transition to verifying", "job", job.ID, "err", err)
	}
	w.verifyAndReplace(ctx, job, file, tmpPath)
}

// verifyAndReplace runs the verification checks and, only on a full pass,
// performs the atomic swap. Any failure keeps the temp, leaves the original
// untouched, and marks the job failed with the verification detail attached.
func (w *Worker) verifyAndReplace(ctx context.Context, job *store.TranscodeJob, file *store.MediaFile, tmpPath string) {
	result, ok := w.verify(ctx, file, tmpPath)
	blob, _ := json.Marshal(result)
	if err := w.store.Jobs.SetVerificationResult(ctx, job.ID, string(blob)); err != nil {
		slog.Error("worker: store verification result", "job", job.ID, "err", err)
	}

	if !ok {
		// Keep the temp for inspection; original untouched.
		w.failJob(ctx, job.ID, "verification failed", strptr(string(blob)))
		return
	}

	if err := w.replace(ctx, job, file, tmpPath); err != nil {
		slog.Error("worker: replace", "job", job.ID, "path", file.Path, "err", err)
		w.failJob(ctx, job.ID, "replace failed: "+err.Error(), strptr(string(blob)))
		return
	}
}

// verificationResult is the JSON blob stored on the job.
type verificationResult struct {
	DurationMatch        bool    `json:"duration_match"`
	DurationDeltaSeconds float64 `json:"duration_delta_seconds"`
	Playable             bool    `json:"playable"`
	StreamCountMatch     bool    `json:"stream_count_match"`
	ResolutionMatch      bool    `json:"resolution_match"`
	Passed               bool    `json:"passed"`
}

// verify runs the verification checks against the temp output, re-probing the
// original on disk for the freshest comparison truth.
func (w *Worker) verify(ctx context.Context, file *store.MediaFile, tmpPath string) (verificationResult, bool) {
	var res verificationResult

	out, err := w.inspect(ctx, tmpPath)
	if err != nil {
		// Not playable: ffprobe can't read the output. All other checks moot.
		res.Playable = false
		return res, false
	}
	res.Playable = true

	src, err := w.inspect(ctx, file.Path)
	if err != nil {
		// Can't read the source to compare against — refuse to swap.
		return res, false
	}

	res.DurationDeltaSeconds = math.Abs(src.DurationSeconds - out.DurationSeconds)
	res.DurationMatch = res.DurationDeltaSeconds <= durationToleranceSeconds

	res.StreamCountMatch = src.VideoStreams == out.VideoStreams &&
		src.AudioStreams == out.AudioStreams &&
		src.SubtitleStreams == out.SubtitleStreams

	res.ResolutionMatch = src.Width == out.Width && src.Height == out.Height

	res.Passed = res.DurationMatch && res.Playable && res.StreamCountMatch && res.ResolutionMatch
	return res, res.Passed
}

// replace performs the atomic swap: original → .reclaim-backup, temp →
// original, delete backup, then update the row + stats. The backup window means
// a failure mid-swap is recoverable rather than leaving no original.
func (w *Worker) replace(ctx context.Context, job *store.TranscodeJob, file *store.MediaFile, tmpPath string) error {
	backupPath := file.Path + backupSuffix

	if err := os.Rename(file.Path, backupPath); err != nil {
		return err // original untouched, temp kept
	}
	if err := os.Rename(tmpPath, file.Path); err != nil {
		// Step 2 failed: restore the original from backup so we never lose it.
		if rerr := os.Rename(backupPath, file.Path); rerr != nil {
			slog.Error("worker: CRITICAL restore failed", "path", file.Path, "backup", backupPath, "err", rerr)
		}
		return err
	}
	if err := os.Remove(backupPath); err != nil {
		// Non-fatal: the swap is done; the orphan sweep will delete the backup.
		slog.Warn("worker: remove backup", "path", backupPath, "err", err)
	}

	info, err := os.Stat(file.Path)
	if err != nil {
		return err
	}
	newSize := info.Size()
	fp, err := media.Fingerprint(file.Path)
	if err != nil {
		slog.Warn("worker: fingerprint after swap", "path", file.Path, "err", err)
	}

	now := w.clock().Unix()
	if err := w.store.Media.ReplaceWithEncoded(ctx, file.ID, newSize, fp, now); err != nil {
		return err
	}

	// Bundle MarkCompleted + event insert in one transaction.
	completedMsg := "Encoded " + filepath.Base(file.Path)
	completedMeta := jsonMeta(map[string]any{
		"job_id":              job.ID,
		"file_id":             file.ID,
		"output_size_bytes":   newSize,
		"original_size_bytes": file.SizeBytes,
	})
	eventID, err := w.store.CompleteJob(ctx, job.ID, newSize, now, completedMsg, completedMeta)
	if err != nil {
		slog.Error("worker: complete job", "job", job.ID, "err", err)
	} else {
		w.hub.Broadcast("event_created", eventBroadcast(eventID, store.EventJobCompleted, store.SeverityInfo, completedMsg, completedMeta, now))
	}

	// After completing a job, check whether this codec now has enough samples
	// to override its seed savings estimate with observed data.
	if file.VideoCodec != nil {
		if err := w.refineRatioIfReady(ctx, *file.VideoCodec); err != nil {
			slog.Warn("worker: savings refinement", "codec", *file.VideoCodec, "err", err)
		}
	}

	w.hub.Broadcast("job_completed", map[string]any{
		"job_id":            job.ID,
		"media_file_id":     file.ID,
		"output_size_bytes": newSize,
	})
	return nil
}

// refineRatioIfReady checks whether the given source codec has accumulated
// enough completed jobs to replace its seed savings ratio with an observed one.
// If so, it batch-updates predicted_savings_bytes for all active files with
// that codec and reconciles library_stats.
func (w *Worker) refineRatioIfReady(ctx context.Context, codec string) error {
	learned, err := w.store.Jobs.LearnedRatios(ctx, store.LearnedRatioMinSamples)
	if err != nil {
		return err
	}
	lr, ok := learned[strings.ToLower(codec)]
	if !ok {
		return nil
	}
	n, err := w.store.Media.UpdatePredictedSavingsByCodec(ctx, strings.ToLower(codec), lr.Ratio)
	if err != nil {
		return err
	}
	if n > 0 {
		slog.Info("worker: refined savings model",
			"codec", codec, "ratio", lr.Ratio,
			"samples", lr.SampleCount, "files_updated", n)
		return w.store.Stats.Recompute(ctx)
	}
	return nil
}

// cancelJob handles a user-cancelled running job: the temp is removed, the
// original is untouched, and the job goes to cancelled.
func (w *Worker) cancelJob(jobID int64, tmpPath string) {
	removeIfExists(tmpPath)
	now := w.clock().Unix()
	meta := jsonMeta(map[string]any{"job_id": jobID})
	// Use a detached context so a worker shutdown doesn't abort the bookkeeping.
	eventID, err := w.store.CancelJob(context.Background(), jobID, now, meta)
	if err != nil {
		slog.Error("worker: cancel job", "job", jobID, "err", err)
	} else {
		w.hub.Broadcast("event_created", eventBroadcast(eventID, store.EventJobCancelled, store.SeverityInfo, "Job cancelled", meta, now))
	}
	w.hub.Broadcast("job_cancelled", map[string]any{"job_id": jobID})
}

// failJob marks a job failed, broadcasts it, and optionally carries the
// verification detail.
func (w *Worker) failJob(ctx context.Context, jobID int64, msg string, verification *string) {
	bg := context.WithoutCancel(ctx)
	now := w.clock().Unix()
	metaData := map[string]any{"job_id": jobID, "error": msg}
	if verification != nil {
		metaData["verification_result"] = *verification
	}
	meta := jsonMeta(metaData)
	eventID, err := w.store.FailJob(bg, jobID, msg, now, meta)
	if err != nil {
		slog.Error("worker: fail job", "job", jobID, "err", err)
	} else {
		w.hub.Broadcast("event_created", eventBroadcast(eventID, store.EventJobFailed, store.SeverityError, "Encode failed: "+msg, meta, now))
	}
	data := map[string]any{"job_id": jobID, "error": msg}
	if verification != nil {
		data["verification_result"] = *verification
	}
	w.hub.Broadcast("job_failed", data)
}

// reconcileInterrupted marks any job left running/verifying by a crash as failed
// and removes its temp output.
func (w *Worker) reconcileInterrupted(ctx context.Context) {
	stuck, err := w.store.Jobs.ListInterrupted(ctx)
	if err != nil {
		slog.Error("worker: list interrupted jobs", "err", err)
		return
	}
	for _, j := range stuck {
		if j.OutputPath != nil {
			removeIfExists(*j.OutputPath)
		}
		const reconcileMsg = "Job interrupted by shutdown"
		meta := jsonMeta(map[string]any{"job_id": j.ID})
		if err := w.store.Jobs.MarkFailed(ctx, j.ID, reconcileMsg, w.clock().Unix()); err != nil {
			slog.Error("worker: reconcile job", "job", j.ID, "err", err)
			continue
		}
		slog.Warn("worker: reconciled interrupted job to failed", "job", j.ID)
		// Event insert is separate (no tx) — reconcile runs at startup before
		// any WS clients are connected so no broadcast is needed.
		if _, err := w.store.Events.Insert(ctx, store.EventJobFailed, store.SeverityWarn, reconcileMsg, meta); err != nil {
			slog.Error("worker: reconcile event", "job", j.ID, "err", err)
		}
	}
}

// sweepOrphans walks the media roots and cleans up stray reclaim files:
// stale .reclaim-tmp are deleted; for each .reclaim-backup the backup is restored
// iff its original is missing (a crash between swap steps), else deleted. Temps
// belonging to an in-flight job are skipped.
func (w *Worker) sweepOrphans(ctx context.Context) {
	active := w.activeTempPaths()

	for _, root := range w.roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.IsDir() {
				return nil
			}
			name := d.Name()
			switch {
			case strings.Contains(name, tmpSuffix):
				if _, busy := active[path]; busy {
					return nil
				}
				if err := os.Remove(path); err != nil {
					slog.Warn("worker: remove stale temp", "path", path, "err", err)
				} else {
					slog.Info("worker: removed stale temp", "path", path)
				}
			case strings.HasSuffix(name, backupSuffix):
				orig := strings.TrimSuffix(path, backupSuffix)
				if _, serr := os.Stat(orig); serr == nil {
					// Original present → backup is leftover from a completed swap.
					if err := os.Remove(path); err != nil {
						slog.Warn("worker: remove stale backup", "path", path, "err", err)
					}
				} else {
					// Original missing → restore it (crash between swap steps).
					if err := os.Rename(path, orig); err != nil {
						slog.Error("worker: restore backup", "backup", path, "orig", orig, "err", err)
					} else {
						slog.Warn("worker: restored backup over missing original", "orig", orig)
						bg := context.Background()
						restoreMsg := "Restored backup over missing original: " + filepath.Base(orig)
						restoreMeta := jsonMeta(map[string]any{"original": orig, "backup": path})
						eventID, err := w.store.Events.Insert(bg, store.EventOrphanRestored, store.SeverityWarn, restoreMsg, restoreMeta)
						if err != nil {
							slog.Error("worker: orphan restored event", "err", err)
						} else {
							w.hub.Broadcast("event_created", eventBroadcast(eventID, store.EventOrphanRestored, store.SeverityWarn, restoreMsg, restoreMeta, time.Now().Unix()))
						}
					}
				}
			}
			return nil
		})
	}
}

func (w *Worker) activeTempPaths() map[string]struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make(map[string]struct{}, len(w.active))
	for _, aj := range w.active {
		out[aj.tempPath] = struct{}{}
	}
	return out
}

// tempPathFor returns the temp output path for an original, preserving the
// extension so ffmpeg muxes the same container (e.g. a.mkv → a.mkv.reclaim-tmp.mkv).
func tempPathFor(orig string) string {
	return orig + tmpSuffix + filepath.Ext(orig)
}

func removeIfExists(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Warn("worker: remove temp", "path", path, "err", err)
	}
}

func strptr(s string) *string { return &s }

// jsonMeta serializes v to a compact JSON string for storage in events.metadata.
func jsonMeta(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// eventBroadcast builds the WS payload for an event_created broadcast so the
// frontend can prepend it to the notifications list without a round-trip.
func eventBroadcast(id int64, eventType, severity, message, meta string, createdAt int64) map[string]any {
	m := map[string]any{
		"id":         id,
		"type":       eventType,
		"severity":   severity,
		"message":    message,
		"created_at": createdAt,
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
