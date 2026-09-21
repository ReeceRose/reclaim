# Reclaim — HTTP API

REST + WebSocket reference for the Go backend. The server listens on port `8080`
(same inside and outside the container in the default `docker-compose.yml`).

- **Base URL (local dev):** `http://localhost:8080`
- **Content type:** request and response bodies are JSON unless noted.
- **Auth:** a signed, HTTP-only session cookie (`reclaim_session`). See [Authentication](#authentication).
- **Error shape:** non-2xx responses return `{ "error": "message" }`.

---

## Authentication

Reclaim uses a first-run setup + login-cookie model (no credentials in env).

| State | Behavior |
|---|---|
| Setup not complete | Every protected route redirects (`302`) to `/setup`. Only `/api/setup`, `/api/session`, `/api/login`, `/api/logout`, and `/healthz` are reachable. |
| Setup complete, no/invalid cookie | `/api/*` and the WS upgrade return `401`; non-API (SPA) paths redirect (`302`) to `/login`. |
| `DISABLE_AUTH=true` | The gate is bypassed entirely; every route is open. |

The cookie is set by `POST /api/setup` and `POST /api/login`. It is `HttpOnly`,
`SameSite=Lax`, has a 30-day `Max-Age`, and is `Secure` only when the request is
HTTPS (`X-Forwarded-Proto: https` or a TLS connection).

### Unprotected routes
`GET /healthz`, `POST /api/setup`, `POST /api/login`, `POST /api/logout`, `GET /api/session`.

---

## Health

### `GET /healthz`
Liveness probe. Always `200`.

```json
{ "status": "ok" }
```

---

## Auth endpoints

### `POST /api/setup`
First-run only. Creates the single account, stores a bcrypt hash, stamps
`setup_completed_at`, and logs the caller in (sets the session cookie).

**Body**
```json
{ "username": "admin", "password": "at-least-8-chars" }
```

**Responses**
- `200` → `{ "username": "admin" }` (+ `Set-Cookie`)
- `400` → validation error (empty username / password < 8 chars)
- `409` → setup already complete

### `POST /api/login`
Validates credentials (constant-time bcrypt) and issues a session cookie.
Lightly rate-limited per client IP (10/min).

**Body**
```json
{ "username": "admin", "password": "..." }
```

**Responses**
- `200` → `{ "username": "admin" }` (+ `Set-Cookie`)
- `401` → invalid username or password
- `429` → too many attempts

### `POST /api/logout`
Clears the session cookie. `204 No Content`.

### `GET /api/session`
Whoami / setup-state probe used by the SPA on load. Reachable unauthenticated.

```json
{
  "setup_complete": true,
  "authenticated": true,
  "username": "admin",
  "version": "0.0.42",
  "commit": "7398c12"
}
```
When unauthenticated: `authenticated: false`, `username: null`. `version` is
`"dev"` on an untagged build. Release notes for `version` come from
[`GET /api/releases`](#get-apireleases).

### `PUT /api/settings/credentials`
Changes username/password on an already-configured instance (re-bcrypts; never
returns the hash). Takes effect on the next login, no restart. **Requires a session.**

**Body**
```json
{ "username": "admin", "password": "new-password-8+" }
```
- `200` → `{ "username": "admin" }`
- `400` → validation error / setup not complete

---

## Library data

### Media file object (shared shape)

Most file endpoints return objects with these fields. Nullable columns serialize
as `null`.

`id, path, library_type, size_bytes, mtime, video_codec, video_codec_profile,
width, height, duration_seconds, bitrate_kbps, audio_codec, audio_channels,
container_format, is_efficient_codec, predicted_savings_bytes, oversize_ratio,
is_oversized, last_probed_at, probe_error, status, candidate_state, poster_path,
backdrop_path`

| Field | Notes |
|---|---|
| `status` | `active` or `missing` (soft-deleted when the path disappears) |
| `is_efficient_codec` | `true` when the file is already HEVC, AV1, or VVC. Such files are never re-encode candidates, whatever the profile's target codec: moving between efficient codecs costs a generation of quality for little or negative size change |
| `candidate_state` | Why a file can or can't be queued: `candidate`, `already_efficient`, `probe_failed`, `unknown_codec`, `queued`, `completed`, `missing` |
| `oversize_ratio` | How many times larger the file's bitrate is than a well-encoded file of the same codec and resolution — `actual_bitrate / expected_bitrate`. Codec-aware (efficient codecs get a tighter ceiling), so it flags bloat in any codec, HEVC included. `0` when not computable (missing duration/size or unknown resolution) |
| `is_oversized` | `true` when `oversize_ratio` meets or exceeds the live `oversize_threshold` setting |
| `poster_path`, `backdrop_path` | TMDB image paths (e.g. `/abc123.jpg`); prefix with `https://image.tmdb.org/t/p/<size>`. Populated on grouped TV/movie views and `GET /api/files/:id` when TMDB is configured. Movie list pages also attach posters when configured. |

`GET /api/files/:id` additionally includes TMDB detail fields when metadata
exists: `overview`, `tagline`, `genres`, `vote_average`, `vote_count`,
`release_year`, `runtime_mins`.

Resolution filter values (`height` query param) are stable buckets, classified
by width OR height (whichever is larger wins): `uhd8k` (8K), `uhd` (4K/UHD),
`qhd` (1440p), `fhd` (1080p), `hd` (720p), `sd`, and `unknown`. Exact numeric
heights like `1080` are still accepted for compatibility.

### `GET /api/stats`
Precomputed library overview (O(buckets), not O(files)).

```json
{
  "savings_target_codec": "hevc",
  "total_files": 1234,
  "total_bytes": 9876543210,
  "total_recoverable_bytes": 3210000000,
  "by_codec": [
    { "codec": "h264", "file_count": 900, "total_bytes": 8000000000,
      "predicted_savings_bytes": 3000000000, "ratio_source": "learned",
      "learned_sample_count": 42 }
  ],
  "by_resolution": [
    { "band": "fhd", "file_count": 800, "total_bytes": 7000000000, "predicted_savings_bytes": 2500000000 }
  ],
  "by_library": [
    { "library_type": "movies", "file_count": 400, "total_bytes": 5000000000, "predicted_savings_bytes": 1500000000 },
    { "library_type": "tv", "file_count": 834, "total_bytes": 4876543210, "predicted_savings_bytes": 1710000000 }
  ],
  "savings": {
    "files_encoded": 47, "bytes_saved": 412000000000, "original_bytes": 980000000000,
    "output_bytes": 568000000000, "compression_ratio": 0.58, "encode_seconds_total": 154800,
    "files_encoded_7d": 4, "bytes_saved_7d": 31000000000,
    "files_encoded_30d": 19, "bytes_saved_30d": 168000000000,
    "first_completed_at": 1735689600, "last_completed_at": 1756252800,
    "best_saved_bytes": 24000000000, "best_path": "/movies/Dune (2021)/Dune (2021).mkv",
    "savings_estimate_ratio": 0.91, "savings_estimate_samples": 40,
    "duration_estimate_ratio": 1.08, "duration_estimate_samples": 40,
    "mean_encode_seconds": 3293.6, "bytes_saved_per_encode_hour": 9581395348,
    "projected_remaining_encode_seconds": 2107904, "remaining_candidates": 640
  },
  "replacements": {
    "files_replaced": 12, "bytes_delta": 35000000000,
    "...": "same shape as replacements.summary on GET /api/stats/savings"
  }
}
```

`savings_target_codec` is the codec every `predicted_savings_bytes` figure is
priced against — the default profile's codec (`hevc` or `av1`). Changing the
default profile, or its codec, reprices the whole library.

`ratio_source` on each `by_codec` entry is `seed` (shipped rule-of-thumb per
source and target codec) or `learned` (byte-weighted output/original ratio from
completed encodes to the target codec on this instance, after ≥10 samples per
source codec). HEVC and AV1 encodes never share a learned ratio. A learned
ratio applies to every file of that codec, including files indexed later;
predictions are repriced at boot, after each completed encode, and after any
profile change.

The `savings` block reports *realized* savings — measured from completed
encodes — as opposed to the `predicted_savings_bytes` figures elsewhere in the
response, which are estimates. `compression_ratio` is output÷original, so 0.58
means encoded files ended up 58% of their original size. Both
`*_estimate_ratio` fields are actual÷predicted: a `savings_estimate_ratio`
below 1 means predictions ran optimistic, and a `duration_estimate_ratio`
above 1 means encodes took longer than estimated. They are null until at least
one job carries the relevant queue-time snapshot.

The `replacements` block is the same measurement for storage reclaimed *without*
encoding — a file deleted and re-acquired as a leaner release. It is reported
separately rather than folded into `savings` because the encode figures double
as the accuracy record for the savings and duration models, which a replacement
has no prediction to be scored against. Lifetime reclaimed storage is
`savings.bytes_saved + replacements.bytes_delta`. See § `GET /api/stats/savings`
for the full field list.

### `GET /api/stats/savings`
The full realized-savings report backing the Insights page. Reads the
`savings_ledger`, which holds an append-only row per reclaim: one per completed
encode (`source: "encode"`), and one per file deleted and re-acquired out of
band (`source: "replace"`). The two are reported side by side rather than
merged — see the `replacements` block below.

**Query params**

| Param | Notes |
|---|---|
| `days` | Window for the `daily` series, 1–3650. Default `90`. Out-of-range or non-numeric values 400. |

**Response**

```json
{
  "summary": { "...": "same shape as the savings block on GET /api/stats" },
  "by_codec": [
    { "key": "h264", "files_encoded": 40, "original_bytes": 800000000000,
      "output_bytes": 460000000000, "bytes_saved": 340000000000, "compression_ratio": 0.575 },
    { "key": "mpeg2video", "files_encoded": 5, "original_bytes": 120000000000,
      "output_bytes": 48000000000, "bytes_saved": 72000000000, "compression_ratio": 0.40 },
    { "key": "unknown", "files_encoded": 2, "original_bytes": 60000000000,
      "output_bytes": 60000000000, "bytes_saved": 0, "compression_ratio": 1.0 }
  ],
  "by_target_codec": [
    { "key": "hevc", "files_encoded": 38, "original_bytes": 760000000000,
      "output_bytes": 440000000000, "bytes_saved": 320000000000, "compression_ratio": 0.579 },
    { "key": "av1", "files_encoded": 9, "original_bytes": 220000000000,
      "output_bytes": 128000000000, "bytes_saved": 92000000000, "compression_ratio": 0.582 }
  ],
  "by_library": [
    { "key": "movies", "files_encoded": 22, "original_bytes": 600000000000,
      "output_bytes": 350000000000, "bytes_saved": 250000000000, "compression_ratio": 0.583 }
  ],
  "by_resolution": [
    { "key": "uhd", "files_encoded": 12, "original_bytes": 500000000000,
      "output_bytes": 280000000000, "bytes_saved": 220000000000, "compression_ratio": 0.56 }
  ],
  "daily": [
    { "day": "2026-08-26", "files_encoded": 2, "bytes_saved": 18000000000,
      "files_replaced": 1, "bytes_replaced": 4000000000 }
  ],
  "top_wins": [
    { "job_id": 91, "media_file_id": 412, "path": "/movies/Dune (2021)/Dune (2021).mkv",
      "library_type": "movies", "source_codec": "h264", "result_codec": "hevc",
      "width": 3840, "height": 2160,
      "original_size_bytes": 48000000000, "output_size_bytes": 24000000000,
      "bytes_saved": 24000000000, "encode_seconds": 7200, "completed_at": 1756252800 }
  ],
  "recent": [ { "...": "same shape as top_wins, newest first" } ],
  "replacements": {
    "summary": {
      "files_replaced": 12, "original_bytes": 96000000000, "output_bytes": 61000000000,
      "bytes_delta": 35000000000, "bytes_reclaimed": 38000000000, "bytes_added": 3000000000,
      "replacements_smaller": 11, "replacements_larger": 1,
      "files_replaced_7d": 3, "bytes_delta_7d": 9000000000,
      "files_replaced_30d": 8, "bytes_delta_30d": 24000000000,
      "first_replaced_at": 1753660800, "last_replaced_at": 1756252800,
      "best_saved_bytes": 9000000000,
      "best_path": "/tv/Severance/Season 1/Severance.S01E01.1080p.x264.mkv"
    },
    "by_library": [
      { "key": "tv", "files_encoded": 9, "original_bytes": 60000000000,
        "output_bytes": 34000000000, "bytes_saved": 26000000000, "compression_ratio": 0.566 }
    ],
    "recent": [
      { "media_file_id": 812, "path": "/tv/Severance/Season 1/Severance.S01E01.1080p.x265-NEW.mkv",
        "previous_path": "/tv/Severance/Season 1/Severance.S01E01.1080p.x264.mkv",
        "library_type": "tv", "match_kind": "redownload",
        "source_codec": "h264", "result_codec": "hevc",
        "width": 1920, "height": 1080, "result_width": 1920, "result_height": 1080,
        "original_size_bytes": 6000000000, "output_size_bytes": 2500000000,
        "bytes_saved": 3500000000, "completed_at": 1756252800 }
    ],
    "top": [ { "...": "same shape as recent, biggest reclaim first" } ]
  },
  "job_outcomes": { "completed": 47, "failed": 2, "cancelled": 1 },
  "days": 90
}
```

`day` keys are bucketed in the configured display timezone (`TIMEZONE`), not
UTC, so the series lines up with the clock the UI renders. Days with no
encodes are omitted rather than zero-filled.

`source_codec` is captured *before* the encode swaps the file, because the
swap rewrites `media_files.video_codec` to the target codec and destroys the
original value. `result_codec` is the codec the job encoded to; `by_target_codec`
groups on it. Encodes that predate AV1 support are recorded as `hevc`, which is
the only codec they could have produced. Rows backfilled from pre-existing job history when the ledger migration
first ran carry a null `source_codec` for that reason — it is genuinely
unrecoverable once the swap has happened, and the migration records null rather
than guessing.

Those rows are grouped under the `unknown` key in `by_codec` (the same
convention `library_stats` uses), so the buckets always sum to
`summary.bytes_saved`. They are still excluded from the learned-ratio model,
which needs a known source codec to be meaningful.

#### Replacements

`summary`, `by_codec`, `by_target_codec`, `by_library`, `by_resolution`,
`top_wins`, and `recent` are **encode-only**; everything about a replacement lives under `replacements`.
Lifetime reclaimed storage is therefore `summary.bytes_saved +
replacements.summary.bytes_delta`, which is what the Insights headline renders.

`bytes_delta` is signed. Replacing a 1080p h264 release with a 4K HEVC one is a
legitimate replacement that *costs* disk, and the ledger records the cost rather
than dropping it. `bytes_reclaimed` and `bytes_added` split the net into its two
halves (both non-negative) so neither is hidden inside the other, and
`replacements_smaller` / `replacements_larger` count each side.

`match_kind` names the reconciliation that found the pair:
- `redownload` — the old row had already gone `missing`, and a later arrival
  matched it on content identity (show/season/episode, or movie folder). This is
  the delete-and-re-acquire case, bounded by `replace_lookback`.
- `supersede` — the file was replaced in place under the same name with a
  different extension, matched by path stem.

`source_codec` / `width` / `height` describe the file that was replaced;
`result_codec` / `result_width` / `result_height` describe the one that replaced
it. A replacement has no prediction to be scored against, so it never
contributes to the estimate-accuracy figures in `summary`, and it is excluded
from the learned-ratio model — another release's size ratio says nothing about
what this encoder achieves at this CRF.

Ledger rows outlive their media file: pruning a missing file deletes its
`transcode_jobs` history but leaves the ledger intact, so lifetime totals never
shrink retroactively.

### `GET /api/candidates`
One page of ranked re-encode candidates. Excludes files that are already in an
efficient codec (HEVC, AV1, VVC), `missing`, failed to probe, or already
queued/completed. Ranked by `predicted_savings_bytes`, priced against
`savings_target_codec`.

**Query params**

| Param | Notes |
|---|---|
| `sort` | `savings_desc` (default), `size_desc`, `size_asc`, `codec`, `resolution`, `mtime_desc`, `mtime_asc`, `library_type`, `release_desc`, `release_asc` (by `release_date`; undated files last in both directions) |
| `library_type` | filter: `movies` or `tv` |
| `video_codec` | filter, exact source codec, e.g. `h264` |
| `height` | resolution filter: `uhd8k`, `uhd`, `qhd`, `fhd`, `hd`, `sd`, `unknown`; exact numeric heights like `1080` are still accepted for compatibility |
| `search` | path substring filter |
| `limit` | page size (default 50, max 200) |
| `offset` | for non-default sorts only |
| `after_savings`, `after_id` | **keyset cursor** for the default `savings_desc` sort; pass both, taken from the previous page's `next_cursor` |

**Response**
```json
{
  "items": [ { "id": 5, "path": "/media/movies/a.mkv", "video_codec": "h264",
               "size_bytes": 5000, "predicted_savings_bytes": 2000,
               "candidate_state": "candidate", "...": "..." } ],
  "total_count": 842,
  "next_cursor": { "after_savings": 2000, "after_id": 5 }
}
```
Every file item carries `release_date`: a movie's earliest theatrical release in
any country or a TV episode's original air date (`YYYY-MM-DD`, from TMDB), or,
for a movie TMDB has no date for, the release year in its folder or file name
(`YYYY`). `null` when nothing is known. A bare year sorts below every full date
in that year.

`total_count` is included on the first page of the default `savings_desc` sort
(no cursor, no offset). `next_cursor` is present only for the default sort when
the page is full (`len(items) == limit`). Walk pages until `items` is shorter
than `limit`.

### `GET /api/files`
One page of all scanned files (the Library view). Includes already-efficient, missing,
probe-failed, queued, and completed files — each with a `candidate_state` explaining
eligibility.

**Query params**

| Param | Notes |
|---|---|
| `sort` | `path_asc` (default), `size_desc`, `size_asc`, `codec`, `resolution`, `mtime_desc`, `mtime_asc`, `library_type`, `oversize_desc` (most oversized first), `release_desc`, `release_asc` |
| `library_type` | filter: `movies` or `tv` |
| `video_codec` | filter, exact source codec, e.g. `h264` |
| `height` | resolution bucket filter (same values as `/api/candidates`) |
| `search` | path substring filter |
| `status` | `active` or `missing` |
| `candidate_state` | `candidate`, `already_efficient`, `probe_failed`, `unknown_codec`, `queued`, `completed`, `missing`. The pre-AV1 name `already_hevc` is still accepted as an alias for `already_efficient` |
| `oversized` | `true` → only files flagged oversized (`oversize_ratio ≥` the live `oversize_threshold`), any codec including HEVC and AV1 |
| `limit` | page size (default 50, max 200) |
| `offset` | page offset |

**Response**
```json
{
  "items": [ { "id": 5, "path": "/media/movies/a.mkv", "candidate_state": "already_efficient", "...": "..." } ],
  "total_count": 1234
}
```
`total_count` is included on the first page (`offset=0`). Movie rows may include
`poster_path` and `backdrop_path` when a TMDB API key is configured.

### `GET /api/files/grouped`
TV series/season summaries for the Library **By series** view. Movies use the
paginated `/api/files` endpoint.

**Query params:** same filters as `/api/files` (`library_type`, `video_codec`,
`height`, `search`, `status`, `candidate_state`), plus `progress` (see below),
`limit` (default 50, max 200) and `offset`. Returns an empty `series` list when
`library_type=movies` (movies use the flat `/api/files` endpoint).

`progress` narrows the list by how far through re-encoding each **series** is,
evaluated over the group's aggregates rather than per file. An unrecognised
value is a `400`.

| Value | Keeps series where |
|---|---|
| `converted` | `eligible_count = 0`, `queued_count = 0`, and `missing_count = 0` — the "All converted" badge |
| `partial` | some files are done and some are still eligible or queued |
| `unconverted` | every non-missing file is still eligible or queued |
| `missing` | `missing_count > 0` |

"Eligible" is the same gate the savings totals use: active, not already HEVC/AV1, probeable,
known codec, and not already queued/running/verifying/completed. `queued_count`
is active files with a `queued`, `running`, or `verifying` job.

```json
{
  "series": [
    { "title": "Breaking Bad", "library_type": "tv", "file_count": 12,
      "eligible_count": 8, "queued_count": 0, "missing_count": 0, "season_count": 2, "total_bytes": 50000000000,
      "predicted_savings_bytes": 15000000000,
      "poster_path": "/abc123.jpg", "backdrop_path": null }
  ],
  "total_count": 42
}
```

`poster_path` and `backdrop_path` are present when a TMDB API key is configured.

### `GET /api/files/grouped/seasons`
Season breakdown for one TV series.

**Query params:** `series` (required).

```json
{
  "seasons": [
    { "season": 1, "file_count": 6, "eligible_count": 4, "queued_count": 0, "missing_count": 0,
      "total_bytes": 25000000000, "predicted_savings_bytes": 7000000000,
      "episode_ids": [1, 2, 3, 4, 5, 6] }
  ]
}
```

### `GET /api/seasons`
Every `(series, season)` pair across the whole TV library, ranked together — the
"largest seasons" leaderboard. One `GROUP BY` query, so it stays O(1) regardless
of library size.

**Query params:** `sort` (`size_desc` default | `savings_desc`), `search`
(series-title substring), `progress` (same four values as
`/api/files/grouped`, applied per season instead of per series), `limit`
(default 50, max 200), `offset`. `savings_desc` counts only eligible (not HEVC/AV1,
probeable, not already queued/done) episodes, matching the per-series season
breakdown.

```json
{
  "seasons": [
    { "series_title": "Breaking Bad", "season": 3, "file_count": 6,
      "eligible_count": 4, "queued_count": 0, "missing_count": 0, "total_bytes": 51000000000,
      "predicted_savings_bytes": 14000000000, "poster_path": "/abc.jpg" }
  ],
  "total_count": 42
}
```

### `GET /api/files/grouped/episodes`
Episode rows for one TV series season in the Library view.

**Query params:** `series`, `season` (required), same filters as `/api/files`,
plus `limit` (default 50, max 200) and `offset`.

### `GET /api/files/:id`
Single media file by id. Returns the shared file object plus TMDB detail fields
(`overview`, `tagline`, `genres`, etc.) when metadata exists for the movie path
key or TV series title.
- `200` → file object
- `404` → not found

---

## Scanning

### `POST /api/scan`
Triggers an incremental (diff) rescan in the background. `202 Accepted`.
```json
{ "started": true, "kind": "incremental" }
```

### `POST /api/scan/full`
Force re-probe of every file + stats recompute. `202 Accepted` (`"kind": "full"`).

Both broadcast `scan_started`, throttled `scan_progress`, and `scan_completed` (or `scan_failed`) over the WebSocket. The startup and scheduled scans use the same lifecycle events; clients that connect mid-scan receive a retained `scan_started` on WebSocket registration.

### `POST /api/files/rescan`
Re-probes a caller-specified set of files (a single file, a season's episodes, or a whole show's episodes) and returns their refreshed rows. Unlike `/api/scan` this is synchronous — the request blocks until every ID has been re-probed — and it doesn't walk the filesystem tree, so it's cheap to call for a handful of files. A file whose path has vanished from disk is marked `missing` rather than causing an error. Bounded to 2000 IDs per request.

**Body:** `{ "ids": number[] }`
```json
{ "items": [ /* file objects, see GET /api/files/:id */ ] }
```
- `200` → items (IDs that no longer resolve to a row are silently dropped)
- `400` → `ids` empty or over the batch limit

---

## Transcode profiles (CRUD)

A profile object: `{ "id", "name", "codec", "crf", "preset", "extra_args", "is_default" }`.

`codec` is the target the profile encodes to, and it decides the encoder and
the vocabulary `crf` and `preset` are validated against:

| `codec` | Encoder | `crf` | `preset` | Defaults |
|---|---|---|---|---|
| `hevc` | `libx265` | 0–51 | `ultrafast` … `veryslow`, `placebo` | CRF 26, `medium` |
| `av1` | `libsvtav1` | 0–63 | `0` (slowest) … `13` (fastest) | CRF 30, `6` |

The default profile's `codec` is `savings_target_codec` — what every predicted
savings figure is priced against — so creating, updating, or deleting a profile
reprices the library.

### `GET /api/profiles`
```json
{ "items": [ { "id": 1, "name": "Space Saver", "codec": "hevc", "crf": 26, "preset": "medium",
               "extra_args": null, "is_default": true } ] }
```

### `POST /api/profiles`
**Body:** `{ "name", "codec"?, "crf", "preset", "extra_args"?, "is_default"? }`

`codec` defaults to `hevc` when omitted, so pre-AV1 clients keep working.
- `201` → created profile
- `400` → validation error: unknown codec, `crf` outside the codec's range, a
  `preset` the codec's encoder doesn't accept, or a codec whose encoder this
  host's ffmpeg was not built with

### `PUT /api/profiles/:id`
Same body and validation as create.
- `200` → updated profile · `404` → not found · `400` → validation error

### `DELETE /api/profiles/:id`
`204 No Content`.

### `GET /api/encoders`
Every target codec a profile can encode to, whether this host can run it, and
its CRF/preset vocabulary. Availability is detected once at boot from
`ffmpeg -encoders`; a distro ffmpeg may ship `libx265` without `libsvtav1`.

```json
{
  "items": [
    { "codec": "hevc", "label": "HEVC", "encoder": "libx265", "available": true,
      "crf_min": 0, "crf_max": 51, "default_crf": 26,
      "presets": ["ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow", "placebo"],
      "default_preset": "medium" },
    { "codec": "av1", "label": "AV1", "encoder": "libsvtav1", "available": true,
      "crf_min": 0, "crf_max": 63, "default_crf": 30,
      "presets": ["0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13"],
      "default_preset": "6" }
  ],
  "savings_target_codec": "hevc"
}
```

---

## Jobs

Job lifecycle: `queued → running → verifying → completed | failed | cancelled`.

A job object:
```
id, media_file_id, profile_id, status, queued_at, started_at, completed_at,
original_size_bytes, output_size_bytes, progress_percent, output_path,
error_message, verification_result, source_path, queue_position, forced,
encode_codec, encode_preset, encode_crf, encode_extra_args,
estimated_duration_seconds, encode_duration_seconds,
estimate_source, estimate_sample_count, predicted_savings_bytes
```

| Field | Notes |
|---|---|
| `queue_position` | 1-based for `queued` jobs, `0` otherwise |
| `forced` | `true` when the job was marked to bypass the encode window |
| `encode_codec`, `encode_preset`, `encode_crf`, `encode_extra_args` | Snapshot of the profile settings at queue time. The worker encodes with the **live** profile and restamps these columns with what it actually ran when it claims the job, so learning, the savings ledger, and history reflect the real encode even if the profile was edited in between |
| `estimated_duration_seconds` | Wall-clock encode estimate in seconds. For `queued`/`running` jobs this is a live, continuously-refreshed prediction. For `completed` jobs this is the frozen queue-time snapshot (`null` for jobs queued before this snapshot was introduced) |
| `encode_duration_seconds` | Actual wall-clock encode time (`completed_at − started_at`). Populated for `completed` jobs only |
| `estimate_source` | Where `estimated_duration_seconds` came from: `seed`, `learned_profile`, `learned_preset_crf`, `learned_preset`, or `learned_global` (every completed encode to the same codec). Every tier is scoped to the job's `encode_codec` — x265 timings never estimate an SVT-AV1 job. Only set for `queued`/`running` jobs |
| `estimate_sample_count` | Number of completed jobs in the bucket that produced the estimate. Omitted for `seed` |
| `predicted_savings_bytes` | Queue-time prediction of bytes reclaimed, snapshotted so history can compare it against the actual outcome even after the source file has since been re-encoded. Priced against the queuing profile's codec, which may differ from `savings_target_codec`. `null` for jobs queued before this snapshot was introduced |

**Encode time estimates** are computed at read time from this instance's completed jobs, bucketed by profile first with fallbacks (preset+CRF → preset → codec-wide → conservative seed rates per codec and preset), every tier scoped to the target codec. See [`docs/ENCODE-TIME-PLAN.md`](ENCODE-TIME-PLAN.md) for the rate model. Estimates require probed `duration_seconds` on the media file; without duration, no estimate is returned.

### `POST /api/jobs`
Enqueues one job per eligible file and **echoes the resolved selection** so the
UI can show an honest confirm step (§9.1). Each created job stores a snapshot
of the resolved profile's `codec`, `preset`, `crf`, and `extra_args` on the job row.

**Body**
```json
{ "file_ids": [5, 6, 7], "profile_id": 1 }
```
`profile_id` is optional; the default profile is used when omitted.

**Response (`200`)**
```json
{
  "profile": { "id": 1, "name": "Space Saver", "...": "..." },
  "queued":  [ { "job_id": 10, "media_file_id": 5, "path": "/media/movies/a.mkv" } ],
  "skipped": [ { "media_file_id": 6, "reason": "file is already in an efficient codec" } ]
}
```
Skip reasons: `file not found`, `file is not active`, `file is already in an
efficient codec`, `file already has an active or completed job`.
- `400` → empty `file_ids`, unknown `profile_id`, no default profile when
  omitted, or a profile whose encoder this host's ffmpeg was not built with.

### `GET /api/jobs`
One page of jobs, optionally filtered by status.
**Query:**
- `status` — optional, comma-separated (e.g. `queued`, `completed,failed`). Omit for all statuses.
- `order` — `queue` (default): oldest-queued-first, matching `queue_position` order. `recent`: newest-completed-first (falls back to `queued_at` for jobs never started) — use for history views.
- `limit` (default 50, max 200), `offset`.

**Response**
```json
{
  "items": [
    {
      "id": 10,
      "status": "queued",
      "queue_position": 1,
      "encode_preset": "medium",
      "encode_crf": 26,
      "estimated_duration_seconds": 840,
      "estimate_source": "seed",
      "...": "..."
    },
    {
      "id": 9,
      "status": "completed",
      "encode_duration_seconds": 2820,
      "encode_preset": "medium",
      "encode_crf": 26,
      "...": "..."
    }
  ],
  "total_count": 42,
  "queue_total_estimated_seconds": 8400,
  "queue_estimated_finish_at": 1790000000,
  "queued_count": 8,
  "queue_total_original_bytes": 732000000000,
  "queue_total_predicted_savings_bytes": 401000000000,
  "history": {
    "completed_count": 31,
    "failed_count": 2,
    "cancelled_count": 0,
    "original_size_bytes": 1240000000000,
    "output_size_bytes": 480000000000,
    "bytes_saved": 760000000000,
    "encode_seconds": 756000
  }
}
```

Every summary field describes the **whole** matching set, not this request's
page, and is returned on every page — numbered pagination needs `total_count`
to size the pager, and the header totals must not vanish when the client steps
off `offset=0`.

`total_count` reflects the requested `status` filter. The `queue_*` fields and
`queued_count` are computed over the entire queued+running set regardless of
the requested `status`/page, and are returned only when the request's `status`
includes (or omits) `queued`, and omitted when the queue is empty:

- `queue_total_estimated_seconds` sums per-job estimates for all queued jobs
  plus remaining time for any running job (estimated total minus elapsed since
  `started_at`).
- `queue_estimated_finish_at` (unix seconds) projects when the queue drains by
  replaying those estimates against the encode window: the running job and any
  forced jobs run immediately, and every other job starts only while the window
  is open — but, like the worker, runs to completion once started, so a night
  can overrun the window's end by up to one job. Returned alongside
  `queue_total_estimated_seconds`.
- `queue_total_original_bytes` and `queue_total_predicted_savings_bytes` sum
  `original_size_bytes` and `predicted_savings_bytes` over **queued jobs only**,
  matching `queued_count`. The running job is excluded: part of its output is
  already on disk as the temp file, so counting it as outstanding would
  overstate what is left to reclaim.

The `history` block is returned only when the request's `status` includes (or
omits) `completed` or `failed`, and aggregates non-dismissed jobs matching that
filter. Byte and duration figures cover completed jobs only — a failed job never
swapped a file, so it has no output size to weigh in. `bytes_saved` is
`original_size_bytes - output_size_bytes`, and `encode_seconds` sums
`completed_at - started_at` across completed jobs.

### `POST /api/jobs/:id/cancel`
Cancels a `queued`/`running`/`verifying` job. The worker kills the ffmpeg process
and cleans up temp files for running jobs.
- `200` → `{ "job_id": 10, "status": "cancelled" }`
- `404` → not found · `409` → not cancellable in current state

### `POST /api/jobs/:id/force`
Marks a `queued` job as forced so the worker runs it immediately, bypassing the
encode window.
- `200` → `{ "job_id": 10, "forced": true }`
- `404` → not found · `409` → job is not in the `queued` state

### `DELETE /api/jobs/:id`
Removes a `completed`/`failed`/`cancelled` job from the history list. The row
isn't actually deleted — it's hidden from `GET /api/jobs` but still counts
toward learned compression ratios, encode-time learning, and prevents the file
from being re-queued as a duplicate. Queued/running/verifying jobs must be
cancelled first.
- `204` → dismissed
- `404` → not found · `409` → job is still queued/running/verifying

---

## Events

Persistent audit log (also pushed live over WebSocket as `event_created`).

### `GET /api/events`
Newest first. Keyset-paginated via `after_id`.

**Query params:** `limit` (default 50, max 200), `after_id`, `severity` (`info`/`warn`/`error`),
`type` (e.g. `job_completed`, `job_failed`, `job_cancelled`, `scan_completed`, `orphan_restored`,
`missing_pruned`, `file_superseded`, `file_replaced`, `candidates_added`).

A `candidates_added` event covers **one title** — a single TV series (across however many of its
seasons arrived) or a single movie. Its metadata carries `title`, `library_type`, `count`,
`titles`, `size_bytes`, `predicted_savings_bytes`, and for TV a `seasons` array (`season`,
`count`, `size_bytes`, `savings_bytes`). A bulk arrival past the per-flush cap instead produces
one rollup event with `title` empty, `titles` set to the number covered, and up to 10 entries in
`rollup`. See § Notifications.

```json
{
  "items": [
    { "id": 1, "type": "job_completed", "severity": "info", "message": "Encode completed",
      "created_at": 1710000000, "metadata": { "job_id": 10 } }
  ],
  "next_cursor": 1
}
```

### `DELETE /api/events`
Removes every event from the audit log. `204 No Content`.

### `DELETE /api/events/:id`
Removes one event. `204 No Content` · `404` if not found.

---

## Settings

Runtime-mutable knobs, applied without a restart (the scanner/worker read them
live). Mount paths are read-only (env-set). Overrides are in-memory: a restart
re-seeds from env. The exceptions are `clock_format` and the `notify_*` fields,
which are persisted to the `settings` row and therefore survive restarts.

### `GET /api/settings`
```json
{
  "timezone": "America/New_York",
  "clock_format": "12h",
  "server_time": "23:24",
  "window_open": false,
  "window_changes_at": 1690000000,
  "encode_window_start": "00:00",
  "encode_window_end": "06:00",
  "scan_interval": "24h0m0s",
  "scan_anchor": "00:00",
  "probe_concurrency": 4,
  "oversize_threshold": 2.0,
  "missing_retention": "720h0m0s",
  "replace_lookback": "720h0m0s",
  "movies_path": "/media/movies",
  "tv_path": "/media/tv",
  "tmdb_configured": true,
  "missing_files": { "count": 34, "oldest_since": 1690000000, "size_bytes": 187000000000 },
  "notify_enabled": true,
  "notify_delay_seconds": 900,
  "notify_webhook_url": "",
  "notify_webhook_format": "json"
}
```

`timezone` is the IANA zone the encode window and scan anchor are read in — the process clock
and log stamps are always UTC, so this is what maps them to wall time. It is seeded from
`TIMEZONE` (falling back to `TZ`, then `UTC`).
`server_time` is the server's current `HH:MM` in that zone, and `window_open` /
`window_changes_at` (Unix timestamp of the next open→closed or closed→open flip, `null` when
`encode_window_start == encode_window_end` and the window is therefore always open) are the same
values the worker gates on. Clients must render these rather than recomputing the window from the
browser clock, which disagrees whenever the viewer is in a different zone than the server.
`clock_format` is `"12h"` or `"24h"` and decides how the UI renders every wall-clock time
(encode window, scan anchor, window badge). It is instance-wide, has no env var, and defaults
to `"12h"`; unlike the other fields it is stored in the DB rather than in `config.Live`.
`tmdb_configured` is `true` when a TMDB API key is present (set via `TMDB_API_KEY` env var).
`oversize_threshold` (> 1) is the bitrate multiple at or above which a file is flagged oversized.
`missing_retention` is how long a file that vanished from disk is kept as a `missing` row before
the post-scan cleanup deletes it; `"0"` means never prune (the default, from `MISSING_RETENTION`).
`replace_lookback` is how far back the scanner looks when a newly indexed file turns out to be
another copy of something already `missing`; a match folds the two rows together and books the
size difference to the savings ledger. `"0"` disables replacement matching (default `720h`, from
`REPLACE_LOOKBACK`). It interacts with `missing_retention`: a row the cleanup has already pruned
can no longer be matched, however long this window is.
`missing_files` summarizes the rows currently soft-deleted — `oldest_since` is a Unix timestamp,
and both it and `size_bytes` are `0` when `count` is `0`.
The `notify_*` fields configure new-candidate notifications — see § Notifications.

### `PUT /api/settings`
Any subset of the mutable fields. Validated as a set before applying.
```json
{ "timezone": "America/New_York", "clock_format": "24h",
  "encode_window_start": "01:00", "encode_window_end": "07:00",
  "scan_interval": "12h", "probe_concurrency": 8, "oversize_threshold": 2.5,
  "missing_retention": "720h", "replace_lookback": "720h",
  "notify_enabled": true, "notify_delay_seconds": 900,
  "notify_webhook_url": "https://discord.com/api/webhooks/…",
  "notify_webhook_format": "discord" }
```
- `200` → the full settings object (same shape as GET)
- `400` → invalid value (e.g. `encode_window_start: "99:99"`, non-positive interval/concurrency, `oversize_threshold ≤ 1`, unparseable/negative `missing_retention` or `replace_lookback`, a `timezone` that is not a loadable IANA name, a `clock_format` other than `"12h"`/`"24h"`, a `notify_webhook_url` that is not absolute http(s), a `notify_webhook_format` outside the four known values, a `notify_delay_seconds` outside `0…86400`)

Every value is validated before anything is applied, so a rejected field leaves `clock_format`
unwritten even though it persists to a different place than the live knobs. A request carrying
no `notify_*` field leaves the stored notification settings untouched.

### `POST /api/settings/prune-missing`
Immediately hard-deletes every `missing` media row and its job history, ignoring
`missing_retention`. Files on disk are never touched. Rows whose file has a `queued`,
`running`, or `verifying` job are skipped, so `deleted` can be lower than the `count`
reported by `GET /api/settings`. Writes a `missing_pruned` event when anything was removed.
```json
{ "deleted": 34, "missing_files": { "count": 0, "oldest_since": 0, "size_bytes": 0 } }
```

---

## Notifications

Reclaim announces newly-indexed **re-encode candidates** — any newly-added active file that
isn't already HEVC or AV1.

**When** — arrivals are collected until nothing new has landed for `notify_delay_seconds`
(default 900), or until 4× that if files keep trickling in. The quiet period is library-wide.
Every batch is re-checked against the candidate query before it is sent, so renamed, queued, or
already re-encoded rows drop out.

**What** — the batch is then split **one notification per title**: a TV series is one message no
matter how many episodes or seasons of it arrived, and every movie is its own. Two shows landing
in the same window are two notifications, never one mixed message. Past `maxTitlesPerFlush` (10)
titles in a single flush the whole thing collapses into one rollup instead, so a bulk import
doesn't become 200 messages (or trip a chat service's rate limit). Consecutive webhook posts are
spaced 500 ms apart for the same reason.

Example messages:
```
Severance · Season 3 — 9 new re-encode candidates · 42.1 GB recoverable
Severance — 30 new re-encode candidates across 3 seasons · 128.4 GB recoverable
Dune (2021) — 1 new re-encode candidate · 3.1 GB recoverable
47 new re-encode candidates across 23 titles · 120.0 GB recoverable   (rollup)
```

The first scan an instance ever completes is treated as the library baseline and never notifies;
every later scan does, including the startup scan after a restart.

Each batch writes a `candidates_added` event (pushed live as `event_created`) and, when
`notify_webhook_url` is set, POSTs to that URL. Delivery failures are logged only — use the
test endpoint below to verify a receiver.

**`notify_webhook_format`** picks the body shape:

| Value | Body |
|---|---|
| `json` (default) | `{ "event": "candidates_added", "message", "occurred_at", "title", "library_type", "count", "titles", "size_bytes", "predicted_savings_bytes", "seasons": [...] }` |
| `discord` | `{ "embeds": [{ "title", "description", "fields", … }] }` |
| `slack` | `{ "text": "…" }` |
| `ntfy` | plain-text body; the message rides in the `Title` header |

### `POST /api/settings/notify-test`
Delivers a sample batch so a webhook can be verified. The body may override the stored URL and
format, so a value can be tested before it is saved. Writes nothing to the events log.
```json
{ "notify_webhook_url": "https://ntfy.sh/my-topic", "notify_webhook_format": "ntfy" }
```
- `200` → `{ "sent": true }`
- `400` → no webhook configured, an invalid URL, or the receiver rejected the delivery (the message quotes the receiver's status and response)

---

## Metadata (TMDB)

Requires `TMDB_API_KEY` to be set. The background fetcher runs after each scan
and populates the `media_metadata` table. Search and refresh return `400` if the
key is not configured. `PUT /api/metadata` stores manual overrides and works
without a key.

### `GET /api/metadata`
Look up stored metadata for a movie path key or TV series title.

**Query params:** `key` (required).

```json
{
  "key": "Breaking Bad",
  "media_type": "tv",
  "tmdb_id": 1396,
  "title": "Breaking Bad",
  "tagline": "...",
  "overview": "...",
  "poster_path": "/abc123.jpg",
  "backdrop_path": "/def456.jpg",
  "release_year": 2008,
  "runtime_mins": 47,
  "vote_average": 8.9,
  "vote_count": 12000,
  "genres": ["Drama", "Crime"],
  "status": "Ended",
  "network": "AMC",
  "in_production": false,
  "is_manual": false,
  "no_match": false
}
```
Returns `null` when no row exists or TMDB is not configured.

### `GET /api/metadata/search`
Search TMDB for a movie or TV title.

**Query params:** `query` (required), `type` (`movie` or `tv`, default `tv`).

```json
{
  "results": [
    { "tmdb_id": 1396, "title": "Breaking Bad", "year": 2008,
      "poster_url": "https://image.tmdb.org/t/p/w185/abc.jpg" }
  ]
}
```

### `PUT /api/metadata`
Manually override poster/backdrop for a key (movie path key or TV series title).

**Body**
```json
{
  "key": "Breaking Bad",
  "media_type": "tv",
  "poster_url": "https://image.tmdb.org/t/p/w500/abc.jpg",
  "backdrop_url": null
}
```
- `200` → `{ "status": "ok" }`

### `POST /api/metadata/refresh`
Trigger a re-fetch run. With an empty body, queues a full background refresh.
With `key` + `media_type`, force-refreshes a single entry immediately.

**Body (optional)**
```json
{ "key": "Breaking Bad", "media_type": "tv" }
```
- `200` → `{ "status": "ok" }` (single key) or `{ "status": "queued" }` (full run)
- `503` → metadata fetcher unavailable (key not configured)

---

## Release notes

Served from `CHANGELOG.md`, which is compiled into the binary by `changelog.go`.
No request leaves the server: the notes ship with the build, so they work on an
air-gapped install and can never describe a version other than the one running.
`scripts/release.sh` writes the entry, commits it, and tags that commit, which
is what keeps the two in step.

### `GET /api/releases`
Every entry in the embedded changelog, newest first.

```json
{
  "current_version": "0.0.42",
  "repo_url": "https://github.com/ReeceRose/reclaim",
  "whats_new": false,
  "releases": [
    {
      "tag": "v0.0.42",
      "version": "0.0.42",
      "date": "2026-09-20",
      "body": "Documentation and polish…\n\n### What's Changed\n…",
      "url": "https://github.com/ReeceRose/reclaim/releases/tag/v0.0.42",
      "current": true
    }
  ]
}
```
- `body` is raw Markdown — headings at `###` and below, flat bullet lists,
  paragraphs, fenced code, and inline code/bold/links. The frontend renders that
  subset itself rather than accepting HTML.
- `current` marks the entry matching the running build. No entry is current on a
  `dev` build, or on a build whose tag postdates its changelog.
- `whats_new` is `true` when the running version differs from the one whose notes
  were last acknowledged *and* notes for it exist. Always `false` on a `dev`
  build. A fresh install is stamped at first boot, so it is never greeted with a
  changelog for a release it did not upgrade through.

### `POST /api/releases/seen`
Records the running version as acknowledged, clearing `whats_new` until the next
upgrade. The frontend calls this when the release-notes panel is opened.

- `204` → stamped

---

## WebSocket — `GET /api/ws`

Push-only live progress. All commands stay on REST. The session cookie is
validated on the upgrade handshake (unauthenticated upgrades get `401`).

Every message is a typed envelope:
```json
{ "event": "scan_started", "data": { "kind": "incremental" } }
```

| Event | Data | Emitted when |
|---|---|---|
| `scan_started` | `{ "kind": "incremental" \| "full" }` | any scan begins (startup, scheduled, or manual) |
| `scan_progress` | `{ "scan_run_id", "kind", "trigger", "started_at", "files_seen", "files_processed", "files_scanned", "files_added", "files_updated", "files_moved", "files_removed", "errors" }` | scan progress changes (throttled) |
| `scan_completed` | `{ "scan_run_id", "files_scanned", "files_added", "files_updated", "files_moved", "files_removed", "errors" }` | a scan finishes |
| `scan_failed` | `{ "error": "..." }` | a scan errors |
| `jobs_queued` | `{ "count", "profile_id" }` | jobs are enqueued |
| `job_started` | `{ "job_id", "media_file_id" }` | worker begins an encode |
| `job_progress` | `{ "job_id", "percent" }` | ffmpeg progress (throttled ~1/s to DB) |
| `job_completed` | `{ "job_id", "output_size_bytes", ... }` | encode + verify + swap succeeded |
| `job_failed` | `{ "job_id", "error" }` | encode or verification failed |
| `job_cancelled` | `{ "job_id" }` | a job is cancelled |
| `event_created` | event object (same shape as `/api/events` items) | audit log entry written |

The server sends WebSocket pings every ~54s; clients should respond with pongs
(browsers do this automatically). Slow clients that fill their send buffer are
disconnected.

---

## Quick curl walkthrough

```bash
BASE=http://localhost:8080
JAR=/tmp/reclaim.cookies

# First-run setup (also logs you in)
curl -s -c $JAR -X POST $BASE/api/setup \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"password123"}'

# Session state
curl -s -b $JAR $BASE/api/session

# Overview + first candidate page
curl -s -b $JAR $BASE/api/stats
curl -s -b $JAR "$BASE/api/candidates?limit=5"

# Trigger a scan (watch the WS for progress)
curl -s -b $JAR -X POST $BASE/api/scan

# Queue them (inspect estimated_duration_seconds on queued items)
curl -s -b $JAR -X POST $BASE/api/jobs \
  -H 'Content-Type: application/json' -d '{"file_ids":[1,2]}'
curl -s -b $JAR "$BASE/api/jobs?status=queued" | jq '.items[0].estimated_duration_seconds, .queue_total_estimated_seconds'
```

With `DISABLE_AUTH=true` (the `make dev` default) you can drop the `-c/-b $JAR`
flags entirely.
