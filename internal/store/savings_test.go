package store

import (
	"context"
	"testing"
)

func seedEncodedFile(t *testing.T, st *Store, path, codec string, size, output int64, width, height int) (fileID, jobID int64) {
	t.Helper()
	ctx := context.Background()
	fileID, err := st.Media.insertFile(ctx, testFile{
		path:   path,
		size:   size,
		mtime:  1,
		codec:  codec,
		width:  width,
		height: height,
	})
	if err != nil {
		t.Fatalf("insert media: %v", err)
	}
	started := int64(1_700_000_000)
	jobID, err = st.Jobs.Create(ctx, &TranscodeJob{
		MediaFileID:       fileID,
		ProfileID:         1,
		Status:            "queued",
		QueuedAt:          started - 10,
		OriginalSizeBytes: size,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if _, err := st.Jobs.w.ExecContext(ctx,
		`UPDATE transcode_jobs SET status = 'verifying', started_at = ?,
		 predicted_savings_bytes = ?, initial_estimated_duration_seconds = ? WHERE id = ?`,
		started, size-output, 600, jobID,
	); err != nil {
		t.Fatalf("stage job: %v", err)
	}
	return fileID, jobID
}

func TestCommitEncodeSwapRecordsSourceCodecInLedger(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	fileID, jobID := seedEncodedFile(t, st, "/movies/a.mkv", "h264", 1000, 400, 1920, 1080)
	completedAt := int64(1_700_003_600)

	if _, err := st.CommitEncodeSwap(ctx, fileID, jobID, 400, "fp-new", completedAt, "done", ""); err != nil {
		t.Fatalf("commit: %v", err)
	}

	entries, err := st.Savings.Recent(ctx, 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 ledger row, got %d", len(entries))
	}
	e := entries[0]
	if e.SourceCodec == nil || *e.SourceCodec != "h264" {
		t.Errorf("source codec: want h264, got %v", e.SourceCodec)
	}
	if e.OriginalBytes != 1000 || e.OutputBytes != 400 || e.BytesSaved != 600 {
		t.Errorf("sizes: got original=%d output=%d saved=%d", e.OriginalBytes, e.OutputBytes, e.BytesSaved)
	}
	if e.EncodeSeconds == nil || *e.EncodeSeconds != 3600 {
		t.Errorf("encode seconds: want 3600, got %v", e.EncodeSeconds)
	}
	if e.Path != "/movies/a.mkv" {
		t.Errorf("path: got %q", e.Path)
	}

	var codec string
	if err := st.r.QueryRowContext(ctx, "SELECT video_codec FROM media_files WHERE id = ?", fileID).Scan(&codec); err != nil {
		t.Fatalf("read media: %v", err)
	}
	if codec != "hevc" {
		t.Fatalf("media row should be hevc after swap, got %q", codec)
	}
}

func TestSavingsSummaryAggregates(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	now := int64(1_700_003_600)
	for i, tc := range []struct {
		path          string
		size, output  int64
		width, height int
		codec         string
	}{
		{"/movies/a.mkv", 1000, 400, 1920, 1080, "h264"},
		{"/movies/b.mkv", 2000, 1000, 3840, 2160, "mpeg4"},
	} {
		fileID, jobID := seedEncodedFile(t, st, tc.path, tc.codec, tc.size, tc.output, tc.width, tc.height)
		if _, err := st.CommitEncodeSwap(ctx, fileID, jobID, tc.output, "fp-new-"+tc.path, now-int64(i), "done", ""); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
	}

	s, err := st.Savings.Summary(ctx, now)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if s.FilesEncoded != 2 {
		t.Errorf("files encoded: want 2, got %d", s.FilesEncoded)
	}
	if s.BytesSaved != 1600 {
		t.Errorf("bytes saved: want 1600, got %d", s.BytesSaved)
	}
	if s.OriginalBytes != 3000 || s.OutputBytes != 1400 {
		t.Errorf("totals: got original=%d output=%d", s.OriginalBytes, s.OutputBytes)
	}
	if s.BytesSaved7d != 1600 {
		t.Errorf("bytes saved 7d: want 1600, got %d", s.BytesSaved7d)
	}
	if s.BestSavedBytes != 1000 {
		t.Errorf("best save: want 1000, got %d", s.BestSavedBytes)
	}
	if s.PredictedSamples != 2 || s.PredictedSavingsSum != 1600 || s.PredictedActualSum != 1600 {
		t.Errorf("estimate accuracy inputs: samples=%d predicted=%d actual=%d",
			s.PredictedSamples, s.PredictedSavingsSum, s.PredictedActualSum)
	}

	byCodec, err := st.Savings.ByCodec(ctx)
	if err != nil {
		t.Fatalf("by codec: %v", err)
	}
	if len(byCodec) != 2 {
		t.Fatalf("want 2 codec buckets, got %d", len(byCodec))
	}
	if byCodec[0].Key != "mpeg4" || byCodec[0].BytesSaved != 1000 {
		t.Errorf("top codec bucket: got %+v", byCodec[0])
	}

	byRes, err := st.Savings.ByResolution(ctx)
	if err != nil {
		t.Fatalf("by resolution: %v", err)
	}
	bands := map[string]int64{}
	for _, b := range byRes {
		bands[b.Key] = b.BytesSaved
	}
	if bands["uhd"] != 1000 || bands["fhd"] != 600 {
		t.Errorf("resolution bands: got %v", bands)
	}
}

// LearnedRatios must key off the ledger's source codec, not the media row,
// which the swap rewrites to hevc.
func TestLearnedRatiosUsesPreEncodeSourceCodec(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	for i := 0; i < LearnedRatioMinSamples; i++ {
		path := "/movies/h264-" + string(rune('a'+i)) + ".mkv"
		fileID, jobID := seedEncodedFile(t, st, path, "h264", 1000, 500, 1920, 1080)
		if _, err := st.CommitEncodeSwap(ctx, fileID, jobID, 500, "fp-"+path, 1_700_003_600, "done", ""); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
	}

	learned, err := st.Jobs.LearnedRatios(ctx, LearnedRatioMinSamples)
	if err != nil {
		t.Fatalf("learned ratios: %v", err)
	}
	lr, ok := learned["h264"]
	if !ok {
		t.Fatalf("want an h264 bucket, got %v", learned)
	}
	if lr.SampleCount != LearnedRatioMinSamples {
		t.Errorf("samples: want %d, got %d", LearnedRatioMinSamples, lr.SampleCount)
	}
	if lr.Ratio < 0.49 || lr.Ratio > 0.51 {
		t.Errorf("ratio: want ~0.5, got %v", lr.Ratio)
	}
	if _, ok := learned["hevc"]; ok {
		t.Error("hevc must not appear as a learned source codec")
	}
}

// The ledger is the durable record of reclaimed bytes, so pruning a missing
// file must not erase its history the way it erases the job row.
func TestSavingsLedgerSurvivesPrune(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	fileID, jobID := seedEncodedFile(t, st, "/movies/gone.mkv", "h264", 1000, 400, 1920, 1080)
	if _, err := st.CommitEncodeSwap(ctx, fileID, jobID, 400, "fp-new", 1_700_003_600, "done", ""); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := st.Media.MarkMissing(ctx, fileID, ""); err != nil {
		t.Fatalf("mark missing: %v", err)
	}
	if _, err := st.Media.PruneMissing(ctx, 0); err != nil {
		t.Fatalf("prune: %v", err)
	}

	var jobs int64
	if err := st.r.QueryRowContext(ctx, "SELECT COUNT(*) FROM transcode_jobs").Scan(&jobs); err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	if jobs != 0 {
		t.Fatalf("prune should have removed job history, got %d rows", jobs)
	}

	s, err := st.Savings.Summary(ctx, 1_700_003_600)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if s.FilesEncoded != 1 || s.BytesSaved != 600 {
		t.Errorf("ledger lost history after prune: files=%d saved=%d", s.FilesEncoded, s.BytesSaved)
	}
}

// Rows with no recorded source codec — backfilled history, or a file whose
// codec never probed — must still be counted, or the breakdown silently
// disagrees with the lifetime total.
func TestSavingsByCodecBucketsUnknownAndReconciles(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	now := int64(1_700_003_600)

	fileID, jobID := seedEncodedFile(t, st, "/movies/known.mkv", "h264", 1000, 400, 1920, 1080)
	if _, err := st.CommitEncodeSwap(ctx, fileID, jobID, 400, "fp-known", now, "done", ""); err != nil {
		t.Fatalf("commit known: %v", err)
	}
	fileID2, jobID2 := seedEncodedFile(t, st, "/movies/nocodec.mkv", "", 3000, 1000, 1920, 1080)
	if _, err := st.CommitEncodeSwap(ctx, fileID2, jobID2, 1000, "fp-nocodec", now, "done", ""); err != nil {
		t.Fatalf("commit unknown: %v", err)
	}

	byCodec, err := st.Savings.ByCodec(ctx)
	if err != nil {
		t.Fatalf("by codec: %v", err)
	}
	got := map[string]int64{}
	var bucketed int64
	for _, b := range byCodec {
		got[b.Key] = b.BytesSaved
		bucketed += b.BytesSaved
	}
	if got["unknown"] != 2000 {
		t.Errorf("unknown bucket: want 2000, got %d (%v)", got["unknown"], got)
	}
	if got["h264"] != 600 {
		t.Errorf("h264 bucket: want 600, got %d", got["h264"])
	}

	summary, err := st.Savings.Summary(ctx, now)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if bucketed != summary.BytesSaved {
		t.Errorf("buckets must sum to lifetime total: buckets=%d lifetime=%d", bucketed, summary.BytesSaved)
	}

	learned, err := st.Jobs.LearnedRatios(ctx, 1)
	if err != nil {
		t.Fatalf("learned ratios: %v", err)
	}
	if _, ok := learned["unknown"]; ok {
		t.Error("unknown must not become a learned-ratio bucket")
	}
}
