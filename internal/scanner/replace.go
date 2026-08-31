package scanner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"reclaim/internal/media"
	"reclaim/internal/store"
)

// replacement is one reconciled delete-and-reacquire pair: a file that had gone
// missing and the newly indexed file that turned out to be another copy of the
// same content.
type replacement struct {
	oldPath string
	newPath string
	newID   int64
	// delta is old size minus new size, so it is negative when the replacement
	// is a larger release. Upgrading 1080p h264 to 4K HEVC is a legitimate
	// replacement that costs disk, and the ledger records it as such.
	delta int64
}

func (s *Scanner) moviesRoot() string {
	for path, lt := range s.roots {
		if lt == store.LibraryTypeMovies {
			return path
		}
	}
	return ""
}

// replaceKeyFor is the scanner's view of a path's content identity. The roots
// live here, so this is the only layer that can compute it.
func (s *Scanner) replaceKeyFor(path, libraryType string) string {
	return media.ReplaceKey(path, libraryType, s.tvRoot(), s.moviesRoot())
}

// replaceCutoff is the oldest missing_since a replacement may still match,
// and false when replacement matching is switched off.
func (s *Scanner) replaceCutoff() (int64, bool) {
	d := s.replaceLookbackFn()
	if d <= 0 {
		return 0, false
	}
	return time.Now().Add(-d).Unix(), true
}

// matchReplacements pairs freshly inserted rows against files that went missing
// within the lookback window. It runs after the vanished-path reconciliation so
// files deleted and re-acquired between two scans are matched in the same pass
// as ones that were already missing when the scan started.
//
// skip holds rows already consumed by another reconciliation (the surviving
// half of a supersede); matching them again would book the same bytes twice.
func (s *Scanner) matchReplacements(ctx context.Context, newIDs []int64, skip map[int64]struct{}) []replacement {
	cutoff, on := s.replaceCutoff()
	if !on || len(newIDs) == 0 {
		return nil
	}
	var out []replacement
	for _, id := range newIDs {
		if _, skipped := skip[id]; skipped {
			continue
		}
		if r := s.matchReplacement(ctx, id, cutoff); r != nil {
			out = append(out, *r)
		}
	}
	return out
}

// matchReplacement reconciles one newly indexed row against the missing pool,
// returning the pair it folded away or nil when there was nothing to match.
//
// Like supersede, failures are logged rather than counted as scan errors: this
// is best-effort accounting layered on an otherwise healthy scan, and the
// fallback — two unrelated rows, one missing and one new — is always safe.
func (s *Scanner) matchReplacement(ctx context.Context, newID, cutoff int64) *replacement {
	// Re-read rather than trusting the insert: by now a rename or supersede may
	// have already folded this row away, and reassigning job history onto a
	// deleted row would corrupt the very history the fold preserves.
	f, err := s.store.Media.GetByID(ctx, newID)
	if err != nil || f.Status != store.MediaStatusActive {
		return nil
	}

	key := s.replaceKeyFor(f.Path, f.LibraryType)
	if key == "" {
		return nil
	}

	old, err := s.store.Media.FindReplacement(ctx, key, cutoff)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("scanner: find replacement", "path", f.Path, "err", err)
		}
		return nil
	}
	if old.ID == f.ID {
		return nil
	}

	if err := s.store.Media.RecordReplacement(ctx, old.ID, f.ID); err != nil {
		if !errors.Is(err, store.ErrJobInFlight) {
			slog.Error("scanner: record replacement", "from", old.Path, "to", f.Path, "err", err)
		}
		return nil
	}

	delta := old.SizeBytes - f.SizeBytes
	slog.Info("scanner: file replaced", "from", old.Path, "to", f.Path, "delta_bytes", delta)
	return &replacement{oldPath: old.Path, newPath: f.Path, newID: f.ID, delta: delta}
}

// matchArrivedReplacement handles the other ordering: the file at f.Path has
// just vanished, and its replacement was imported first and is already active.
// It returns the pair it folded away, or nil to let the caller fall back to
// marking the row missing.
//
// This runs before MarkMissing rather than after, because the fold hard-deletes
// f — soft-deleting it first would leave a missing row for a file that is not
// actually gone from the library, only from that path.
//
// The candidate must itself have arrived within the lookback. A library holding
// two cuts of one title keys them the same, so without that bound deleting
// either would find the other and book a deletion as a replacement.
func (s *Scanner) matchArrivedReplacement(ctx context.Context, f *store.MediaFile) *replacement {
	cutoff, on := s.replaceCutoff()
	if !on {
		return nil
	}
	key := s.replaceKeyFor(f.Path, f.LibraryType)
	if key == "" {
		return nil
	}

	newFile, err := s.store.Media.FindActiveReplacement(ctx, key, f.ID, cutoff)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("scanner: find active replacement", "path", f.Path, "err", err)
		}
		return nil
	}
	if err := s.store.Media.RecordReplacement(ctx, f.ID, newFile.ID); err != nil {
		if !errors.Is(err, store.ErrJobInFlight) {
			slog.Error("scanner: record replacement", "from", f.Path, "to", newFile.Path, "err", err)
		}
		return nil
	}

	delta := f.SizeBytes - newFile.SizeBytes
	slog.Info("scanner: file replaced", "from", f.Path, "to", newFile.Path, "delta_bytes", delta)

	// The replacement was announced as an arrival when it was imported; it is
	// one half of a swap, so withdraw it the way a supersede does.
	if s.notifier != nil {
		s.notifier.Discard(newFile.ID)
	}
	r := &replacement{oldPath: f.Path, newPath: newFile.Path, newID: newFile.ID, delta: delta}
	s.emitReplacedEvent(ctx, *r)
	return r
}

// emitReplacedEvent writes the activity-feed entry for a single watcher-matched
// replacement.
func (s *Scanner) emitReplacedEvent(ctx context.Context, r replacement) {
	s.emitFileEvent(ctx, store.EventFileReplaced,
		fmt.Sprintf("Replaced %s with %s (%s)",
			filepath.Base(r.oldPath), filepath.Base(r.newPath), deltaLabel(r.delta)),
		scanJsonMeta(map[string]any{
			"replaced":    1,
			"from":        r.oldPath,
			"to":          r.newPath,
			"bytes_delta": r.delta,
			"trigger":     "watcher",
		}))
}

// replacedEventMessage renders the activity-feed line for a batch. The net
// delta is stated in the direction it actually went — a batch of 4K upgrades
// reads as space spent, not as a negative saving.
func replacedEventMessage(count int, net int64) string {
	files := fmt.Sprintf("%d file(s)", count)
	switch {
	case net > 0:
		return fmt.Sprintf("Replaced %s, reclaiming %s", files, formatBytes(net))
	case net < 0:
		return fmt.Sprintf("Replaced %s, using %s more", files, formatBytes(-net))
	default:
		return fmt.Sprintf("Replaced %s, no size change", files)
	}
}

// deltaLabel renders a single replacement's size change for an event message.
func deltaLabel(delta int64) string {
	switch {
	case delta > 0:
		return formatBytes(delta) + " reclaimed"
	case delta < 0:
		return formatBytes(-delta) + " larger"
	default:
		return "no size change"
	}
}

// formatBytes renders a byte count for an event message. Event text is stored
// as written, so it is formatted here rather than left to the client.
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
