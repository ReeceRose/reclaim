package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

// Migration 00019 widens "already HEVC" to "already efficient": existing AV1
// rows must leave the candidate list with their predictions zeroed, and every
// pre-AV1 profile, job, and encode ledger row must be stamped as HEVC so learned
// ratios keep attributing that history to the right target.
func TestMigration19_flagsAV1AndStampsHEVCHistory(t *testing.T) {
	if err := initGoose(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "m19.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if err := goose.UpTo(db, "migrations", 18); err != nil {
		t.Fatalf("migrate to 18: %v", err)
	}

	seed := []string{
		`INSERT INTO media_files (id, path, library_type, size_bytes, mtime, video_codec, is_already_hevc, predicted_savings_bytes)
		 VALUES (1, '/movies/a.mkv', 'movies', 1000, 1, 'h264', 0, 400),
		        (2, '/movies/b.mkv', 'movies', 1000, 1, 'av1', 0, 100),
		        (3, '/movies/c.mkv', 'movies', 1000, 1, 'hevc', 1, 0)`,
		`INSERT INTO transcode_jobs (id, media_file_id, profile_id, status, queued_at, original_size_bytes)
		 VALUES (1, 1, 1, 'completed', 1, 1000)`,
		`INSERT INTO savings_ledger (source, job_id, media_file_id, source_codec, result_codec, original_size_bytes, output_size_bytes, completed_at)
		 VALUES ('encode', 1, 1, 'h264', NULL, 1000, 600, 10),
		        ('replace', NULL, 1, 'mpeg4', 'h264', 2000, 1000, 11)`,
		`INSERT INTO library_stats (dimension, bucket, file_count, total_bytes, predicted_savings_bytes)
		 VALUES ('total', '', 3, 3000, 500)`,
	}
	for _, q := range seed {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed v18: %v", err)
		}
	}

	if err := goose.UpTo(db, "migrations", 19); err != nil {
		t.Fatalf("migrate to 19: %v", err)
	}

	for id, want := range map[int]struct {
		efficient int
		savings   int64
	}{1: {0, 400}, 2: {1, 0}, 3: {1, 0}} {
		var efficient int
		var savings int64
		if err := db.QueryRow(`SELECT is_efficient_codec, predicted_savings_bytes FROM media_files WHERE id = ?`, id).
			Scan(&efficient, &savings); err != nil {
			t.Fatal(err)
		}
		if efficient != want.efficient || savings != want.savings {
			t.Errorf("row %d: efficient=%d savings=%d, want %d/%d", id, efficient, savings, want.efficient, want.savings)
		}
	}

	var profileCodec, jobCodec string
	if err := db.QueryRow(`SELECT codec FROM transcode_profiles WHERE id = 1`).Scan(&profileCodec); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT encode_codec FROM transcode_jobs WHERE id = 1`).Scan(&jobCodec); err != nil {
		t.Fatal(err)
	}
	if profileCodec != "hevc" || jobCodec != "hevc" {
		t.Errorf("profile codec %q, job codec %q, want hevc/hevc", profileCodec, jobCodec)
	}

	var encodeResult, replaceResult string
	if err := db.QueryRow(`SELECT result_codec FROM savings_ledger WHERE source = 'encode'`).Scan(&encodeResult); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT result_codec FROM savings_ledger WHERE source = 'replace'`).Scan(&replaceResult); err != nil {
		t.Fatal(err)
	}
	if encodeResult != "hevc" || replaceResult != "h264" {
		t.Errorf("ledger result codecs: encode %q, replace %q, want hevc and the untouched h264", encodeResult, replaceResult)
	}

	var stats int
	if err := db.QueryRow(`SELECT COUNT(*) FROM library_stats`).Scan(&stats); err != nil {
		t.Fatal(err)
	}
	if stats != 0 {
		t.Errorf("library_stats rows = %d, want 0 so boot rebuilds it", stats)
	}

	if err := goose.Down(db, "migrations"); err != nil {
		t.Fatalf("roll back 19: %v", err)
	}
	var legacy int
	if err := db.QueryRow(`SELECT is_already_hevc FROM media_files WHERE id = 2`).Scan(&legacy); err != nil {
		t.Fatalf("read rolled-back column: %v", err)
	}
	if legacy != 0 {
		t.Errorf("rolled-back AV1 row is_already_hevc = %d, want 0", legacy)
	}
}
