package api

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"reclaim/internal/store"
	"reclaim/internal/tmdb"
)

func (s *Server) handleMetadataGet(c *echo.Context) error {
	key := c.QueryParam("key")
	if key == "" {
		return badRequest(c, "key is required")
	}
	if s.store.Metadata == nil {
		return c.JSON(http.StatusOK, nil)
	}
	ctx := c.Request().Context()
	meta, err := s.store.Metadata.Get(ctx, key)
	if errors.Is(err, store.ErrNotFound) {
		return c.JSON(http.StatusOK, nil)
	}
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"key":           meta.Key,
		"media_type":    meta.MediaType,
		"tmdb_id":       meta.TMDBID,
		"title":         meta.Title,
		"tagline":       meta.Tagline,
		"overview":      meta.Overview,
		"poster_path":   meta.PosterPath,
		"backdrop_path": meta.BackdropPath,
		"release_year":  meta.ReleaseYear,
		"runtime_mins":  meta.RuntimeMins,
		"vote_average":  meta.VoteAverage,
		"vote_count":    meta.VoteCount,
		"genres":        meta.Genres,
		"status":        meta.Status,
		"network":       meta.Network,
		"in_production": meta.InProduction,
		"is_manual":     meta.IsManual,
		"no_match":      meta.NoMatch,
	})
}

func (s *Server) handleMetadataSearch(c *echo.Context) error {
	query := c.QueryParam("query")
	mediaType := c.QueryParam("type")
	if query == "" {
		return badRequest(c, "query is required")
	}

	ctx := c.Request().Context()
	apiKey := s.tmdbAPIKey()
	if apiKey == "" {
		return badRequest(c, "tmdb api key not configured")
	}
	client := tmdb.New(apiKey)

	var (
		results []tmdb.SearchResult
		err     error
	)
	switch mediaType {
	case "movie":
		results, err = client.SearchMovieResults(ctx, query)
	default:
		results, err = client.SearchTVResults(ctx, query)
	}
	if err != nil {
		return serverError(c, err)
	}

	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, map[string]any{
			"tmdb_id":    r.TMDBID,
			"title":      r.Title,
			"year":       r.Year,
			"poster_url": tmdb.PosterURL(r.PosterPath, "w185"),
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"results": out})
}

type metadataOverrideRequest struct {
	Key         string  `json:"key"`
	MediaType   string  `json:"media_type"`
	PosterURL   *string `json:"poster_url"`
	BackdropURL *string `json:"backdrop_url"`
}

func (s *Server) handleMetadataOverride(c *echo.Context) error {
	var req metadataOverrideRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if req.Key == "" || req.MediaType == "" {
		return badRequest(c, "key and media_type are required")
	}
	ctx := c.Request().Context()
	if err := s.store.Metadata.SetManual(ctx, req.Key, req.MediaType, req.PosterURL, req.BackdropURL); err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

type metadataRefreshRequest struct {
	Key       string `json:"key"`
	MediaType string `json:"media_type"`
}

func (s *Server) handleMetadataRefresh(c *echo.Context) error {
	if s.metaFetcher == nil {
		return c.JSON(http.StatusServiceUnavailable, errorBody("metadata fetcher unavailable"))
	}
	var req metadataRefreshRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	ctx := c.Request().Context()
	if req.Key != "" && req.MediaType != "" {
		if err := s.metaFetcher.RefreshKey(ctx, req.Key, req.MediaType); err != nil {
			return serverError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
	s.metaFetcher.Trigger()
	return c.JSON(http.StatusOK, map[string]string{"status": "queued"})
}
