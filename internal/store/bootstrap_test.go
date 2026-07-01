package store

import (
	"context"
	"path/filepath"
	"testing"
)

// insertLegacyRow simulates a pre-upgrade row: probed media with zero savings
// and no library_stats contribution (bypasses the incremental Media.Insert path).
func insertLegacyRow(t *testing.T, s *Store, path, codec string, size int64) int64 {
	t.Helper()
	res, err := s.w.Exec(`
		INSERT INTO media_files (
			path, library_type, size_bytes, mtime, fingerprint,
			video_codec, is_already_hevc, predicted_savings_bytes, status
		) VALUES (?, 'movie', ?, 1, 'fp', ?, 0, 0, 'active')`,
		path, size, codec,
	)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestBootstrap_backfillsSavingsAndStatsOnUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	ctx := context.Background()

	// First open applies migrations; seed a legacy row before bootstrap runs
	// by inserting directly after open on an empty stats table.
	s1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	insertLegacyRow(t, s1, "/m/legacy.mkv", "h264", 10_000)
	s1.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	f, err := s2.Media.GetByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if f.PredictedSavingsBytes <= 0 {
		t.Fatalf("PredictedSavingsBytes = %d after bootstrap, want > 0", f.PredictedSavingsBytes)
	}

	ov, err := s2.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ov.TotalFiles != 1 {
		t.Fatalf("TotalFiles = %d, want 1", ov.TotalFiles)
	}
	if ov.TotalRecoverableBytes != f.PredictedSavingsBytes {
		t.Fatalf("recoverable = %d, want %d", ov.TotalRecoverableBytes, f.PredictedSavingsBytes)
	}

	// Second open is a no-op — bootstrap must not double-apply.
	s2.Close()
	s3, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s3.Close()

	f2, err := s3.Media.GetByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if f2.PredictedSavingsBytes != f.PredictedSavingsBytes {
		t.Fatalf("savings changed on second open: %d → %d", f.PredictedSavingsBytes, f2.PredictedSavingsBytes)
	}
}

func TestBootstrap_noOpOnEmptyLibrary(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	ov, err := s.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ov.TotalFiles != 0 {
		t.Fatalf("TotalFiles = %d, want 0 on fresh store", ov.TotalFiles)
	}
}

func TestEnsureJobEncodeSnapshot_repairsMissingColumns(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	for _, col := range []string{"encode_extra_args", "encode_crf", "encode_preset"} {
		if _, err := st.w.ExecContext(ctx, "ALTER TABLE transcode_jobs DROP COLUMN "+col); err != nil {
			t.Skipf("sqlite DROP COLUMN unavailable: %v", err)
		}
	}

	if err := st.ensureJobEncodeSnapshot(ctx); err != nil {
		t.Fatalf("repair: %v", err)
	}
	has, err := tableHasColumn(ctx, st.w, "transcode_jobs", "encode_preset")
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("encode_preset column missing after repair")
	}
}
