package store

import (
	"context"
	"fmt"
	"strings"
)

// CandidateSort selects the ordering of a candidate page.
type CandidateSort string

const (
	// SortSavingsDesc is the default. It is the only sort that supports keyset
	// pagination (on predicted_savings_bytes, id), which is what keeps the
	// 20k-row default view fast and dupe/gap-free for infinite scroll.
	SortSavingsDesc CandidateSort = "savings_desc"
	SortSizeDesc    CandidateSort = "size_desc"
	SortSizeAsc     CandidateSort = "size_asc"
	SortCodec       CandidateSort = "codec"
	SortResolution  CandidateSort = "resolution"
	SortMtimeDesc    CandidateSort = "mtime_desc"
	SortMtimeAsc     CandidateSort = "mtime_asc"
	SortLibraryType  CandidateSort = "library_type"
)

// orderClauses maps each sort to its ORDER BY. id is always the final tiebreak
// so paging is deterministic. The map is the whitelist that keeps user-supplied
// sort values out of the SQL string.
var orderClauses = map[CandidateSort]string{
	SortSavingsDesc: "predicted_savings_bytes DESC, id ASC",
	SortSizeDesc:    "size_bytes DESC, id ASC",
	SortSizeAsc:     "size_bytes ASC, id ASC",
	SortCodec:       "video_codec ASC, predicted_savings_bytes DESC, id ASC",
	SortResolution:  "height DESC, predicted_savings_bytes DESC, id ASC",
	SortMtimeDesc:    "mtime DESC, id ASC",
	SortMtimeAsc:     "mtime ASC, id ASC",
	SortLibraryType:  "library_type ASC, predicted_savings_bytes DESC, id ASC",
}

// CandidateFilter narrows the candidate list. Zero values mean "no filter".
type CandidateFilter struct {
	LibraryType    string // "movie" | "tv"
	VideoCodec     string // exact source codec, e.g. "h264"
	ResolutionBand string // "sd" | "hd" | "uhd"
}

// CandidateQuery is one page request against the candidate list.
type CandidateQuery struct {
	Sort   CandidateSort
	Filter CandidateFilter
	Limit  int

	// Keyset cursor for SortSavingsDesc. Both nil → first page. Set them from
	// the last row of the previous page to fetch the next.
	AfterSavings *int64
	AfterID      *int64

	// Offset is used for the non-keyset sorts. Ignored for SortSavingsDesc.
	Offset int
}

const defaultCandidateLimit = 50
const maxCandidateLimit = 200

// jobExclusionSQL excludes files that already have an active or completed job so
// the same candidate doesn't keep reappearing. failed/cancelled jobs do
// NOT exclude — those files are eligible to be re-queued.
const jobExclusionSQL = `NOT EXISTS (
	SELECT 1 FROM transcode_jobs j
	WHERE j.media_file_id = media_files.id
	  AND j.status IN ('queued', 'running', 'verifying', 'completed')
)`

// Candidates returns one page of re-encode candidates, ranked and filtered.
// Files that are already HEVC, missing, failed to probe, or already
// queued/completed are excluded.
func (m *Media) Candidates(ctx context.Context, q CandidateQuery) ([]MediaFile, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = defaultCandidateLimit
	}
	if limit > maxCandidateLimit {
		limit = maxCandidateLimit
	}

	sort := q.Sort
	if sort == "" {
		sort = SortSavingsDesc
	}
	order, ok := orderClauses[sort]
	if !ok {
		return nil, fmt.Errorf("unknown candidate sort %q", sort)
	}

	var (
		where = []string{
			"status = 'active'",
			"is_already_hevc = 0",
			"probe_error IS NULL",
			"video_codec IS NOT NULL",
			jobExclusionSQL,
		}
		args []any
	)

	if q.Filter.LibraryType != "" {
		where = append(where, "library_type = ?")
		args = append(args, q.Filter.LibraryType)
	}
	if q.Filter.VideoCodec != "" {
		where = append(where, "video_codec = ?")
		args = append(args, q.Filter.VideoCodec)
	}
	if band := q.Filter.ResolutionBand; band != "" {
		clause, err := resolutionBandClause(band)
		if err != nil {
			return nil, err
		}
		where = append(where, clause)
	}

	// Keyset cursor (default sort only): rows ordered after the given anchor.
	// Matches "ORDER BY predicted_savings_bytes DESC, id ASC".
	if sort == SortSavingsDesc && q.AfterSavings != nil && q.AfterID != nil {
		where = append(where,
			"(predicted_savings_bytes < ? OR (predicted_savings_bytes = ? AND id > ?))")
		args = append(args, *q.AfterSavings, *q.AfterSavings, *q.AfterID)
	}

	query := mediaQ + " WHERE " + strings.Join(where, " AND ") + " ORDER BY " + order

	if sort == SortSavingsDesc {
		query += " LIMIT ?"
		args = append(args, limit)
	} else {
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, q.Offset)
	}

	rows, err := m.r.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MediaFile
	for rows.Next() {
		f, err := scanMedia(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

// DryRunResult is the projected outcome of re-encoding a candidate set: pure
// decision support, queues nothing.
type DryRunResult struct {
	FileCount             int64
	TotalBytes            int64
	PredictedSavingsBytes int64
}

// DryRunSavings sums the projected savings over candidate-eligible files, scoped
// either to an explicit set of ids, a filter, or both. With neither set it spans
// the whole candidate list. The same exclusions as Candidates apply, so a dry
// run never counts a file the user can't actually queue.
func (m *Media) DryRunSavings(ctx context.Context, ids []int64, filter CandidateFilter) (*DryRunResult, error) {
	where := []string{
		"status = 'active'",
		"is_already_hevc = 0",
		"probe_error IS NULL",
		"video_codec IS NOT NULL",
		jobExclusionSQL,
	}
	var args []any

	if len(ids) > 0 {
		placeholders := make([]string, len(ids))
		for i, id := range ids {
			placeholders[i] = "?"
			args = append(args, id)
		}
		where = append(where, "id IN ("+strings.Join(placeholders, ",")+")")
	}
	if filter.LibraryType != "" {
		where = append(where, "library_type = ?")
		args = append(args, filter.LibraryType)
	}
	if filter.VideoCodec != "" {
		where = append(where, "video_codec = ?")
		args = append(args, filter.VideoCodec)
	}
	if filter.ResolutionBand != "" {
		clause, err := resolutionBandClause(filter.ResolutionBand)
		if err != nil {
			return nil, err
		}
		where = append(where, clause)
	}

	query := `SELECT COUNT(*), COALESCE(SUM(size_bytes), 0), COALESCE(SUM(predicted_savings_bytes), 0)
		FROM media_files WHERE ` + strings.Join(where, " AND ")

	var res DryRunResult
	if err := m.r.QueryRowContext(ctx, query, args...).Scan(
		&res.FileCount, &res.TotalBytes, &res.PredictedSavingsBytes,
	); err != nil {
		return nil, err
	}
	return &res, nil
}

// resolutionBandClause returns the height predicate for a band, mirroring the
// bands in resolutionBand / Stats.Recompute.
func resolutionBandClause(band string) (string, error) {
	switch band {
	case "sd":
		return fmt.Sprintf("(height IS NOT NULL AND height > 0 AND height < %d)", resHeightSD), nil
	case "hd":
		return fmt.Sprintf("(height >= %d AND height < %d)", resHeightSD, resHeightHD), nil
	case "uhd":
		return fmt.Sprintf("(height >= %d)", resHeightHD), nil
	default:
		return "", fmt.Errorf("unknown resolution band %q", band)
	}
}
