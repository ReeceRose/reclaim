package tmdb

import (
	"context"
	"fmt"
	"net/url"
)

type MovieDetails struct {
	TMDBID       int
	Title        string
	Tagline      string
	Overview     string
	PosterPath   string
	BackdropPath string
	ReleaseYear  int
	RuntimeMins  int
	VoteAverage  float64
	VoteCount    int
	Genres       []string
	Status       string
	Collection   string
}

type SearchResult struct {
	TMDBID     int
	Title      string
	Year       int
	PosterPath string
}

func (c *Client) FetchMovie(ctx context.Context, title string, year int) (*MovieDetails, error) {
	id, err := c.searchMovieID(ctx, title, year)
	if err != nil {
		return nil, err
	}
	return c.MovieDetails(ctx, id)
}

func (c *Client) SearchMovieResults(ctx context.Context, query string) ([]SearchResult, error) {
	var resp struct {
		Results []struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			ReleaseDate string `json:"release_date"`
			PosterPath  string `json:"poster_path"`
		} `json:"results"`
	}
	if err := c.get(ctx, fmt.Sprintf("/search/movie?query=%s", url.QueryEscape(query)), &resp); err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, min(len(resp.Results), 6))
	for _, r := range resp.Results {
		out = append(out, SearchResult{TMDBID: r.ID, Title: r.Title, Year: parseYear(r.ReleaseDate), PosterPath: r.PosterPath})
		if len(out) == 6 {
			break
		}
	}
	return out, nil
}

func (c *Client) MovieDetails(ctx context.Context, tmdbID int) (*MovieDetails, error) {
	var raw struct {
		ID           int     `json:"id"`
		Title        string  `json:"title"`
		Tagline      string  `json:"tagline"`
		Overview     string  `json:"overview"`
		PosterPath   string  `json:"poster_path"`
		BackdropPath string  `json:"backdrop_path"`
		ReleaseDate  string  `json:"release_date"`
		Runtime      int     `json:"runtime"`
		VoteAverage  float64 `json:"vote_average"`
		VoteCount    int     `json:"vote_count"`
		Status       string  `json:"status"`
		Genres       []struct {
			Name string `json:"name"`
		} `json:"genres"`
		BelongsToCollection *struct {
			Name string `json:"name"`
		} `json:"belongs_to_collection"`
	}
	if err := c.get(ctx, fmt.Sprintf("/movie/%d", tmdbID), &raw); err != nil {
		return nil, err
	}
	genres := make([]string, 0, len(raw.Genres))
	for _, g := range raw.Genres {
		genres = append(genres, g.Name)
	}
	d := &MovieDetails{
		TMDBID: raw.ID, Title: raw.Title, Tagline: raw.Tagline, Overview: raw.Overview,
		PosterPath: raw.PosterPath, BackdropPath: raw.BackdropPath,
		ReleaseYear: parseYear(raw.ReleaseDate), RuntimeMins: raw.Runtime,
		VoteAverage: raw.VoteAverage, VoteCount: raw.VoteCount,
		Genres: genres, Status: raw.Status,
	}
	if raw.BelongsToCollection != nil {
		d.Collection = raw.BelongsToCollection.Name
	}
	return d, nil
}

func (c *Client) searchMovieID(ctx context.Context, title string, year int) (int, error) {
	path := fmt.Sprintf("/search/movie?query=%s", url.QueryEscape(title))
	if year > 0 {
		path += fmt.Sprintf("&year=%d", year)
	}
	var resp struct {
		Results []struct {
			ID int `json:"id"`
		} `json:"results"`
	}
	if err := c.get(ctx, path, &resp); err != nil {
		return 0, err
	}
	if len(resp.Results) > 0 {
		return resp.Results[0].ID, nil
	}
	if year > 0 {
		var resp2 struct {
			Results []struct {
				ID int `json:"id"`
			} `json:"results"`
		}
		path2 := fmt.Sprintf("/search/movie?query=%s", url.QueryEscape(title))
		if err := c.get(ctx, path2, &resp2); err == nil && len(resp2.Results) > 0 {
			return resp2.Results[0].ID, nil
		}
	}
	return 0, ErrNotFound
}

func parseYear(date string) int {
	if len(date) < 4 {
		return 0
	}
	y := 0
	for _, r := range date[:4] {
		if r < '0' || r > '9' {
			return 0
		}
		y = y*10 + int(r-'0')
	}
	return y
}
