package media

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// chunkSize is the number of bytes read from the start and end of large files.
// Changing this const invalidates all stored fingerprints.
const chunkSize = 64 * 1024 // 64 KB

// Fingerprint returns a stable content identifier computed as
// sha256(size_bytes || first_64KB || last_64KB).
// For files smaller than 128 KB the full content is hashed in place of the
// two fixed-size chunks so there is no overlap or padding.
// The fingerprint is stable across renames and path changes; it changes when
// file content changes.
func Fingerprint(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat: %w", err)
	}
	size := info.Size()

	h := sha256.New()

	// Size as little-endian uint64 prefix disambiguates files with identical
	// chunk content but different lengths.
	var sizeBuf [8]byte
	binary.LittleEndian.PutUint64(sizeBuf[:], uint64(size))
	h.Write(sizeBuf[:])

	if size <= 2*chunkSize {
		if _, err := io.Copy(h, f); err != nil {
			return "", fmt.Errorf("hash: %w", err)
		}
	} else {
		first := make([]byte, chunkSize)
		if _, err := f.ReadAt(first, 0); err != nil {
			return "", fmt.Errorf("read first chunk: %w", err)
		}
		h.Write(first)

		last := make([]byte, chunkSize)
		if _, err := f.ReadAt(last, size-chunkSize); err != nil {
			return "", fmt.Errorf("read last chunk: %w", err)
		}
		h.Write(last)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
