package store

import (
	"context"
	"testing"

	"reclaim/internal/compatibility"
)

// insertCompatibilityFile inserts a media row (via testFile) and
// immediately upserts a single-profile verdict for it, returning the file
// ID.
func insertCompatibilityFile(t *testing.T, ctx context.Context, m *Media, tf testFile, profile string, v compatibility.Verdict) int64 {
	t.Helper()
	id, err := m.insertFile(ctx, tf)
	if err != nil {
		t.Fatalf("insertFile: %v", err)
	}
	if err := m.UpsertCompatibility(ctx, id, map[string]compatibility.Verdict{profile: v}); err != nil {
		t.Fatalf("UpsertCompatibility: %v", err)
	}
	return id
}

func verdictFail(riskScore int, reasons ...compatibility.Reason) compatibility.Verdict {
	return compatibility.Verdict{
		DirectPlayPredicted: false,
		RiskScore:           riskScore,
		Reasons:             reasons,
		RecommendedAction:   compatibility.ActionManual,
	}
}

func verdictPass() compatibility.Verdict {
	return compatibility.Verdict{DirectPlayPredicted: true, RecommendedAction: compatibility.ActionNone}
}

func TestUpsertCompatibility_roundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	id, err := s.Media.insertFile(ctx, testFile{path: "/m/a.mkv", codec: "hevc", height: 2160, size: 1000})
	if err != nil {
		t.Fatal(err)
	}

	idx := 1
	verdicts := map[string]compatibility.Verdict{
		"apple_tv_4k": verdictFail(35, compatibility.Reason{
			Code: "container_mkv", Severity: compatibility.Advisory, Stream: nil, Message: "advisory msg",
		}),
		"nvidia_shield": verdictFail(30, compatibility.Reason{
			Code: "audio_truehd", Severity: compatibility.Advisory, Stream: &idx, Message: "shield msg",
		}),
	}
	if err := s.Media.UpsertCompatibility(ctx, id, verdicts); err != nil {
		t.Fatal(err)
	}

	rows, err := s.Media.CompatibilityForFile(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 stored verdicts, got %d: %+v", len(rows), rows)
	}

	byProfile := map[string]CompatibilityRow{}
	for _, r := range rows {
		byProfile[r.ClientProfile] = r
	}

	appleTV, ok := byProfile["apple_tv_4k"]
	if !ok {
		t.Fatal("missing apple_tv_4k row")
	}
	if appleTV.RiskScore != 35 || appleTV.DirectPlayPredicted {
		t.Fatalf("apple_tv_4k: unexpected row %+v", appleTV)
	}
	if len(appleTV.Reasons) != 1 || appleTV.Reasons[0].Code != "container_mkv" || appleTV.Reasons[0].Severity != "advisory" {
		t.Fatalf("apple_tv_4k: unexpected reasons %+v", appleTV.Reasons)
	}

	shield, ok := byProfile["nvidia_shield"]
	if !ok {
		t.Fatal("missing nvidia_shield row")
	}
	if len(shield.Reasons) != 1 || shield.Reasons[0].Stream == nil || *shield.Reasons[0].Stream != 1 {
		t.Fatalf("nvidia_shield: unexpected reasons %+v", shield.Reasons)
	}

	// Re-upserting (as a rescan would) replaces the row rather than duplicating it.
	if err := s.Media.UpsertCompatibility(ctx, id, map[string]compatibility.Verdict{"apple_tv_4k": verdictPass()}); err != nil {
		t.Fatal(err)
	}
	rows, err = s.Media.CompatibilityForFile(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows after re-upsert (still 2 profiles), got %d", len(rows))
	}
	for _, r := range rows {
		if r.ClientProfile == "apple_tv_4k" && !r.DirectPlayPredicted {
			t.Fatalf("apple_tv_4k should have been updated to direct-play, got %+v", r)
		}
	}
}

func TestCompatibilityList_inclusionRules(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// HEVC file that fails compatibility — unlike Candidates, HEVC files
	// ARE eligible here (docs/COMPATIBILITY PLAN.md §8).
	hevcFail := insertCompatibilityFile(t, ctx, s.Media,
		testFile{path: "/m/hevc-fail.mkv", codec: "hevc", hevc: true, height: 2160, size: 1000},
		"plex_web", verdictFail(60, compatibility.Reason{Code: "video_codec_hevc", Severity: compatibility.Hard, Message: "x"}))

	// Direct-play file (no reasons) — excluded by the default direct_play=false filter.
	_ = insertCompatibilityFile(t, ctx, s.Media,
		testFile{path: "/m/direct-play.mp4", codec: "h264", height: 1080, size: 1000},
		"plex_web", verdictPass())

	// Missing file — excluded even though it has a stored verdict.
	_ = insertCompatibilityFile(t, ctx, s.Media,
		testFile{path: "/m/missing.mkv", codec: "hevc", height: 1080, size: 1000, status: "missing"},
		"plex_web", verdictFail(60, compatibility.Reason{Code: "video_codec_hevc", Severity: compatibility.Hard}))

	// Probe error — excluded.
	_ = insertCompatibilityFile(t, ctx, s.Media,
		testFile{path: "/m/err.mkv", probeErr: "boom", size: 1000},
		"plex_web", verdictFail(60, compatibility.Reason{Code: "video_codec_hevc", Severity: compatibility.Hard}))

	// Different profile entirely — excluded from a plex_web query.
	_ = insertCompatibilityFile(t, ctx, s.Media,
		testFile{path: "/m/other-profile.mkv", codec: "mpeg2video", height: 1080, size: 1000},
		"apple_tv_4k", verdictFail(45, compatibility.Reason{Code: "video_codec_mpeg2video", Severity: compatibility.Hard}))

	got, err := s.Media.CompatibilityList(ctx, CompatibilityQuery{Filter: CompatibilityFilter{ClientProfile: "plex_web"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != hevcFail {
		t.Fatalf("want exactly [hevcFail(%d)], got %+v", hevcFail, got)
	}
	if got[0].ClientProfile != "plex_web" {
		t.Fatalf("ClientProfile not populated: %+v", got[0])
	}

	count, err := s.Media.CountCompatibility(ctx, CompatibilityFilter{ClientProfile: "plex_web"})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("CountCompatibility: want 1, got %d", count)
	}
}

func TestCompatibilityList_directPlayFilter(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	fail := insertCompatibilityFile(t, ctx, s.Media,
		testFile{path: "/m/fail.mkv", codec: "hevc", height: 1080, size: 1000},
		"apple_tv_4k", verdictFail(50, compatibility.Reason{Code: "video_codec_hevc", Severity: compatibility.Hard}))
	pass := insertCompatibilityFile(t, ctx, s.Media,
		testFile{path: "/m/pass.mp4", codec: "h264", height: 1080, size: 1000},
		"apple_tv_4k", verdictPass())

	idsOf := func(rows []CompatibilityRow) map[int64]bool {
		m := map[int64]bool{}
		for _, r := range rows {
			m[r.ID] = true
		}
		return m
	}

	def, err := s.Media.CompatibilityList(ctx, CompatibilityQuery{Filter: CompatibilityFilter{ClientProfile: "apple_tv_4k"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := idsOf(def); len(got) != 1 || !got[fail] {
		t.Fatalf("default filter: want only fail(%d), got %v", fail, got)
	}

	trueOnly, err := s.Media.CompatibilityList(ctx, CompatibilityQuery{Filter: CompatibilityFilter{ClientProfile: "apple_tv_4k", DirectPlay: "true"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := idsOf(trueOnly); len(got) != 1 || !got[pass] {
		t.Fatalf("direct_play=true: want only pass(%d), got %v", pass, got)
	}

	all, err := s.Media.CompatibilityList(ctx, CompatibilityQuery{Filter: CompatibilityFilter{ClientProfile: "apple_tv_4k", DirectPlay: "all"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := idsOf(all); len(got) != 2 {
		t.Fatalf("direct_play=all: want both, got %v", got)
	}
}

func TestCompatibilityList_reasonFilter(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	dts := insertCompatibilityFile(t, ctx, s.Media,
		testFile{path: "/m/dts.mkv", codec: "hevc", height: 1080, size: 1000},
		"apple_tv_4k", verdictFail(20, compatibility.Reason{Code: "audio_dts", Severity: compatibility.Advisory}))
	pgs := insertCompatibilityFile(t, ctx, s.Media,
		testFile{path: "/m/pgs.mkv", codec: "hevc", height: 1080, size: 1000},
		"apple_tv_4k", verdictFail(40, compatibility.Reason{Code: "subtitle_pgs", Severity: compatibility.Hard}))

	got, err := s.Media.CompatibilityList(ctx, CompatibilityQuery{Filter: CompatibilityFilter{
		ClientProfile: "apple_tv_4k", DirectPlay: "all", Reason: "audio_dts",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != dts {
		t.Fatalf("reason=audio_dts: want only dts(%d), got %+v", dts, got)
	}

	got, err = s.Media.CompatibilityList(ctx, CompatibilityQuery{Filter: CompatibilityFilter{
		ClientProfile: "apple_tv_4k", DirectPlay: "all", Reason: "subtitle_pgs",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != pgs {
		t.Fatalf("reason=subtitle_pgs: want only pgs(%d), got %+v", pgs, got)
	}
}

func TestCompatibilityList_riskDescKeysetPaginationNoDupesOrGaps(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	const n = 25
	for i := 0; i < n; i++ {
		risk := (i % 5) * 20
		insertCompatibilityFile(t, ctx, s.Media,
			testFile{path: "/m/file" + string(rune('a'+i)) + ".mkv", codec: "hevc", height: 1080, size: 1000},
			"apple_tv_4k", verdictFail(risk, compatibility.Reason{Code: "video_codec_hevc", Severity: compatibility.Hard}))
	}

	seen := map[int64]bool{}
	var afterRisk *int
	var afterID *int64
	pages := 0
	for {
		page, err := s.Media.CompatibilityList(ctx, CompatibilityQuery{
			Filter:    CompatibilityFilter{ClientProfile: "apple_tv_4k"},
			Sort:      CompatibilitySortRiskDesc,
			Limit:     7,
			AfterRisk: afterRisk,
			AfterID:   afterID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		pages++
		if pages > 100 {
			t.Fatal("pagination did not terminate")
		}
		for _, r := range page {
			if seen[r.ID] {
				t.Fatalf("duplicate row %d across pages", r.ID)
			}
			seen[r.ID] = true
		}
		last := page[len(page)-1]
		risk := last.RiskScore
		id := last.ID
		afterRisk, afterID = &risk, &id
	}

	if len(seen) != n {
		t.Fatalf("keyset walk saw %d distinct rows, want %d (gap or dupe)", len(seen), n)
	}
}

func TestCompatibilityStats(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	insertCompatibilityFile(t, ctx, s.Media,
		testFile{path: "/m/1.mkv", codec: "hevc", height: 1080, size: 1000},
		"apple_tv_4k", verdictFail(40, compatibility.Reason{Code: "container_mkv", Severity: compatibility.Advisory}))
	insertCompatibilityFile(t, ctx, s.Media,
		testFile{path: "/m/2.mkv", codec: "hevc", height: 1080, size: 1000},
		"apple_tv_4k", verdictFail(50, compatibility.Reason{Code: "container_mkv", Severity: compatibility.Advisory},
			compatibility.Reason{Code: "audio_dts", Severity: compatibility.Advisory}))
	insertCompatibilityFile(t, ctx, s.Media,
		testFile{path: "/m/3.mp4", codec: "h264", height: 1080, size: 1000},
		"apple_tv_4k", verdictPass())
	// A third at-risk file, this one NOT a savings candidate (already HEVC,
	// no predicted savings) — SavingsOverlapCount should exclude it.
	insertCompatibilityFile(t, ctx, s.Media,
		testFile{path: "/m/4.mkv", codec: "hevc", height: 1080, size: 1000, savings: 500},
		"apple_tv_4k", verdictFail(20, compatibility.Reason{Code: "audio_truehd", Severity: compatibility.Advisory}))

	stats, err := s.Media.CompatibilityStats(ctx, "apple_tv_4k")
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalFiles != 4 {
		t.Fatalf("TotalFiles: want 4, got %d", stats.TotalFiles)
	}
	if stats.DirectPlayCount != 1 {
		t.Fatalf("DirectPlayCount: want 1, got %d", stats.DirectPlayCount)
	}
	if stats.TranscodeRiskCount != 3 {
		t.Fatalf("TranscodeRiskCount: want 3, got %d", stats.TranscodeRiskCount)
	}
	if stats.SavingsOverlapCount != 1 {
		t.Fatalf("SavingsOverlapCount: want 1 (only /m/4.mkv has predicted savings), got %d", stats.SavingsOverlapCount)
	}

	byCode := map[string]int64{}
	for _, r := range stats.ByReason {
		byCode[r.Code] = r.FileCount
	}
	if byCode["container_mkv"] != 2 {
		t.Fatalf("container_mkv: want 2, got %d (%+v)", byCode["container_mkv"], stats.ByReason)
	}
	if byCode["audio_dts"] != 1 {
		t.Fatalf("audio_dts: want 1, got %d (%+v)", byCode["audio_dts"], stats.ByReason)
	}
}

func TestNeedsCompatibilityBackfill(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	needed, err := s.Media.NeedsCompatibilityBackfill(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if needed {
		t.Fatal("empty library should not need backfill")
	}

	id, err := s.Media.insertFile(ctx, testFile{path: "/m/a.mkv", codec: "h264", height: 1080, size: 1000})
	if err != nil {
		t.Fatal(err)
	}
	needed, err = s.Media.NeedsCompatibilityBackfill(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !needed {
		t.Fatal("probed file without compatibility row should need backfill")
	}

	if err := s.Media.UpsertCompatibility(ctx, id, map[string]compatibility.Verdict{
		"apple_tv_4k": verdictPass(),
	}); err != nil {
		t.Fatal(err)
	}
	needed, err = s.Media.NeedsCompatibilityBackfill(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if needed {
		t.Fatal("file with compatibility row should not need backfill")
	}
}

func TestCompatibilityList_requiresClientProfile(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.Media.CompatibilityList(ctx, CompatibilityQuery{}); err == nil {
		t.Fatal("expected error when client_profile is missing")
	}
	if _, err := s.Media.CountCompatibility(ctx, CompatibilityFilter{}); err == nil {
		t.Fatal("expected error when client_profile is missing")
	}
	if _, err := s.Media.CompatibilityStats(ctx, ""); err == nil {
		t.Fatal("expected error when client_profile is missing")
	}
}
