package store

import (
	"context"
	"fmt"
)

func (m *Media) touch(ctx context.Context, id int64) error {
	_, err := m.w.ExecContext(ctx,
		"UPDATE media_files SET mtime = mtime WHERE id = ?", id,
	)
	return err
}

func (m *Media) insertTestRow(ctx context.Context, path string) (int64, error) {
	return m.Insert(ctx, &MediaFile{
		Path:        path,
		LibraryType: "movie",
		SizeBytes:   1000,
		Mtime:       1,
		Fingerprint: fmt.Sprintf("fp-%s", path),
		Status:      "active",
	})
}

// testFile is a convenient builder for store tests that need probe fields set.
type testFile struct {
	path        string
	libraryType string
	size        int64
	mtime       int64
	codec       string // "" → leave video_codec NULL
	height      int
	hevc        bool
	savings     int64
	probeErr    string // non-empty → set probe_error
	status      string // "" → "active"
}

func (tf testFile) toMedia() *MediaFile {
	f := &MediaFile{
		Path:                  tf.path,
		LibraryType:           orDefault(tf.libraryType, "movie"),
		SizeBytes:             tf.size,
		Mtime:                 tf.mtime,
		Fingerprint:           "fp-" + tf.path,
		IsAlreadyHEVC:         tf.hevc,
		PredictedSavingsBytes: tf.savings,
		Status:                orDefault(tf.status, "active"),
	}
	if tf.codec != "" {
		c := tf.codec
		f.VideoCodec = &c
	}
	if tf.height != 0 {
		h := tf.height
		f.Height = &h
	}
	if tf.probeErr != "" {
		e := tf.probeErr
		f.ProbeError = &e
	}
	return f
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func (m *Media) insertFile(ctx context.Context, tf testFile) (int64, error) {
	return m.Insert(ctx, tf.toMedia())
}

// insertJob adds a transcode job row with the given status for exclusion tests.
func (j *Jobs) insertJobWithStatus(ctx context.Context, mediaID int64, status string) error {
	id, err := j.Create(ctx, &TranscodeJob{
		MediaFileID:       mediaID,
		ProfileID:         1,
		Status:            "queued",
		QueuedAt:          1,
		OriginalSizeBytes: 1,
	})
	if err != nil {
		return err
	}
	return j.UpdateStatus(ctx, id, status)
}
