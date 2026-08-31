package scanner

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"reclaim/internal/config"
	"reclaim/internal/store"
)

// newReplaceScanner is newTestScanner with replacement matching switched on.
// The default helper leaves the lookback at zero (off), so tests that don't
// care about replacements keep their existing behaviour.
func newReplaceScanner(t *testing.T, movieRoot, tvRoot string) (*Scanner, *store.Store) {
	t.Helper()
	sc, st, _ := newReplaceScannerAt(t, movieRoot, tvRoot)
	return sc, st
}

// newReplaceScannerAt also returns the database path, for the one test that has
// to reach past the store to backdate a row's arrival time.
func newReplaceScannerAt(t *testing.T, movieRoot, tvRoot string) (*Scanner, *store.Store, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := &config.Config{
		MoviesPath:       movieRoot,
		TVPath:           tvRoot,
		ProbeConcurrency: 2,
		ScanInterval:     24 * time.Hour,
		ReplaceLookback:  30 * 24 * time.Hour,
	}
	sc, err := New(st, cfg, WithProbeFunc(mockProbe), WithDebounceDur(50*time.Millisecond))
	if err != nil {
		t.Fatalf("new scanner: %v", err)
	}
	return sc, st, dbPath
}

// ageFirstSeen backdates a row's arrival so it falls outside the lookback.
func ageFirstSeen(t *testing.T, dbPath, filePath string, at time.Time) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(
		"UPDATE media_files SET first_seen_at = ? WHERE path = ?", at.Unix(), filePath,
	); err != nil {
		t.Fatal(err)
	}
}

func writeSizedFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The headline case: delete an episode, download another release of it under a
// completely different filename, and the two scans reconcile into one ledger
// entry rather than a lost file and an unrelated arrival.
func TestScan_matchesRedownloadAcrossScans(t *testing.T) {
	root := t.TempDir()
	movies := filepath.Join(root, "movies")
	tv := filepath.Join(root, "tv")
	sc, st := newReplaceScanner(t, movies, tv)
	ctx := context.Background()

	dir := filepath.Join(tv, "Breaking Bad", "Season 1")
	oldPath := filepath.Join(dir, "Breaking.Bad.S01E01.1080p.WEB-DL.x264-GRP.mkv")
	writeSizedFile(t, oldPath, 4000)

	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatalf("first scan: %v", err)
	}
	original, err := st.Media.GetByPath(ctx, oldPath)
	if err != nil {
		t.Fatalf("original not indexed: %v", err)
	}

	// The delete lands first and is seen on its own, as it would be for a
	// library the user cleared out days before re-downloading.
	if err := os.Remove(oldPath); err != nil {
		t.Fatal(err)
	}
	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatalf("second scan: %v", err)
	}
	gone, err := st.Media.GetByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("row vanished entirely: %v", err)
	}
	if gone.Status != store.MediaStatusMissing {
		t.Fatalf("status = %q, want missing", gone.Status)
	}

	newPath := filepath.Join(dir, "Breaking Bad - S01E01 - 1080p HEVC-OTHR.mkv")
	writeSizedFile(t, newPath, 1500)
	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatalf("third scan: %v", err)
	}

	if _, err := st.Media.GetByID(ctx, original.ID); err == nil {
		t.Error("the replaced row should have been folded away")
	}
	replacement, err := st.Media.GetByPath(ctx, newPath)
	if err != nil {
		t.Fatalf("replacement not indexed: %v", err)
	}

	entries, err := st.Savings.RecentReplacements(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d ledger entries, want 1", len(entries))
	}
	e := entries[0]
	if e.MediaFileID != replacement.ID || e.PreviousPath != oldPath || e.Path != newPath {
		t.Errorf("entry = %+v, want %s → %s", e, oldPath, newPath)
	}
	if e.BytesSaved != 2500 {
		t.Errorf("BytesSaved = %d, want 2500", e.BytesSaved)
	}
}

// A file replaced and removed within one scan is reconciled in that same pass:
// the vanished loop stamps the identity, the arrival pass matches it.
func TestScan_matchesRedownloadWithinOneScan(t *testing.T) {
	root := t.TempDir()
	movies := filepath.Join(root, "movies")
	tv := filepath.Join(root, "tv")
	sc, st := newReplaceScanner(t, movies, tv)
	ctx := context.Background()

	dir := filepath.Join(tv, "Severance", "Season 2")
	oldPath := filepath.Join(dir, "Severance.S02E03.1080p.x264.mkv")
	writeSizedFile(t, oldPath, 5000)
	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(oldPath); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(dir, "Severance.S02E03.2160p.x265-GRP.mkv")
	writeSizedFile(t, newPath, 2000)

	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatal(err)
	}

	entries, err := st.Savings.RecentReplacements(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d ledger entries, want 1", len(entries))
	}
	if entries[0].BytesSaved != 3000 {
		t.Errorf("BytesSaved = %d, want 3000", entries[0].BytesSaved)
	}
}

// A different episode arriving after a delete is not a replacement, however
// close in time — matching is on content identity, not on adjacency.
func TestScan_doesNotMatchADifferentEpisode(t *testing.T) {
	root := t.TempDir()
	movies := filepath.Join(root, "movies")
	tv := filepath.Join(root, "tv")
	sc, st := newReplaceScanner(t, movies, tv)
	ctx := context.Background()

	dir := filepath.Join(tv, "The Wire", "Season 3")
	oldPath := filepath.Join(dir, "The.Wire.S03E01.mkv")
	writeSizedFile(t, oldPath, 4000)
	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(oldPath); err != nil {
		t.Fatal(err)
	}
	writeSizedFile(t, filepath.Join(dir, "The.Wire.S03E02.mkv"), 1000)
	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatal(err)
	}

	entries, err := st.Savings.RecentReplacements(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("matched an unrelated episode: %+v", entries)
	}
}

// The other ordering: the replacement is imported first and the old file is
// removed afterwards, which is how an *arr upgrade normally lands.
func TestScan_matchesWhenTheReplacementArrivesFirst(t *testing.T) {
	root := t.TempDir()
	movies := filepath.Join(root, "movies")
	tv := filepath.Join(root, "tv")
	sc, st := newReplaceScanner(t, movies, tv)
	ctx := context.Background()

	dir := filepath.Join(tv, "Severance", "Season 1")
	oldPath := filepath.Join(dir, "Severance.S01E01.1080p.x264.mkv")
	writeSizedFile(t, oldPath, 6000)
	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatal(err)
	}
	original, err := st.Media.GetByPath(ctx, oldPath)
	if err != nil {
		t.Fatal(err)
	}

	newPath := filepath.Join(dir, "Severance.S01E01.1080p.x265-NEW.mkv")
	writeSizedFile(t, newPath, 2500)
	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(oldPath); err != nil {
		t.Fatal(err)
	}

	// The watcher path handles this ordering; drive it directly rather than
	// waiting on fsnotify debounces.
	sc.checkVanishedFile(ctx, oldPath)

	if _, err := st.Media.GetByID(ctx, original.ID); err == nil {
		t.Error("the replaced row should have been folded away, not marked missing")
	}
	entries, err := st.Savings.RecentReplacements(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d ledger entries, want 1", len(entries))
	}
	if entries[0].BytesSaved != 3500 {
		t.Errorf("BytesSaved = %d, want 3500", entries[0].BytesSaved)
	}
}

// With the lookback at zero the whole mechanism is off and a delete is just a
// delete.
func TestScan_replacementMatchingCanBeDisabled(t *testing.T) {
	root := t.TempDir()
	sc, st := newTestScanner(t, filepath.Join(root, "movies"), filepath.Join(root, "tv"))
	ctx := context.Background()

	dir := filepath.Join(root, "tv", "Severance", "Season 1")
	oldPath := filepath.Join(dir, "Severance.S01E01.x264.mkv")
	writeSizedFile(t, oldPath, 4000)
	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatal(err)
	}
	original, err := st.Media.GetByPath(ctx, oldPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(oldPath); err != nil {
		t.Fatal(err)
	}
	writeSizedFile(t, filepath.Join(dir, "Severance.S01E01.x265.mkv"), 1000)
	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatal(err)
	}

	gone, err := st.Media.GetByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("row folded away with matching disabled: %v", err)
	}
	if gone.Status != store.MediaStatusMissing {
		t.Errorf("status = %q, want missing", gone.Status)
	}
	entries, err := st.Savings.RecentReplacements(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("ledger wrote %d entries with matching disabled", len(entries))
	}
}

// Deleting one of two copies a library deliberately keeps side by side is a
// deletion, not a replacement. Both rows share a content key — the ambiguity
// guard cannot separate them, since removing either leaves exactly one
// survivor — so the survivor's own arrival time is what has to rule it out.
func TestScan_deletingOneOfTwoLongStandingCopiesIsNotAReplacement(t *testing.T) {
	root := t.TempDir()
	movies := filepath.Join(root, "movies")
	tv := filepath.Join(root, "tv")
	sc, st, dbPath := newReplaceScannerAt(t, movies, tv)
	ctx := context.Background()

	dir := filepath.Join(tv, "Severance", "Season 1")
	hd := filepath.Join(dir, "Severance.S01E01.1080p.x264.mkv")
	uhd := filepath.Join(dir, "Severance.S01E01.2160p.x265.mkv")
	writeSizedFile(t, hd, 4000)
	writeSizedFile(t, uhd, 9000)
	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatal(err)
	}
	original, err := st.Media.GetByPath(ctx, hd)
	if err != nil {
		t.Fatal(err)
	}

	// Age the 4K copy past the lookback: it has been in the library for a year,
	// so it cannot be what replaced anything today.
	ageFirstSeen(t, dbPath, uhd, time.Now().Add(-365*24*time.Hour))

	if err := os.Remove(hd); err != nil {
		t.Fatal(err)
	}
	sc.checkVanishedFile(ctx, hd)

	gone, err := st.Media.GetByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("the deleted row should still exist as missing: %v", err)
	}
	if gone.Status != store.MediaStatusMissing {
		t.Errorf("status = %q, want %q", gone.Status, store.MediaStatusMissing)
	}
	entries, err := st.Savings.RecentReplacements(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("booked %d replacement(s) for a plain deletion, want 0", len(entries))
	}
}

// Re-acquiring the same release is not a replacement: the identical file comes
// back under a new name, so the original row is revived at the new path with
// its history rather than folded into a 0-byte ledger entry.
func TestScan_identicalRedownloadIsAReturnNotAReplacement(t *testing.T) {
	root := t.TempDir()
	movies := filepath.Join(root, "movies")
	tv := filepath.Join(root, "tv")
	sc, st := newReplaceScanner(t, movies, tv)
	ctx := context.Background()

	dir := filepath.Join(tv, "Naked and Afraid", "Season 1")
	oldPath := filepath.Join(dir, "Naked and Afraid - S01E05 - Smoke Signals.mkv")
	writeSizedFile(t, oldPath, 4000)
	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatal(err)
	}
	original, err := st.Media.GetByPath(ctx, oldPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(oldPath); err != nil {
		t.Fatal(err)
	}
	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatal(err)
	}

	// Same bytes, new name — the release the user downloaded a second time.
	newPath := filepath.Join(dir, "Naked.And.Afraid.S01E05.720p.WEB.H264-JFF.mkv")
	writeSizedFile(t, newPath, 4000)
	if _, err := sc.Scan(ctx, "manual", false); err != nil {
		t.Fatal(err)
	}

	back, err := st.Media.GetByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("the original row should have been revived, not folded away: %v", err)
	}
	if back.Path != newPath {
		t.Errorf("path = %q, want %q", back.Path, newPath)
	}
	if back.Status != store.MediaStatusActive {
		t.Errorf("status = %q, want %q", back.Status, store.MediaStatusActive)
	}

	entries, err := st.Savings.RecentReplacements(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("booked %d replacement(s) for a file that returned unchanged: %+v", len(entries), entries)
	}
}
