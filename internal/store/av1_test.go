package store

import (
	"context"
	"fmt"
	"testing"

	"reclaim/internal/media"
)

// An AV1 job's swap must record AV1 everywhere the encode lands: the media row,
// the efficient flag that drops it from the candidate list, and the ledger's
// result codec that splits learned ratios and Insights by target.
func TestCommitEncodeSwap_av1Target(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	fileID, jobID := seedEncodedFile(t, st, "/movies/av1.mkv", "h264", 1000, 300, 1920, 1080)
	if err := st.Jobs.SetEncodeSettings(ctx, jobID, "av1", "6", 30, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CommitEncodeSwap(ctx, fileID, jobID, 300, "fp-new", 1_700_003_600, "done", ""); err != nil {
		t.Fatal(err)
	}

	f, err := st.Media.GetByID(ctx, fileID)
	if err != nil {
		t.Fatal(err)
	}
	if f.VideoCodec == nil || *f.VideoCodec != "av1" || !f.IsEfficientCodec || f.PredictedSavingsBytes != 0 {
		t.Fatalf("media row after AV1 swap: codec=%v efficient=%v savings=%d", f.VideoCodec, f.IsEfficientCodec, f.PredictedSavingsBytes)
	}

	entries, err := st.Savings.Recent(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ResultCodec == nil || *entries[0].ResultCodec != "av1" ||
		entries[0].SourceCodec == nil || *entries[0].SourceCodec != "h264" {
		t.Fatalf("ledger entry = %+v, want h264 → av1", entries)
	}

	buckets, err := st.Savings.ByTargetCodec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 || buckets[0].Key != "av1" || buckets[0].BytesSaved != 700 {
		t.Fatalf("by target codec = %+v, want one av1 bucket of 700", buckets)
	}

	job, err := st.Jobs.GetByID(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.EncodeCodec == nil || *job.EncodeCodec != "av1" || job.EncodePreset == nil || *job.EncodePreset != "6" {
		t.Fatalf("job snapshot = codec %v preset %v, want av1/6", job.EncodeCodec, job.EncodePreset)
	}
}

// h264 shrinks further under SVT-AV1 than under x265; pooling the two would
// mis-price both targets.
func TestLearnedRatios_splitByTarget(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	for i := range 3 {
		path := fmt.Sprintf("/movies/hevc-%d.mkv", i)
		fileID, jobID := seedEncodedFile(t, st, path, "h264", 1000, 600, 1920, 1080)
		if _, err := st.CommitEncodeSwap(ctx, fileID, jobID, 600, "fp-"+path, 1_700_003_600, "done", ""); err != nil {
			t.Fatal(err)
		}

		path = fmt.Sprintf("/movies/av1-%d.mkv", i)
		fileID, jobID = seedEncodedFile(t, st, path, "h264", 1000, 400, 1920, 1080)
		if err := st.Jobs.SetEncodeSettings(ctx, jobID, "av1", "6", 30, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := st.CommitEncodeSwap(ctx, fileID, jobID, 400, "fp-"+path, 1_700_003_600, "done", ""); err != nil {
			t.Fatal(err)
		}
	}

	for target, want := range map[media.TargetCodec]float64{media.TargetHEVC: 0.6, media.TargetAV1: 0.4} {
		learned, err := st.Jobs.LearnedRatios(ctx, target, 3)
		if err != nil {
			t.Fatal(err)
		}
		lr, ok := learned["h264"]
		if !ok || lr.SampleCount != 3 || lr.Ratio < want-0.001 || lr.Ratio > want+0.001 {
			t.Errorf("%s: h264 = %+v (ok=%v), want ratio %v over 3 samples", target, lr, ok, want)
		}
	}
}

// The library's stored predictions follow the default profile's codec, so
// switching it reprices every candidate and switching back restores them.
func TestSavingsModel_followsDefaultProfileCodec(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	id, err := st.Media.insertFile(ctx, testFile{
		path: "/movies/pending.mkv", size: 10_000, codec: "h264", savings: 4_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Stats.Recompute(ctx); err != nil {
		t.Fatal(err)
	}

	def, err := st.Profiles.GetDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	setCodec := func(codec, preset string, crf int) {
		t.Helper()
		def.Codec, def.Preset, def.CRF = codec, preset, crf
		if err := st.Profiles.Update(ctx, def); err != nil {
			t.Fatal(err)
		}
		if _, err := st.SavingsModel.Refresh(ctx); err != nil {
			t.Fatal(err)
		}
	}
	savings := func() int64 {
		t.Helper()
		f, err := st.Media.GetByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return f.PredictedSavingsBytes
	}

	setCodec("av1", "6", 30)
	if got := st.SavingsModel.Target(); got != media.TargetAV1 {
		t.Fatalf("target = %q, want av1", got)
	}
	if got := savings(); got != 5_200 {
		t.Errorf("stored prediction against AV1: want 5200, got %d", got)
	}
	ov, err := st.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ov.TotalRecoverableBytes != 5_200 {
		t.Errorf("library_stats recoverable: want 5200, got %d", ov.TotalRecoverableBytes)
	}
	if got := st.SavingsModel.Predict(strp("h264"), false, 1000); got != 520 {
		t.Errorf("new probe against AV1: want 520, got %d", got)
	}
	if got := st.SavingsModel.PredictFor(media.TargetHEVC, strp("h264"), false, 1000); got != 400 {
		t.Errorf("explicit HEVC prediction: want 400, got %d", got)
	}

	setCodec("hevc", "medium", 26)
	if got := savings(); got != 4_000 {
		t.Errorf("stored prediction back on HEVC: want 4000, got %d", got)
	}
}

func TestCandidates_excludeAV1Sources(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	if _, err := st.Media.insertFile(ctx, testFile{path: "/movies/a.mkv", size: 1000, codec: "h264", savings: 400}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Media.insertFile(ctx, testFile{path: "/movies/b.mkv", size: 1000, codec: "av1", hevc: true}); err != nil {
		t.Fatal(err)
	}

	got, err := st.Media.Candidates(ctx, CandidateQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "/movies/a.mkv" {
		t.Fatalf("candidates = %+v, want only the h264 file", got)
	}

	for _, state := range []string{string(CandidateStateAlreadyEfficient), "already_hevc"} {
		files, err := st.Media.Files(ctx, FileQuery{Filter: FileFilter{CandidateState: state}})
		if err != nil {
			t.Fatalf("%s: %v", state, err)
		}
		if len(files) != 1 || files[0].Path != "/movies/b.mkv" {
			t.Errorf("%s filter = %+v, want only the AV1 file", state, files)
		}
	}
}
