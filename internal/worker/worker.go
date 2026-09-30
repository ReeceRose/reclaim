// Package worker executes transcode jobs: it pulls queued work only inside the
// encode window, runs ffmpeg to a temp file, verifies the output before touching
// the original, atomically swaps it in, and cleans up orphaned temp/backup files
// left by a crash.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"reclaim/internal/config"
	"reclaim/internal/ffmpeg"
	"reclaim/internal/ffprobe"
	"reclaim/internal/jobs"
	"reclaim/internal/media"
	"reclaim/internal/store"
)

const (
	tmpSuffix    = media.TempSuffix
	backupSuffix = media.BackupSuffix

	// durationToleranceSeconds is the ±window for the duration match.
	durationToleranceSeconds = 1.0
)

// errPostSwapCommit marks a DB failure after the filesystem swap succeeded.
// The job is left in verifying for reconcile to retry CommitEncodeSwap.
var errPostSwapCommit = errors.New("post-swap db commit failed")

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
	Location() *time.Location
}

// EncodeFunc runs one encode; defaults to ffmpeg.Encode, injectable for tests.
type EncodeFunc func(ctx context.Context, opts ffmpeg.Options, onProgress ffmpeg.ProgressFunc) error

// InspectFunc inspects a file for verification; defaults to ffprobe.Inspect.
type InspectFunc func(ctx context.Context, path string) (*ffprobe.Inspection, error)

// ProbeFunc probes a file for codec metadata; defaults to ffprobe.Probe.
// Used by post-swap reconcile to detect an encoded file on disk.
type ProbeFunc func(ctx context.Context, path string) (*ffprobe.Result, error)

// Worker is the single-worker encode loop.
type Worker struct {
	store  *store.Store
	window liveWindow
	hub    Broadcaster
	roots  []string

	encode  EncodeFunc
	inspect InspectFunc
	probe   ProbeFunc
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
func WithProbeFunc(fn ProbeFunc) Option       { return func(w *Worker) { w.probe = fn } }
func WithClock(fn func() time.Time) Option    { return func(w *Worker) { w.clock = fn } }
func WithPollInterval(d time.Duration) Option { return func(w *Worker) { w.pollInterval = d } }
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
		probe:            ffprobe.Probe,
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
// and its timezone are read live so a settings change applies immediately. The
// clock is UTC process-wide, so the configured location is what turns it into
// the operator's wall time.
func (w *Worker) withinWindow() bool {
	loc := w.window.Location()
	if loc == nil {
		loc = time.UTC
	}
	open, _ := config.WindowState(w.clock().In(loc), w.window.EncodeWindowStart(), w.window.EncodeWindowEnd())
	return open
}

// processJob runs one job end-to-end: encode → verify → replace. The job is
// already in `running` (ClaimNextQueued set it). A running job is never
// interrupted by the window closing — only new pulls are gated.
func (w *Worker) processJob(ctx context.Context, job *store.TranscodeJob) {
	file, err := w.store.Media.GetByID(ctx, job.MediaFileID)
	if err != nil {
		slog.Error("worker: media file not found", "job", job.ID, "media_file_id", job.MediaFileID, "err", err)
		w.failJob(ctx, job.ID, "media file not found: "+err.Error(), nil)
		return
	}
	profile, err := w.store.Profiles.GetByID(ctx, job.ProfileID)
	if err != nil {
		slog.Error("worker: profile not found", "job", job.ID, "profile_id", job.ProfileID, "err", err)
		w.failJob(ctx, job.ID, "profile not found: "+err.Error(), nil)
		return
	}
	codec := media.NormalizeTargetCodec(profile.Codec)
	enc, ok := media.EncoderFor(codec)
	if !ok {
		w.failJob(ctx, job.ID, fmt.Sprintf("profile %q has unsupported codec %q", profile.Name, profile.Codec), nil)
		return
	}
	if err := w.store.Jobs.SetEncodeSettings(ctx, job.ID, string(codec), profile.Preset, profile.CRF, profile.ExtraArgs); err != nil {
		slog.Warn("worker: stamp encode settings", "job", job.ID, "err", err)
	}

	ext, muxer := media.OutputContainer(file.Path, codec)
	outPath := media.WithExt(file.Path, ext)
	if outPath != file.Path {
		if msg := w.remuxConflict(ctx, outPath); msg != "" {
			w.failJob(ctx, job.ID, msg, nil)
			return
		}
	}

	tmpPath := tempPathFor(file.Path, ext)
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
		Encoder:         enc.FFmpegEncoder,
		CRF:             profile.CRF,
		Preset:          profile.Preset,
		ExtraArgs:       extra,
		Format:          muxer,
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
			slog.Error("worker: encode failed", "job", job.ID, "path", file.Path, "err", encErr)
			removeIfExists(tmpPath)
			// No temp file survives an encode failure, so clear the breadcrumb —
			// otherwise the UI would point at a path that no longer exists.
			if cerr := w.store.Jobs.ClearOutputPath(ctx, job.ID); cerr != nil {
				slog.Error("worker: clear output path", "job", job.ID, "err", cerr)
			}
			w.failJob(ctx, job.ID, "encode failed: "+encErr.Error(), nil)
		}
		return
	}

	// Encode succeeded → verify before touching the original.
	if err := w.store.Jobs.Transition(ctx, job.ID, string(jobs.StatusRunning), string(jobs.StatusVerifying)); err != nil {
		slog.Error("worker: transition to verifying", "job", job.ID, "err", err)
	}
	w.verifyAndReplace(ctx, job, file, tmpPath, outPath, codec)
}

// remuxConflict explains why an encode cannot move to outPath, or returns ""
// when it can. A file already there would be overwritten by the swap, and a
// row already holding the path — a missing file's — would collide with this
// one's when the swap moves it there.
func (w *Worker) remuxConflict(ctx context.Context, outPath string) string {
	if _, err := os.Stat(outPath); err == nil {
		return "cannot change container: " + filepath.Base(outPath) + " already exists"
	}
	if _, err := w.store.Media.GetByPath(ctx, outPath); err == nil {
		return "cannot change container: the library already has a row for " + filepath.Base(outPath)
	}
	return ""
}

// verifyAndReplace runs the verification checks and, only on a full pass,
// performs the atomic swap. A failed check deletes the temp — it sits in the
// library beside the original, where Plex would list it as a duplicate — leaves
// the original untouched, and marks the job failed with the verification detail
// attached.
func (w *Worker) verifyAndReplace(ctx context.Context, job *store.TranscodeJob, file *store.MediaFile, tmpPath, outPath string, codec media.TargetCodec) {
	result, ok := w.verify(ctx, file, tmpPath, codec)
	blob, _ := json.Marshal(result)
	if err := w.store.Jobs.SetVerificationResult(ctx, job.ID, string(blob)); err != nil {
		slog.Error("worker: store verification result", "job", job.ID, "err", err)
	}

	if !ok {
		removeIfExists(tmpPath)
		if err := w.store.Jobs.ClearOutputPath(ctx, job.ID); err != nil {
			slog.Error("worker: clear output path", "job", job.ID, "err", err)
		}
		w.failJob(ctx, job.ID, "verification failed", strptr(string(blob)))
		return
	}

	if err := w.replace(ctx, job, file, tmpPath, outPath); err != nil {
		if errors.Is(err, errPostSwapCommit) {
			slog.Error("worker: CRITICAL swap ok but db commit failed; job left verifying for reconcile",
				"job", job.ID, "path", file.Path, "err", err)
			if setErr := w.store.Jobs.SetCommitError(ctx, job.ID, err.Error()); setErr != nil {
				slog.Error("worker: set commit error", "job", job.ID, "err", setErr)
			}
			return
		}
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
	CodecMatch           bool    `json:"codec_match"`
	Passed               bool    `json:"passed"`
}

// verify runs the verification checks against the temp output, re-probing the
// original on disk for the freshest comparison truth. The output must also be
// in the target codec: extra args can override -c:v, and an output still in
// the source codec would otherwise be swapped in and recorded as HEVC or AV1.
func (w *Worker) verify(ctx context.Context, file *store.MediaFile, tmpPath string, codec media.TargetCodec) (verificationResult, bool) {
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

	res.CodecMatch = media.NormalizeTargetCodec(out.VideoCodec) == codec

	res.Passed = res.DurationMatch && res.Playable && res.StreamCountMatch && res.ResolutionMatch && res.CodecMatch
	return res, res.Passed
}

// replace performs the atomic swap: original → .reclaim-backup, temp →
// original, delete backup, then update the row + stats. The backup window means
// a failure mid-swap is recoverable rather than leaving no original.
//
// An encode that changed container lands at outPath instead, beside the
// original rather than over it, so it needs no backup: the temp is renamed into
// place, then the original deleted. A crash between the two leaves both, which
// tryCompletePostSwap resolves on the next boot.
func (w *Worker) replace(ctx context.Context, job *store.TranscodeJob, file *store.MediaFile, tmpPath, outPath string) error {
	if outPath != file.Path {
		if err := moveEncoded(file.Path, tmpPath, outPath); err != nil {
			return err
		}
	} else if err := swapEncoded(file.Path, tmpPath); err != nil {
		return err
	}
	return w.commitSwap(ctx, job, file, outPath)
}

// moveEncoded renames a verified temp to outPath and deletes the original it
// replaces. Failing to delete the original undoes the move, so the library is
// never left holding both.
func moveEncoded(origPath, tmpPath, outPath string) error {
	if _, err := os.Stat(outPath); err == nil {
		return fmt.Errorf("%s already exists", filepath.Base(outPath))
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		return err // original untouched, temp kept
	}
	if err := os.Remove(origPath); err != nil {
		if rerr := os.Rename(outPath, tmpPath); rerr != nil {
			slog.Error("worker: undo move", "path", outPath, "err", rerr)
		}
		return err
	}
	return nil
}

// swapEncoded replaces the original with a verified temp at the same path.
func swapEncoded(origPath, tmpPath string) error {
	backupPath := origPath + backupSuffix

	if err := os.Rename(origPath, backupPath); err != nil {
		return err // original untouched, temp kept
	}
	if err := os.Rename(tmpPath, origPath); err != nil {
		// Step 2 failed: restore the original from backup so we never lose it.
		if rerr := os.Rename(backupPath, origPath); rerr != nil {
			slog.Error("worker: CRITICAL restore failed", "path", origPath, "backup", backupPath, "err", rerr)
		}
		return err
	}
	if err := os.Remove(backupPath); err != nil {
		// Non-fatal: the swap is done; the orphan sweep will delete the backup.
		slog.Warn("worker: remove backup", "path", backupPath, "err", err)
	}
	return nil
}

// commitSwap records an encode that is now on disk at outPath.
func (w *Worker) commitSwap(ctx context.Context, job *store.TranscodeJob, file *store.MediaFile, outPath string) error {
	info, err := os.Stat(outPath)
	if err != nil {
		return err
	}
	newSize := info.Size()
	fp, err := media.Fingerprint(outPath)
	if err != nil {
		slog.Warn("worker: fingerprint after swap", "path", outPath, "err", err)
	}

	now := w.clock().Unix()
	completedMsg := "Encoded " + filepath.Base(outPath)
	completedMeta := jsonMeta(map[string]any{
		"job_id":              job.ID,
		"file_id":             file.ID,
		"output_size_bytes":   newSize,
		"original_size_bytes": file.SizeBytes,
	})
	eventID, err := w.store.CommitEncodeSwap(ctx, file.ID, job.ID, movedTo(file.Path, outPath), newSize, fp, now, completedMsg, completedMeta)
	if err != nil {
		return fmt.Errorf("%w: %v", errPostSwapCommit, err)
	}
	w.hub.Broadcast("event_created", eventBroadcast(eventID, store.EventJobCompleted, store.SeverityInfo, completedMsg, completedMeta, now))

	w.refreshSavingsModel(ctx)

	w.hub.Broadcast("job_completed", map[string]any{
		"job_id":            job.ID,
		"media_file_id":     file.ID,
		"output_size_bytes": newSize,
	})
	return nil
}

// refreshSavingsModel folds a newly recorded encode into the learned savings
// ratios and reprices the library against them. A failure only leaves
// predictions a step stale, so it is logged rather than failing the job.
func (w *Worker) refreshSavingsModel(ctx context.Context) {
	n, err := w.store.SavingsModel.Refresh(context.WithoutCancel(ctx))
	if err != nil {
		slog.Warn("worker: savings model refresh", "err", err)
		return
	}
	if n > 0 {
		slog.Info("worker: refined savings model", "files_updated", n)
	}
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
		metaData["verification_result"] = json.RawMessage(*verification)
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
		if j.Status == string(jobs.StatusVerifying) && w.tryCompletePostSwap(ctx, &j) {
			continue
		}
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

// tryCompletePostSwap finishes a verifying job whose filesystem swap already
// succeeded but whose DB commit failed or was interrupted. Returns true when the
// job was recovered.
func (w *Worker) tryCompletePostSwap(ctx context.Context, job *store.TranscodeJob) bool {
	file, err := w.store.Media.GetByID(ctx, job.MediaFileID)
	if err != nil {
		return false
	}
	if file.IsEfficientCodec {
		return false
	}
	target := w.jobTargetCodec(ctx, job)
	ext, _ := media.OutputContainer(file.Path, target)
	outPath := media.WithExt(file.Path, ext)
	res, err := w.probe(ctx, outPath)
	if err != nil || res.VideoCodec == nil || media.NormalizeTargetCodec(*res.VideoCodec) != target {
		return false
	}
	info, err := os.Stat(outPath)
	if err != nil {
		return false
	}
	newSize := info.Size()
	fp, err := media.Fingerprint(outPath)
	if err != nil {
		slog.Warn("worker: reconcile fingerprint", "path", outPath, "err", err)
	}
	if outPath != file.Path {
		// The move landed but the original may not have been deleted.
		if err := os.Remove(file.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("worker: reconcile remove original", "path", file.Path, "err", err)
			return false
		}
	}
	now := w.clock().Unix()
	completedMsg := "Encoded " + filepath.Base(outPath)
	completedMeta := jsonMeta(map[string]any{
		"job_id":              job.ID,
		"file_id":             file.ID,
		"output_size_bytes":   newSize,
		"original_size_bytes": file.SizeBytes,
	})
	eventID, err := w.store.CommitEncodeSwap(ctx, file.ID, job.ID, movedTo(file.Path, outPath), newSize, fp, now, completedMsg, completedMeta)
	if err != nil {
		slog.Warn("worker: reconcile post-swap commit", "job", job.ID, "err", err)
		return false
	}
	w.hub.Broadcast("event_created", eventBroadcast(eventID, store.EventJobCompleted, store.SeverityInfo, completedMsg, completedMeta, now))
	w.refreshSavingsModel(ctx)
	w.hub.Broadcast("job_completed", map[string]any{
		"job_id":            job.ID,
		"media_file_id":     file.ID,
		"output_size_bytes": newSize,
	})
	slog.Info("worker: reconciled post-swap db commit", "job", job.ID, "path", outPath)
	return true
}

// jobTargetCodec is the codec a job encoded to: the snapshot the worker stamped
// when it claimed the job, else its profile's codec, else the default.
func (w *Worker) jobTargetCodec(ctx context.Context, job *store.TranscodeJob) media.TargetCodec {
	if job.EncodeCodec != nil && *job.EncodeCodec != "" {
		return media.NormalizeTargetCodec(*job.EncodeCodec)
	}
	if profile, err := w.store.Profiles.GetByID(ctx, job.ProfileID); err == nil {
		return media.NormalizeTargetCodec(profile.Codec)
	}
	return media.DefaultTargetCodec
}

// sweepOrphans cleans up stray reclaim temp/backup files at paths derived from
// the DB and in-flight encodes, rather than walking entire media trees.
func (w *Worker) sweepOrphans(ctx context.Context) {
	active := w.activeTempPaths()

	paths, err := w.reclaimArtifactPaths(ctx)
	if err != nil {
		slog.Error("worker: list reclaim artifacts", "err", err)
		return
	}

	for _, path := range paths {
		if ctx.Err() != nil {
			return
		}
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			slog.Warn("worker: stat reclaim artifact", "path", path, "err", err)
			continue
		}
		if info.IsDir() {
			continue
		}
		name := info.Name()
		switch {
		case strings.Contains(name, tmpSuffix):
			if _, busy := active[path]; busy {
				continue
			}
			if err := os.Remove(path); err != nil {
				slog.Warn("worker: remove stale temp", "path", path, "err", err)
			} else {
				slog.Info("worker: removed stale temp", "path", path)
			}
		case strings.HasSuffix(name, backupSuffix):
			orig := strings.TrimSuffix(path, backupSuffix)
			if _, serr := os.Stat(orig); serr == nil {
				if err := os.Remove(path); err != nil {
					slog.Warn("worker: remove stale backup", "path", path, "err", err)
				}
			} else {
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
	}
}

func (w *Worker) reclaimArtifactPaths(ctx context.Context) ([]string, error) {
	seen := make(map[string]struct{})

	for p := range w.activeTempPaths() {
		seen[p] = struct{}{}
	}

	outputs, err := w.store.Jobs.OutputPaths(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range outputs {
		if p != "" {
			seen[p] = struct{}{}
		}
	}

	mediaPaths, err := w.store.Media.ActivePaths(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range mediaPaths {
		seen[tempPathFor(p, filepath.Ext(p))] = struct{}{}
		seen[tempPathFor(p, media.RemuxExt)] = struct{}{}
		seen[p+backupSuffix] = struct{}{}
	}

	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	return out, nil
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

// tempPathFor returns the temp output path for an original encoding to a
// container with extension ext, which ffmpeg infers the muxer from
// (a.mkv → a.mkv.reclaim-tmp.mkv, a.avi → a.avi.reclaim-tmp.mkv).
func tempPathFor(orig, ext string) string {
	return orig + tmpSuffix + ext
}

// movedTo is the new path CommitEncodeSwap records: outPath when the encode
// changed container, "" when it replaced the original in place.
func movedTo(origPath, outPath string) string {
	if outPath == origPath {
		return ""
	}
	return outPath
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
