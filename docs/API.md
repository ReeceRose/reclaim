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
`SameSite=Lax`, and `Secure` only when the request is HTTPS (`X-Forwarded-Proto: https`
or a TLS connection).

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
    { "band": "1080", "file_count": 800, "total_bytes": 7000000000, "predicted_savings_bytes": 2500000000 }
  ]
}
```

### `GET /api/candidates`
One page of ranked re-encode candidates. Excludes files that are already HEVC,
`missing`, failed to probe, or already queued/completed.

**Query params**

| Param | Notes |
|---|---|
| `sort` | `savings_desc` (default), `size_desc`, `size_asc`, `codec`, `resolution`, `mtime_desc`, `mtime_asc`, `library_type` |
| `library_type` | filter: `movies` or `tv` |
| `video_codec` | filter, exact source codec, e.g. `h264` |
| `height` | filter, pixel height as decimal string, e.g. `720`, `1080`, or `unknown` |
| `search` | path substring filter |
| `limit` | page size (default 50, max 200) |
| `offset` | for non-default sorts only |
| `after_savings`, `after_id` | **keyset cursor** for the default `savings_desc` sort; pass both, taken from the previous page's `next_cursor` |

**Response**
```json
{
  "items": [ { "id": 5, "path": "/media/movies/a.mkv", "video_codec": "h264",
               "size_bytes": 5000, "predicted_savings_bytes": 2000, "...": "..." } ],
  "next_cursor": { "after_savings": 2000, "after_id": 5 }
}
```
`next_cursor` is present only for the default sort when the page is non-empty.
Walk pages until `items` is shorter than `limit`.

A candidate/file object has these fields:
`id, path, library_type, size_bytes, mtime, video_codec, video_codec_profile,
width, height, duration_seconds, bitrate_kbps, audio_codec, audio_channels,
container_format, is_already_hevc, predicted_savings_bytes, last_probed_at,
probe_error, status`. Nullable columns serialize as `null`.

### `GET /api/files/grouped`
TV series/season summaries for the Library **By series** view. Movies use the
paginated `/api/files` endpoint.

**Query params:** same filters as `/api/files` (`library_type`, `video_codec`,
`height`, `search`, `status`, `candidate_state`), plus `limit` (default 50,
max 200) and `offset`.

```json
{
  "series": [
    { "title": "Breaking Bad", "library_type": "tv", "file_count": 12,
      "eligible_count": 8, "season_count": 2, "total_bytes": 50000000000,
      "predicted_savings_bytes": 15000000000,
      "seasons": [ { "season": 1, "file_count": 6, "eligible_count": 4, "episode_ids": [1, 2] } ] }
  ],
  "total_count": 42
}
```

### `GET /api/files/grouped/episodes`
Episode rows for one TV series season in the Library view.

**Query params:** `series`, `season` (required), same filters as `/api/files`,
plus `limit` (default 50, max 200) and `offset`.

### `GET /api/candidates/grouped`
TV series/season summaries for the **By series** candidate view. Movies use the
paginated `/api/candidates` endpoint.

**Query params:** same filters as `/api/candidates` (`library_type`, `video_codec`,
`height`, `search`), plus `limit` (default 50, max 200) and `offset`.

```json
{
  "series": [
    { "title": "Breaking Bad", "library_type": "tv", "candidate_count": 12,
      "season_count": 2, "total_bytes": 50000000000, "predicted_savings_bytes": 15000000000,
      "seasons": [ { "season": 1, "candidate_count": 6, "episode_ids": [5, 6] } ] }
  ],
  "total_count": 42
}
```

### `GET /api/candidates/grouped/episodes`
Episode rows for one TV series season in the candidate view.

**Query params:** `series`, `season` (required), same filters as `/api/candidates`,
plus `limit` (default 50, max 200) and `offset`.

### `GET /api/files/:id`
Single media file by id.
- `200` → file object
- `404` → not found

### `GET /api/dry-run`
Projects total savings for a set or filter. **Queues nothing.** Uses the same
candidate exclusions as `/api/candidates`.

**Query params:** `ids` (comma-separated ids), and/or `library_type`, `video_codec`,
`height`, `search`. With none, spans the whole candidate list.

```json
{ "file_count": 2, "total_bytes": 2000, "predicted_savings_bytes": 800 }
```

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
error_message, verification_result, source_path, queue_position, forced
```
`queue_position` is 1-based for `queued` jobs, `0` otherwise. `forced` is `true`
when the job was marked to bypass the encode window.

### `POST /api/jobs`
Enqueues one job per eligible file and **echoes the resolved selection** so the
UI can show an honest confirm step (§9.1).

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
Combined queue + history, newest first.
**Query:** `status` (optional filter, e.g. `queued`, `running`, `completed`, `failed`, `cancelled`),
`limit` (default 50, max 200), `offset`.
```json
{ "items": [ { "id": 10, "status": "queued", "queue_position": 1, "...": "..." } ], "total_count": 42 }
```

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

---

## Events

Persistent audit log (also pushed live over WebSocket as `event_created`).

### `GET /api/events`
Newest first. Keyset-paginated via `after_id`.

**Query params:** `limit` (default 50, max 200), `after_id`, `severity` (`info`/`warn`/`error`),
`type` (e.g. `job_completed`, `job_failed`, `job_cancelled`, `scan_completed`, `orphan_restored`).

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
re-seeds from env.

### `GET /api/settings`
```json
{
  "encode_window_start": "00:00",
  "encode_window_end": "06:00",
  "scan_interval": "24h0m0s",
  "probe_concurrency": 4,
  "movies_path": "/media/movies",
  "tv_path": "/media/tv"
}
```

### `PUT /api/settings`
Any subset of the mutable fields. Validated as a set before applying.
```json
{ "encode_window_start": "01:00", "encode_window_end": "07:00",
  "scan_interval": "12h", "probe_concurrency": 8 }
```
- `200` → the full settings object (same shape as GET)
- `400` → invalid value (e.g. `encode_window_start: "99:99"`, non-positive interval/concurrency)

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

# Dry-run a couple of ids
curl -s -b $JAR "$BASE/api/dry-run?ids=1,2"

# Queue them
curl -s -b $JAR -X POST $BASE/api/jobs \
  -H 'Content-Type: application/json' -d '{"file_ids":[1,2]}'
curl -s -b $JAR "$BASE/api/jobs?status=queued"
```

With `DISABLE_AUTH=true` (the `make dev` default) you can drop the `-c/-b $JAR`
flags entirely.
