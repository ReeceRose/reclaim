package store

import (
	"context"
	"fmt"
	"testing"

	"reclaim/internal/media"
)

func TestSavingsModel_seedUntilLearned(t *testing.T) {
	st := openTestStore(t)
	if got := st.SavingsModel.Predict(strp("h264"), false, 1000); got != 400 {
		t.Fatalf("no samples: want seed 400, got %d", got)
	}
}

// Refresh must reprice files that were stored on the seed, and later probes
// must price on the learned ratio rather than reverting to the seed.
func TestSavingsModel_refreshRepricesLibraryAndNewProbes(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	pending, err := st.Media.insertFile(ctx, testFile{
		path: "/movies/pending.mkv", size: 10_000, codec: "h264", savings: 4_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Stats.Recompute(ctx); err != nil {
		t.Fatal(err)
	}

	for i := range LearnedRatioMinSamples {
		path := fmt.Sprintf("/movies/done-%d.mkv", i)
		fileID, jobID := seedEncodedFile(t, st, path, "h264", 1000, 350, 1920, 1080)
		if _, err := st.CommitEncodeSwap(ctx, fileID, jobID, 350, "fp-"+path, 1_700_003_600, "done", ""); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
	}

	n, err := st.SavingsModel.Refresh(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("rows repriced: want 1, got %d", n)
	}

	f, err := st.Media.GetByID(ctx, pending)
	if err != nil {
		t.Fatal(err)
	}
	if f.PredictedSavingsBytes != 6_500 {
		t.Errorf("stored prediction: want 6500, got %d", f.PredictedSavingsBytes)
	}

	ov, err := st.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range ov.ByCodec {
		if c.Codec == "h264" && c.PredictedSavingsBytes != 6_500 {
			t.Errorf("library_stats h264 savings: want 6500, got %d", c.PredictedSavingsBytes)
		}
	}

	if got := st.SavingsModel.Predict(strp("H264"), false, 1000); got != 650 {
		t.Errorf("new probe: want learned 650, got %d", got)
	}
	if got, want := st.SavingsModel.Predict(strp("mpeg4"), false, 1000), media.PredictedSavingsBytes(strp("mpeg4"), false, 1000); got != want {
		t.Errorf("unlearned codec: want seed %d, got %d", want, got)
	}

	if n, err := st.SavingsModel.Refresh(ctx); err != nil || n != 0 {
		t.Errorf("idempotent refresh: want 0 rows, got %d (err %v)", n, err)
	}
}

// Large sources dominate the library's remaining savings, so the learned ratio
// is weighted by bytes rather than averaged per file.
func TestLearnedRatios_byteWeighted(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	for i := range LearnedRatioMinSamples {
		path := fmt.Sprintf("/movies/w-%d.mkv", i)
		size, out := int64(1000), int64(800)
		if i == 0 {
			size, out = 91_000, 18_200
		}
		fileID, jobID := seedEncodedFile(t, st, path, "h264", size, out, 1920, 1080)
		if _, err := st.CommitEncodeSwap(ctx, fileID, jobID, out, "fp-"+path, 1_700_003_600, "done", ""); err != nil {
			t.Fatal(err)
		}
	}

	learned, err := st.Jobs.LearnedRatios(ctx, LearnedRatioMinSamples)
	if err != nil {
		t.Fatal(err)
	}
	if r := learned["h264"].Ratio; r < 0.253 || r > 0.255 {
		t.Errorf("ratio: want 0.254 (25400/100000), got %v", r)
	}
}
