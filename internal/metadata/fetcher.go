package metadata

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"reclaim/internal/media"
	"reclaim/internal/store"
	"reclaim/internal/tmdb"
)

type Fetcher struct {
	store      *store.Store
	moviesPath string
	tvPath     string
	trigger    chan struct{}
}

func New(s *store.Store, moviesPath, tvPath string) *Fetcher {
	return &Fetcher{
		store:      s,
		moviesPath: moviesPath,
		tvPath:     tvPath,
		trigger:    make(chan struct{}, 1),
	}
}

func (f *Fetcher) Trigger() {
	select {
	case f.trigger <- struct{}{}:
	default:
	}
}

func (f *Fetcher) Start(ctx context.Context) {
	f.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-f.trigger:
			f.runOnce(ctx)
		}
	}
}

func (f *Fetcher) runOnce(ctx context.Context) {
	apiKey, err := f.store.Settings.GetTMDBKey(ctx)
	if err != nil || apiKey == "" {
		return
	}
	client := tmdb.New(apiKey)
	if err := f.fetchMissing(ctx, client); err != nil && !errors.Is(err, context.Canceled) {
		slog.Warn("metadata fetch failed", "err", err)
	}
}

func (f *Fetcher) fetchMissing(ctx context.Context, client *tmdb.Client) error {
	existing, err := f.store.Metadata.StaleEntries(ctx)
	if err != nil {
		return err
	}
	existingMap := make(map[string]*store.StaleEntry, len(existing))
	for i := range existing {
		existingMap[existing[i].Key] = &existing[i]
	}

	now := time.Now().Unix()

	titles, err := f.store.Media.DistinctSeriesTitles(ctx)
	if err != nil {
		return err
	}
	for _, title := range titles {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		entry, ok := existingMap[title]
		if ok && entry.NoMatch {
			continue
		}
		if ok && !isStale(entry, now) {
			continue
		}
		if err := f.fetchTV(ctx, client, title); err != nil {
			slog.Warn("tmdb: tv fetch failed", "title", title, "err", err)
		}
	}

	movieKeys, err := f.store.Media.DistinctMovieKeys(ctx, f.moviesPath)
	if err != nil {
		return err
	}
	for _, key := range movieKeys {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		entry, ok := existingMap[key]
		if ok && entry.NoMatch {
			continue
		}
		if ok && !isStale(entry, now) {
			continue
		}
		if err := f.fetchMovie(ctx, client, key); err != nil {
			slog.Warn("tmdb: movie fetch failed", "key", key, "err", err)
		}
	}
	return nil
}

func (f *Fetcher) fetchTV(ctx context.Context, client *tmdb.Client, title string) error {
	details, err := client.FetchTV(ctx, title)
	if errors.Is(err, tmdb.ErrNotFound) {
		return f.store.Metadata.SetNoMatch(ctx, title, "tv")
	}
	if err != nil {
		return err
	}
	inProd := details.InProduction
	meta := &store.MediaMetadata{
		Key:          title,
		MediaType:    "tv",
		TMDBID:       int64ptr(int64(details.TMDBID)),
		Title:        strptr(details.Name),
		Tagline:      strptr(details.Tagline),
		Overview:     strptr(details.Overview),
		PosterPath:   strptr(details.PosterPath),
		BackdropPath: strptr(details.BackdropPath),
		ReleaseYear:  intptr(details.FirstAirYear),
		RuntimeMins:  intptr(details.EpisodeRuntime),
		VoteAverage:  f64ptr(details.VoteAverage),
		VoteCount:    int64ptr(int64(details.VoteCount)),
		Genres:       details.Genres,
		Status:       strptr(details.Status),
		Network:      strptr(details.Network),
		InProduction: &inProd,
		FetchedAt:    time.Now().Unix(),
	}
	return f.store.Metadata.Upsert(ctx, meta)
}

func (f *Fetcher) fetchMovie(ctx context.Context, client *tmdb.Client, key string) error {
	title, year := media.ParseMovieInfo(key+"/placeholder", "")
	if title == "" {
		title = key
	}
	details, err := client.FetchMovie(ctx, title, year)
	if errors.Is(err, tmdb.ErrNotFound) {
		return f.store.Metadata.SetNoMatch(ctx, key, "movie")
	}
	if err != nil {
		return err
	}
	meta := &store.MediaMetadata{
		Key:          key,
		MediaType:    "movie",
		TMDBID:       int64ptr(int64(details.TMDBID)),
		Title:        strptr(details.Title),
		Tagline:      strptr(details.Tagline),
		Overview:     strptr(details.Overview),
		PosterPath:   strptr(details.PosterPath),
		BackdropPath: strptr(details.BackdropPath),
		ReleaseYear:  intptr(details.ReleaseYear),
		RuntimeMins:  intptr(details.RuntimeMins),
		VoteAverage:  f64ptr(details.VoteAverage),
		VoteCount:    int64ptr(int64(details.VoteCount)),
		Genres:       details.Genres,
		Status:       strptr(details.Status),
		Collection:   strptr(details.Collection),
		FetchedAt:    time.Now().Unix(),
	}
	return f.store.Metadata.Upsert(ctx, meta)
}

func (f *Fetcher) RefreshKey(ctx context.Context, key, mediaType string) error {
	apiKey, err := f.store.Settings.GetTMDBKey(ctx)
	if err != nil || apiKey == "" {
		return errors.New("tmdb api key not configured")
	}
	client := tmdb.New(apiKey)
	switch mediaType {
	case "tv":
		return f.fetchTV(ctx, client, key)
	case "movie":
		return f.fetchMovie(ctx, client, key)
	default:
		return errors.New("unknown media type")
	}
}

func isStale(e *store.StaleEntry, now int64) bool {
	if e.FetchedAt == 0 {
		return true
	}
	days := (now - e.FetchedAt) / 86400
	switch {
	case e.Status != nil && (*e.Status == "Ended" || *e.Status == "Cancelled" || *e.Status == "Released"):
		return days > 90
	case e.Status != nil && *e.Status == "Returning Series":
		return days > 14
	default:
		return days > 30
	}
}

func strptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intptr(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

func int64ptr(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func f64ptr(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}
