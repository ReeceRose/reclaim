package store

import (
	"context"
	"reflect"
	"sort"
	"testing"
)

func normalize(ls *LibraryStats) *LibraryStats {
	sort.Slice(ls.ByCodec, func(i, j int) bool { return ls.ByCodec[i].Codec < ls.ByCodec[j].Codec })
	sort.Slice(ls.ByResolution, func(i, j int) bool { return ls.ByResolution[i].Band < ls.ByResolution[j].Band })
	sort.Slice(ls.ByLibrary, func(i, j int) bool { return ls.ByLibrary[i].LibraryType < ls.ByLibrary[j].LibraryType })
	return ls
}

func TestStats_incrementalEqualsRecompute(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// A mixed sequence of inserts, an update, a remove, and a move — every
	// mutation that maintains stats incrementally.
	ids := make([]int64, 0)
	add := func(tf testFile) int64 {
		id, err := s.Media.insertFile(ctx, tf)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		return id
	}

	a := add(testFile{path: "/m/a.mkv", size: 5000, codec: "h264", height: 1080, savings: 2000})
	add(testFile{path: "/m/b.mkv", size: 3000, codec: "mpeg2video", height: 480, savings: 1800})
	add(testFile{path: "/m/c.mkv", size: 8000, codec: "h264", height: 2160, savings: 3200})
	hevc := add(testFile{path: "/m/d.mkv", size: 4000, codec: "hevc", height: 1080, hevc: true})

	// Update: re-probe 'a' as a larger file with a different codec.
	updated := testFile{path: "/m/a.mkv", size: 9000, codec: "mpeg2video", height: 720, savings: 5400}.toMedia()
	updated.ID = a
	if err := s.Media.UpdateProbe(ctx, updated); err != nil {
		t.Fatal(err)
	}

	// Remove: mark the HEVC file missing.
	if err := s.Media.MarkMissing(ctx, hevc); err != nil {
		t.Fatal(err)
	}

	// Move: a vanished file matched to a freshly-inserted destination row.
	moveFrom := add(testFile{path: "/m/old.mkv", size: 6000, codec: "h264", height: 1080, savings: 2400})
	dst := add(testFile{path: "/m/new.mkv", size: 6000, codec: "h264", height: 1080, savings: 2400})
	if err := s.Media.RecordMove(ctx, moveFrom, dst, "/m/new.mkv"); err != nil {
		t.Fatal(err)
	}

	incremental, err := s.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Stats.Recompute(ctx); err != nil {
		t.Fatal(err)
	}
	recomputed, err := s.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(normalize(incremental), normalize(recomputed)) {
		t.Fatalf("incremental != recompute\nincremental: %+v\nrecompute:   %+v", incremental, recomputed)
	}
}

func TestStats_overviewMatchesKnownTotals(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/a.mkv", size: 5000, codec: "h264", height: 1080, savings: 2000}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/b.mkv", size: 3000, codec: "h264", height: 480, savings: 1200}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/c.mkv", size: 4000, codec: "hevc", height: 1080, hevc: true}); err != nil {
		t.Fatal(err)
	}

	ov, err := s.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ov.TotalFiles != 3 {
		t.Fatalf("TotalFiles = %d, want 3", ov.TotalFiles)
	}
	if ov.TotalBytes != 12000 {
		t.Fatalf("TotalBytes = %d, want 12000", ov.TotalBytes)
	}
	if ov.TotalRecoverableBytes != 3200 {
		t.Fatalf("TotalRecoverableBytes = %d, want 3200", ov.TotalRecoverableBytes)
	}

	byCodec := map[string]CodecStat{}
	for _, c := range ov.ByCodec {
		byCodec[c.Codec] = c
	}
	if byCodec["h264"].FileCount != 2 || byCodec["h264"].TotalBytes != 8000 {
		t.Fatalf("h264 codec stat wrong: %+v", byCodec["h264"])
	}
	if byCodec["hevc"].FileCount != 1 {
		t.Fatalf("hevc codec stat wrong: %+v", byCodec["hevc"])
	}
}

func TestStats_encodeCompletionUpdatesWithoutRecompute(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	id, err := s.Media.insertFile(ctx, testFile{path: "/m/a.mkv", size: 10000, codec: "h264", height: 1080, savings: 4000})
	if err != nil {
		t.Fatal(err)
	}

	before, err := s.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if before.TotalRecoverableBytes != 4000 {
		t.Fatalf("before recoverable = %d, want 4000", before.TotalRecoverableBytes)
	}

	// Simulate the post-encode row update: now HEVC, smaller, no savings.
	post := testFile{path: "/m/a.mkv", size: 6000, codec: "hevc", height: 1080, hevc: true, savings: 0}.toMedia()
	post.ID = id
	if err := s.Media.UpdateProbe(ctx, post); err != nil {
		t.Fatal(err)
	}

	// No Recompute call — the delta from UpdateProbe must be reflected directly.
	after, err := s.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.TotalRecoverableBytes != 0 {
		t.Fatalf("after recoverable = %d, want 0", after.TotalRecoverableBytes)
	}
	if after.TotalBytes != 6000 {
		t.Fatalf("after total bytes = %d, want 6000", after.TotalBytes)
	}
	if after.TotalFiles != 1 {
		t.Fatalf("after total files = %d, want 1", after.TotalFiles)
	}

	// And it agrees with a from-scratch recompute.
	if err := s.Stats.Recompute(ctx); err != nil {
		t.Fatal(err)
	}
	recomputed, err := s.Stats.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalize(after), normalize(recomputed)) {
		t.Fatalf("after != recompute\nafter:     %+v\nrecompute: %+v", after, recomputed)
	}
}
