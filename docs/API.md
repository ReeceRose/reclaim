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
{ "setup_complete": true, "authenticated": true, "username": "admin" }
```
When unauthenticated: `authenticated: false`, `username: null`.

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
container_format, is_already_hevc, predicted_savings_bytes, oversize_ratio,
is_oversized, last_probed_at, probe_error, status, candidate_state, poster_path,
backdrop_path`

| Field | Notes |
|---|---|
| `status` | `active` or `missing` (soft-deleted when the path disappears) |
| `candidate_state` | Why a file can or can't be queued: `candidate`, `already_hevc`, `probe_failed`, `unknown_codec`, `queued`, `completed`, `missing` |
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
  ]
}
```

`ratio_source` on each `by_codec` entry is `seed` (shipped rule-of-thumb per
codec) or `learned` (mean output/original ratio from completed jobs on this
instance, after ≥10 samples per codec).

### `GET /api/candidates`
One page of ranked re-encode candidates. Excludes files that are already HEVC,
`missing`, failed to probe, or already queued/completed.

**Query params**

| Param | Notes |
|---|---|
| `sort` | `savings_desc` (default), `size_desc`, `size_asc`, `codec`, `resolution`, `mtime_desc`, `mtime_asc`, `library_type` |
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
`total_count` is included on the first page of the default `savings_desc` sort
(no cursor, no offset). `next_cursor` is present only for the default sort when
the page is full (`len(items) == limit`). Walk pages until `items` is shorter
than `limit`.

### `GET /api/files`
One page of all scanned files (the Library view). Includes already-HEVC, missing,
probe-failed, queued, and completed files — each with a `candidate_state` explaining
eligibility.

**Query params**

| Param | Notes |
|---|---|
| `sort` | `path_asc` (default), `size_desc`, `size_asc`, `codec`, `resolution`, `mtime_desc`, `mtime_asc`, `library_type`, `oversize_desc` (most oversized first) |
| `library_type` | filter: `movies` or `tv` |
| `video_codec` | filter, exact source codec, e.g. `h264` |
| `height` | resolution bucket filter (same values as `/api/candidates`) |
| `search` | path substring filter |
| `status` | `active` or `missing` |
| `candidate_state` | `candidate`, `already_hevc`, `probe_failed`, `unknown_codec`, `queued`, `completed`, `missing` |
| `oversized` | `true` → only files flagged oversized (`oversize_ratio ≥` the live `oversize_threshold`), any codec including HEVC |
| `limit` | page size (default 50, max 200) |
| `offset` | page offset |

**Response**
```json
{
  "items": [ { "id": 5, "path": "/media/movies/a.mkv", "candidate_state": "already_hevc", "...": "..." } ],
  "total_count": 1234
}
```
`total_count` is included on the first page (`offset=0`). Movie rows may include
`poster_path` and `backdrop_path` when a TMDB API key is configured.

### `GET /api/files/grouped`
TV series/season summaries for the Library **By series** view. Movies use the
paginated `/api/files` endpoint.

**Query params:** same filters as `/api/files` (`library_type`, `video_codec`,
`height`, `search`, `status`, `candidate_state`), plus `limit` (default 50,
max 200) and `offset`. Returns an empty `series` list when `library_type=movies`
(movies use the flat `/api/files` endpoint).

```json
{
  "series": [
    { "title": "Breaking Bad", "library_type": "tv", "file_count": 12,
      "eligible_count": 8, "missing_count": 0, "season_count": 2, "total_bytes": 50000000000,
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
    { "season": 1, "file_count": 6, "eligible_count": 4, "missing_count": 0,
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
(series-title substring), `limit` (default 50, max 200), `offset`.
`savings_desc` counts only eligible (non-HEVC, probeable, not already
queued/done) episodes, matching the per-series season breakdown.

```json
{
  "seasons": [
    { "series_title": "Breaking Bad", "season": 3, "file_count": 6,
      "eligible_count": 4, "missing_count": 0, "total_bytes": 51000000000,
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

A profile object: `{ "id", "name", "crf", "preset", "extra_args", "is_default" }`.

### `GET /api/profiles`
```json
{ "items": [ { "id": 1, "name": "Space Saver", "crf": 26, "preset": "medium",
               "extra_args": null, "is_default": true } ] }
```

### `POST /api/profiles`
**Body:** `{ "name", "crf" (0–51), "preset", "extra_args"?, "is_default"? }`
- `201` → created profile
- `400` → validation error

### `PUT /api/profiles/:id`
Same body as create.
- `200` → updated profile · `404` → not found · `400` → validation error

### `DELETE /api/profiles/:id`
`204 No Content`.

---

## Jobs

Job lifecycle: `queued → running → verifying → completed | failed | cancelled`.

A job object:
```
id, media_file_id, profile_id, status, queued_at, started_at, completed_at,
original_size_bytes, output_size_bytes, progress_percent, output_path,
error_message, verification_result, source_path, queue_position, forced,
encode_preset, encode_crf, encode_extra_args,
estimated_duration_seconds, encode_duration_seconds,
estimate_source, estimate_sample_count, predicted_savings_bytes
```

| Field | Notes |
|---|---|
| `queue_position` | 1-based for `queued` jobs, `0` otherwise |
| `forced` | `true` when the job was marked to bypass the encode window |
| `encode_preset`, `encode_crf`, `encode_extra_args` | Snapshot of the profile settings at queue time. The worker still reads the **live** profile when encoding, but learning and history display use these columns |
| `estimated_duration_seconds` | Wall-clock encode estimate in seconds. For `queued`/`running` jobs this is a live, continuously-refreshed prediction. For `completed` jobs this is the frozen queue-time snapshot (`null` for jobs queued before this snapshot was introduced) |
| `encode_duration_seconds` | Actual wall-clock encode time (`completed_at − started_at`). Populated for `completed` jobs only |
| `estimate_source` | Where `estimated_duration_seconds` came from: `seed`, `learned_profile`, `learned_preset_crf`, `learned_preset`, or `learned_global`. Only set for `queued`/`running` jobs |
| `estimate_sample_count` | Number of completed jobs in the bucket that produced the estimate. Omitted for `seed` |
| `predicted_savings_bytes` | Queue-time prediction of bytes reclaimed, snapshotted so history can compare it against the actual outcome even after the source file has since become HEVC. `null` for jobs queued before this snapshot was introduced |

**Encode time estimates** are computed at read time from this instance's completed jobs, bucketed by profile first with fallbacks (preset+CRF → preset → global → conservative seed rates per preset). See [`docs/ENCODE-TIME-PLAN.md`](ENCODE-TIME-PLAN.md) for the rate model. Estimates require probed `duration_seconds` on the media file; without duration, no estimate is returned.

### `POST /api/jobs`
Enqueues one job per eligible file and **echoes the resolved selection** so the
UI can show an honest confirm step (§9.1). Each created job stores a snapshot
of the resolved profile's `preset`, `crf`, and `extra_args` on the job row.

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
  "skipped": [ { "media_file_id": 6, "reason": "file is already HEVC" } ]
}
```
Skip reasons: `file not found`, `file is not active`, `file is already HEVC`,
`file already has an active or completed job`.
- `400` → empty `file_ids`, unknown `profile_id`, or no default profile when omitted.

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
  "queued_count": 8
}
```

`total_count` is included on the first page only (`offset=0`) and reflects the
requested `status` filter. `queue_total_estimated_seconds` and `queued_count`
are also first-page-only, but are always computed over the **entire**
queued+running set regardless of the requested `status`/page — they sum
per-job estimates for all queued jobs plus remaining time for any running job
(estimated total minus elapsed since `started_at`) — so they stay accurate
once the queue list itself is paginated. Both are included only when the
request's `status` includes (or omits) `queued`, and omitted when zero.

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
`missing_pruned`, `file_superseded`, `candidates_added`).

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
`missing_files` summarizes the rows currently soft-deleted — `oldest_since` is a Unix timestamp,
and both it and `size_bytes` are `0` when `count` is `0`.
The `notify_*` fields configure new-candidate notifications — see § Notifications.

### `PUT /api/settings`
Any subset of the mutable fields. Validated as a set before applying.
```json
{ "timezone": "America/New_York", "clock_format": "24h",
  "encode_window_start": "01:00", "encode_window_end": "07:00",
  "scan_interval": "12h", "probe_concurrency": 8, "oversize_threshold": 2.5,
  "missing_retention": "720h",
  "notify_enabled": true, "notify_delay_seconds": 900,
  "notify_webhook_url": "https://discord.com/api/webhooks/…",
  "notify_webhook_format": "discord" }
```
- `200` → the full settings object (same shape as GET)
- `400` → invalid value (e.g. `encode_window_start: "99:99"`, non-positive interval/concurrency, `oversize_threshold ≤ 1`, unparseable/negative `missing_retention`, a `timezone` that is not a loadable IANA name, a `clock_format` other than `"12h"`/`"24h"`, a `notify_webhook_url` that is not absolute http(s), a `notify_webhook_format` outside the four known values, a `notify_delay_seconds` outside `0…86400`)

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
isn't already HEVC.

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
