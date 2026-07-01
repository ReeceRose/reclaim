# Encode Time Estimation — Plan

> **Status: implemented** (migration `00009_job_encode_snapshot.sql`, Jul 2026).
> Authoritative API fields: [`docs/API.md` § Jobs](API.md#jobs).

## Goal

Show honest, instance-specific encode time estimates:

- **Queued jobs:** `~14m` per file
- **Running job:** live remaining time
- **Queue header:** `~2h 20m for 8 jobs`
- **History:** `took 47m` (actual elapsed, not estimated)

Learn from this instance's own completed jobs, bucketed by **profile first**, with smart fallbacks when history is thin.

---

## 1. What we already have

| Signal | Where | Use |
|---|---|---|
| `profile_id` | `transcode_jobs` | Primary learning bucket |
| `started_at` / `completed_at` | `transcode_jobs` | Wall-clock encode duration |
| `duration_seconds`, `width`, `height` | `media_files` | Scale estimate to file length/resolution |
| `preset`, `crf`, `extra_args` | `transcode_jobs` snapshot columns (migration 00009) + live profile | Fallback buckets + seeds; learning uses snapshot |
| `progress_percent` + WS ticks | running job | Live remaining time |

**Wall-clock encode seconds** = `completed_at - started_at` (only when both set and `completed_at > started_at`).

### Profile snapshot (implemented)

Every job stores `profile_id` from the initial schema. Migration `00009` added
snapshot columns so learning stays honest when a profile is edited after jobs ran:

```sql
ALTER TABLE transcode_jobs ADD COLUMN encode_preset TEXT;
ALTER TABLE transcode_jobs ADD COLUMN encode_crf INTEGER;
ALTER TABLE transcode_jobs ADD COLUMN encode_extra_args TEXT;
```

`Jobs.Create` populates these from the resolved profile at queue time. The worker
still reads the **live** profile when encoding (mid-queue edits apply to future
encodes), but **`LearnedEncodeRates` uses the snapshot columns**. Existing rows
were backfilled from the current profile (imperfect for pre-migration edits, better
than NULL).

---

## 2. The rate model

Define one normalized rate per historical job:

```
pixel_factor = (width × height) / (1920 × 1080)   // min 0.25, cap at 16× — avoid garbage dimensions
effective_source_seconds = duration_seconds × pixel_factor
normalized_rate = elapsed_seconds / effective_source_seconds
```

This is **"wall seconds per 1080p-equivalent source second"** — resolution scales linearly (good enough for x265); preset/CRF/profile capture the encoder settings.

**Predict a new job:**

```
estimated_seconds = normalized_rate × duration_seconds × pixel_factor
```

Return `0` / omit UI when `duration_seconds` is unknown.

---

## 3. Learning buckets — profile-first cascade

Mirror `LearnedRatios`, but with a **fallback ladder** so new profiles are not blind:

| Priority | Bucket key | Min samples | When used |
|---|---|---|---|
| 1 | `profile_id` | 3 | Same profile, same machine — best signal |
| 2 | `preset + crf` | 5 | Profile too new; siblings with same encoder knobs |
| 3 | `preset` only | 5 | CRF variants of same speed class |
| 4 | Global instance | 10 | "This box encodes at roughly X" |
| 5 | Seed table | — | Conservative per-preset guesses until any history exists |

### Store layer

`Jobs.LearnedEncodeRates(ctx, minSamples)` returns maps for each tier:

```go
type LearnedEncodeRate struct {
    Rate        float64 // normalized wall-sec / 1080p-equiv source-sec
    SampleCount int
}

type EncodeRateLookup struct {
    ByProfileID map[int64]LearnedEncodeRate
    ByPresetCRF map[string]LearnedEncodeRate // key: "medium:26"
    ByPreset    map[string]LearnedEncodeRate
    Global      *LearnedEncodeRate
}
```

**Outlier filter** (before aggregation): drop jobs where per-job `normalized_rate` is outside `[0.02, 25]` (~25× slower than realtime to ~50× faster — catches crash-recovery skew, probe errors, tiny test files).

**Clamp** aggregate to `[0.05, 20]` after averaging.

### Media layer

`internal/media/encodetime.go` — sibling to `savings.go`:

- `seedEncodeRates` per preset (conservative, README-aligned)
- `ResolveEncodeRate(profileID, preset, crf, lookup) (rate, source, sampleCount)`
- `PredictedEncodeSeconds(rate, duration, width, height) int64`
- Reuse existing `RatioSource` (`seed` / `learned`) or add `EncodeRateSource` with values: `learned_profile`, `learned_preset_crf`, `learned_preset`, `learned_global`, `seed`

---

## 4. API changes *(implemented — see [`API.md` § Jobs](API.md#jobs))*

Extend the job list query — one join, no N+1:

```sql
-- extend jobWithPathQ
LEFT JOIN media_files m ...
-- add: m.duration_seconds, m.width, m.height
-- snapshot cols: j.encode_preset, j.encode_crf
```

`handleListJobs` — fetch `LearnedEncodeRates` once per request, compute per job:

```go
type jobDTO struct {
    // ...existing...
    EstimatedDurationSeconds *int64  `json:"estimated_duration_seconds"`
    EncodeDurationSeconds    *int64  `json:"encode_duration_seconds"` // completed only: actual
    EstimateSource           string  `json:"estimate_source"`         // seed | learned_profile | ...
    EstimateSampleCount      *int    `json:"estimate_sample_count"`
}
```

Only populate `estimated_*` for `queued` and `running`. History gets `encode_duration_seconds` from `completed_at - started_at`.

**Optional:** include in list response *(included on first page when `offset=0`)*:

```json
{ "queue_total_estimated_seconds": 8400, "queued_count": 8 }
```

Sum of queued + running remaining estimates.

---

## 5. Frontend — Queue page *(implemented in `web/app/(app)/queue/page.tsx`)*

### Queued row

Next to file size:

```
186.7 MB · ~14m estimated
```

Tooltip when `estimate_source !== learned_profile`: *"Based on 12 jobs at medium/CRF 26 — your Space Saver profile has 1 completed job"*.

### Running card

Under progress bar:

```ts
const elapsed = now - (runningJob.started_at ?? now);
const remaining =
  livePercent >= 3
    ? elapsed * (100 - livePercent) / livePercent
    : Math.max((runningJob.estimated_duration_seconds ?? 0) - elapsed, 0);
// show: "47% · ~22m remaining"
```

Re-renders on WS `job_progress` — no extra timer.

### Queue header

When `queued.length > 0`:

```
Queued · 8 · ~2h 20m total
```

### History row

Small muted line:

```
took 47m · medium · CRF 26
```

Uses actual elapsed + snapshot (or profile name lookup). Helps users sanity-check whether estimates feel right.

Use `formatDurationCompact()` from `web/lib/format.ts` for `~14m`-style labels.
Label seed-based estimates with an "estimated" badge (same honesty as Overview's
savings badge).

---

## 6. Out of scope (v1)

- **Encode window wait time** — ETA is encode duration, not "when will this start"
- **Parallel workers** — single worker assumption holds
- **Source codec in the model** — let sample count absorb it; add later if estimates are consistently off for MPEG-4 vs H.264
- **Persisting estimates on the job row** — compute at read time; queue is tiny

---

## 7. Tests

| Test | Assert |
|---|---|
| `LearnedEncodeRates` | 3 jobs same profile → rate present; 2 jobs → absent, falls back |
| Outlier exclusion | 10s encode of 2hr file excluded from AVG |
| Cascade | profile miss → preset+crf hit → preset hit → seed |
| `PredictedEncodeSeconds` | 1080p 1hr @ rate 2.0 → 7200s; 4K same duration → ~4× |
| Snapshot on create | `Jobs.Create` writes preset/crf/extra_args |
| API | queued job has `estimated_duration_seconds`; completed has `encode_duration_seconds` |
| Profile edit | job queued before edit uses snapshot for learning, live profile for encode |

---

## 8. Rollout

- [x] Migration + snapshot on `Jobs.Create` + backfill SQL for existing rows
- [x] `LearnedEncodeRates` + `encodetime.go` + tests
- [x] API wiring in `handleListJobs`
- [x] Queue UI (per-job → running remaining → queue total → history actual)
- [x] One line in README under throughput
- [x] `docs/API.md` Jobs section

No restart needed beyond deploy. Estimates start seed-based, self-correct as history grows — same lifecycle users already understand from savings learning.

---

## 9. Relation to prior work

The Jul 1, 2026 design session covered preset-only bucketing and compute-at-read-time. This plan upgrades that with:

1. **Bucket by `profile_id` first** (already stored on every job)
2. **Snapshot encode settings at queue time** (honest learning when profiles are edited)
3. **Cascade fallbacks** so new/custom profiles are not stuck on generic seeds
4. **Actual elapsed time in history** — cheap, builds trust

Do not confuse this with **`dd17210` — learned savings ratios**, which learns compression ratios (`output_size / original_size`) for disk savings predictions, not encode duration.
