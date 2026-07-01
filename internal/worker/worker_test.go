package worker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"reclaim/internal/ffmpeg"
	"reclaim/internal/ffprobe"
	"reclaim/internal/store"
)

// --- test doubles ----------------------------------------------------------

type fakeHub struct {
	mu     sync.Mutex
	events []string
}

func (f *fakeHub) Broadcast(event string, _ any) {
	f.mu.Lock()
	f.events = append(f.events, event)
	f.mu.Unlock()
}

func (f *fakeHub) has(event string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.events {
		if e == event {
			return true
		}
	}
	return false
}

type fakeWindow struct{ start, end time.Duration }

func (f fakeWindow) EncodeWindowStart() time.Duration { return f.start }
func (f fakeWindow) EncodeWindowEnd() time.Duration   { return f.end }

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// seedRunningJob writes a source file, inserts its media row, and claims a job so
// it is in the `running` state ready for processJob.
func seedRunningJob(t *testing.T, st *store.Store, srcPath, content string) *store.TranscodeJob {
	t.Helper()
	ctx := context.Background()
	if err := os.WriteFile(srcPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	codec := "h264"
	dur := 10.0
	w, h := 1920, 1080
	id, err := st.Media.Insert(ctx, &store.MediaFile{
		Path: srcPath, LibraryType: "movie", SizeBytes: int64(len(content)),
		Mtime: 1, Fingerprint: "fp", VideoCodec: &codec,
		Width: &w, Height: &h, DurationSeconds: &dur,
		PredictedSavingsBytes: 100, Status: "active",
	})
	if err != nil {
		t.Fatalf("insert media: %v", err)
	}
	if _, err := st.Jobs.Create(ctx, &store.TranscodeJob{
		MediaFileID: id, ProfileID: 1, Status: "queued", QueuedAt: 1,
		OriginalSizeBytes: int64(len(content)),
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}
	job, err := st.Jobs.ClaimNextQueued(ctx, 100)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	return job
}

func matchingInspect(*ffprobe.Inspection) InspectFunc {
	return func(context.Context, string) (*ffprobe.Inspection, error) {
		return &ffprobe.Inspection{
			DurationSeconds: 10, Width: 1920, Height: 1080,
			VideoStreams: 1, AudioStreams: 1,
		}, nil
	}
}

// --- window ----------------------------------------------------------------

func TestWithinWindow(t *testing.T) {
	at := func(h, m int) func() time.Time {
		return func() time.Time { return time.Date(2026, 1, 1, h, m, 0, 0, time.Local) }
	}
	cases := []struct {
		name       string
		start, end time.Duration
		clock      func() time.Time
		wantOpen   bool
	}{
		{"inside daytime", 9 * time.Hour, 17 * time.Hour, at(12, 0), true},
		{"before daytime", 9 * time.Hour, 17 * time.Hour, at(8, 0), false},
		{"at end is closed", 9 * time.Hour, 17 * time.Hour, at(17, 0), false},
		{"overnight inside late", 22 * time.Hour, 6 * time.Hour, at(23, 30), true},
		{"overnight inside early", 22 * time.Hour, 6 * time.Hour, at(2, 0), true},
		{"overnight outside", 22 * time.Hour, 6 * time.Hour, at(12, 0), false},
		{"zero-length always open", 3 * time.Hour, 3 * time.Hour, at(12, 0), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := New(nil, fakeWindow{c.start, c.end}, &fakeHub{}, nil, WithClock(c.clock))
			if got := w.withinWindow(); got != c.wantOpen {
				t.Errorf("withinWindow = %v, want %v", got, c.wantOpen)
			}
		})
	}
}

// --- processJob ------------------------------------------------------------

func TestProcessJobHappyPath(t *testing.T) {
	st := newStore(t)
	hub := &fakeHub{}
	dir := t.TempDir()
	src := filepath.Join(dir, "movie.mkv")
	job := seedRunningJob(t, st, src, "original-bytes")

	encode := func(_ context.Context, opts ffmpeg.Options, onProgress ffmpeg.ProgressFunc) error {
		if err := os.WriteFile(opts.OutputPath, []byte("hevc"), 0o644); err != nil {
			return err
		}
		onProgress(50)
		onProgress(100)
		return nil
	}

	w := New(st, fakeWindow{0, 0}, hub, []string{dir},
		WithEncodeFunc(encode),
		WithInspectFunc(matchingInspect(nil)),
		WithProgressDBPeriod(0),
	)
	w.processJob(context.Background(), job)

	got, _ := st.Jobs.GetByID(context.Background(), job.ID)
	if got.Status != "completed" {
		t.Fatalf("status = %q, want completed", got.Status)
	}
	if got.OutputSizeBytes == nil || *got.OutputSizeBytes != int64(len("hevc")) {
		t.Errorf("output size = %v, want 4", got.OutputSizeBytes)
	}
	if got.VerificationResult == nil {
		t.Error("verification_result not stored")
	}

	// Original was atomically replaced by the encoded content.
	b, _ := os.ReadFile(src)
	if string(b) != "hevc" {
		t.Errorf("source content = %q, want swapped to %q", b, "hevc")
	}
	// Temp and backup are gone.
	if _, err := os.Stat(tempPathFor(src)); !os.IsNotExist(err) {
		t.Error("temp file should be removed after swap")
	}
	if _, err := os.Stat(src + backupSuffix); !os.IsNotExist(err) {
		t.Error("backup file should be removed after swap")
	}
	// Media row converted to HEVC → drops out of candidates.
	f, _ := st.Media.GetByID(context.Background(), job.MediaFileID)
	if !f.IsAlreadyHEVC {
		t.Error("media row not marked HEVC")
	}
	if !hub.has("job_started") || !hub.has("job_completed") {
		t.Errorf("missing broadcasts: %v", hub.events)
	}
}

func TestProcessJobVerificationFailureKeepsTemp(t *testing.T) {
	st := newStore(t)
	hub := &fakeHub{}
	dir := t.TempDir()
	src := filepath.Join(dir, "movie.mkv")
	job := seedRunningJob(t, st, src, "original")

	encode := func(_ context.Context, opts ffmpeg.Options, _ ffmpeg.ProgressFunc) error {
		return os.WriteFile(opts.OutputPath, []byte("truncated"), 0o644)
	}
	// Output duration is way off → duration check fails.
	inspect := func(_ context.Context, path string) (*ffprobe.Inspection, error) {
		if path == src {
			return &ffprobe.Inspection{DurationSeconds: 10, Width: 1920, Height: 1080, VideoStreams: 1, AudioStreams: 1}, nil
		}
		return &ffprobe.Inspection{DurationSeconds: 2, Width: 1920, Height: 1080, VideoStreams: 1, AudioStreams: 1}, nil
	}

	w := New(st, fakeWindow{0, 0}, hub, []string{dir},
		WithEncodeFunc(encode), WithInspectFunc(inspect))
	w.processJob(context.Background(), job)

	got, _ := st.Jobs.GetByID(context.Background(), job.ID)
	if got.Status != "failed" {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	if got.VerificationResult == nil {
		t.Error("verification_result should be stored on failure")
	}
	// Temp kept for inspection; original untouched.
	if _, err := os.Stat(tempPathFor(src)); err != nil {
		t.Errorf("temp should be kept on verification failure: %v", err)
	}
	b, _ := os.ReadFile(src)
	if string(b) != "original" {
		t.Errorf("original modified: %q", b)
	}
	if !hub.has("job_failed") {
		t.Errorf("expected job_failed broadcast: %v", hub.events)
	}
}

func TestProcessJobEncodeFailureDeletesTemp(t *testing.T) {
	st := newStore(t)
	hub := &fakeHub{}
	dir := t.TempDir()
	src := filepath.Join(dir, "movie.mkv")
	job := seedRunningJob(t, st, src, "original")

	encode := func(_ context.Context, opts ffmpeg.Options, _ ffmpeg.ProgressFunc) error {
		_ = os.WriteFile(opts.OutputPath, []byte("partial"), 0o644)
		return &ffmpeg.EncodeError{Path: opts.InputPath, Msg: "boom"}
	}

	w := New(st, fakeWindow{0, 0}, hub, []string{dir},
		WithEncodeFunc(encode), WithInspectFunc(matchingInspect(nil)))
	w.processJob(context.Background(), job)

	got, _ := st.Jobs.GetByID(context.Background(), job.ID)
	if got.Status != "failed" {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	if _, err := os.Stat(tempPathFor(src)); !os.IsNotExist(err) {
		t.Error("partial temp should be deleted on encode failure")
	}
	b, _ := os.ReadFile(src)
	if string(b) != "original" {
		t.Errorf("original modified: %q", b)
	}
}

func TestCancelRunningJob(t *testing.T) {
	st := newStore(t)
	hub := &fakeHub{}
	dir := t.TempDir()
	src := filepath.Join(dir, "movie.mkv")
	job := seedRunningJob(t, st, src, "original")

	started := make(chan struct{})
	encode := func(ctx context.Context, opts ffmpeg.Options, _ ffmpeg.ProgressFunc) error {
		_ = os.WriteFile(opts.OutputPath, []byte("partial"), 0o644)
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}

	w := New(st, fakeWindow{0, 0}, hub, []string{dir},
		WithEncodeFunc(encode), WithInspectFunc(matchingInspect(nil)))

	done := make(chan struct{})
	go func() {
		w.processJob(context.Background(), job)
		close(done)
	}()

	<-started
	if !w.Cancel(job.ID) {
		t.Fatal("Cancel returned false for a running job")
	}
	<-done

	got, _ := st.Jobs.GetByID(context.Background(), job.ID)
	if got.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", got.Status)
	}
	if _, err := os.Stat(tempPathFor(src)); !os.IsNotExist(err) {
		t.Error("temp should be removed on cancel")
	}
	b, _ := os.ReadFile(src)
	if string(b) != "original" {
		t.Errorf("original modified: %q", b)
	}
}

// --- orphan cleanup --------------------------------------------------------

func TestSweepOrphans(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	dir := t.TempDir()

	pathA := filepath.Join(dir, "a.mkv")
	pathB := filepath.Join(dir, "b.mkv")
	pathC := filepath.Join(dir, "c.mkv")

	// A stale temp → deleted.
	staleTmp := tempPathFor(pathA)
	mustWrite(t, staleTmp, "junk")

	// A backup whose original exists → backup deleted.
	mustWrite(t, pathB, "orig")
	mustWrite(t, pathB+backupSuffix, "old")

	// A backup whose original is missing → restored.
	mustWrite(t, pathC+backupSuffix, "recovered")

	for _, path := range []string{pathA, pathB, pathC} {
		if _, err := st.Media.Insert(ctx, &store.MediaFile{
			Path: path, LibraryType: "movie", SizeBytes: 1, Mtime: 1,
			Fingerprint: path, Status: "active",
		}); err != nil {
			t.Fatalf("insert %s: %v", path, err)
		}
	}

	w := New(st, fakeWindow{0, 0}, &fakeHub{}, []string{dir})
	w.sweepOrphans(ctx)

	if _, err := os.Stat(staleTmp); !os.IsNotExist(err) {
		t.Error("stale temp not removed")
	}
	if _, err := os.Stat(pathB + backupSuffix); !os.IsNotExist(err) {
		t.Error("leftover backup not removed when original present")
	}
	b, err := os.ReadFile(pathC)
	if err != nil || string(b) != "recovered" {
		t.Errorf("backup not restored over missing original: %q %v", b, err)
	}
	if _, err := os.Stat(pathC + backupSuffix); !os.IsNotExist(err) {
		t.Error("backup should be gone after restore")
	}
}

func TestSweepOrphansEmptyLibraryFast(t *testing.T) {
	st := newStore(t)
	w := New(st, fakeWindow{0, 0}, &fakeHub{}, []string{t.TempDir()})
	start := time.Now()
	w.sweepOrphans(context.Background())
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("sweep took %v, want <1s on empty library", elapsed)
	}
}

func TestSweepOrphansSkipsActiveTemp(t *testing.T) {
	st := newStore(t)
	dir := t.TempDir()
	activeTmp := filepath.Join(dir, "live.mkv"+tmpSuffix+".mkv")
	mustWrite(t, activeTmp, "in-progress")

	w := New(st, fakeWindow{0, 0}, &fakeHub{}, []string{dir})
	w.active[42] = &activeJob{tempPath: activeTmp}

	w.sweepOrphans(context.Background())
	if _, err := os.Stat(activeTmp); err != nil {
		t.Error("active temp should not be swept")
	}
}

func TestReconcileInterrupted(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	dir := t.TempDir()
	src := filepath.Join(dir, "movie.mkv")
	job := seedRunningJob(t, st, src, "original") // status running

	tmp := tempPathFor(src)
	mustWrite(t, tmp, "partial")
	if err := st.Jobs.SetOutputPath(ctx, job.ID, tmp); err != nil {
		t.Fatalf("set output path: %v", err)
	}

	w := New(st, fakeWindow{0, 0}, &fakeHub{}, []string{dir})
	w.reconcileInterrupted(ctx)

	got, _ := st.Jobs.GetByID(ctx, job.ID)
	if got.Status != "failed" {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Error("interrupted job's temp should be removed")
	}
}

func TestReconcilePostSwapCommit(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	dir := t.TempDir()
	src := filepath.Join(dir, "movie.mkv")
	mustWrite(t, src, "encoded-hevc-bytes")

	codec := "h264"
	id, err := st.Media.Insert(ctx, &store.MediaFile{
		Path: src, LibraryType: "movie", SizeBytes: int64(len("encoded-hevc-bytes")),
		Mtime: 1, Fingerprint: "fp", VideoCodec: &codec, Status: "active",
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	started := int64(100)
	jid, err := st.Jobs.Create(ctx, &store.TranscodeJob{
		MediaFileID: id, ProfileID: 1, Status: "verifying",
		QueuedAt: 1, StartedAt: &started, OriginalSizeBytes: 100,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	hub := &fakeHub{}
	hevc := "hevc"
	w := New(st, fakeWindow{0, 0}, hub, []string{dir}, WithProbeFunc(func(_ context.Context, _ string) (*ffprobe.Result, error) {
		return &ffprobe.Result{VideoCodec: &hevc, IsAlreadyHEVC: true}, nil
	}))
	w.reconcileInterrupted(ctx)

	got, _ := st.Jobs.GetByID(ctx, jid)
	if got.Status != "completed" {
		t.Fatalf("status = %q, want completed", got.Status)
	}
	if !hub.has("job_completed") {
		t.Fatal("expected job_completed broadcast after reconcile")
	}
	f, _ := st.Media.GetByID(ctx, id)
	if !f.IsAlreadyHEVC {
		t.Fatal("media row not updated after reconcile")
	}
}

// --- real ffmpeg integration ----------------------------------------------

func TestEncodeVerifyReplaceReal(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}

	st := newStore(t)
	ctx := context.Background()
	hub := &fakeHub{}
	dir := t.TempDir()
	src := filepath.Join(dir, "sample.mkv")

	// Generate a tiny real source: 1 video + 1 audio stream, ~1s, 128x96.
	gen := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=128x96:rate=10",
		"-f", "lavfi", "-i", "sine=frequency=1000:duration=1",
		"-c:v", "libx264", "-c:a", "aac", "-shortest", src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("could not generate fixture (%v): %s", err, out)
	}

	// Probe + index it the way the scanner would.
	insp, err := ffprobe.Inspect(ctx, src)
	if err != nil {
		t.Fatalf("inspect source: %v", err)
	}
	codec := "h264"
	id, err := st.Media.Insert(ctx, &store.MediaFile{
		Path: src, LibraryType: "movie", SizeBytes: fileSize(t, src), Mtime: 1,
		Fingerprint: "fp", VideoCodec: &codec,
		Width: &insp.Width, Height: &insp.Height, DurationSeconds: &insp.DurationSeconds,
		Status: "active",
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := st.Jobs.Create(ctx, &store.TranscodeJob{
		MediaFileID: id, ProfileID: 1, Status: "queued", QueuedAt: 1, OriginalSizeBytes: 1,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}
	job, err := st.Jobs.ClaimNextQueued(ctx, 100)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Real encode + verify + replace, with a fast preset to keep the test quick.
	fastProfile := "ultrafast"
	if err := st.Profiles.Update(ctx, &store.TranscodeProfile{
		ID: 1, Name: "Test", CRF: 30, Preset: fastProfile, IsDefault: true,
	}); err != nil {
		t.Fatalf("update profile: %v", err)
	}

	w := New(st, fakeWindow{0, 0}, hub, []string{dir})
	w.processJob(ctx, job)

	got, _ := st.Jobs.GetByID(ctx, job.ID)
	if got.Status != "completed" {
		t.Fatalf("status = %q, want completed (err=%v verification=%v)", got.Status, deref(got.ErrorMessage), deref(got.VerificationResult))
	}

	// The swapped-in file is real HEVC now.
	res, err := ffprobe.Probe(ctx, src)
	if err != nil {
		t.Fatalf("probe result: %v", err)
	}
	if !res.IsAlreadyHEVC {
		t.Errorf("swapped file is not HEVC: codec=%v", deref(res.VideoCodec))
	}
	f, _ := st.Media.GetByID(ctx, id)
	if !f.IsAlreadyHEVC {
		t.Error("media row not marked HEVC after real encode")
	}
}

// --- helpers ---------------------------------------------------------------

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Size()
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
