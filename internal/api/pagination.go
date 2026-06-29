package api

import (
	"strconv"

	"github.com/labstack/echo/v5"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 200
)

func parseLimitOffset(c *echo.Context, defaultLimit, maxLimit int) (limit, offset int, err error) {
	limit = defaultLimit
	if v := c.QueryParam("limit"); v != "" {
		n, parseErr := strconv.Atoi(v)
		if parseErr != nil || n < 1 {
			return 0, 0, badRequest(c, "limit must be a positive integer")
		}
		limit = n
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset = 0
	if v := c.QueryParam("offset"); v != "" {
		n, parseErr := strconv.Atoi(v)
		if parseErr != nil || n < 0 {
			return 0, 0, badRequest(c, "offset must be a non-negative integer")
		}
		offset = n
	}
	return limit, offset, nil
}

func slicePage[T any](all []T, offset, limit int) []T {
	if offset >= len(all) {
		return nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end]
}
