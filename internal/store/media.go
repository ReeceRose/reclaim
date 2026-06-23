package store

import (
	"context"
	"database/sql"
	"errors"
)

type MediaFile struct {
	ID                    int64
	Path                  string
	LibraryType           string
	SizeBytes             int64
	Mtime                 int64
	Fingerprint           string
	VideoCodec            *string
	VideoCodecProfile     *string
	Width                 *int
	Height                *int
	DurationSeconds       *float64
	BitrateKbps           *int
	AudioCodec            *string
	AudioChannels         *int
	ContainerFormat       *string
	IsAlreadyHEVC         bool
	PredictedSavingsBytes int64
	LastProbedAt          *int64
	ProbeError            *string
	Status                string
}

type Media struct {
	r, w *sql.DB
}

func (m *Media) GetByID(ctx context.Context, id int64) (*MediaFile, error) {
	return scanMedia(m.r.QueryRowContext(ctx, mediaQ+" WHERE id = ?", id))
}

func (m *Media) GetByPath(ctx context.Context, path string) (*MediaFile, error) {
	return scanMedia(m.r.QueryRowContext(ctx, mediaQ+" WHERE path = ?", path))
}

func (m *Media) GetByFingerprint(ctx context.Context, fp string) (*MediaFile, error) {
	return scanMedia(m.r.QueryRowContext(ctx, mediaQ+" WHERE fingerprint = ? AND status = 'active'", fp))
}

func (m *Media) Insert(ctx context.Context, f *MediaFile) (int64, error) {
	res, err := m.w.ExecContext(ctx, `
		INSERT INTO media_files (
			path, library_type, size_bytes, mtime, fingerprint,
			video_codec, video_codec_profile, width, height, duration_seconds,
			bitrate_kbps, audio_codec, audio_channels, container_format,
			is_already_hevc, predicted_savings_bytes, last_probed_at, probe_error, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.Path, f.LibraryType, f.SizeBytes, f.Mtime, f.Fingerprint,
		f.VideoCodec, f.VideoCodecProfile, f.Width, f.Height, f.DurationSeconds,
		f.BitrateKbps, f.AudioCodec, f.AudioChannels, f.ContainerFormat,
		btoi(f.IsAlreadyHEVC), f.PredictedSavingsBytes, f.LastProbedAt, f.ProbeError, f.Status,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (m *Media) UpdateProbe(ctx context.Context, f *MediaFile) error {
	_, err := m.w.ExecContext(ctx, `
		UPDATE media_files SET
			size_bytes = ?, mtime = ?, fingerprint = ?,
			video_codec = ?, video_codec_profile = ?, width = ?, height = ?,
			duration_seconds = ?, bitrate_kbps = ?, audio_codec = ?, audio_channels = ?,
			container_format = ?, is_already_hevc = ?, predicted_savings_bytes = ?,
			last_probed_at = ?, probe_error = ?, status = ?
		WHERE id = ?`,
		f.SizeBytes, f.Mtime, f.Fingerprint,
		f.VideoCodec, f.VideoCodecProfile, f.Width, f.Height,
		f.DurationSeconds, f.BitrateKbps, f.AudioCodec, f.AudioChannels,
		f.ContainerFormat, btoi(f.IsAlreadyHEVC), f.PredictedSavingsBytes,
		f.LastProbedAt, f.ProbeError, f.Status, f.ID,
	)
	return err
}

func (m *Media) UpdatePath(ctx context.Context, id int64, newPath string) error {
	_, err := m.w.ExecContext(ctx,
		"UPDATE media_files SET path = ?, status = 'active' WHERE id = ?", newPath, id,
	)
	return err
}

func (m *Media) MarkMissing(ctx context.Context, id int64) error {
	_, err := m.w.ExecContext(ctx,
		"UPDATE media_files SET status = 'missing' WHERE id = ?", id,
	)
	return err
}

// ActivePaths returns a path→id map of all active media files. Used by the
// scanner to diff the filesystem against known state.
func (m *Media) ActivePaths(ctx context.Context) (map[string]int64, error) {
	rows, err := m.r.QueryContext(ctx, "SELECT id, path FROM media_files WHERE status = 'active'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		out[path] = id
	}
	return out, rows.Err()
}

const mediaQ = `
	SELECT id, path, library_type, size_bytes, mtime, fingerprint,
		video_codec, video_codec_profile, width, height, duration_seconds,
		bitrate_kbps, audio_codec, audio_channels, container_format,
		is_already_hevc, predicted_savings_bytes, last_probed_at, probe_error, status
	FROM media_files`

func scanMedia(s rowScanner) (*MediaFile, error) {
	var f MediaFile
	var isHEVC int
	err := s.Scan(
		&f.ID, &f.Path, &f.LibraryType, &f.SizeBytes, &f.Mtime, &f.Fingerprint,
		&f.VideoCodec, &f.VideoCodecProfile, &f.Width, &f.Height, &f.DurationSeconds,
		&f.BitrateKbps, &f.AudioCodec, &f.AudioChannels, &f.ContainerFormat,
		&isHEVC, &f.PredictedSavingsBytes, &f.LastProbedAt, &f.ProbeError, &f.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	f.IsAlreadyHEVC = isHEVC != 0
	return &f, nil
}
