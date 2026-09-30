package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

// Migration 00023 drops the rows the scanner made for the worker's temp
// outputs, with their jobs, and clears failed jobs' pointers at kept temps.
func TestMigration23_purgesIndexedTemps(t *testing.T) {
	if err := initGoose(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "m23.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if err := goose.UpTo(db, "migrations", 22); err != nil {
		t.Fatalf("migrate to 22: %v", err)
	}

	seed := []string{
		`INSERT INTO media_files (id, path, library_type, size_bytes, mtime, video_codec)
		 VALUES (1, '/tv/Joey/Season 2/S02E12.avi', 'tv', 1000, 1, 'mpeg4'),
		        (2, '/tv/Joey/Season 2/S02E12.avi.reclaim-tmp.avi', 'tv', 800, 1, 'rawvideo')`,
		`INSERT INTO transcode_jobs (id, media_file_id, profile_id, status, queued_at, original_size_bytes, output_path)
		 VALUES (1, 1, 1, 'failed', 1, 1000, '/tv/Joey/Season 2/S02E12.avi.reclaim-tmp.avi'),
		        (2, 2, 1, 'queued', 2, 800, NULL)`,
		`INSERT INTO library_stats (dimension, bucket, file_count, total_bytes, predicted_savings_bytes)
		 VALUES ('total', '', 2, 1800, 500)`,
	}
	for _, q := range seed {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed v22: %v", err)
		}
	}

	if err := goose.UpTo(db, "migrations", 23); err != nil {
		t.Fatalf("migrate to 23: %v", err)
	}

	count := func(q string) int {
		t.Helper()
		var n int
		if err := db.QueryRow(q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(`SELECT COUNT(*) FROM media_files WHERE id = 2`); n != 0 {
		t.Errorf("temp row survived")
	}
	if n := count(`SELECT COUNT(*) FROM media_files WHERE id = 1`); n != 1 {
		t.Errorf("original row deleted")
	}
	if n := count(`SELECT COUNT(*) FROM transcode_jobs WHERE id = 2`); n != 0 {
		t.Errorf("temp row's job survived")
	}
	if n := count(`SELECT COUNT(*) FROM transcode_jobs WHERE id = 1 AND output_path IS NULL`); n != 1 {
		t.Errorf("failed job's output_path not cleared")
	}
	if n := count(`SELECT COUNT(*) FROM library_stats`); n != 0 {
		t.Errorf("library_stats not emptied for rebuild")
	}
}
