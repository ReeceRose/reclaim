package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestFindSuperseder_matchesOnlyExactStem(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	gone, err := s.Media.insertFile(ctx, testFile{path: "/tv/S07E01.mkv", size: 1800, codec: "h264", height: 1080})
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := s.Media.insertFile(ctx, testFile{path: "/tv/S07E01.mp4", size: 350, codec: "hevc", height: 1080, hevc: true})
	if err != nil {
		t.Fatal(err)
	}
	// Neither of these may be mistaken for the replacement: one only shares a
	// prefix, the other lives in a different directory.
	if _, err := s.Media.insertFile(ctx, testFile{path: "/tv/S07E01.2160p.mp4", size: 900, codec: "hevc", height: 2160}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.insertFile(ctx, testFile{path: "/tv/extras/S07E01.mp4", size: 100, codec: "hevc", height: 1080}); err != nil {
		t.Fatal(err)
	}

	got, err := s.Media.FindSuperseder(ctx, "/tv/S07E01.mkv", gone)
	if err != nil {
		t.Fatalf("FindSuperseder: %v", err)
	}
	if got.ID != replacement {
		t.Errorf("matched id %d (%s), want %d (/tv/S07E01.mp4)", got.ID, got.Path, replacement)
	}
}

func TestFindSuperseder_skipsAmbiguousAndMissingCandidates(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	gone, err := s.Media.insertFile(ctx, testFile{path: "/m/a.mkv", size: 1000, codec: "h264", height: 1080})
	if err != nil {
		t.Fatal(err)
	}

	// No sibling at all.
	if _, err := s.Media.FindSuperseder(ctx, "/m/a.mkv", gone); !errors.Is(err, ErrNotFound) {
		t.Errorf("no sibling: err = %v, want ErrNotFound", err)
	}

	// A sibling that is itself missing is not a replacement.
	stale, err := s.Media.insertFile(ctx, testFile{path: "/m/a.avi", size: 900, codec: "mpeg4", height: 480})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Media.MarkMissing(ctx, stale, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.FindSuperseder(ctx, "/m/a.mkv", gone); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing sibling: err = %v, want ErrNotFound", err)
	}

	// Two live siblings: refuse to guess.
	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/a.mp4", size: 400, codec: "hevc", height: 1080}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/a.m4v", size: 420, codec: "hevc", height: 1080}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.FindSuperseder(ctx, "/m/a.mkv", gone); !errors.Is(err, ErrNotFound) {
		t.Errorf("ambiguous siblings: err = %v, want ErrNotFound", err)
	}
}

func TestSupersede_transfersJobsAndLeavesStatsConsistent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	old, err := s.Media.insertFile(ctx, testFile{path: "/tv/e1.mkv", size: 1800, codec: "h264", height: 1080, savings: 900})
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := s.Media.insertFile(ctx, testFile{path: "/tv/e1.mp4", size: 350, codec: "hevc", height: 1080, hevc: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Jobs.insertJobWithStatus(ctx, old, "completed"); err != nil {
		t.Fatal(err)
	}

	// The old row is still active here: the scanner supersedes before it would
	// have marked the file missing, so its contribution must come off now.
	if err := s.Media.Supersede(ctx, old, replacement); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Media.GetByID(ctx, old); !errors.Is(err, ErrNotFound) {
		t.Errorf("old row: err = %v, want ErrNotFound (hard-deleted)", err)
	}
	survivor, err := s.Media.GetByID(ctx, replacement)
	if err != nil {
		t.Fatal(err)
	}
	if survivor.SizeBytes != 350 || !survivor.IsAlreadyHEVC {
		t.Errorf("survivor = %d bytes hevc=%v, want its own probe data (350, true)", survivor.SizeBytes, survivor.IsAlreadyHEVC)
	}

	var jobs int
	if err := s.Media.r.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM transcode_jobs WHERE media_file_id = ?", replacement,
	).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if jobs != 1 {
		t.Errorf("jobs on survivor = %d, want 1 (history follows the file)", jobs)
	}

	incremental, err := s.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Stats.Recompute(ctx); err != nil {
		t.Fatal(err)
	}
	recomputed, err := s.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalize(incremental), normalize(recomputed)) {
		t.Errorf("stats drifted across supersede:\nincremental %+v\nrecomputed  %+v", incremental, recomputed)
	}
}

func TestSupersede_alreadyMissingRowDoesNotDoubleDiscount(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	old, err := s.Media.insertFile(ctx, testFile{path: "/tv/e2.mkv", size: 1800, codec: "h264", height: 1080, savings: 900})
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := s.Media.insertFile(ctx, testFile{path: "/tv/e2.mp4", size: 350, codec: "hevc", height: 1080, hevc: true})
	if err != nil {
		t.Fatal(err)
	}
	// The watcher marked it missing first; the next scan supersedes it.
	if err := s.Media.MarkMissing(ctx, old, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Media.Supersede(ctx, old, replacement); err != nil {
		t.Fatal(err)
	}

	incremental, err := s.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Stats.Recompute(ctx); err != nil {
		t.Fatal(err)
	}
	recomputed, err := s.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalize(incremental), normalize(recomputed)) {
		t.Errorf("stats double-discounted:\nincremental %+v\nrecomputed  %+v", incremental, recomputed)
	}
}

func TestSupersede_refusesWhileJobInFlight(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	old, err := s.Media.insertFile(ctx, testFile{path: "/tv/e3.mkv", size: 1800, codec: "h264", height: 1080})
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := s.Media.insertFile(ctx, testFile{path: "/tv/e3.mp4", size: 350, codec: "hevc", height: 1080, hevc: true})
	if err != nil {
		t.Fatal(err)
	}
	// The worker is mid-swap: the original only looks absent.
	if err := s.Jobs.insertJobWithStatus(ctx, old, "running"); err != nil {
		t.Fatal(err)
	}

	if err := s.Media.Supersede(ctx, old, replacement); !errors.Is(err, ErrJobInFlight) {
		t.Fatalf("Supersede: err = %v, want ErrJobInFlight", err)
	}
	if _, err := s.Media.GetByID(ctx, old); err != nil {
		t.Errorf("old row should survive a refused supersede: %v", err)
	}
}
