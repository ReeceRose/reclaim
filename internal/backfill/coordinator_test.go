package backfill

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"reclaim/internal/scanner"
	"reclaim/internal/store"
)

type fakeScanner struct {
	mu       sync.Mutex
	scanning bool
	calls    int32
	block    chan struct{}
}

func (f *fakeScanner) StartScan(ctx context.Context, trigger string, force bool) error {
	f.mu.Lock()
	if f.scanning {
		f.mu.Unlock()
		return scanner.ErrScanInProgress
	}
	f.scanning = true
	f.mu.Unlock()
	atomic.AddInt32(&f.calls, 1)

	go func() {
		if f.block != nil {
			<-f.block
		}
		f.mu.Lock()
		f.scanning = false
		f.mu.Unlock()
	}()
	return nil
}

func (f *fakeScanner) finishScan() {
	f.mu.Lock()
	block := f.block
	f.mu.Unlock()
	if block != nil {
		close(block)
	}
}

func openTestDB(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func insertProbedFile(t *testing.T, ctx context.Context, s *store.Store, path string) {
	t.Helper()
	codec := "h264"
	height := 1080
	_, err := s.Media.Insert(ctx, &store.MediaFile{
		Path:        path,
		LibraryType: store.LibraryTypeMovies,
		SizeBytes:   1000,
		Mtime:       1,
		Fingerprint: "fp-" + path,
		VideoCodec:  &codec,
		Height:      &height,
		Status:      store.MediaStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCoordinator_startsFullScanWhenNeeded(t *testing.T) {
	s := openTestDB(t)
	ctx := context.Background()
	insertProbedFile(t, ctx, s, "/m/a.mkv")

	needed, err := s.Media.NeedsCompatibilityBackfill(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !needed {
		t.Fatal("expected compatibility backfill needed")
	}

	fs := &fakeScanner{block: make(chan struct{})}
	coord := NewCoordinator(s, fs, DefaultTasks())
	coord.Start(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&fs.calls) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&fs.calls) != 1 {
		t.Fatal("expected coordinator to start a full scan")
	}

	fs.finishScan()
	coord.OnScanCompleted()

	status, err := coord.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != 1 {
		t.Fatalf("status len = %d, want 1", len(status))
	}
	if status[0].Key != TaskCompatibilityProbe {
		t.Fatalf("key = %q", status[0].Key)
	}
	if !status[0].Needed {
		t.Fatal("expected needed=true until real probe runs")
	}
	if status[0].Running {
		t.Fatal("expected running=false after scan completed")
	}
}

func TestCoordinator_waitsForScanInProgress(t *testing.T) {
	s := openTestDB(t)
	ctx := context.Background()
	insertProbedFile(t, ctx, s, "/m/a.mkv")

	fs := &fakeScanner{block: make(chan struct{})}
	fs.mu.Lock()
	fs.scanning = true
	fs.mu.Unlock()

	coord := NewCoordinator(s, fs, DefaultTasks())
	coord.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&fs.calls) != 0 {
		t.Fatal("should not start while scan in progress")
	}

	fs.mu.Lock()
	fs.scanning = false
	fs.mu.Unlock()
	coord.OnScanCompleted()

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&fs.calls) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&fs.calls) != 1 {
		t.Fatal("expected backfill scan after prior scan finished")
	}
	fs.finishScan()
}
