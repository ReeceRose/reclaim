package store

import (
	"context"
	"database/sql"
)

// MediaStream is one row of media_streams: a single ffprobe stream (video,
// audio, or subtitle) belonging to a media_files row. Unlike MediaFile's
// denormalized primary-stream columns, this captures every stream so the
// compatibility engine can reason about multi-audio-track and multi-subtitle
// files (internal/compatibility).
type MediaStream struct {
	ID                 int64
	MediaFileID        int64
	StreamIndex        int
	CodecType          string // video | audio | subtitle
	CodecName          *string
	Profile            *string
	Channels           *int
	Language           *string
	DispositionDefault bool
	ExtraJSON          *string // pix_fmt, level, bit_rate, sample_rate, color_* — see ffprobe.StreamInfo.Extra
}

// Streams provides access to media_streams rows.
type Streams struct {
	r, w *sql.DB
}

// ReplaceForFile atomically swaps all stream rows for fileID: delete then
// bulk-insert. Called once per probe (scanner.probeAndStore), so a file's
// stream layout is always a clean replacement rather than an incremental
// diff — simpler and correct even when stream count/order changes between
// probes (e.g. a file was replaced on disk with a different encode).
func (s *Streams) ReplaceForFile(ctx context.Context, fileID int64, streams []MediaStream) error {
	tx, err := s.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM media_streams WHERE media_file_id = ?", fileID,
	); err != nil {
		return err
	}

	if len(streams) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO media_streams (
				media_file_id, stream_index, codec_type, codec_name, profile,
				channels, language, disposition_default, extra_json
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, st := range streams {
			if _, err := stmt.ExecContext(ctx,
				fileID, st.StreamIndex, st.CodecType, st.CodecName, st.Profile,
				st.Channels, st.Language, btoi(st.DispositionDefault), st.ExtraJSON,
			); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// ListForFile returns all streams for fileID ordered by their original
// ffprobe stream_index.
func (s *Streams) ListForFile(ctx context.Context, fileID int64) ([]MediaStream, error) {
	rows, err := s.r.QueryContext(ctx, `
		SELECT id, media_file_id, stream_index, codec_type, codec_name, profile,
			channels, language, disposition_default, extra_json
		FROM media_streams
		WHERE media_file_id = ?
		ORDER BY stream_index ASC`,
		fileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MediaStream
	for rows.Next() {
		var st MediaStream
		var dispositionDefault int
		if err := rows.Scan(
			&st.ID, &st.MediaFileID, &st.StreamIndex, &st.CodecType, &st.CodecName, &st.Profile,
			&st.Channels, &st.Language, &dispositionDefault, &st.ExtraJSON,
		); err != nil {
			return nil, err
		}
		st.DispositionDefault = dispositionDefault != 0
		out = append(out, st)
	}
	return out, rows.Err()
}
