package scanner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"reclaim/internal/config"
	"reclaim/internal/ffprobe"
	"reclaim/internal/store"
)

// mockProbe returns a deterministic result so tests don't need a real ffprobe.
func mockProbe(_ context.Context, path string) (*ffprobe.Result, error) {
	codec := "h264"
	dur := 100.0
	return &ffprobe.Result{
		VideoCodec:      &codec,
		DurationSeconds: &dur,
		IsAlreadyHEVC:   false,
	}, nil
}

func newTestScanner(t *testing.T, movieRoot, tvRoot string) (*Scanner, *store.Store) {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := &config.Config{
		MoviesPath:       movieRoot,
		TVPath:           tvRoot,
		ProbeConcurrency: 2,
		ScanInterval:     24 * time.Hour,
	}
	sc, err := New(st, cfg, WithProbeFunc(mockProbe), WithDebounceDur(50*time.Millisecond))
	if err != nil {
		t.Fatalf("new scanner: %v", err)
	}
	return sc, st
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fake media content"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestScanNewFile verifies a file added to disk appears in the DB after a scan.
func TestScanNewFile(t *testing.T) {
	root := t.TempDir()
	sc, st := newTestScanner(t, root, root)
	ctx := context.Background()

	path := filepath.Join(root, "movie.mkv")
	writeFile(t, path)

	run, err := sc.Scan(ctx, "manual", false)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if run.FilesAdded != 1 {
		t.Errorf("FilesAdded = %d, want 1", run.FilesAdded)
	}

	f, err := st.Media.GetByPath(ctx, path)
	if err != nil {
		t.Fatalf("get by path: %v", err)
	}
	if f.Status != "active" {
		t.Errorf("status = %q, want active", f.Status)
	}
}

// TestSecondScanUnchangedSkips verifies that a second scan of an unchanged
// tree probes ~0 files (the plan acceptance: "probes ~0 files").
func TestSecondScanUnchangedSkips(t *testing.T) {
	root := t.TempDir()
	sc, _ := newTestScanner(t, root, root)
	ctx := context.Background()

	writeFile(t, filepath.Join(root, "a.mkv"))
	writeFile(t, filepath.Join(root, "b.mkv"))

	if _, err := sc.Scan(ctx, "first", false); err != nil {
		t.Fatalf("first scan: %v", err)
	}

	run, err := sc.Scan(ctx, "second", false)
	if err != nil {
		t.Fatalf("second scan: %v", err)
	}
	if run.FilesAdded != 0 || run.FilesUpdated != 0 {
		t.Errorf("second scan re-probed: added=%d updated=%d, want both 0",
			run.FilesAdded, run.FilesUpdated)
	}
	if run.FilesScanned != 2 {
		t.Errorf("FilesScanned = %d, want 2", run.FilesScanned)
	}
}

func TestScanNoChangesCreatesEvent(t *testing.T) {
	root := t.TempDir()
	sc, st := newTestScanner(t, root, root)
	ctx := context.Background()

	writeFile(t, filepath.Join(root, "movie.mkv"))
	if _, err := sc.Scan(ctx, TriggerManual, false); err != nil {
		t.Fatalf("initial scan: %v", err)
	}
	if _, err := sc.Scan(ctx, TriggerManual, false); err != nil {
		t.Fatalf("second scan: %v", err)
	}

	events, err := st.Events.List(ctx, store.EventFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2 (one per scan)", len(events))
	}
	if events[0].Message != "Manual scan: no changes" {
		t.Errorf("message = %q, want %q", events[0].Message, "Manual scan: no changes")
	}
	if events[0].Type != store.EventScanCompleted {
		t.Errorf("type = %q, want scan_completed", events[0].Type)
	}
}

func TestScanEventMessage(t *testing.T) {
	tests := []struct {
		name string
		run  store.ScanRun
		want string
	}{
		{"manual no changes", store.ScanRun{Trigger: TriggerManual}, "Manual scan: no changes"},
		{"startup no changes", store.ScanRun{Trigger: TriggerStartup}, "Startup scan: no changes"},
		{"scheduled with changes", store.ScanRun{Trigger: TriggerScheduled, FilesAdded: 2, FilesUpdated: 1}, "Scheduled scan: 2 added, 1 updated, 0 moved, 0 removed"},
		{"manual with errors", store.ScanRun{Trigger: TriggerManual, Errors: 1}, "Manual scan: 0 added, 0 updated, 0 moved, 0 removed, 1 errors"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scanEventMessage(&tt.run); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestScanDeleteFile verifies that a deleted file is marked as missing.
func TestScanDeleteFile(t *testing.T) {
	root := t.TempDir()
	sc, st := newTestScanner(t, root, root)
	ctx := context.Background()

	path := filepath.Join(root, "movie.mkv")
	writeFile(t, path)

	if _, err := sc.Scan(ctx, "initial", false); err != nil {
		t.Fatalf("initial scan: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	run, err := sc.Scan(ctx, "after-delete", false)
	if err != nil {
		t.Fatalf("scan after delete: %v", err)
	}
	if run.FilesRemoved != 1 {
		t.Errorf("FilesRemoved = %d, want 1", run.FilesRemoved)
	}

	f, err := st.Media.GetByPath(ctx, path)
	if err != nil {
		t.Fatalf("get by path: %v", err)
	}
	if f.Status != "missing" {
		t.Errorf("status = %q, want missing", f.Status)
	}
}

// TestScanPrunesMissingAfterRetention verifies the post-scan cleanup hook.
// Retention defaults to zero (off), so a soft-deleted row must survive until an
// operator opts in; once it does, an aged-out row is hard-deleted and the
// removal is written to the activity log. Cutoff boundaries themselves are
// covered by the store's prune tests.
func TestScanPrunesMissingAfterRetention(t *testing.T) {
	root := t.TempDir()
	sc, st := newTestScanner(t, root, root)
	ctx := context.Background()

	path := filepath.Join(root, "movie.mkv")
	writeFile(t, path)

	if _, err := sc.Scan(ctx, "initial", false); err != nil {
		t.Fatalf("initial scan: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := sc.Scan(ctx, "after-delete", false); err != nil {
		t.Fatalf("scan after delete: %v", err)
	}

	// Retention off: the row survives as soft-deleted.
	if _, err := st.Media.GetByPath(ctx, path); err != nil {
		t.Fatalf("row should survive with retention off: %v", err)
	}
	overview, err := st.Media.MissingOverview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Count != 1 {
		t.Fatalf("missing rows with retention off = %d, want 1", overview.Count)
	}

	// Opt in with a retention short enough that the row is already past it.
	sc.missingRetentionFn = func() time.Duration { return time.Nanosecond }

	if _, err := sc.Scan(ctx, "after-retention", false); err != nil {
		t.Fatalf("scan with retention: %v", err)
	}
	if _, err := st.Media.GetByPath(ctx, path); err == nil {
		t.Error("aged-out row should have been pruned")
	}

	events, err := st.Events.List(ctx, store.EventFilter{Limit: 20, Type: store.EventMissingPruned})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("missing_pruned events = %d, want 1", len(events))
	}
}

// TestScanSupersedesContainerChange verifies the path-based fallback: an
// out-of-band re-encode that also changed container leaves the old row nowhere
// to match on fingerprint (the bytes differ), so the vanished row is folded into
// the same-stem survivor instead of lingering as a missing duplicate.
func TestScanSupersedesContainerChange(t *testing.T) {
	root := t.TempDir()
	sc, st := newTestScanner(t, root, root)
	ctx := context.Background()

	src := filepath.Join(root, "Show", "S01E01.mkv")
	dst := filepath.Join(root, "Show", "S01E01.mp4")
	writeFile(t, src)

	if _, err := sc.Scan(ctx, "initial", false); err != nil {
		t.Fatalf("initial scan: %v", err)
	}

	// A re-encode, not a rename: same name, new container, different bytes.
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("re-encoded, much smaller"), 0o644); err != nil {
		t.Fatal(err)
	}

	run, err := sc.Scan(ctx, "after-reencode", false)
	if err != nil {
		t.Fatalf("scan after re-encode: %v", err)
	}
	if run.FilesRemoved != 0 {
		t.Errorf("FilesRemoved = %d, want 0 (the file was replaced, not lost)", run.FilesRemoved)
	}
	if run.FilesMoved != 1 {
		t.Errorf("FilesMoved = %d, want 1", run.FilesMoved)
	}

	if _, err := st.Media.GetByPath(ctx, src); err == nil {
		t.Error("superseded .mkv row still in DB")
	}
	survivor, err := st.Media.GetByPath(ctx, dst)
	if err != nil {
		t.Fatalf("get replacement row: %v", err)
	}
	if survivor.Status != "active" {
		t.Errorf("replacement status = %q, want active", survivor.Status)
	}

	overview, err := st.Media.MissingOverview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Count != 0 {
		t.Errorf("missing rows = %d, want 0 (no phantom duplicate)", overview.Count)
	}

	events, err := st.Events.List(ctx, store.EventFilter{Limit: 20, Type: store.EventFileSuperseded})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("file_superseded events = %d, want 1", len(events))
	}
}

// TestScanKeepsMissingWhenStemDiffers guards the narrowness of the rule: a
// replacement that is not the same file by name must still read as missing
// rather than being silently folded into an unrelated row.
func TestScanKeepsMissingWhenStemDiffers(t *testing.T) {
	root := t.TempDir()
	sc, st := newTestScanner(t, root, root)
	ctx := context.Background()

	src := filepath.Join(root, "Show", "S01E01.mkv")
	other := filepath.Join(root, "Show", "S01E01.1080p.mp4")
	writeFile(t, src)

	if _, err := sc.Scan(ctx, "initial", false); err != nil {
		t.Fatalf("initial scan: %v", err)
	}
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("a different release"), 0o644); err != nil {
		t.Fatal(err)
	}

	run, err := sc.Scan(ctx, "after-swap", false)
	if err != nil {
		t.Fatalf("scan after swap: %v", err)
	}
	if run.FilesRemoved != 1 {
		t.Errorf("FilesRemoved = %d, want 1", run.FilesRemoved)
	}

	gone, err := st.Media.GetByPath(ctx, src)
	if err != nil {
		t.Fatalf("original row should still exist as missing: %v", err)
	}
	if gone.Status != "missing" {
		t.Errorf("original status = %q, want missing", gone.Status)
	}
}

// TestScanRenameFile verifies that a renamed file is recorded as a move with
// job history preserved (old row updated, duplicate deleted).
func TestScanRenameFile(t *testing.T) {
	root := t.TempDir()
	sc, st := newTestScanner(t, root, root)
	ctx := context.Background()

	src := filepath.Join(root, "original.mkv")
	dst := filepath.Join(root, "renamed.mkv")
	writeFile(t, src)

	if _, err := sc.Scan(ctx, "initial", false); err != nil {
		t.Fatalf("initial scan: %v", err)
	}

	origFile, err := st.Media.GetByPath(ctx, src)
	if err != nil {
		t.Fatalf("get original row: %v", err)
	}
	origID := origFile.ID

	if err := os.Rename(src, dst); err != nil {
		t.Fatal(err)
	}

	run, err := sc.Scan(ctx, "after-rename", false)
	if err != nil {
		t.Fatalf("scan after rename: %v", err)
	}
	if run.FilesMoved != 1 {
		t.Errorf("FilesMoved = %d, want 1", run.FilesMoved)
	}
	if run.FilesRemoved != 0 {
		t.Errorf("FilesRemoved = %d, want 0 (should be a move, not delete+add)", run.FilesRemoved)
	}

	// The OLD row's ID should now point to the new path — job history preserved.
	movedFile, err := st.Media.GetByPath(ctx, dst)
	if err != nil {
		t.Fatalf("get renamed row: %v", err)
	}
	if movedFile.ID != origID {
		t.Errorf("moved file has new ID %d, want original ID %d (job history broken)", movedFile.ID, origID)
	}
	if movedFile.Status != "active" {
		t.Errorf("moved file status = %q, want active", movedFile.Status)
	}

	// Old path must not be findable.
	_, err = st.Media.GetByPath(ctx, src)
	if err == nil {
		t.Error("old path still in DB after rename")
	}
}

// TestScanCounts verifies scan_run counts match what actually happened.
func TestScanCounts(t *testing.T) {
	root := t.TempDir()
	sc, _ := newTestScanner(t, root, root)
	ctx := context.Background()

	writeFile(t, filepath.Join(root, "a.mkv"))
	writeFile(t, filepath.Join(root, "b.mkv"))
	writeFile(t, filepath.Join(root, "c.mkv"))

	run, err := sc.Scan(ctx, "manual", false)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if run.FilesAdded != 3 {
		t.Errorf("FilesAdded = %d, want 3", run.FilesAdded)
	}
	if run.FilesScanned != 0 {
		t.Errorf("FilesScanned = %d, want 0 (all are new)", run.FilesScanned)
	}
	if run.Errors != 0 {
		t.Errorf("Errors = %d, want 0", run.Errors)
	}
}

// TestScanFullForceRescan verifies that force=true re-probes even unchanged files.
func TestScanFullForceRescan(t *testing.T) {
	root := t.TempDir()
	sc, _ := newTestScanner(t, root, root)
	ctx := context.Background()

	writeFile(t, filepath.Join(root, "a.mkv"))
	writeFile(t, filepath.Join(root, "b.mkv"))

	if _, err := sc.Scan(ctx, "first", false); err != nil {
		t.Fatalf("first scan: %v", err)
	}

	run, err := sc.Scan(ctx, "forced", true)
	if err != nil {
		t.Fatalf("force scan: %v", err)
	}
	if run.FilesUpdated != 2 {
		t.Errorf("force rescan: FilesUpdated = %d, want 2", run.FilesUpdated)
	}
}

// TestRescanFiles verifies that re-probing a set of explicit IDs picks up
// on-disk changes (a size change here, standing in for a re-encode outside
// Reclaim) without requiring a full library scan.
func TestRescanFiles(t *testing.T) {
	root := t.TempDir()
	sc, st := newTestScanner(t, root, root)
	ctx := context.Background()

	pathA := filepath.Join(root, "a.mkv")
	pathB := filepath.Join(root, "b.mkv")
	writeFile(t, pathA)
	writeFile(t, pathB)

	if _, err := sc.Scan(ctx, "initial", false); err != nil {
		t.Fatalf("initial scan: %v", err)
	}

	fileA, err := st.Media.GetByPath(ctx, pathA)
	if err != nil {
		t.Fatalf("get by path: %v", err)
	}
	fileB, err := st.Media.GetByPath(ctx, pathB)
	if err != nil {
		t.Fatalf("get by path: %v", err)
	}

	// Mutate only A on disk; B is untouched and should round-trip unchanged.
	if err := os.WriteFile(pathA, []byte("changed content, different size"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := sc.RescanFiles(ctx, []int64{fileA.ID, fileB.ID})
	if err != nil {
		t.Fatalf("rescan files: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	updatedA, err := st.Media.GetByID(ctx, fileA.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if updatedA.SizeBytes == fileA.SizeBytes {
		t.Errorf("rescanned file A size unchanged: %d", updatedA.SizeBytes)
	}
	if updatedA.LastProbedAt == nil {
		t.Error("rescanned file A LastProbedAt not set")
	}
}

// TestRescanFilesMarksMissing verifies that rescanning a file whose path has
// vanished from disk marks it missing rather than erroring the whole batch.
func TestRescanFilesMarksMissing(t *testing.T) {
	root := t.TempDir()
	sc, st := newTestScanner(t, root, root)
	ctx := context.Background()

	path := filepath.Join(root, "a.mkv")
	writeFile(t, path)

	if _, err := sc.Scan(ctx, "initial", false); err != nil {
		t.Fatalf("initial scan: %v", err)
	}

	file, err := st.Media.GetByPath(ctx, path)
	if err != nil {
		t.Fatalf("get by path: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	if _, err := sc.RescanFiles(ctx, []int64{file.ID}); err != nil {
		t.Fatalf("rescan files: %v", err)
	}

	updated, err := st.Media.GetByID(ctx, file.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if updated.Status != "missing" {
		t.Errorf("status = %q, want missing", updated.Status)
	}
}

// TestProbeConcurrencyCap proves the scanner-wide semaphore bounds concurrent
// probes to PROBE_CONCURRENCY across *all* entry points: a walk fills the cap,
// then a watcher probe for a fresh file must queue on the same semaphore rather
// than starting an extra ffprobe. With the old per-Scan() semaphore (and the
// watcher bypassing it entirely) this test would observe a probe beyond the cap.
func TestProbeConcurrencyCap(t *testing.T) {
	const capN = 2
	const nFiles = 10

	root := t.TempDir()
	for i := 0; i < nFiles; i++ {
		writeFile(t, filepath.Join(root, fmt.Sprintf("f%02d.mkv", i)))
	}

	var cur, peak int32
	release := make(chan struct{})
	// Buffered generously so probes never block on the send; they only block on
	// release. This lets us observe exactly how many got past the semaphore.
	started := make(chan struct{}, nFiles+1)

	probe := func(_ context.Context, _ string) (*ffprobe.Result, error) {
		n := atomic.AddInt32(&cur, 1)
		for {
			old := atomic.LoadInt32(&peak)
			if n <= old || atomic.CompareAndSwapInt32(&peak, old, n) {
				break
			}
		}
		started <- struct{}{}
		<-release
		atomic.AddInt32(&cur, -1)
		codec := "h264"
		return &ffprobe.Result{VideoCodec: &codec}, nil
	}

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := &config.Config{
		MoviesPath:       root,
		TVPath:           root,
		ProbeConcurrency: capN,
		ScanInterval:     time.Hour,
	}
	sc, err := New(st, cfg, WithProbeFunc(probe), WithDebounceDur(10*time.Millisecond))
	if err != nil {
		t.Fatalf("new scanner: %v", err)
	}

	ctx := context.Background()
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		if _, err := sc.Scan(ctx, "manual", false); err != nil {
			t.Errorf("scan: %v", err)
		}
	}()

	// Wait until the walk saturates the cap.
	for i := 0; i < capN; i++ {
		<-started
	}

	// Fire a watcher probe for a brand-new file while every slot is occupied. It
	// shares the scanner-wide semaphore, so it must block on acquire and never
	// reach the probe func until a slot frees.
	watchPath := filepath.Join(root, "watched.mkv")
	writeFile(t, watchPath)
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		sc.probeSingleFile(ctx, watchPath)
	}()

	// No probe may start beyond the cap while the slots stay full.
	select {
	case <-started:
		t.Fatalf("a probe started beyond the cap of %d (semaphore not shared across walk + watcher)", capN)
	case <-time.After(250 * time.Millisecond):
	}

	close(release)
	<-scanDone
	<-watchDone

	if p := atomic.LoadInt32(&peak); p > capN {
		t.Errorf("peak concurrent probes = %d, want <= %d", p, capN)
	}
}

// TestScanComputesSavingsAndStats verifies that a scan computes a
// predicted-savings estimate at probe time and the incrementally-maintained
// library stats reflect what was indexed.
func TestScanComputesSavingsAndStats(t *testing.T) {
	root := t.TempDir()
	sc, st := newTestScanner(t, root, root)
	ctx := context.Background()

	path := filepath.Join(root, "movie.mkv")
	writeFile(t, path)

	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatalf("scan: %v", err)
	}

	f, err := st.Media.GetByPath(ctx, path)
	if err != nil {
		t.Fatalf("get by path: %v", err)
	}
	// mockProbe reports h264 (non-HEVC), so savings must be computed and positive.
	if f.PredictedSavingsBytes <= 0 {
		t.Errorf("PredictedSavingsBytes = %d, want > 0 for an h264 file", f.PredictedSavingsBytes)
	}

	ov, err := st.Stats.Overview(ctx)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if ov.TotalFiles != 1 {
		t.Errorf("stats TotalFiles = %d, want 1", ov.TotalFiles)
	}
	if ov.TotalRecoverableBytes != f.PredictedSavingsBytes {
		t.Errorf("stats recoverable = %d, want %d", ov.TotalRecoverableBytes, f.PredictedSavingsBytes)
	}

	// The file should surface as a candidate.
	cands, err := st.Media.Candidates(ctx, store.CandidateQuery{})
	if err != nil {
		t.Fatalf("candidates: %v", err)
	}
	if len(cands) != 1 || cands[0].ID != f.ID {
		t.Errorf("candidates = %+v, want exactly the indexed file", cands)
	}
}

// TestWatcherProbeNewFile exercises the debounced probe path that fires when
// the watcher sees a new file.
func TestWatcherProbeNewFile(t *testing.T) {
	root := t.TempDir()
	sc, st := newTestScanner(t, root, root)
	ctx := context.Background()

	// Initial scan of empty dir.
	if _, err := sc.Scan(ctx, "initial", false); err != nil {
		t.Fatalf("initial scan: %v", err)
	}

	// Simulate the watcher firing the debounced probe (debounce is 50ms in tests).
	path := filepath.Join(root, "new.mkv")
	writeFile(t, path)

	sc.probeSingleFile(ctx, path)

	f, err := st.Media.GetByPath(ctx, path)
	if err != nil {
		t.Fatalf("file not in DB after watcher probe: %v", err)
	}
	if f.Status != "active" {
		t.Errorf("status = %q, want active", f.Status)
	}
}

func TestScanSingleFlight(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.mkv"))

	entered := make(chan struct{}, 1)
	gate := make(chan struct{})
	probe := func(_ context.Context, _ string) (*ffprobe.Result, error) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-gate
		return mockProbe(context.Background(), "")
	}

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := &config.Config{
		MoviesPath:       root,
		TVPath:           root,
		ProbeConcurrency: 2,
		ScanInterval:     time.Hour,
	}
	sc, err := New(st, cfg, WithProbeFunc(probe), WithDebounceDur(10*time.Millisecond))
	if err != nil {
		t.Fatalf("new scanner: %v", err)
	}

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := sc.Scan(ctx, "first", false); err != nil {
			t.Errorf("first scan: %v", err)
		}
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first scan never blocked in probe")
	}

	if _, err := sc.Scan(ctx, "second", false); !errors.Is(err, ErrScanInProgress) {
		t.Fatalf("second scan: want ErrScanInProgress, got %v", err)
	}

	close(gate)
	<-done
}

func TestLiveProbeConcurrency(t *testing.T) {
	const capLow, capHigh = 1, 3
	root := t.TempDir()
	for i := 0; i < 6; i++ {
		writeFile(t, filepath.Join(root, fmt.Sprintf("f%d.mkv", i)))
	}

	var cur, peak int32
	release := make(chan struct{})
	started := make(chan struct{}, 8)
	probe := func(_ context.Context, _ string) (*ffprobe.Result, error) {
		n := atomic.AddInt32(&cur, 1)
		for {
			old := atomic.LoadInt32(&peak)
			if n <= old || atomic.CompareAndSwapInt32(&peak, old, n) {
				break
			}
		}
		started <- struct{}{}
		<-release
		atomic.AddInt32(&cur, -1)
		codec := "h264"
		return &ffprobe.Result{VideoCodec: &codec}, nil
	}

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := &config.Config{
		MoviesPath:       root,
		TVPath:           root,
		ProbeConcurrency: capLow,
		ScanInterval:     time.Hour,
	}
	live := config.NewLive(cfg)
	sc, err := New(st, cfg, WithProbeFunc(probe), WithLiveConfig(live), WithDebounceDur(10*time.Millisecond))
	if err != nil {
		t.Fatalf("new scanner: %v", err)
	}

	ctx := context.Background()
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		if _, err := sc.Scan(ctx, "manual", false); err != nil {
			t.Errorf("scan: %v", err)
		}
	}()

	for i := 0; i < capLow; i++ {
		<-started
	}
	select {
	case <-started:
		t.Fatalf("more than %d probes started before live cap raised", capLow)
	case <-time.After(200 * time.Millisecond):
	}

	high := capHigh
	if err := live.Update(nil, nil, nil, nil, &high, nil, nil, nil); err != nil {
		t.Fatalf("live update: %v", err)
	}
	close(release)
	<-scanDone

	if got := atomic.LoadInt32(&peak); got > int32(capHigh) {
		t.Fatalf("peak concurrent probes = %d, want <= %d", got, capHigh)
	}
}

func TestScanWorkerPoolBounded(t *testing.T) {
	const capN = 4
	const nFiles = 48

	root := t.TempDir()
	for i := 0; i < nFiles; i++ {
		writeFile(t, filepath.Join(root, fmt.Sprintf("f%02d.mkv", i)))
	}

	var cur, peak int32
	release := make(chan struct{})
	started := make(chan struct{}, nFiles)

	probe := func(_ context.Context, _ string) (*ffprobe.Result, error) {
		n := atomic.AddInt32(&cur, 1)
		for {
			old := atomic.LoadInt32(&peak)
			if n <= old || atomic.CompareAndSwapInt32(&peak, old, n) {
				break
			}
		}
		started <- struct{}{}
		<-release
		atomic.AddInt32(&cur, -1)
		codec := "h264"
		return &ffprobe.Result{VideoCodec: &codec}, nil
	}

	st, err := store.Open(filepath.Join(t.TempDir(), "pool.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := &config.Config{
		MoviesPath:       root,
		TVPath:           root,
		ProbeConcurrency: capN,
		ScanInterval:     time.Hour,
	}
	sc, err := New(st, cfg, WithProbeFunc(probe), WithDebounceDur(10*time.Millisecond))
	if err != nil {
		t.Fatalf("new scanner: %v", err)
	}

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := sc.Scan(ctx, "manual", true); err != nil {
			t.Errorf("scan: %v", err)
		}
	}()

	for i := 0; i < capN; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatalf("probe %d never started", i+1)
		}
	}

	select {
	case <-started:
		t.Fatalf("more than %d probes running concurrently (worker pool not bounded)", capN)
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	<-done

	if got := atomic.LoadInt32(&peak); got > int32(capN) {
		t.Fatalf("peak concurrent probes = %d, want <= %d", got, capN)
	}
}

// fakeNotifier records what the scanner hands to the new-candidate notifier.
type fakeNotifier struct {
	mu       sync.Mutex
	added    []int64
	discards []int64
}

func (f *fakeNotifier) Add(ids ...int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.added = append(f.added, ids...)
}

func (f *fakeNotifier) Discard(ids ...int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.discards = append(f.discards, ids...)
}

func (f *fakeNotifier) snapshot() (added, discards []int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.added...), append([]int64(nil), f.discards...)
}

// The first scan an instance ever runs is the library baseline, not news: every
// file in it is an insert, and announcing the whole library would be noise.
func TestScanBaselineSkipsNotification(t *testing.T) {
	root := t.TempDir()
	sc, st := newTestScanner(t, root, root)
	notifier := &fakeNotifier{}
	sc.SetNotifier(notifier)
	ctx := context.Background()

	writeFile(t, filepath.Join(root, "Existing.mkv"))
	if _, err := sc.Scan(ctx, TriggerStartup, false); err != nil {
		t.Fatalf("baseline scan: %v", err)
	}
	if added, _ := notifier.snapshot(); len(added) != 0 {
		t.Fatalf("baseline scan queued %d ids, want none", len(added))
	}

	// Everything after the baseline is a real arrival.
	arrival := filepath.Join(root, "Arrival.mkv")
	writeFile(t, arrival)
	if _, err := sc.Scan(ctx, TriggerScheduled, false); err != nil {
		t.Fatalf("second scan: %v", err)
	}

	added, _ := notifier.snapshot()
	if len(added) != 1 {
		t.Fatalf("queued %v, want exactly the new file", added)
	}
	f, err := st.Media.GetByPath(ctx, arrival)
	if err != nil {
		t.Fatalf("get arrival: %v", err)
	}
	if added[0] != f.ID {
		t.Errorf("queued id %d, want %d (%s)", added[0], f.ID, arrival)
	}
}

// A container swap inserts a row for the new path, but it is the same content
// under a new extension — an arrival only in the bookkeeping sense.
func TestScanSupersededRowNotAnnounced(t *testing.T) {
	root := t.TempDir()
	sc, _ := newTestScanner(t, root, root)
	notifier := &fakeNotifier{}
	sc.SetNotifier(notifier)
	ctx := context.Background()

	src := filepath.Join(root, "Show", "S01E01.mkv")
	dst := filepath.Join(root, "Show", "S01E01.mp4")
	writeFile(t, src)

	if _, err := sc.Scan(ctx, TriggerStartup, false); err != nil {
		t.Fatalf("baseline scan: %v", err)
	}

	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("re-encoded, much smaller"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := sc.Scan(ctx, TriggerScheduled, false); err != nil {
		t.Fatalf("scan after re-encode: %v", err)
	}

	if added, _ := notifier.snapshot(); len(added) != 0 {
		t.Errorf("queued %v, want nothing for a container swap", added)
	}
}
