package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	baseURL      = "https://api.themoviedb.org/3"
	imageBaseURL = "https://image.tmdb.org/t/p"
)

type Client struct {
	apiKey  string
	http    *http.Client
	limiter *rate.Limiter
}

func New(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 10 * time.Second},
		limiter: rate.NewLimiter(rate.Limit(3), 5),
	}
}

var ErrNotFound = fmt.Errorf("tmdb: not found")

func (c *Client) get(ctx context.Context, path string, out any) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}
	sep := "&"
	if !strings.Contains(path, "?") {
		sep = "?"
	}
	rawURL := fmt.Sprintf("%s%s%sapi_key=%s", baseURL, path, sep, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tmdb: HTTP %d for %s", resp.StatusCode, path)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
