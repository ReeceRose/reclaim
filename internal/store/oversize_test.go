package store

import (
	"context"
	"testing"
)

func TestFiles_oversizeFilterAndSort(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// A bloated HEVC file (high ratio), a normal file, and a modest one.
	bigHEVC, err := s.Media.insertFile(ctx, testFile{path: "/m/big-hevc.mkv", size: 1000, codec: "hevc", height: 1080, hevc: true, oversize: 3.2})
	if err != nil {
		t.Fatal(err)
	}
	mid, err := s.Media.insertFile(ctx, testFile{path: "/m/mid.mkv", size: 1000, codec: "h264", height: 1080, savings: 400, oversize: 2.1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Media.insertFile(ctx, testFile{path: "/m/normal.mkv", size: 1000, codec: "h264", height: 1080, savings: 400, oversize: 1.1}); err != nil {
		t.Fatal(err)
	}

	// Filter: threshold 2.0 keeps only the two oversized files, across codecs.
	got, err := s.Media.Files(ctx, FileQuery{Filter: FileFilter{OversizedMinRatio: 2.0}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("oversized filter: want 2 files, got %d", len(got))
	}

	// Sort: most oversized first, so the bloated HEVC file leads.
	sorted, err := s.Media.Files(ctx, FileQuery{Sort: FileSortOversizeDesc})
	if err != nil {
		t.Fatal(err)
	}
	if len(sorted) != 3 || sorted[0].ID != bigHEVC || sorted[1].ID != mid {
		t.Fatalf("oversize_desc order unexpected: %+v", sorted)
	}
}
