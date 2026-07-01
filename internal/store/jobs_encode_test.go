package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func insertCompletedEncodeJob(t *testing.T, st *Store, profileID int64, preset string, crf int, startedAt, completedAt int64, duration float64, width, height int) int64 {
	t.Helper()
	ctx := context.Background()
	path := fmt.Sprintf("/movies/enc-%s-%d.mkv", preset, startedAt)
	id, err := st.Media.Insert(ctx, &MediaFile{
		Path:        path,
		LibraryType: "movie",
		SizeBytes:   1_000_000_000,
		Mtime:       1,
		Fingerprint: fmt.Sprintf("fp-enc-%d", startedAt),
		Status:      "active",
		DurationSeconds: func() *float64 {
			d := duration
			return &d
		}(),
		Width:  &width,
		Height: &height,
	})
	if err != nil {
		t.Fatalf("insert media: %v", err)
	}
	presetCopy := preset
	jid, err := st.Jobs.Create(ctx, &TranscodeJob{
		MediaFileID:       id,
		ProfileID:         profileID,
		Status:            "completed",
		QueuedAt:          startedAt - 10,
		StartedAt:         &startedAt,
		CompletedAt:       &completedAt,
		OriginalSizeBytes: 1_000_000_000,
		EncodePreset:      &presetCopy,
		EncodeCRF:         &crf,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	_, err = st.Jobs.w.ExecContext(ctx,
		`UPDATE transcode_jobs SET status = 'completed', started_at = ?, completed_at = ? WHERE id = ?`,
		startedAt, completedAt, jid,
	)
	if err != nil {
		t.Fatalf("mark completed: %v", err)
	}
	return jid
}

func TestJobsCreate_writesEncodeSnapshot(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	profile, err := st.Profiles.GetDefault(ctx)
	if err != nil {
		t.Fatalf("default profile: %v", err)
	}

	mediaID, err := st.Media.insertTestRow(ctx, "/movies/snap.mkv")
	if err != nil {
		t.Fatalf("insert media: %v", err)
	}

	jid, err := st.Jobs.Create(ctx, &TranscodeJob{
		MediaFileID:       mediaID,
		ProfileID:         profile.ID,
		Status:            "queued",
		QueuedAt:          time.Now().Unix(),
		OriginalSizeBytes: 1000,
		EncodePreset:      &profile.Preset,
		EncodeCRF:         &profile.CRF,
		EncodeExtraArgs:   profile.ExtraArgs,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	job, err := st.Jobs.GetByID(ctx, jid)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.EncodePreset == nil || *job.EncodePreset != profile.Preset {
		t.Fatalf("encode_preset = %v, want %q", job.EncodePreset, profile.Preset)
	}
	if job.EncodeCRF == nil || *job.EncodeCRF != profile.CRF {
		t.Fatalf("encode_crf = %v, want %d", job.EncodeCRF, profile.CRF)
	}
}

func TestLearnedEncodeRates_profileBucket(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	profile, err := st.Profiles.GetDefault(ctx)
	if err != nil {
		t.Fatalf("default profile: %v", err)
	}

	// 1 hour 1080p encoded in 1 hour → normalized rate 1.0
	for i := 0; i < 3; i++ {
		start := int64(1_000_000 + i*4000)
		insertCompletedEncodeJob(t, st, profile.ID, profile.Preset, profile.CRF, start, start+3600, 3600, 1920, 1080)
	}

	lookup, err := st.Jobs.LearnedEncodeRates(ctx)
	if err != nil {
		t.Fatalf("LearnedEncodeRates: %v", err)
	}
	lr, ok := lookup.ByProfileID[profile.ID]
	if !ok {
		t.Fatal("expected profile bucket after 3 samples")
	}
	if lr.SampleCount != 3 {
		t.Fatalf("sample count = %d, want 3", lr.SampleCount)
	}
	if lr.Rate < 0.9 || lr.Rate > 1.1 {
		t.Fatalf("rate = %v, want ~1.0", lr.Rate)
	}
}

func TestLearnedEncodeRates_profileNeedsThreeSamples(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	profile, err := st.Profiles.GetDefault(ctx)
	if err != nil {
		t.Fatalf("default profile: %v", err)
	}

	for i := 0; i < 2; i++ {
		start := int64(2_000_000 + i*4000)
		insertCompletedEncodeJob(t, st, profile.ID, profile.Preset, profile.CRF, start, start+3600, 3600, 1920, 1080)
	}

	lookup, err := st.Jobs.LearnedEncodeRates(ctx)
	if err != nil {
		t.Fatalf("LearnedEncodeRates: %v", err)
	}
	if _, ok := lookup.ByProfileID[profile.ID]; ok {
		t.Fatal("profile bucket should be absent with only 2 samples")
	}
}

func TestLearnedEncodeRates_excludesOutlier(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	profile, err := st.Profiles.GetDefault(ctx)
	if err != nil {
		t.Fatalf("default profile: %v", err)
	}

	// 3 normal 1hr encodes
	for i := 0; i < 3; i++ {
		start := int64(3_000_000 + i*4000)
		insertCompletedEncodeJob(t, st, profile.ID, profile.Preset, profile.CRF, start, start+3600, 3600, 1920, 1080)
	}
	// Outlier: 10s encode of 2hr file
	insertCompletedEncodeJob(t, st, profile.ID, profile.Preset, profile.CRF, 3_100_000, 3_100_010, 7200, 1920, 1080)

	lookup, err := st.Jobs.LearnedEncodeRates(ctx)
	if err != nil {
		t.Fatalf("LearnedEncodeRates: %v", err)
	}
	lr := lookup.ByProfileID[profile.ID]
	if lr.SampleCount != 3 {
		t.Fatalf("sample count = %d, want 3 (outlier excluded)", lr.SampleCount)
	}
}
