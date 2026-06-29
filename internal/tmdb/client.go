package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
	var rawURL string
	var bearer bool
	if strings.Contains(c.apiKey, ".") {
		rawURL = baseURL + path
		bearer = true
	} else {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		rawURL = fmt.Sprintf("%s%s%sapi_key=%s", baseURL, path, sep, c.apiKey)
	}

	const maxRetries = 3
	for attempt := range maxRetries {
		if err := c.limiter.Wait(ctx); err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		if bearer {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt == maxRetries-1 {
				return fmt.Errorf("tmdb: rate limited after %d attempts", maxRetries)
			}
			delay := 5 * time.Second
			if s := resp.Header.Get("Retry-After"); s != "" {
				if secs, err := strconv.Atoi(s); err == nil {
					delay = time.Duration(secs) * time.Second
				}
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		if resp.StatusCode == http.StatusNotFound {
			return ErrNotFound
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("tmdb: HTTP %d for %s", resp.StatusCode, path)
		}
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return fmt.Errorf("tmdb: rate limited after %d attempts", maxRetries)
}
