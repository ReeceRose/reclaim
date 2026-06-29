package tmdb

import (
	"context"
	"fmt"
	"net/url"
)

type TVDetails struct {
	TMDBID         int
	Name           string
	Tagline        string
	Overview       string
	PosterPath     string
	BackdropPath   string
	FirstAirYear   int
	EpisodeRuntime int
	VoteAverage    float64
	VoteCount      int
	Genres         []string
	Status         string
	InProduction   bool
	Network        string
}

func (c *Client) FetchTV(ctx context.Context, title string) (*TVDetails, error) {
	id, err := c.searchTVID(ctx, title)
	if err != nil {
		return nil, err
	}
	return c.TVDetails(ctx, id)
}

func (c *Client) SearchTVResults(ctx context.Context, query string) ([]SearchResult, error) {
	var resp struct {
		Results []struct {
			ID           int    `json:"id"`
			Name         string `json:"name"`
			FirstAirDate string `json:"first_air_date"`
			PosterPath   string `json:"poster_path"`
		} `json:"results"`
	}
	if err := c.get(ctx, fmt.Sprintf("/search/tv?query=%s", url.QueryEscape(query)), &resp); err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, min(len(resp.Results), 6))
	for _, r := range resp.Results {
		out = append(out, SearchResult{TMDBID: r.ID, Title: r.Name, Year: parseYear(r.FirstAirDate), PosterPath: r.PosterPath})
		if len(out) == 6 {
			break
		}
	}
	return out, nil
}

func (c *Client) TVDetails(ctx context.Context, tmdbID int) (*TVDetails, error) {
	var raw struct {
		ID             int     `json:"id"`
		Name           string  `json:"name"`
		Tagline        string  `json:"tagline"`
		Overview       string  `json:"overview"`
		PosterPath     string  `json:"poster_path"`
		BackdropPath   string  `json:"backdrop_path"`
		FirstAirDate   string  `json:"first_air_date"`
		EpisodeRunTime []int   `json:"episode_run_time"`
		VoteAverage    float64 `json:"vote_average"`
		VoteCount      int     `json:"vote_count"`
		Status         string  `json:"status"`
		InProduction   bool    `json:"in_production"`
		Genres         []struct {
			Name string `json:"name"`
		} `json:"genres"`
		Networks []struct {
			Name string `json:"name"`
		} `json:"networks"`
	}
	if err := c.get(ctx, fmt.Sprintf("/tv/%d", tmdbID), &raw); err != nil {
		return nil, err
	}
	genres := make([]string, 0, len(raw.Genres))
	for _, g := range raw.Genres {
		genres = append(genres, g.Name)
	}
	runtime := 0
	if len(raw.EpisodeRunTime) > 0 {
		runtime = raw.EpisodeRunTime[0]
	}
	network := ""
	if len(raw.Networks) > 0 {
		network = raw.Networks[0].Name
	}
	return &TVDetails{
		TMDBID: raw.ID, Name: raw.Name, Tagline: raw.Tagline, Overview: raw.Overview,
		PosterPath: raw.PosterPath, BackdropPath: raw.BackdropPath,
		FirstAirYear: parseYear(raw.FirstAirDate), EpisodeRuntime: runtime,
		VoteAverage: raw.VoteAverage, VoteCount: raw.VoteCount,
		Genres: genres, Status: raw.Status, InProduction: raw.InProduction,
		Network: network,
	}, nil
}

func (c *Client) searchTVID(ctx context.Context, title string) (int, error) {
	var resp struct {
		Results []struct {
			ID int `json:"id"`
		} `json:"results"`
	}
	if err := c.get(ctx, fmt.Sprintf("/search/tv?query=%s", url.QueryEscape(title)), &resp); err != nil {
		return 0, err
	}
	if len(resp.Results) == 0 {
		return 0, ErrNotFound
	}
	return resp.Results[0].ID, nil
}
