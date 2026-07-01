package api

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"

	"reclaim/internal/backfill"
)

// BackfillCoordinator reports backfill task state for GET /api/backfill.
type BackfillCoordinator interface {
	Status(ctx context.Context) ([]backfill.TaskStatus, error)
}

// handleBackfill returns the status of all registered backfill tasks.
func (s *Server) handleBackfill(c *echo.Context) error {
	if s.backfill == nil {
		return c.JSON(http.StatusOK, map[string]any{"tasks": []backfill.TaskStatus{}})
	}
	tasks, err := s.backfill.Status(c.Request().Context())
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"tasks": tasks})
}
