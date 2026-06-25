package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"

	"reclaim/internal/store"
)

// handleListEvents returns the audit event log, newest-first.
// Query params: ?limit=50 &after_id=<id> &severity=error &type=job_failed
func (s *Server) handleListEvents(c *echo.Context) error {
	ctx := c.Request().Context()

	f := store.EventFilter{Limit: 50}

	if v := c.QueryParam("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			return badRequest(c, "limit must be an integer between 1 and 200")
		}
		f.Limit = n
	}
	if v := c.QueryParam("after_id"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return badRequest(c, "after_id must be an integer")
		}
		f.AfterID = n
	}
	f.Severity = c.QueryParam("severity")
	f.Type = c.QueryParam("type")

	events, err := s.store.Events.List(ctx, f)
	if err != nil {
		return serverError(c, err)
	}

	items := make([]map[string]any, 0, len(events))
	for i := range events {
		items = append(items, toEventDTO(&events[i]))
	}

	resp := map[string]any{"items": items}
	// Emit a next_cursor when a full page was returned (there may be more).
	if len(events) == f.Limit {
		resp["next_cursor"] = events[len(events)-1].ID
	}
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) handleDeleteEvent(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return badRequest(c, "invalid event id")
	}
	if err := s.store.Events.Delete(c.Request().Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return c.JSON(http.StatusNotFound, errorBody("event not found"))
		}
		return serverError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) handleDeleteAllEvents(c *echo.Context) error {
	if err := s.store.Events.DeleteAll(c.Request().Context()); err != nil {
		return serverError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// toEventDTO converts a store.Event to the wire shape, decoding the JSON
// metadata string to an object so the frontend receives it as a nested map.
func toEventDTO(e *store.Event) map[string]any {
	m := map[string]any{
		"id":         e.ID,
		"type":       e.Type,
		"severity":   e.Severity,
		"message":    e.Message,
		"created_at": e.CreatedAt,
		"metadata":   nil,
	}
	if e.Metadata != nil && *e.Metadata != "" {
		var v any
		if err := json.Unmarshal([]byte(*e.Metadata), &v); err == nil {
			m["metadata"] = v
		}
	}
	return m
}

// apiEventPayload builds the event_created WS broadcast payload from an API
// handler that directly cancels a job (bypassing the worker).
func apiEventPayload(eventID int64, eventType, severity, message string, jobID int64) map[string]any {
	return map[string]any{
		"id":         eventID,
		"type":       eventType,
		"severity":   severity,
		"message":    message,
		"created_at": time.Now().Unix(),
		"metadata":   map[string]any{"job_id": jobID},
	}
}
