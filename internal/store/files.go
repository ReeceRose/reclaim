package store

import (
	"context"
	"fmt"
	"strings"
)

// CandidateState explains whether a scanned file can be queued from the Library
// view, and if not, why not.
type CandidateState string

const (
	CandidateStateCandidate    CandidateState = "candidate"
	CandidateStateAlreadyHEVC  CandidateState = "already_hevc"
	CandidateStateProbeFailed  CandidateState = "probe_failed"
	CandidateStateUnknownCodec CandidateState = "unknown_codec"
	CandidateStateQueued       CandidateState = "queued"
	CandidateStateCompleted    CandidateState = "completed"
	CandidateStateMissing      CandidateState = "missing"
)

// FileSort selects the ordering of the all-files Library page.
type FileSort string

const (
	SortPathAsc         FileSort = "path_asc"
	FileSortSizeDesc    FileSort = "size_desc"
	FileSortSizeAsc     FileSort = "size_asc"
	FileSortCodec       FileSort = "codec"
	FileSortResolution  FileSort = "resolution"
	FileSortMtimeDesc   FileSort = "mtime_desc"
	FileSortMtimeAsc    FileSort = "mtime_asc"
	FileSortLibraryType FileSort = "library_type"
)

var fileOrderClauses = map[FileSort]string{
	SortPathAsc:         "path ASC, id ASC",
	FileSortSizeDesc:    "size_bytes DESC, id ASC",
	FileSortSizeAsc:     "size_bytes ASC, id ASC",
	FileSortCodec:       "video_codec ASC, path ASC, id ASC",
	FileSortResolution:  "height DESC, path ASC, id ASC",
	FileSortMtimeDesc:   "mtime DESC, id ASC",
	FileSortMtimeAsc:    "mtime ASC, id ASC",
	FileSortLibraryType: "library_type ASC, path ASC, id ASC",
}

// FileFilter narrows the all-files Library view. Zero values mean "no filter".
type FileFilter struct {
	LibraryType    string
	VideoCodec     string
	ResolutionBand string
	Search         string
	Status         string
	CandidateState string
}

// FileQuery is one page request against all scanned files.
type FileQuery struct {
	Sort   FileSort
	Filter FileFilter
	Limit  int
	Offset int
}

const defaultFileLimit = 50
const maxFileLimit = 200

func appendFileFilter(where []string, args []any, f FileFilter) ([]string, []any, error) {
	where, args, err := appendFilter(where, args, CandidateFilter{
		LibraryType:    f.LibraryType,
		VideoCodec:     f.VideoCodec,
		ResolutionBand: f.ResolutionBand,
		Search:         f.Search,
	})
	if err != nil {
		return nil, nil, err
	}
	if f.Status != "" {
		switch f.Status {
		case MediaStatusActive, MediaStatusMissing:
			where = append(where, "status = ?")
			args = append(args, f.Status)
		default:
			return nil, nil, fmt.Errorf("unknown file status %q", f.Status)
		}
	}
	if f.CandidateState != "" {
		clause, err := candidateStateClause(f.CandidateState)
		if err != nil {
			return nil, nil, err
		}
		where = append(where, clause)
	}
	return where, args, nil
}

func candidateStateClause(state string) (string, error) {
	switch CandidateState(state) {
	case CandidateStateCandidate:
		return "status = 'active' AND is_already_hevc = 0 AND probe_error IS NULL AND video_codec IS NOT NULL AND " + jobExclusionSQL, nil
	case CandidateStateAlreadyHEVC:
		return "status = 'active' AND probe_error IS NULL AND is_already_hevc = 1 AND " + jobExclusionSQL, nil
	case CandidateStateProbeFailed:
		return "status = 'active' AND probe_error IS NOT NULL", nil
	case CandidateStateUnknownCodec:
		return "status = 'active' AND probe_error IS NULL AND is_already_hevc = 0 AND video_codec IS NULL AND " + jobExclusionSQL, nil
	case CandidateStateQueued:
		return `status = 'active' AND probe_error IS NULL AND EXISTS (
			SELECT 1 FROM transcode_jobs j
			WHERE j.media_file_id = media_files.id
			  AND j.status IN ('queued', 'running', 'verifying')
		)`, nil
	case CandidateStateCompleted:
		return `status = 'active' AND probe_error IS NULL AND NOT EXISTS (
			SELECT 1 FROM transcode_jobs j
			WHERE j.media_file_id = media_files.id
			  AND j.status IN ('queued', 'running', 'verifying')
		) AND EXISTS (
			SELECT 1 FROM transcode_jobs j
			WHERE j.media_file_id = media_files.id
			  AND j.status = 'completed'
		)`, nil
	case CandidateStateMissing:
		return "status = 'missing'", nil
	default:
		return "", fmt.Errorf("unknown candidate_state %q", state)
	}
}

// Files returns one page of all scanned files for the Library view. Unlike
// Candidates, this intentionally includes HEVC, missing, and probe-error rows.
func (m *Media) Files(ctx context.Context, q FileQuery) ([]MediaFile, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = defaultFileLimit
	}
	if limit > maxFileLimit {
		limit = maxFileLimit
	}

	sort := q.Sort
	if sort == "" {
		sort = SortPathAsc
	}
	order, ok := fileOrderClauses[sort]
	if !ok {
		return nil, fmt.Errorf("unknown file sort %q", sort)
	}

	where, args, err := appendFileFilter(nil, nil, q.Filter)
	if err != nil {
		return nil, err
	}

	query := mediaQ
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY " + order + " LIMIT ? OFFSET ?"
	args = append(args, limit, q.Offset)

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

// CountFiles returns how many rows match the Library filter.
func (m *Media) CountFiles(ctx context.Context, filter FileFilter) (int64, error) {
	where, args, err := appendFileFilter(nil, nil, filter)
	if err != nil {
		return 0, err
	}

	query := `SELECT COUNT(*) FROM media_files`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	var n int64
	if err := m.r.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// AllFiles returns every scanned file matching the filter. It backs the grouped
// Library view, which aggregates the full TV set in one pass.
func (m *Media) AllFiles(ctx context.Context, filter FileFilter) ([]MediaFile, error) {
	where, args, err := appendFileFilter(nil, nil, filter)
	if err != nil {
		return nil, err
	}

	query := mediaQ
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY path ASC, id ASC"

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

// FilesUnderPathPrefix returns Library rows whose path starts with prefix.
func (m *Media) FilesUnderPathPrefix(ctx context.Context, filter FileFilter, prefix string) ([]MediaFile, error) {
	where := []string{"path LIKE ? ESCAPE '\\'"}
	args := []any{likePrefix(prefix)}
	var err error
	where, args, err = appendFileFilter(where, args, filter)
	if err != nil {
		return nil, err
	}

	query := mediaQ + " WHERE " + strings.Join(where, " AND ") + " ORDER BY path ASC, id ASC"
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

// CandidateStates derives the current Library eligibility state for each file.
func (m *Media) CandidateStates(ctx context.Context, files []MediaFile) (map[int64]CandidateState, error) {
	states := make(map[int64]CandidateState, len(files))
	if len(files) == 0 {
		return states, nil
	}

	placeholders := make([]string, 0, len(files))
	args := make([]any, 0, len(files))
	for _, f := range files {
		placeholders = append(placeholders, "?")
		args = append(args, f.ID)
	}

	jobStates := make(map[int64]CandidateState, len(files))
	rows, err := m.r.QueryContext(ctx, `
		SELECT media_file_id, status
		FROM transcode_jobs
		WHERE media_file_id IN (`+strings.Join(placeholders, ",")+`)
		  AND status IN ('queued', 'running', 'verifying', 'completed')`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var status string
		if err := rows.Scan(&id, &status); err != nil {
			return nil, err
		}
		if status == "completed" {
			if _, exists := jobStates[id]; !exists {
				jobStates[id] = CandidateStateCompleted
			}
			continue
		}
		jobStates[id] = CandidateStateQueued
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, f := range files {
		states[f.ID] = candidateStateForFile(f, jobStates[f.ID])
	}
	return states, nil
}

func candidateStateForFile(f MediaFile, jobState CandidateState) CandidateState {
	if f.Status == MediaStatusMissing {
		return CandidateStateMissing
	}
	if f.ProbeError != nil {
		return CandidateStateProbeFailed
	}
	if jobState != "" {
		return jobState
	}
	if f.IsAlreadyHEVC {
		return CandidateStateAlreadyHEVC
	}
	if f.VideoCodec == nil {
		return CandidateStateUnknownCodec
	}
	return CandidateStateCandidate
}
