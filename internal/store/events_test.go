package store

import (
	"context"
	"errors"
	"testing"
)

func TestEvents_DeleteAndDeleteAll(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	id1, err := s.Events.Insert(ctx, EventScanCompleted, SeverityInfo, "scan one", "")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := s.Events.Insert(ctx, EventJobCompleted, SeverityInfo, "job one", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Events.Delete(ctx, id1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	events, err := s.Events.List(ctx, EventFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != id2 {
		t.Fatalf("after delete: got %+v, want only id %d", events, id2)
	}

	if err := s.Events.Delete(ctx, id1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing: got %v, want ErrNotFound", err)
	}

	if err := s.Events.DeleteAll(ctx); err != nil {
		t.Fatalf("delete all: %v", err)
	}
	events, err = s.Events.List(ctx, EventFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("after delete all: got %d events", len(events))
	}
}
