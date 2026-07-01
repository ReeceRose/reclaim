package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"reclaim/internal/compatibility"
)

// CompatibilityReason is the JSON-persisted shape of compatibility.Reason.
// Severity is stored as its string form ("hard"/"advisory") rather than the
// int enum so the DB and API never depend on internal/compatibility's iota
// ordering.
type CompatibilityReason struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Stream   *int   `json:"stream,omitempty"`
	Message  string `json:"message"`
}

// CompatibilityRow is a MediaFile joined with its stored verdict for one
// client profile.
type CompatibilityRow struct {
	MediaFile
	ClientProfile       string
	RiskScore           int
	DirectPlayPredicted bool
	Reasons             []CompatibilityReason
	RecommendedAction   string
	EvaluatedAt         int64
}

// CompatibilitySort selects the ordering of a compatibility list page.
type CompatibilitySort string

const (
	// CompatibilitySortRiskDesc is the default, and the only sort with
	// keyset pagination (on risk_score, id) — mirrors CandidateQuery's
	// SortSavingsDesc.
	CompatibilitySortRiskDesc    CompatibilitySort = "risk_desc"
	CompatibilitySortSizeDesc    CompatibilitySort = "size_desc"
	CompatibilitySortMtimeDesc   CompatibilitySort = "mtime_desc"
	CompatibilitySortLibraryType CompatibilitySort = "library_type"
	CompatibilitySortCodec       CompatibilitySort = "codec"
)

var compatibilityOrderClauses = map[CompatibilitySort]string{
	CompatibilitySortRiskDesc:    "mc.risk_score DESC, media_files.id ASC",
	CompatibilitySortSizeDesc:    "media_files.size_bytes DESC, media_files.id ASC",
	CompatibilitySortMtimeDesc:   "media_files.mtime DESC, media_files.id ASC",
	CompatibilitySortLibraryType: "media_files.library_type ASC, mc.risk_score DESC, media_files.id ASC",
	CompatibilitySortCodec:       "media_files.video_codec ASC, mc.risk_score DESC, media_files.id ASC",
}

// CompatibilityFilter narrows the compatibility list. ClientProfile is
// required; the rest mirror CandidateFilter plus two compatibility-specific
// knobs.
type CompatibilityFilter struct {
	ClientProfile string
	LibraryType   string
	VideoCodec    string
	Height        string
	Search        string
	Reason        string // reason code, e.g. "audio_dts" (exact match against reasons_json)
	DirectPlay    string // "false" (default) | "true" | "all"
}

// CompatibilityQuery is one page request against the compatibility list.
type CompatibilityQuery struct {
	Filter CompatibilityFilter
	Sort   CompatibilitySort
	Limit  int

	// Keyset cursor for CompatibilitySortRiskDesc. Both nil -> first page.
	AfterRisk *int
	AfterID   *int64

	// Offset is used for the non-keyset sorts. Ignored for CompatibilitySortRiskDesc.
	Offset int
}

const defaultCompatibilityLimit = 50
const maxCompatibilityLimit = 200

// compatibilitySelectCols mirrors mediaQ's column list, table-qualified so
// it can be joined with media_compatibility without ambiguity. Keep in sync
// with mediaQ (internal/store/media.go) and scanCompatibilityRow's Scan
// order below.
const compatibilitySelectCols = `
	media_files.id, media_files.path, media_files.library_type, media_files.size_bytes, media_files.mtime, media_files.fingerprint,
	media_files.video_codec, media_files.video_codec_profile, media_files.width, media_files.height, media_files.duration_seconds,
	media_files.bitrate_kbps, media_files.audio_codec, media_files.audio_channels, media_files.container_format,
	media_files.is_already_hevc, media_files.predicted_savings_bytes, media_files.last_probed_at, media_files.probe_error, media_files.status,
	media_files.series_title, media_files.season_number,
	media_files.pixel_format, media_files.video_bit_depth, media_files.color_transfer, media_files.color_primaries,
	media_files.audio_sample_rate, media_files.subtitle_codec`

// compatibilityBaseWhere are the inclusion rules shared by
// CompatibilityList, CountCompatibility, and CompatibilityStats — the
// inverse of the savings-candidate filter (see docs/COMPATIBILITY PLAN.md
// §8): HEVC files are included, since HEVC-in-MKV or HEVC-with-DTS can
// still fail direct play.
func compatibilityBaseWhere(profile string) ([]string, []any) {
	return []string{
		"mc.client_profile = ?",
		"media_files.status = 'active'",
		"media_files.probe_error IS NULL",
		"media_files.video_codec IS NOT NULL",
	}, []any{profile}
}

// appendCompatibilityFilter adds the compatibility-specific predicates
// (direct_play, reason) plus the shared candidate-style predicates
// (library_type, video_codec, height, search) to a WHERE slice already
// seeded by compatibilityBaseWhere.
func appendCompatibilityFilter(where []string, args []any, f CompatibilityFilter) ([]string, []any, error) {
	switch strings.ToLower(strings.TrimSpace(f.DirectPlay)) {
	case "", "false":
		where = append(where, "mc.direct_play_predicted = 0")
	case "true":
		where = append(where, "mc.direct_play_predicted = 1")
	case "all":
		// no filter — show everything evaluated for this profile
	default:
		return nil, nil, fmt.Errorf("unknown direct_play filter %q", f.DirectPlay)
	}

	if r := strings.TrimSpace(f.Reason); r != "" {
		where = append(where, `EXISTS (
			SELECT 1 FROM json_each(mc.reasons_json) je
			WHERE json_extract(je.value, '$.code') = ?
		)`)
		args = append(args, r)
	}

	return appendFilter(where, args, CandidateFilter{
		LibraryType: f.LibraryType,
		VideoCodec:  f.VideoCodec,
		Height:      f.Height,
		Search:      f.Search,
	})
}

// CompatibilityList returns one page of the compatibility list for one
// client profile, ranked and filtered. Mirrors Candidates' shape (keyset
// pagination on the default sort, offset pagination otherwise).
func (m *Media) CompatibilityList(ctx context.Context, q CompatibilityQuery) ([]CompatibilityRow, error) {
	if q.Filter.ClientProfile == "" {
		return nil, fmt.Errorf("client_profile is required")
	}

	limit := q.Limit
	if limit <= 0 {
		limit = defaultCompatibilityLimit
	}
	if limit > maxCompatibilityLimit {
		limit = maxCompatibilityLimit
	}

	sort := q.Sort
	if sort == "" {
		sort = CompatibilitySortRiskDesc
	}
	order, ok := compatibilityOrderClauses[sort]
	if !ok {
		return nil, fmt.Errorf("unknown compatibility sort %q", sort)
	}

	where, args := compatibilityBaseWhere(q.Filter.ClientProfile)
	var err error
	where, args, err = appendCompatibilityFilter(where, args, q.Filter)
	if err != nil {
		return nil, err
	}

	// Keyset cursor (default sort only): rows ordered after the given anchor.
	// Matches "ORDER BY mc.risk_score DESC, media_files.id ASC".
	if sort == CompatibilitySortRiskDesc && q.AfterRisk != nil && q.AfterID != nil {
		where = append(where,
			"(mc.risk_score < ? OR (mc.risk_score = ? AND media_files.id > ?))")
		args = append(args, *q.AfterRisk, *q.AfterRisk, *q.AfterID)
	}

	query := `SELECT ` + compatibilitySelectCols + `,
			mc.risk_score, mc.direct_play_predicted, mc.reasons_json, mc.recommended_action, mc.evaluated_at
		FROM media_files
		JOIN media_compatibility mc ON mc.media_file_id = media_files.id
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY ` + order

	if sort == CompatibilitySortRiskDesc {
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

	var out []CompatibilityRow
	for rows.Next() {
		row, err := scanCompatibilityRow(rows, q.Filter.ClientProfile)
		if err != nil {
			return nil, err
		}
		out = append(out, *row)
	}
	return out, rows.Err()
}

// CountCompatibility returns how many files match the compatibility filter.
// Uses the same predicates as CompatibilityList so totals line up with
// paging.
func (m *Media) CountCompatibility(ctx context.Context, f CompatibilityFilter) (int64, error) {
	if f.ClientProfile == "" {
		return 0, fmt.Errorf("client_profile is required")
	}
	where, args := compatibilityBaseWhere(f.ClientProfile)
	var err error
	where, args, err = appendCompatibilityFilter(where, args, f)
	if err != nil {
		return 0, err
	}

	query := `SELECT COUNT(*)
		FROM media_files
		JOIN media_compatibility mc ON mc.media_file_id = media_files.id
		WHERE ` + strings.Join(where, " AND ")
	var n int64
	if err := m.r.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// CompatibilityReasonCount is one row of the compatibility stats
// "by_reason" breakdown.
type CompatibilityReasonCount struct {
	Code      string
	FileCount int64
}

// NeedsCompatibilityBackfill reports whether active probed files are missing
// media_compatibility rows — i.e. indexed before the compatibility engine
// shipped and not yet through a full re-probe.
func (m *Media) NeedsCompatibilityBackfill(ctx context.Context) (bool, error) {
	var n int
	err := m.r.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM media_files mf
		WHERE mf.status = ?
		  AND mf.probe_error IS NULL
		  AND mf.video_codec IS NOT NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM media_compatibility mc WHERE mc.media_file_id = mf.id
		  )`,
		MediaStatusActive,
	).Scan(&n)
	return n > 0, err
}

// CompatibilityStats is the overview for one client profile (docs/COMPATIBILITY
// PLAN.md §8 GET /api/compatibility/stats).
type CompatibilityStats struct {
	ClientProfile      string
	TotalFiles         int64
	DirectPlayCount    int64
	TranscodeRiskCount int64
	// SavingsOverlapCount is how many of the transcode-risk files are also
	// savings candidates (predicted_savings_bytes > 0) — backs the
	// mandatory overview/file-detail "N of your M savings candidates also
	// fail compatibility" cross-reference callout (docs/COMPATIBILITY
	// PLAN.md §10, §14 "Product confusion" risk). Not in the original §8
	// sketch — added for PR4 since that callout can't be computed from the
	// two independent stats blocks alone without refetching/intersecting
	// full lists client-side, which doesn't scale to 20k files.
	SavingsOverlapCount int64
	ByReason            []CompatibilityReasonCount
}

// CompatibilityStats computes the compatibility overview for one client
// profile. Same base inclusion rules as CompatibilityList, but ignores
// DirectPlay/Reason/library filters — this is meant to be the unfiltered
// "big picture" number the filtered list drills into.
func (m *Media) CompatibilityStats(ctx context.Context, clientProfile string) (*CompatibilityStats, error) {
	if clientProfile == "" {
		return nil, fmt.Errorf("client_profile is required")
	}
	where, args := compatibilityBaseWhere(clientProfile)
	baseWhere := strings.Join(where, " AND ")

	stats := &CompatibilityStats{ClientProfile: clientProfile}
	row := m.r.QueryRowContext(ctx, `
		SELECT COUNT(*),
			SUM(CASE WHEN mc.direct_play_predicted = 1 THEN 1 ELSE 0 END),
			SUM(CASE WHEN mc.direct_play_predicted = 0 AND media_files.predicted_savings_bytes > 0 THEN 1 ELSE 0 END)
		FROM media_files
		JOIN media_compatibility mc ON mc.media_file_id = media_files.id
		WHERE `+baseWhere, args...)
	var directPlay, savingsOverlap sql.NullInt64
	if err := row.Scan(&stats.TotalFiles, &directPlay, &savingsOverlap); err != nil {
		return nil, err
	}
	stats.DirectPlayCount = directPlay.Int64
	stats.TranscodeRiskCount = stats.TotalFiles - stats.DirectPlayCount
	stats.SavingsOverlapCount = savingsOverlap.Int64

	reasonRows, err := m.r.QueryContext(ctx, `
		SELECT json_extract(je.value, '$.code') AS code, COUNT(DISTINCT media_files.id) AS file_count
		FROM media_files
		JOIN media_compatibility mc ON mc.media_file_id = media_files.id
		JOIN json_each(mc.reasons_json) je
		WHERE `+baseWhere+`
		GROUP BY code
		ORDER BY file_count DESC, code ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer reasonRows.Close()
	for reasonRows.Next() {
		var rc CompatibilityReasonCount
		if err := reasonRows.Scan(&rc.Code, &rc.FileCount); err != nil {
			return nil, err
		}
		stats.ByReason = append(stats.ByReason, rc)
	}
	if err := reasonRows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

// CompatibilityForFile returns every stored client-profile verdict for one
// file — every built-in profile it's been evaluated against — for the
// file-detail "per-profile verdict toggle" (docs/COMPATIBILITY PLAN.md
// §10). Unlike CompatibilityList this doesn't apply the
// status/probe_error/video_codec inclusion filter: a specific file was
// already resolved by the caller.
func (m *Media) CompatibilityForFile(ctx context.Context, fileID int64) ([]CompatibilityRow, error) {
	rows, err := m.r.QueryContext(ctx, `
		SELECT client_profile, risk_score, direct_play_predicted, reasons_json, recommended_action, evaluated_at
		FROM media_compatibility
		WHERE media_file_id = ?
		ORDER BY client_profile ASC`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CompatibilityRow
	for rows.Next() {
		var row CompatibilityRow
		var directPlay int
		var reasonsJSON string
		if err := rows.Scan(&row.ClientProfile, &row.RiskScore, &directPlay, &reasonsJSON, &row.RecommendedAction, &row.EvaluatedAt); err != nil {
			return nil, err
		}
		row.MediaFile.ID = fileID
		row.DirectPlayPredicted = directPlay != 0
		if reasonsJSON != "" {
			if err := json.Unmarshal([]byte(reasonsJSON), &row.Reasons); err != nil {
				return nil, err
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// CompatibilityVerdict returns the stored verdict for one file × client
// profile, used by the Phase 2 job-queue path (POST /api/jobs with
// source="compatibility") to validate recommended_action == reencode_hevc
// server-side before enqueuing — see docs/COMPATIBILITY PLAN.md §8 "Server
// validates each file's recommended_action == reencode_hevc before
// accepting." Returns ErrNotFound if the file hasn't been evaluated against
// this profile yet (e.g. probed before the engine shipped, pending
// backfill).
func (m *Media) CompatibilityVerdict(ctx context.Context, fileID int64, clientProfile string) (*CompatibilityRow, error) {
	row := m.r.QueryRowContext(ctx, `
		SELECT risk_score, direct_play_predicted, reasons_json, recommended_action, evaluated_at
		FROM media_compatibility
		WHERE media_file_id = ? AND client_profile = ?`, fileID, clientProfile)

	var r CompatibilityRow
	var directPlay int
	var reasonsJSON string
	err := row.Scan(&r.RiskScore, &directPlay, &reasonsJSON, &r.RecommendedAction, &r.EvaluatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.MediaFile.ID = fileID
	r.ClientProfile = clientProfile
	r.DirectPlayPredicted = directPlay != 0
	if reasonsJSON != "" {
		if err := json.Unmarshal([]byte(reasonsJSON), &r.Reasons); err != nil {
			return nil, err
		}
	}
	return &r, nil
}

// UpsertCompatibility replaces the stored verdicts for fileID, one per
// client profile ID in verdicts. Called once per probe (scanner.probeAndStore)
// for every profile in compatibility.BuiltinProfiles() — see
// docs/COMPATIBILITY PLAN.md §9 "Scanner hook". Keyed by profile ID rather
// than the plan's literal `[]compatibility.Verdict` signature, since
// Verdict alone doesn't carry which profile it was evaluated against.
func (m *Media) UpsertCompatibility(ctx context.Context, fileID int64, verdicts map[string]compatibility.Verdict) error {
	if len(verdicts) == 0 {
		return nil
	}
	tx, err := m.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO media_compatibility (
			media_file_id, client_profile, risk_score, direct_play_predicted,
			reasons_json, recommended_action, evaluated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(media_file_id, client_profile) DO UPDATE SET
			risk_score = excluded.risk_score,
			direct_play_predicted = excluded.direct_play_predicted,
			reasons_json = excluded.reasons_json,
			recommended_action = excluded.recommended_action,
			evaluated_at = excluded.evaluated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for profileID, v := range verdicts {
		reasonsJSON, err := json.Marshal(toCompatibilityReasons(v.Reasons))
		if err != nil {
			return err
		}
		if _, err := stmt.ExecContext(ctx,
			fileID, profileID, v.RiskScore, btoi(v.DirectPlayPredicted),
			string(reasonsJSON), string(v.RecommendedAction), now,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func toCompatibilityReasons(reasons []compatibility.Reason) []CompatibilityReason {
	out := make([]CompatibilityReason, 0, len(reasons))
	for _, r := range reasons {
		out = append(out, CompatibilityReason{
			Code:     r.Code,
			Severity: r.Severity.String(),
			Stream:   r.Stream,
			Message:  r.Message,
		})
	}
	return out
}

// scanCompatibilityRow scans one row selected by compatibilitySelectCols
// plus the trailing media_compatibility columns. Column order must match
// CompatibilityList/CountCompatibility's SELECT exactly.
func scanCompatibilityRow(s rowScanner, clientProfile string) (*CompatibilityRow, error) {
	var r CompatibilityRow
	var isHEVC, directPlay int
	var reasonsJSON string
	err := s.Scan(
		&r.ID, &r.Path, &r.LibraryType, &r.SizeBytes, &r.Mtime, &r.Fingerprint,
		&r.VideoCodec, &r.VideoCodecProfile, &r.Width, &r.Height, &r.DurationSeconds,
		&r.BitrateKbps, &r.AudioCodec, &r.AudioChannels, &r.ContainerFormat,
		&isHEVC, &r.PredictedSavingsBytes, &r.LastProbedAt, &r.ProbeError, &r.Status,
		&r.SeriesTitle, &r.SeasonNumber,
		&r.PixelFormat, &r.VideoBitDepth, &r.ColorTransfer, &r.ColorPrimaries,
		&r.AudioSampleRate, &r.SubtitleCodec,
		&r.RiskScore, &directPlay, &reasonsJSON, &r.RecommendedAction, &r.EvaluatedAt,
	)
	if err != nil {
		return nil, err
	}
	r.IsAlreadyHEVC = isHEVC != 0
	r.DirectPlayPredicted = directPlay != 0
	r.ClientProfile = clientProfile
	if reasonsJSON != "" {
		if err := json.Unmarshal([]byte(reasonsJSON), &r.Reasons); err != nil {
			return nil, err
		}
	}
	return &r, nil
}
