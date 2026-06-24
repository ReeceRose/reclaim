package media

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFingerprintStabilityAcrossRename(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "original.mkv")

	content := bytes.Repeat([]byte("reclaim test content "), 100)
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}

	fp1, err := Fingerprint(src)
	if err != nil {
		t.Fatalf("first fingerprint: %v", err)
	}

	dst := filepath.Join(dir, "renamed.mkv")
	if err := os.Rename(src, dst); err != nil {
		t.Fatal(err)
	}

	fp2, err := Fingerprint(dst)
	if err != nil {
		t.Fatalf("second fingerprint: %v", err)
	}

	if fp1 != fp2 {
		t.Errorf("fingerprint changed after rename: %s → %s", fp1, fp2)
	}
}

func TestFingerprintSensitivityToContentChange(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.mkv")
	b := filepath.Join(dir, "b.mkv")

	if err := os.WriteFile(a, []byte("content version A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("content version B"), 0o644); err != nil {
		t.Fatal(err)
	}

	fpA, err := Fingerprint(a)
	if err != nil {
		t.Fatalf("fingerprint A: %v", err)
	}
	fpB, err := Fingerprint(b)
	if err != nil {
		t.Fatalf("fingerprint B: %v", err)
	}

	if fpA == fpB {
		t.Error("fingerprint did not change when content changed")
	}
}

func TestFingerprintSmallFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "small.mkv")

	// Well under 128 KB — exercises the full-content path.
	content := []byte("tiny file")
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}

	fp, err := Fingerprint(p)
	if err != nil {
		t.Fatalf("fingerprint small file: %v", err)
	}
	if fp == "" {
		t.Error("fingerprint is empty for small file")
	}

	// Stable on second call.
	fp2, err := Fingerprint(p)
	if err != nil {
		t.Fatal(err)
	}
	if fp != fp2 {
		t.Errorf("fingerprint not stable: %s vs %s", fp, fp2)
	}
}

func TestFingerprintLargeFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "large.mkv")

	// 300 KB — forces the first/last chunk path.
	content := bytes.Repeat([]byte{0xAB}, 300*1024)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}

	fp, err := Fingerprint(p)
	if err != nil {
		t.Fatalf("fingerprint large file: %v", err)
	}
	if fp == "" {
		t.Error("fingerprint is empty for large file")
	}

	// A single byte change in the middle shouldn't collide with the original.
	content[150*1024] ^= 0xFF
	p2 := filepath.Join(dir, "large_modified.mkv")
	if err := os.WriteFile(p2, content, 0o644); err != nil {
		t.Fatal(err)
	}
	fp2, err := Fingerprint(p2)
	if err != nil {
		t.Fatal(err)
	}
	// Middle-byte change won't affect first/last chunks, so fingerprints match —
	// this is a known and documented limitation of the chunk-based scheme.
	// The test documents it rather than asserting inequality.
	_ = fp2

	// But a change at the very start must produce a different fingerprint.
	content[0] ^= 0xFF
	p3 := filepath.Join(dir, "large_start_changed.mkv")
	if err := os.WriteFile(p3, content, 0o644); err != nil {
		t.Fatal(err)
	}
	fp3, err := Fingerprint(p3)
	if err != nil {
		t.Fatal(err)
	}
	if fp == fp3 {
		t.Error("fingerprint did not change when start-of-file byte changed")
	}
}
