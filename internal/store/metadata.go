package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type MediaMetadata struct {
	Key          string
	MediaType    string
	TMDBID       *int64
	Title        *string
	Tagline      *string
	Overview     *string
	PosterPath   *string
	BackdropPath *string
	ReleaseYear  *int
	RuntimeMins  *int
	VoteAverage  *float64
	VoteCount    *int64
	Genres       []string
	Status       *string
	Collection   *string
	Network      *string
	InProduction *bool
	IsManual     bool
	NoMatch      bool
	FetchedAt    int64
}

type StaleEntry struct {
	Key       string
	MediaType string
	Status    *string
	IsManual  bool
	NoMatch   bool
	FetchedAt int64
}

type Metadata struct {
	r, w *sql.DB
}

func (m *Metadata) Get(ctx context.Context, key string) (*MediaMetadata, error) {
	row := m.r.QueryRowContext(ctx, metadataQ+" WHERE key = ?", key)
	return scanMetadata(row)
}

func (m *Metadata) GetBatch(ctx context.Context, keys []string) (map[string]*MediaMetadata, error) {
	if len(keys) == 0 {
		return map[string]*MediaMetadata{}, nil
	}
	placeholders := strings.Repeat("?,", len(keys))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(keys))
	for i, k := range keys {
		args[i] = k
	}
	rows, err := m.r.QueryContext(ctx, metadataQ+" WHERE key IN ("+placeholders+")", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]*MediaMetadata, len(keys))
	for rows.Next() {
		meta, err := scanMetadata(rows)
		if err != nil {
			return nil, err
		}
		out[meta.Key] = meta
	}
	return out, rows.Err()
}

func (m *Metadata) Upsert(ctx context.Context, meta *MediaMetadata) error {
	genres, _ := json.Marshal(meta.Genres)
	var inProd *int
	if meta.InProduction != nil {
		v := btoi(*meta.InProduction)
		inProd = &v
	}
	_, err := m.w.ExecContext(ctx, `
		INSERT INTO media_metadata (
			key, media_type, tmdb_id, title, tagline, overview,
			poster_path, backdrop_path, release_year, runtime_mins,
			vote_average, vote_count, genres, status, collection, network,
			in_production, is_manual, no_match, fetched_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(key) DO UPDATE SET
			media_type=excluded.media_type, tmdb_id=excluded.tmdb_id,
			title=excluded.title, tagline=excluded.tagline, overview=excluded.overview,
			poster_path=excluded.poster_path, backdrop_path=excluded.backdrop_path,
			release_year=excluded.release_year, runtime_mins=excluded.runtime_mins,
			vote_average=excluded.vote_average, vote_count=excluded.vote_count,
			genres=excluded.genres, status=excluded.status,
			collection=excluded.collection, network=excluded.network,
			in_production=excluded.in_production,
			is_manual=CASE WHEN media_metadata.is_manual=1 THEN 1 ELSE excluded.is_manual END,
			no_match=excluded.no_match, fetched_at=excluded.fetched_at`,
		meta.Key, meta.MediaType, meta.TMDBID, meta.Title, meta.Tagline, meta.Overview,
		meta.PosterPath, meta.BackdropPath, meta.ReleaseYear, meta.RuntimeMins,
		meta.VoteAverage, meta.VoteCount, string(genres), meta.Status,
		meta.Collection, meta.Network, inProd,
		btoi(meta.IsManual), btoi(meta.NoMatch), meta.FetchedAt,
	)
	return err
}

func (m *Metadata) SetNoMatch(ctx context.Context, key, mediaType string) error {
	now := time.Now().Unix()
	_, err := m.w.ExecContext(ctx, `
		INSERT INTO media_metadata (key, media_type, no_match, fetched_at)
		VALUES (?, ?, 1, ?)
		ON CONFLICT(key) DO UPDATE SET no_match=1, fetched_at=excluded.fetched_at
		WHERE is_manual=0`,
		key, mediaType, now,
	)
	return err
}

func (m *Metadata) SetManual(ctx context.Context, key, mediaType string, posterPath, backdropPath *string) error {
	now := time.Now().Unix()
	_, err := m.w.ExecContext(ctx, `
		INSERT INTO media_metadata (key, media_type, poster_path, backdrop_path, is_manual, no_match, fetched_at)
		VALUES (?, ?, ?, ?, 1, 0, ?)
		ON CONFLICT(key) DO UPDATE SET
			poster_path=excluded.poster_path,
			backdrop_path=excluded.backdrop_path,
			is_manual=1, no_match=0, fetched_at=excluded.fetched_at`,
		key, mediaType, posterPath, backdropPath, now,
	)
	return err
}

func (m *Metadata) StaleEntries(ctx context.Context) ([]StaleEntry, error) {
	rows, err := m.r.QueryContext(ctx,
		"SELECT key, media_type, status, is_manual, no_match, fetched_at FROM media_metadata WHERE is_manual=0 AND no_match=0",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StaleEntry
	for rows.Next() {
		var e StaleEntry
		var isManual, noMatch int
		if err := rows.Scan(&e.Key, &e.MediaType, &e.Status, &isManual, &noMatch, &e.FetchedAt); err != nil {
			return nil, err
		}
		e.IsManual = isManual != 0
		e.NoMatch = noMatch != 0
		out = append(out, e)
	}
	return out, rows.Err()
}

const metadataQ = `
	SELECT key, media_type, tmdb_id, title, tagline, overview,
		poster_path, backdrop_path, release_year, runtime_mins,
		vote_average, vote_count, genres, status, collection, network,
		in_production, is_manual, no_match, fetched_at
	FROM media_metadata`

func scanMetadata(s rowScanner) (*MediaMetadata, error) {
	var m MediaMetadata
	var (
		tmdbID     sql.NullInt64
		voteCount  sql.NullInt64
		genresJSON sql.NullString
		inProd     sql.NullInt64
		isManual   int
		noMatch    int
	)
	err := s.Scan(
		&m.Key, &m.MediaType, &tmdbID, &m.Title, &m.Tagline, &m.Overview,
		&m.PosterPath, &m.BackdropPath, &m.ReleaseYear, &m.RuntimeMins,
		&m.VoteAverage, &voteCount, &genresJSON, &m.Status,
		&m.Collection, &m.Network, &inProd, &isManual, &noMatch, &m.FetchedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if tmdbID.Valid {
		v := tmdbID.Int64
		m.TMDBID = &v
	}
	if voteCount.Valid {
		v := voteCount.Int64
		m.VoteCount = &v
	}
	if genresJSON.Valid && genresJSON.String != "" {
		_ = json.Unmarshal([]byte(genresJSON.String), &m.Genres)
	}
	if inProd.Valid {
		v := inProd.Int64 != 0
		m.InProduction = &v
	}
	m.IsManual = isManual != 0
	m.NoMatch = noMatch != 0
	return &m, nil
}
