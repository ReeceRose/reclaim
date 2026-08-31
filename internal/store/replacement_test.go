package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

const epKey = "tv|breakingbad|s001|e0001"

func TestFindReplacement_matchesOneMissingRowInWindow(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	gone, err := s.Media.insertFile(ctx, testFile{path: "/tv/x264.mkv", size: 4000, codec: "h264", height: 1080})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Media.MarkMissing(ctx, gone, epKey); err != nil {
		t.Fatal(err)
	}

	got, err := s.Media.FindReplacement(ctx, epKey, 0)
	if err != nil {
		t.Fatalf("FindReplacement: %v", err)
	}
	if got.ID != gone {
		t.Errorf("matched id %d, want %d", got.ID, gone)
	}

	// An active row with the same key is not a replacement target — only files
	// that actually vanished can be replaced.
	if _, err := s.Media.insertFile(ctx, testFile{
		path: "/tv/other.mkv", size: 100, replaceKey: epKey,
	}); err != nil {
		t.Fatal(err)
	}
	if got, err = s.Media.FindReplacement(ctx, epKey, 0); err != nil || got.ID != gone {
		t.Errorf("active sibling changed the match: id=%v err=%v", got, err)
	}
}

func TestFindReplacement_refusesAmbiguityEmptyKeysAndStaleRows(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.Media.FindReplacement(ctx, epKey, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("no match: err = %v, want ErrNotFound", err)
	}

	// An empty key must never match, however many unidentifiable rows exist.
	for _, p := range []string{"/tv/a.mkv", "/tv/b.mkv"} {
		id, err := s.Media.insertFile(ctx, testFile{path: p, size: 10})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Media.MarkMissing(ctx, id, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Media.FindReplacement(ctx, "", 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("empty key: err = %v, want ErrNotFound", err)
	}

	// Two missing copies of the same episode give no basis for a choice.
	for _, p := range []string{"/tv/dup1.mkv", "/tv/dup2.mkv"} {
		id, err := s.Media.insertFile(ctx, testFile{path: p, size: 10})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Media.MarkMissing(ctx, id, epKey); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Media.FindReplacement(ctx, epKey, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("ambiguous: err = %v, want ErrNotFound", err)
	}
}

func TestFindReplacement_respectsLookbackCutoff(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	gone, err := s.Media.insertFile(ctx, testFile{path: "/tv/old.mkv", size: 4000})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Media.MarkMissing(ctx, gone, epKey); err != nil {
		t.Fatal(err)
	}
	// Backdate the disappearance well past any plausible lookback.
	if _, err := s.Media.w.ExecContext(ctx,
		"UPDATE media_files SET missing_since = ? WHERE id = ?",
		time.Now().Add(-365*24*time.Hour).Unix(), gone,
	); err != nil {
		t.Fatal(err)
	}

	cutoff := time.Now().Add(-30 * 24 * time.Hour).Unix()
	if _, err := s.Media.FindReplacement(ctx, epKey, cutoff); !errors.Is(err, ErrNotFound) {
		t.Errorf("outside window: err = %v, want ErrNotFound", err)
	}
	if _, err := s.Media.FindReplacement(ctx, epKey, 0); err != nil {
		t.Errorf("no cutoff: err = %v, want a match", err)
	}
}

func TestRecordReplacement_foldsRowsAndBooksTheDelta(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	old, err := s.Media.insertFile(ctx, testFile{
		path: "/tv/x264.mkv", libraryType: "tv", size: 4000, codec: "h264", width: 1920, height: 1080,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Jobs.insertJobWithStatus(ctx, old, "completed"); err != nil {
		t.Fatal(err)
	}
	if err := s.Media.MarkMissing(ctx, old, epKey); err != nil {
		t.Fatal(err)
	}

	newer, err := s.Media.insertFile(ctx, testFile{
		path: "/tv/x265.mkv", libraryType: "tv", size: 1500, codec: "hevc", width: 1920, height: 1080, hevc: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Media.RecordReplacement(ctx, old, newer); err != nil {
		t.Fatalf("RecordReplacement: %v", err)
	}

	if _, err := s.Media.GetByID(ctx, old); !errors.Is(err, ErrNotFound) {
		t.Errorf("old row survived: err = %v", err)
	}
	var jobs int
	if err := s.Media.r.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM transcode_jobs WHERE media_file_id = ?", newer,
	).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if jobs != 1 {
		t.Errorf("job history not transferred: %d jobs on the surviving row", jobs)
	}

	sum, err := s.Savings.ReplacementSummary(ctx, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	if sum.Files != 1 || sum.BytesDelta != 2500 || sum.BytesReclaimed != 2500 || sum.BytesAdded != 0 {
		t.Errorf("summary = %+v, want 1 file / +2500 reclaimed", sum)
	}

	entries, err := s.Savings.RecentReplacements(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d ledger entries, want 1", len(entries))
	}
	e := entries[0]
	if e.PreviousPath != "/tv/x264.mkv" || e.Path != "/tv/x265.mkv" {
		t.Errorf("paths = %q → %q", e.PreviousPath, e.Path)
	}
	if e.MatchKind != MatchKindRedownload {
		t.Errorf("match kind = %q, want %q", e.MatchKind, MatchKindRedownload)
	}
	if e.SourceCodec == nil || *e.SourceCodec != "h264" || e.ResultCodec == nil || *e.ResultCodec != "hevc" {
		t.Errorf("codecs = %v → %v, want h264 → hevc", e.SourceCodec, e.ResultCodec)
	}
}

// An upgrade to a bigger release is still a replacement; the ledger records the
// cost rather than dropping it, so the lifetime figure stays honest.
func TestRecordReplacement_recordsUpgradesAsNegativeDelta(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	old, err := s.Media.insertFile(ctx, testFile{path: "/tv/1080p.mkv", libraryType: "tv", size: 2000, codec: "h264"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Media.MarkMissing(ctx, old, epKey); err != nil {
		t.Fatal(err)
	}
	newer, err := s.Media.insertFile(ctx, testFile{path: "/tv/2160p.mkv", libraryType: "tv", size: 9000, codec: "hevc", hevc: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Media.RecordReplacement(ctx, old, newer); err != nil {
		t.Fatal(err)
	}

	sum, err := s.Savings.ReplacementSummary(ctx, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	if sum.BytesDelta != -7000 {
		t.Errorf("BytesDelta = %d, want -7000", sum.BytesDelta)
	}
	if sum.BytesAdded != 7000 || sum.BytesReclaimed != 0 {
		t.Errorf("added/reclaimed = %d/%d, want 7000/0", sum.BytesAdded, sum.BytesReclaimed)
	}
	if sum.Larger != 1 || sum.Smaller != 0 {
		t.Errorf("larger/smaller = %d/%d, want 1/0", sum.Larger, sum.Smaller)
	}
}

func TestRecordReplacement_refusesWhileAJobIsInFlight(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	old, err := s.Media.insertFile(ctx, testFile{path: "/tv/busy.mkv", size: 4000})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Jobs.insertJobWithStatus(ctx, old, "running"); err != nil {
		t.Fatal(err)
	}
	newer, err := s.Media.insertFile(ctx, testFile{path: "/tv/fresh.mkv", size: 1000})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Media.RecordReplacement(ctx, old, newer); !errors.Is(err, ErrJobInFlight) {
		t.Fatalf("err = %v, want ErrJobInFlight", err)
	}
	if _, err := s.Media.GetByID(ctx, old); err != nil {
		t.Errorf("old row was folded away despite the live job: %v", err)
	}
	entries, err := s.Savings.RecentReplacements(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("ledger wrote %d entries for a refused fold", len(entries))
	}
}

// Replacement rows share the ledger with encodes but say nothing about what
// this encoder achieves, so they must not train the savings model.
func TestLearnedRatios_ignoresReplacements(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := range 5 {
		old, err := s.Media.insertFile(ctx, testFile{
			path: "/tv/old" + string(rune('a'+i)) + ".mkv", libraryType: "tv", size: 1000, codec: "h264",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Media.MarkMissing(ctx, old, ""); err != nil {
			t.Fatal(err)
		}
		newer, err := s.Media.insertFile(ctx, testFile{
			path: "/tv/new" + string(rune('a'+i)) + ".mkv", libraryType: "tv", size: 100, codec: "hevc", hevc: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Media.RecordReplacement(ctx, old, newer); err != nil {
			t.Fatal(err)
		}
	}

	ratios, err := s.Jobs.LearnedRatios(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	if r, ok := ratios["h264"]; ok {
		t.Errorf("replacements leaked into the learned ratios: h264 = %+v", r)
	}
}

func TestFindActiveReplacement_boundsCandidatesByArrival(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// The file the user just deleted, and the copy that has been sitting beside
	// it for a year. A library that keeps a 1080p and a 4K cut of one title
	// gives both rows the same key, so the ambiguity guard cannot help here:
	// deleting either leaves exactly one survivor.
	gone, err := s.Media.insertFile(ctx, testFile{path: "/tv/1080p.mkv", size: 4000, replaceKey: epKey})
	if err != nil {
		t.Fatal(err)
	}
	oldCopy, err := s.Media.insertFile(ctx, testFile{path: "/tv/2160p.mkv", size: 9000, replaceKey: epKey})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.w.ExecContext(ctx,
		"UPDATE media_files SET first_seen_at = ? WHERE id = ?",
		time.Now().Add(-365*24*time.Hour).Unix(), oldCopy,
	); err != nil {
		t.Fatal(err)
	}

	cutoff := time.Now().Add(-30 * 24 * time.Hour).Unix()
	if _, err := s.Media.FindActiveReplacement(ctx, epKey, gone, cutoff); !errors.Is(err, ErrNotFound) {
		t.Errorf("long-standing copy matched: err = %v, want ErrNotFound", err)
	}

	// The same lookup succeeds once a genuinely new arrival is the candidate,
	// which is the *arr upgrade this direction exists to catch.
	arrived, err := s.Media.insertFile(ctx, testFile{path: "/tv/new.mkv", size: 3000, replaceKey: epKey})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Media.FindActiveReplacement(ctx, epKey, gone, cutoff)
	if err != nil {
		t.Fatalf("FindActiveReplacement: %v", err)
	}
	if got.ID != arrived {
		t.Errorf("matched id %d, want %d", got.ID, arrived)
	}
}

func TestFindActiveReplacement_refusesAmbiguityAndEmptyKeys(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	gone, err := s.Media.insertFile(ctx, testFile{path: "/tv/gone.mkv", size: 4000, replaceKey: epKey})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.FindActiveReplacement(ctx, "", gone, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("empty key: err = %v, want ErrNotFound", err)
	}
	for _, p := range []string{"/tv/one.mkv", "/tv/two.mkv"} {
		if _, err := s.Media.insertFile(ctx, testFile{path: p, size: 10, replaceKey: epKey}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Media.FindActiveReplacement(ctx, epKey, gone, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("ambiguous: err = %v, want ErrNotFound", err)
	}
}

// A rename keeps the old row and deletes the freshly inserted one, so the
// survivor would otherwise carry the identity of the path it no longer has.
func TestRecordMove_rewritesTheReplaceKey(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	const newKey = "tv|breakingbad|s001|e0002"

	keep, err := s.Media.insertFile(ctx, testFile{path: "/tv/S01E01.mkv", size: 4000, replaceKey: epKey})
	if err != nil {
		t.Fatal(err)
	}
	dup, err := s.Media.insertFile(ctx, testFile{path: "/tv/S01E02.mkv", size: 4000, replaceKey: newKey})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Media.RecordMove(ctx, keep, dup, "/tv/S01E02.mkv", newKey); err != nil {
		t.Fatal(err)
	}

	got, err := s.Media.GetByID(ctx, keep)
	if err != nil {
		t.Fatal(err)
	}
	if got.ReplaceKey != newKey {
		t.Errorf("replace_key = %q, want %q", got.ReplaceKey, newKey)
	}
}
