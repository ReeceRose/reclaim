package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

// Migration 00016 rebuilds savings_ledger to make job_id nullable, which in
// SQLite means copying every row into a new table. An existing instance's
// lifetime reclaimed total has to survive that copy intact.
func TestMigration16_preservesExistingLedgerRows(t *testing.T) {
	if err := initGoose(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "m16.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if err := goose.UpTo(db, "migrations", 15); err != nil {
		t.Fatalf("migrate to 15: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO savings_ledger (
			job_id, media_file_id, path, library_type, source_codec,
			width, height, original_size_bytes, output_size_bytes,
			predicted_savings_bytes, encode_seconds, completed_at
		) VALUES (7, 42, '/movies/Dune.mkv', 'movies', 'h264',
			3840, 2160, 48000000000, 24000000000, 26000000000, 7200, 1756252800)`,
	); err != nil {
		t.Fatalf("seed v15 ledger row: %v", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("migrate to head: %v", err)
	}

	var (
		source            string
		jobID             sql.NullInt64
		path, codec       string
		original, output  int64
		predicted, encSec sql.NullInt64
	)
	if err := db.QueryRow(`
		SELECT source, job_id, path, source_codec, original_size_bytes,
		       output_size_bytes, predicted_savings_bytes, encode_seconds
		FROM savings_ledger`,
	).Scan(&source, &jobID, &path, &codec, &original, &output, &predicted, &encSec); err != nil {
		t.Fatalf("read migrated row: %v", err)
	}

	if source != "encode" {
		t.Errorf("source = %q, want encode", source)
	}
	if !jobID.Valid || jobID.Int64 != 7 {
		t.Errorf("job_id = %v, want 7", jobID)
	}
	if path != "/movies/Dune.mkv" || codec != "h264" {
		t.Errorf("path/codec = %q/%q", path, codec)
	}
	if original != 48000000000 || output != 24000000000 {
		t.Errorf("sizes = %d → %d", original, output)
	}
	if !predicted.Valid || predicted.Int64 != 26000000000 || !encSec.Valid || encSec.Int64 != 7200 {
		t.Errorf("predicted/encode_seconds = %v/%v", predicted, encSec)
	}

	// The partial unique index must still reject a second row for the same job
	// while letting any number of replacement rows share a NULL job_id.
	if _, err := db.Exec(`
		INSERT OR IGNORE INTO savings_ledger (source, job_id, media_file_id, original_size_bytes, output_size_bytes, completed_at)
		VALUES ('encode', 7, 42, 1, 1, 1)`); err != nil {
		t.Fatalf("duplicate job insert: %v", err)
	}
	for range 2 {
		if _, err := db.Exec(`
			INSERT INTO savings_ledger (source, match_kind, media_file_id, original_size_bytes, output_size_bytes, completed_at)
			VALUES ('replace', 'redownload', 99, 10, 4, 1)`); err != nil {
			t.Fatalf("replacement insert: %v", err)
		}
	}

	var encodes, replaces int
	if err := db.QueryRow(
		"SELECT COUNT(*) FILTER (WHERE source = 'encode'), COUNT(*) FILTER (WHERE source = 'replace') FROM savings_ledger",
	).Scan(&encodes, &replaces); err != nil {
		t.Fatal(err)
	}
	if encodes != 1 {
		t.Errorf("encode rows = %d, want 1 (the duplicate job should have been ignored)", encodes)
	}
	if replaces != 2 {
		t.Errorf("replace rows = %d, want 2", replaces)
	}

	// Rolling back drops the replacement rows (the v15 schema has nowhere to put
	// them) but must keep every encode.
	if err := goose.Down(db, "migrations"); err != nil {
		t.Fatalf("migrate down: %v", err)
	}
	var remaining int
	if err := db.QueryRow("SELECT COUNT(*) FROM savings_ledger").Scan(&remaining); err != nil {
		t.Fatalf("read rolled-back ledger: %v", err)
	}
	if remaining != 1 {
		t.Errorf("rows after rollback = %d, want 1 (the encode)", remaining)
	}
}
