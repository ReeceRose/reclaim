package store

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
)

var jobFixtureSeq atomic.Int64

func seedJobFixture(t *testing.T, st *Store) (mediaID, jobID int64) {
	t.Helper()
	ctx := context.Background()
	n := jobFixtureSeq.Add(1)
	codec := "h264"
	id, err := st.Media.Insert(ctx, &MediaFile{
		Path: fmt.Sprintf("/media/movies/x%d.mkv", n), LibraryType: "movie", SizeBytes: 5000,
		Mtime: 1, Fingerprint: fmt.Sprintf("fpx%d", n), VideoCodec: &codec,
		PredictedSavingsBytes: 2000, Status: "active",
	})
	if err != nil {
		t.Fatalf("insert media: %v", err)
	}
	jid, err := st.Jobs.Create(ctx, &TranscodeJob{
		MediaFileID: id, ProfileID: 1, Status: "queued",
		QueuedAt: 10, OriginalSizeBytes: 5000,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	return id, jid
}

func TestClaimNextQueued(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	_, jid := seedJobFixture(t, st)

	job, err := st.Jobs.ClaimNextQueued(ctx, 100)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if job.ID != jid {
		t.Fatalf("claimed job %d, want %d", job.ID, jid)
	}
	if job.Status != "running" {
		t.Errorf("status = %q, want running", job.Status)
	}
	if job.StartedAt == nil || *job.StartedAt != 100 {
		t.Errorf("started_at = %v, want 100", job.StartedAt)
	}

	// Queue now empty.
	if _, err := st.Jobs.ClaimNextQueued(ctx, 200); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second claim err = %v, want ErrNotFound", err)
	}
}

func TestClaimNextQueuedOrdering(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	mid, _ := seedJobFixture(t, st)

	// A second, later-queued job.
	later, err := st.Jobs.Create(ctx, &TranscodeJob{
		MediaFileID: mid, ProfileID: 1, Status: "queued",
		QueuedAt: 20, OriginalSizeBytes: 5000,
	})
	if err != nil {
		t.Fatalf("create later: %v", err)
	}

	job, err := st.Jobs.ClaimNextQueued(ctx, 100)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if job.ID == later {
		t.Fatalf("claimed the later job; expected the oldest queued first")
	}
}

func TestTransitionGuard(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	_, jid := seedJobFixture(t, st)

	// queued→running legal.
	if err := st.Jobs.Transition(ctx, jid, "queued", "running"); err != nil {
		t.Fatalf("queued→running: %v", err)
	}
	// Repeating from the wrong source state is rejected.
	if err := st.Jobs.Transition(ctx, jid, "queued", "running"); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("stale transition err = %v, want ErrIllegalTransition", err)
	}
	// running→verifying legal.
	if err := st.Jobs.Transition(ctx, jid, "running", "verifying"); err != nil {
		t.Fatalf("running→verifying: %v", err)
	}
}

func TestTerminalSetters(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	t.Run("completed requires verifying", func(t *testing.T) {
		_, jid := seedJobFixture(t, st)
		// From queued it should be rejected.
		if err := st.Jobs.MarkCompleted(ctx, jid, 1234, 999); !errors.Is(err, ErrIllegalTransition) {
			t.Fatalf("complete from queued err = %v, want ErrIllegalTransition", err)
		}
		// Move to verifying, then complete.
		_ = st.Jobs.Transition(ctx, jid, "queued", "running")
		_ = st.Jobs.Transition(ctx, jid, "running", "verifying")
		if err := st.Jobs.MarkCompleted(ctx, jid, 1234, 999); err != nil {
			t.Fatalf("complete: %v", err)
		}
		job, _ := st.Jobs.GetByID(ctx, jid)
		if job.Status != "completed" || job.OutputSizeBytes == nil || *job.OutputSizeBytes != 1234 {
			t.Fatalf("completed job = %+v", job)
		}
		if job.ProgressPercent != 100 {
			t.Errorf("progress = %v, want 100", job.ProgressPercent)
		}
	})

	t.Run("failed from running", func(t *testing.T) {
		_, jid := seedJobFixture(t, st)
		_ = st.Jobs.Transition(ctx, jid, "queued", "running")
		if err := st.Jobs.MarkFailed(ctx, jid, "boom", 5); err != nil {
			t.Fatalf("fail: %v", err)
		}
		job, _ := st.Jobs.GetByID(ctx, jid)
		if job.Status != "failed" || job.ErrorMessage == nil || *job.ErrorMessage != "boom" {
			t.Fatalf("failed job = %+v", job)
		}
	})

	t.Run("cancelled from queued", func(t *testing.T) {
		_, jid := seedJobFixture(t, st)
		if err := st.Jobs.MarkCancelled(ctx, jid, 7); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		job, _ := st.Jobs.GetByID(ctx, jid)
		if job.Status != "cancelled" {
			t.Fatalf("status = %q, want cancelled", job.Status)
		}
	})
}

func TestDismiss(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	t.Run("rejects non-terminal job", func(t *testing.T) {
		_, jid := seedJobFixture(t, st)
		if err := st.Jobs.Dismiss(ctx, jid, 100); !errors.Is(err, ErrIllegalTransition) {
			t.Fatalf("dismiss queued job err = %v, want ErrIllegalTransition", err)
		}
	})

	t.Run("hides a cancelled job from ListWithPath without deleting it", func(t *testing.T) {
		_, jid := seedJobFixture(t, st)
		if err := st.Jobs.MarkCancelled(ctx, jid, 7); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		if err := st.Jobs.Dismiss(ctx, jid, 100); err != nil {
			t.Fatalf("dismiss: %v", err)
		}

		// Row still exists...
		job, err := st.Jobs.GetByID(ctx, jid)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if job.Status != "cancelled" {
			t.Fatalf("status = %q, want cancelled", job.Status)
		}

		// ...but no longer shows up in the history list.
		jobs, err := st.Jobs.ListWithPath(ctx, JobListQuery{})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		for _, j := range jobs {
			if j.ID == jid {
				t.Fatalf("dismissed job %d still present in ListWithPath", jid)
			}
		}
	})

	t.Run("unknown job", func(t *testing.T) {
		if err := st.Jobs.Dismiss(ctx, 999999, 100); !errors.Is(err, ErrNotFound) {
			t.Fatalf("dismiss unknown job err = %v, want ErrNotFound", err)
		}
	})
}

func TestListInterrupted(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	mid, running := seedJobFixture(t, st)
	_ = st.Jobs.Transition(ctx, running, "queued", "running")

	// A completed job should not be listed.
	done, _ := st.Jobs.Create(ctx, &TranscodeJob{
		MediaFileID: mid, ProfileID: 1, Status: "queued", QueuedAt: 30, OriginalSizeBytes: 1,
	})
	_ = st.Jobs.Transition(ctx, done, "queued", "running")
	_ = st.Jobs.Transition(ctx, done, "running", "verifying")
	_ = st.Jobs.MarkCompleted(ctx, done, 1, 1)

	stuck, err := st.Jobs.ListInterrupted(ctx)
	if err != nil {
		t.Fatalf("list interrupted: %v", err)
	}
	if len(stuck) != 1 || stuck[0].ID != running {
		t.Fatalf("interrupted = %+v, want only job %d", stuck, running)
	}
}

func TestReplaceWithEncodedUpdatesStatsAndDropsCandidate(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	codec := "h264"
	mid, err := st.Media.Insert(ctx, &MediaFile{
		Path: "/media/movies/replace.mkv", LibraryType: "movie", SizeBytes: 5000,
		Mtime: 1, Fingerprint: "fprepl", VideoCodec: &codec,
		PredictedSavingsBytes: 2000, Status: "active",
	})
	if err != nil {
		t.Fatalf("insert media: %v", err)
	}

	// Before: appears as a candidate.
	before, err := st.Media.Candidates(ctx, CandidateQuery{})
	if err != nil {
		t.Fatalf("candidates before: %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("candidates before = %d, want 1", len(before))
	}

	if err := st.Media.ReplaceWithEncoded(ctx, mid, 2000, "newfp", 12345); err != nil {
		t.Fatalf("replace: %v", err)
	}

	f, _ := st.Media.GetByID(ctx, mid)
	if !f.IsAlreadyHEVC || f.VideoCodec == nil || *f.VideoCodec != "hevc" {
		t.Fatalf("row not converted to hevc: %+v", f)
	}
	if f.SizeBytes != 2000 || f.PredictedSavingsBytes != 0 || f.Fingerprint != "newfp" {
		t.Fatalf("row fields wrong after replace: %+v", f)
	}

	// After: drops out of the candidate list (is_already_hevc).
	after, _ := st.Media.Candidates(ctx, CandidateQuery{})
	if len(after) != 0 {
		t.Fatalf("candidates after = %d, want 0", len(after))
	}

	// Stats reflect the reclaimed bytes (incremental == recompute).
	got, _ := st.Stats.Overview(ctx)
	if got.TotalBytes != 2000 {
		t.Errorf("total bytes = %d, want 2000", got.TotalBytes)
	}
	if got.TotalRecoverableBytes != 0 {
		t.Errorf("recoverable = %d, want 0", got.TotalRecoverableBytes)
	}
	if err := st.Stats.Recompute(ctx); err != nil {
		t.Fatalf("recompute: %v", err)
	}
	reco, _ := st.Stats.Overview(ctx)
	if reco.TotalBytes != got.TotalBytes || reco.TotalRecoverableBytes != got.TotalRecoverableBytes {
		t.Errorf("incremental stats drifted from recompute: %+v vs %+v", got, reco)
	}
}

// TestReplaceWithEncodedReactivatesMissingRow guards against a regression where
// a file marked "missing" (e.g. a transient NAS hiccup) before a successful
// re-encode stayed stuck as "missing" forever afterward, even though the swap
// succeeded and the row's codec/size were updated.
func TestReplaceWithEncodedReactivatesMissingRow(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	codec := "h264"
	mid, err := st.Media.Insert(ctx, &MediaFile{
		Path: "/media/tv/missing-then-encoded.mkv", LibraryType: "tv", SizeBytes: 5000,
		Mtime: 1, Fingerprint: "fpmissing", VideoCodec: &codec,
		PredictedSavingsBytes: 2000, Status: "active",
	})
	if err != nil {
		t.Fatalf("insert media: %v", err)
	}
	if err := st.Media.MarkMissing(ctx, mid); err != nil {
		t.Fatalf("mark missing: %v", err)
	}

	if err := st.Media.ReplaceWithEncoded(ctx, mid, 2000, "newfp", 12345); err != nil {
		t.Fatalf("replace: %v", err)
	}

	f, _ := st.Media.GetByID(ctx, mid)
	if f.Status != MediaStatusActive {
		t.Fatalf("status = %q, want active — file should be reactivated after a successful re-encode", f.Status)
	}

	got, _ := st.Stats.Overview(ctx)
	if got.TotalBytes != 2000 {
		t.Errorf("total bytes = %d, want 2000 — row should count toward stats once reactivated", got.TotalBytes)
	}
	if err := st.Stats.Recompute(ctx); err != nil {
		t.Fatalf("recompute: %v", err)
	}
	reco, _ := st.Stats.Overview(ctx)
	if reco.TotalBytes != got.TotalBytes {
		t.Errorf("incremental stats drifted from recompute: %+v vs %+v", got, reco)
	}
}
