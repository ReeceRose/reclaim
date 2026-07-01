package scanner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// TestScanPersistsStreams verifies that a probe result's full stream list is
// written to media_streams, and that a later re-probe with a different
// stream layout replaces (not appends to) the old rows.
func TestScanPersistsStreams(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	streamsToReturn := []ffprobe.StreamInfo{
		{Index: 0, CodecType: "video", CodecName: "hevc", Profile: "Main 10", DispositionDefault: true, Extra: map[string]any{"pix_fmt": "yuv420p10le"}},
		{Index: 1, CodecType: "audio", CodecName: "truehd", Channels: 8, Language: "eng"},
		{Index: 2, CodecType: "subtitle", CodecName: "hdmv_pgs_subtitle", Language: "eng"},
	}
	probe := func(_ context.Context, _ string) (*ffprobe.Result, error) {
		codec := "hevc"
		return &ffprobe.Result{VideoCodec: &codec, IsAlreadyHEVC: true, Streams: streamsToReturn}, nil
	}

	cfg := &config.Config{MoviesPath: root, TVPath: root, ProbeConcurrency: 2, ScanInterval: 24 * time.Hour}
	sc, err := New(st, cfg, WithProbeFunc(probe), WithDebounceDur(50*time.Millisecond))
	if err != nil {
		t.Fatalf("new scanner: %v", err)
	}

	path := filepath.Join(root, "movie.mkv")
	writeFile(t, path)

	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatalf("scan: %v", err)
	}

	f, err := st.Media.GetByPath(ctx, path)
	if err != nil {
		t.Fatalf("get by path: %v", err)
	}

	got, err := st.Streams.ListForFile(ctx, f.ID)
	if err != nil {
		t.Fatalf("list streams: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("streams len = %d, want 3", len(got))
	}
	if got[0].CodecType != "video" || got[0].CodecName == nil || *got[0].CodecName != "hevc" {
		t.Errorf("stream[0] = %+v", got[0])
	}
	if !got[0].DispositionDefault {
		t.Error("stream[0].DispositionDefault = false, want true")
	}
	if got[1].Channels == nil || *got[1].Channels != 8 {
		t.Errorf("stream[1].Channels = %v, want 8", got[1].Channels)
	}
	if got[2].CodecName == nil || *got[2].CodecName != "hdmv_pgs_subtitle" {
		t.Errorf("stream[2] = %+v", got[2])
	}

	// Force rescan with a smaller stream layout (e.g. file was replaced) —
	// old rows must be gone, not just appended to.
	streamsToReturn = []ffprobe.StreamInfo{
		{Index: 0, CodecType: "video", CodecName: "h264"},
	}
	if _, err := sc.Scan(ctx, "manual", true); err != nil {
		t.Fatalf("rescan: %v", err)
	}
	got, err = st.Streams.ListForFile(ctx, f.ID)
	if err != nil {
		t.Fatalf("list streams after rescan: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("streams len after rescan = %d, want 1", len(got))
	}
}

// TestScanPersistsCompatibility verifies that a probe result is evaluated
// against every built-in client profile and the verdicts land in
// media_compatibility, and that a rescan with different metadata updates
// (not duplicates) those rows.
func TestScanPersistsCompatibility(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	// HEVC 10-bit + DTS-HD in MKV: on plex_web this is a hard video-codec
	// failure (h264-only allowlist); on apple_tv_4k it should be
	// advisory-only (see docs/COMPATIBILITY PLAN.md §6 corrections).
	mkvHevcDTS := func(_ context.Context, _ string) (*ffprobe.Result, error) {
		codec := "hevc"
		bitDepth := 10
		container := "matroska,webm"
		return &ffprobe.Result{
			VideoCodec:      &codec,
			VideoBitDepth:   &bitDepth,
			ContainerFormat: &container,
			IsAlreadyHEVC:   true,
			Streams: []ffprobe.StreamInfo{
				{Index: 0, CodecType: "video", CodecName: "hevc"},
				{Index: 1, CodecType: "audio", CodecName: "dts", Profile: "DTS-HD MA", Channels: 6},
			},
		}, nil
	}

	cfg := &config.Config{MoviesPath: root, TVPath: root, ProbeConcurrency: 2, ScanInterval: 24 * time.Hour}
	sc, err := New(st, cfg, WithProbeFunc(mkvHevcDTS), WithDebounceDur(50*time.Millisecond))
	if err != nil {
		t.Fatalf("new scanner: %v", err)
	}

	path := filepath.Join(root, "movie.mkv")
	writeFile(t, path)

	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatalf("scan: %v", err)
	}

	f, err := st.Media.GetByPath(ctx, path)
	if err != nil {
		t.Fatalf("get by path: %v", err)
	}

	rows, err := st.Media.CompatibilityForFile(ctx, f.ID)
	if err != nil {
		t.Fatalf("CompatibilityForFile: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("want a verdict for all 4 built-in profiles, got %d: %+v", len(rows), rows)
	}

	byProfile := map[string]store.CompatibilityRow{}
	for _, r := range rows {
		byProfile[r.ClientProfile] = r
	}

	plexWeb, ok := byProfile["plex_web"]
	if !ok {
		t.Fatal("missing plex_web verdict")
	}
	if plexWeb.DirectPlayPredicted {
		t.Errorf("plex_web: want direct_play_predicted=false (h264-only), got verdict %+v", plexWeb)
	}
	foundHardVideoReason := false
	for _, r := range plexWeb.Reasons {
		if r.Code == "video_codec_hevc" && r.Severity == "hard" {
			foundHardVideoReason = true
		}
	}
	if !foundHardVideoReason {
		t.Errorf("plex_web: want a Hard video_codec_hevc reason, got %+v", plexWeb.Reasons)
	}

	appleTV, ok := byProfile["apple_tv_4k"]
	if !ok {
		t.Fatal("missing apple_tv_4k verdict")
	}
	for _, r := range appleTV.Reasons {
		if r.Severity == "hard" {
			t.Errorf("apple_tv_4k: want no Hard reasons for HEVC 10-bit + DTS-HD MKV, got %+v", appleTV.Reasons)
		}
	}

	// Rescan with a plain H.264/AAC/MP4 file — verdicts must be replaced in
	// place, not accumulated as duplicate rows per profile.
	h264AAC := func(_ context.Context, _ string) (*ffprobe.Result, error) {
		codec := "h264"
		container := "mov,mp4,m4a,3gp,3g2,mj2"
		return &ffprobe.Result{
			VideoCodec:      &codec,
			ContainerFormat: &container,
			Streams: []ffprobe.StreamInfo{
				{Index: 0, CodecType: "video", CodecName: "h264"},
				{Index: 1, CodecType: "audio", CodecName: "aac", Channels: 2},
			},
		}, nil
	}
	sc.probeFunc = h264AAC
	if _, err := sc.Scan(ctx, "manual", true); err != nil {
		t.Fatalf("rescan: %v", err)
	}

	rows, err = st.Media.CompatibilityForFile(ctx, f.ID)
	if err != nil {
		t.Fatalf("CompatibilityForFile after rescan: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("want still exactly 4 verdicts (replaced, not accumulated), got %d", len(rows))
	}
	for _, r := range rows {
		if !r.DirectPlayPredicted {
			t.Errorf("%s: want direct_play_predicted=true for plain h264/aac/mp4 after rescan, got %+v", r.ClientProfile, r)
		}
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
	if err := live.Update(nil, nil, nil, nil, &high); err != nil {
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
