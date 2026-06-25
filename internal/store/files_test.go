package store

import (
	"context"
	"testing"
)

func TestFiles_includesAllStatusesAndDerivesCandidateStates(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	candidateID, err := s.Media.insertFile(ctx, testFile{path: "/m/a-candidate.mkv", size: 1000, codec: "h264", height: 1080, savings: 400})
	if err != nil {
		t.Fatal(err)
	}
	hevcID, err := s.Media.insertFile(ctx, testFile{path: "/m/b-hevc.mkv", size: 1000, codec: "hevc", height: 1080, hevc: true})
	if err != nil {
		t.Fatal(err)
	}
	missingID, err := s.Media.insertFile(ctx, testFile{path: "/m/c-missing.mkv", size: 1000, codec: "h264", height: 1080, status: "missing"})
	if err != nil {
		t.Fatal(err)
	}
	probeID, err := s.Media.insertFile(ctx, testFile{path: "/m/d-probe.mkv", size: 1000, probeErr: "boom"})
	if err != nil {
		t.Fatal(err)
	}
	unknownID, err := s.Media.insertFile(ctx, testFile{path: "/m/e-unknown.mkv", size: 1000})
	if err != nil {
		t.Fatal(err)
	}
	queuedID, err := s.Media.insertFile(ctx, testFile{path: "/m/f-queued.mkv", size: 1000, codec: "h264", height: 1080, savings: 400})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Jobs.insertJobWithStatus(ctx, queuedID, "queued"); err != nil {
		t.Fatal(err)
	}
	completedID, err := s.Media.insertFile(ctx, testFile{path: "/m/g-completed.mkv", size: 1000, codec: "h264", height: 1080, savings: 400})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Jobs.insertJobWithStatus(ctx, completedID, "completed"); err != nil {
		t.Fatal(err)
	}

	files, err := s.Media.Files(ctx, FileQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 7 {
		t.Fatalf("want all 7 files, got %d", len(files))
	}
	if files[0].ID != candidateID || files[6].ID != completedID {
		t.Fatalf("default path order unexpected: %+v", files)
	}

	states, err := s.Media.CandidateStates(ctx, files)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64]CandidateState{
		candidateID: CandidateStateCandidate,
		hevcID:      CandidateStateAlreadyHEVC,
		missingID:   CandidateStateMissing,
		probeID:     CandidateStateProbeFailed,
		unknownID:   CandidateStateUnknownCodec,
		queuedID:    CandidateStateQueued,
		completedID: CandidateStateCompleted,
	}
	for id, state := range want {
		if states[id] != state {
			t.Fatalf("file %d state = %q, want %q", id, states[id], state)
		}
	}
}

func TestFiles_filtersByCandidateState(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/candidate.mkv", size: 1000, codec: "h264", height: 1080, savings: 400}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/hevc.mkv", size: 1000, codec: "hevc", height: 1080, hevc: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/missing.mkv", size: 1000, codec: "h264", height: 1080, status: "missing"}); err != nil {
		t.Fatal(err)
	}

	count := func(state CandidateState) int {
		got, err := s.Media.Files(ctx, FileQuery{Filter: FileFilter{CandidateState: string(state)}})
		if err != nil {
			t.Fatal(err)
		}
		return len(got)
	}
	if n := count(CandidateStateCandidate); n != 1 {
		t.Fatalf("candidate: want 1, got %d", n)
	}
	if n := count(CandidateStateAlreadyHEVC); n != 1 {
		t.Fatalf("already_hevc: want 1, got %d", n)
	}
	if n := count(CandidateStateMissing); n != 1 {
		t.Fatalf("missing: want 1, got %d", n)
	}
}
