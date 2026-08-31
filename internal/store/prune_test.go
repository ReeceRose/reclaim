package store

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"
)

// missingSince reads the raw column so tests can assert the retention clock
// without exposing it on MediaFile.
func (m *Media) missingSince(ctx context.Context, id int64) (*int64, error) {
	var ts sql.NullInt64
	if err := m.r.QueryRowContext(ctx,
		"SELECT missing_since FROM media_files WHERE id = ?", id,
	).Scan(&ts); err != nil {
		return nil, err
	}
	if !ts.Valid {
		return nil, nil
	}
	return &ts.Int64, nil
}

// backdateMissing rewrites the retention clock to age a row artificially.
func (m *Media) backdateMissing(ctx context.Context, id, since int64) error {
	_, err := m.w.ExecContext(ctx,
		"UPDATE media_files SET missing_since = ? WHERE id = ?", since, id,
	)
	return err
}

func TestMarkMissing_stampsSinceOnceAndClearsOnReturn(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	id, err := s.Media.insertFile(ctx, testFile{path: "/m/a.mkv", size: 1000, codec: "h264", height: 1080, savings: 400})
	if err != nil {
		t.Fatal(err)
	}
	if since, err := s.Media.missingSince(ctx, id); err != nil || since != nil {
		t.Fatalf("active row: missing_since = %v (err %v), want NULL", since, err)
	}

	before := time.Now().Unix()
	if err := s.Media.MarkMissing(ctx, id, ""); err != nil {
		t.Fatal(err)
	}
	first, err := s.Media.missingSince(ctx, id)
	if err != nil || first == nil {
		t.Fatalf("missing_since after mark = %v (err %v), want a timestamp", first, err)
	}
	if *first < before {
		t.Errorf("missing_since = %d, want >= %d", *first, before)
	}

	// A second mark (watcher event, then scan diff) must not restart the clock.
	if err := s.Media.backdateMissing(ctx, id, 1000); err != nil {
		t.Fatal(err)
	}
	if err := s.Media.MarkMissing(ctx, id, ""); err != nil {
		t.Fatal(err)
	}
	again, err := s.Media.missingSince(ctx, id)
	if err != nil || again == nil || *again != 1000 {
		t.Fatalf("missing_since after re-mark = %v, want 1000 (clock must not reset)", again)
	}

	// The file comes back on a later scan: re-probing clears the clock.
	back := testFile{path: "/m/a.mkv", size: 1000, codec: "h264", height: 1080, savings: 400}.toMedia()
	back.ID = id
	if err := s.Media.UpdateProbe(ctx, back); err != nil {
		t.Fatal(err)
	}
	if since, err := s.Media.missingSince(ctx, id); err != nil || since != nil {
		t.Fatalf("missing_since after return = %v (err %v), want NULL", since, err)
	}
}

func TestPruneMissing_respectsCutoffAndLiveJobs(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	add := func(path string) int64 {
		id, err := s.Media.insertFile(ctx, testFile{path: path, size: 1000, codec: "h264", height: 1080, savings: 400})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}

	active := add("/m/active.mkv")
	old := add("/m/old.mkv")
	recent := add("/m/recent.mkv")
	queued := add("/m/queued.mkv")
	encoded := add("/m/encoded.mkv")

	now := time.Now().Unix()
	for _, id := range []int64{old, recent, queued, encoded} {
		if err := s.Media.MarkMissing(ctx, id, ""); err != nil {
			t.Fatal(err)
		}
	}
	// old, queued, and encoded vanished 30 days ago; recent vanished an hour ago.
	for _, id := range []int64{old, queued, encoded} {
		if err := s.Media.backdateMissing(ctx, id, now-30*86400); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Media.backdateMissing(ctx, recent, now-3600); err != nil {
		t.Fatal(err)
	}

	// A live job protects its file; a finished job does not.
	if err := s.Jobs.insertJobWithStatus(ctx, queued, "queued"); err != nil {
		t.Fatal(err)
	}
	if err := s.Jobs.insertJobWithStatus(ctx, encoded, "completed"); err != nil {
		t.Fatal(err)
	}

	deleted, err := s.Media.PruneMissing(ctx, now-7*86400)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2 (old + encoded)", deleted)
	}

	for _, id := range []int64{active, recent, queued} {
		if _, err := s.Media.GetByID(ctx, id); err != nil {
			t.Errorf("id %d should have survived the prune: %v", id, err)
		}
	}
	for _, id := range []int64{old, encoded} {
		if _, err := s.Media.GetByID(ctx, id); err == nil {
			t.Errorf("id %d should have been pruned", id)
		}
	}

	// The pruned file's job history goes with it (the FK has no cascade, so a
	// leftover row would also have failed the delete outright).
	var jobs int
	if err := s.Media.r.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM transcode_jobs WHERE media_file_id = ?", encoded,
	).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if jobs != 0 {
		t.Errorf("jobs for pruned file = %d, want 0", jobs)
	}
}

func TestPruneMissing_zeroCutoffPurgesAll(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	a, err := s.Media.insertFile(ctx, testFile{path: "/m/a.mkv", size: 1000, codec: "h264", height: 1080})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Media.insertFile(ctx, testFile{path: "/m/b.mkv", size: 2000, codec: "h264", height: 1080})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Media.MarkMissing(ctx, a, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Media.MarkMissing(ctx, b, ""); err != nil {
		t.Fatal(err)
	}

	overview, err := s.Media.MissingOverview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Count != 2 || overview.SizeBytes != 3000 {
		t.Fatalf("overview = %+v, want count 2 / size 3000", overview)
	}

	// Nothing has aged past a 7-day retention, but a manual purge ignores it.
	if n, err := s.Media.PruneMissing(ctx, time.Now().Add(-7*24*time.Hour).Unix()); err != nil || n != 0 {
		t.Fatalf("retention prune deleted %d (err %v), want 0", n, err)
	}
	if n, err := s.Media.PruneMissing(ctx, 0); err != nil || n != 2 {
		t.Fatalf("manual purge deleted %d (err %v), want 2", n, err)
	}

	after, err := s.Media.MissingOverview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.Count != 0 {
		t.Errorf("missing rows after purge = %d, want 0", after.Count)
	}
}

// Pruning must not disturb library totals: MarkMissing already removed each
// row's contribution, so deleting the row is a no-op for library_stats.
func TestPruneMissing_leavesStatsUntouched(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/keep.mkv", size: 5000, codec: "h264", height: 1080, savings: 2000}); err != nil {
		t.Fatal(err)
	}
	gone, err := s.Media.insertFile(ctx, testFile{path: "/m/gone.mkv", size: 8000, codec: "mpeg2video", height: 480, savings: 4800})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Media.MarkMissing(ctx, gone, ""); err != nil {
		t.Fatal(err)
	}

	before, err := s.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.PruneMissing(ctx, 0); err != nil {
		t.Fatal(err)
	}
	after, err := s.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalize(before), normalize(after)) {
		t.Errorf("stats drifted across prune:\nbefore %+v\nafter  %+v", before, after)
	}

	// And the incremental totals still agree with a full recompute.
	if err := s.Stats.Recompute(ctx); err != nil {
		t.Fatal(err)
	}
	recomputed, err := s.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalize(after), normalize(recomputed)) {
		t.Errorf("incremental != recompute after prune:\nincremental %+v\nrecomputed  %+v", after, recomputed)
	}
}
