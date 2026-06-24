package scanner

import (
	"context"
	"os"
	"path/filepath"
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
