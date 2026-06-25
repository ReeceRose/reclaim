# Reclaim — Media Codec Audit & Re-encode Tool

**Design Spec (merged v3)**
**Working name:** Reclaim (placeholder — rename freely)
**Status:** Draft v3.1 — ready to scaffold
**Owner:** Reece

> This version merges the two prior drafts: the detailed safety/indexing design from v1 with the structural improvements from v2 (single-container deployment, transcode profiles, package layout, dependency checks, subtitle/resolution handling, rename fingerprinting).
>
> **v3.1 change:** authentication moved from a pre-hashed env var to a first-run setup flow with DB-stored credentials and a login-page/session-cookie mechanism, matching the *arr "Forms auth" pattern (see §4.2). No more hand-generating a bcrypt hash for the compose file.

---

## 1. Problem Statement

A large Plex library (~20,000+ files) accumulates mixed video codecs over years — H264, MPEG-2, VC-1, early/inefficient HEVC encodes, and so on. The *arr stack (Sonarr/Radarr/Prowlarr) and Plex itself provide no way to:

- See aggregate codec and storage analytics across the whole library
- Identify which files are the best candidates for a space-saving HEVC re-encode
- Rank those candidates by potential space saved
- Do this through a **safe, manual-first** workflow rather than blind automation
- Track what has already been re-encoded versus what's still pending

Reclaim fills that gap: a filesystem-native audit dashboard plus a manual, opt-in re-encode workflow, running alongside the existing *arr stack in Docker.

---

## 2. Core Design Principles

These are load-bearing. Every current and future feature should be checked against them — they're the difference between a tool that helps you make decisions and one that makes risky decisions for you.

1. **No autonomous encoding, ever.** The tool never decides on its own to transcode a file. It surfaces and ranks candidates; *you* choose what gets queued. No "auto-queue everything below efficiency score X," no silent background batch jobs. Queuing is always an explicit, visible user action — even for large multi-select batches, the selection is shown and confirmed before anything is queued.

2. **Manual-first — the UI is decision support, not automation.** Stats, sorting, and filtering exist to make manual review fast across 20k+ files. They never remove the human from the loop.

3. **Filesystem is the source of truth.** No dependency on Sonarr/Radarr/Plex APIs. The tool scans mounted media folders directly with `ffprobe`. This keeps it decoupled from *arr config/auth and means it works even when those services are down or reconfigured.

4. **Non-destructive until verified.** A source file is never overwritten or deleted until its replacement has passed integrity verification. The replacement is built in a temp file and atomically swapped in only after it checks out (see §9).

5. **Scheduling controls *when jobs run*, not *what gets queued*.** You queue jobs whenever you want, from anywhere in the UI. A configurable time window (e.g. overnight) controls when the worker is allowed to *pull* queued jobs and start encoding. Outside that window, queued jobs simply wait. This is the only form of scheduling in the system — nothing ever initiates new work on its own.

---

## 3. Tech Stack

A stated goal is avoiding the *arr ecosystem's biggest UX complaint — slow load times. Stack choices treat startup speed and large-list rendering as first-class constraints, not afterthoughts.

### 3.1 Why the *arr tools feel slow (what to avoid repeating)

Worth naming so the choices below aren't cargo-culted: the .NET runtime behind the *arr stack has real JIT warmup on cold start, and their frontends tend to fetch and render near-full library lists without virtualization, so the DOM grows linearly with library size. The database (SQLite — same choice here) is rarely the bottleneck; it's the query patterns and rendering strategy on top of it. Reclaim sidesteps both directly.

### 3.2 Backend — Go

Both Go and Rust were considered. Go wins here, and it's worth being precise about why, since "fast" means different things:

- **This workload is I/O-bound and subprocess-bound, not CPU-bound in the app layer.** The scanner waits on disk and `ffprobe`; the worker waits on `ffmpeg`. Actual encode speed is identical regardless of orchestrating language — `libx265` does the same work either way. Rust's headline advantages (zero-cost abstractions, no GC pause, raw CPU throughput) matter most in CPU-bound hot loops this tool doesn't have outside ffmpeg itself.
- **Concurrency fits without the friction.** Goroutines + channels make the concurrent-`ffprobe` scanner and the queue worker straightforward — a buffered channel as a semaphore caps probe concurrency in a few lines. (Given this is a tool checked once or twice a week, not a 24/7 unattended daemon, the stronger compile-time concurrency guarantees Rust would provide are worth less here than Go's faster iteration.)
- **Fast startup** — single static binary, no JIT/bootstrap delay. Directly targets the "loads slowly" complaint.
- **Simple subprocess handling** via `os/exec`, wrapped once in a typed `ffprobe`/`ffmpeg` package so verbosity stays contained.

**Suggested libraries:** `chi` for routing; `modernc.org/sqlite` (pure-Go, no cgo — keeps the image small and the build simple) for SQLite; `fsnotify` for the filesystem watcher (§6.2); `gorilla/websocket` for live job progress; stdlib `encoding/json`.

### 3.3 Frontend — Next.js, static export

Next.js for the conventions you already use, but built as a **static export** (`output: 'export'`), not a running Next server:

- No SSR. Every data-touching page is a client component fetching directly from the Go API in the browser. There's no SEO or slow-network-first-paint reason to render on a server for a single-user LAN tool, and SSR would add a pointless extra hop.
- The static build is served **directly by the Go backend** (see §4) — no separate frontend container, no CORS.
- **Virtualized tables from day one** (`@tanstack/react-virtual`) — the single biggest lever against "browsing a large library feels slow." The DOM holds only the ~30–50 visible rows regardless of library size.
- **Server-side pagination** (server = the Go backend) — initial load fetches one page (50–100 rows), not the whole candidate list. With virtualization, perceived load time stays flat as the library grows.
- **TanStack Query** for fetching/caching; **Tailwind** for styling.

### 3.4 Why not the alternatives

- **Not .NET/Angular** (the *arr pattern) — plausibly a big part of why those tools feel heavy; no reason to inherit the same warmup and bundle characteristics.
- **Not Rust** — legitimate in the abstract, but for this I/O-bound, weekly-touched workload it trades faster iteration for guarantees that matter less here.
- **Not Python/FastAPI or Node/Express** — fine subprocess ergonomics, but neither matches Go's zero-warmup static binary plus low-friction concurrency for the scanner/worker.

---

## 4. Deployment Model

**Single container.** This is the key structural decision: one image containing the Go backend (API + scanner + worker + queue), the bundled `ffmpeg`/`ffprobe` binaries, and the static frontend build that Go serves directly.

```
reclaim/  (one container)
├── Go backend
│   ├── REST API + WebSocket
│   ├── Scanner (ffprobe + fsnotify watcher)
│   ├── Worker (ffmpeg)
│   ├── Job queue
│   └── Static UI hosting  ← serves the Next export
├── ffmpeg / ffprobe (bundled binaries)
└── SQLite DB (on a mounted volume)
```

Why single-container over the earlier two-container split: no inter-container networking, no CORS, no second image to build/version, and it keeps the whole thing inside the target image size (§14). For a single-user LAN tool this is strictly simpler with no real downside.

### 4.1 docker-compose

```yaml
services:
  reclaim:
    image: reclaim:latest
    ports:
      - "8080:8080"                          # Go serves both API and UI
    volumes:
      - /path/to/movies:/media/movies:rw     # rw needed only for in-place swap (§9.4)
      - /path/to/tv:/media/tv:rw
      - reclaim-data:/data                   # SQLite DB lives here
    environment:
      - MOVIES_PATH=/media/movies
      - TV_PATH=/media/tv
      - DB_PATH=/data/reclaim.db
      - ENCODE_WINDOW_START=00:00            # 24h local time
      - ENCODE_WINDOW_END=06:00
      - SCAN_INTERVAL=24h                    # scheduled safety-net rescan
      - PROBE_CONCURRENCY=4                  # parallel ffprobe cap
      # No auth credentials here — username/password are set once via the
      # first-run setup page and stored bcrypt-hashed in the DB (§4.2, §8).
      # - DISABLE_AUTH=false                  # optional: bypass login on a trusted LAN
      # - RESET_AUTH=false                    # optional: clear stored creds on boot → re-run setup

volumes:
  reclaim-data:
```

**Mount note:** scanning and `ffprobe` need only read; the in-place replace step (§9.4) needs write + delete on whichever library the file lives in. Given single-user/LAN-only with a login gate, mounting both read-write is the reasonable default. Revisit only if the tool is ever exposed beyond the LAN.

### 4.2 Authentication (first-run setup)

Modeled on the *arr "Forms auth" pattern: **no credentials are baked into config.** They're created once, in the running app, and stored hashed in the DB — so there's never a plaintext password (or a hand-computed hash) in the compose file or environment.

**First-run setup.** On boot, if the `settings` row has no `setup_completed_at`, the app is in **setup mode**: every protected route and the SPA redirect to a one-time setup page, and only the setup endpoint is reachable. The page takes a username + password once (over the LAN), the backend **bcrypts the password server-side**, writes `auth_username` + `auth_password_hash`, and stamps `setup_completed_at`. Plaintext is never persisted.

**Normal operation.** A login page authenticates username + password (constant-time bcrypt compare) and issues an **HTTP-only, SameSite session cookie** signed with a server-generated `session_secret` (created on first boot, persisted in `settings`, so sessions survive restarts). Protected REST routes, the static SPA, and the WebSocket all require a valid session cookie. A logout endpoint clears it. The `secure` cookie flag is set when served over HTTPS; on a plain-HTTP LAN it's omitted (documented tradeoff, same as the *arr tools).

**Why a session cookie, not HTTP Basic.** A first-run setup page and a native Basic-auth browser prompt don't coexist cleanly — the browser would challenge before the setup form could render. A login page + cookie also gives a real logout and a clean credential-change flow, which Basic can't.

**Changing credentials.** The Settings UI re-runs the same bcrypt-and-store path (no plaintext kept). Optional light rate-limiting on the login endpoint slows brute force without the lockout headaches the *arr stack warns about.

**Recovery / escape hatches** (both off by default, env-gated):
- `RESET_AUTH=true` — on boot, clears the stored credential + `setup_completed_at`, dropping back into first-run setup (the documented "I forgot my password" path; mirrors editing *arr's config to recover).
- `DISABLE_AUTH=true` — bypasses the login gate entirely for a fully trusted LAN. Off by default; surfaced as a deliberate, logged choice.

---

## 5. External Dependencies & Startup Checks

Required binaries, bundled in the image: **`ffmpeg`** and **`ffprobe`**.

On startup, the backend:

1. Verifies both binaries are present and executable — **fails fast with a clear error** if either is missing, rather than discovering it mid-job.
2. Logs the detected `ffmpeg`/`ffprobe` versions (useful when a codec behaves differently across builds).
3. Verifies the DB path is writable and runs migrations.
4. Verifies the configured media mounts exist and are readable.
5. Generates and persists a `session_secret` if one doesn't exist, and checks setup state: if `setup_completed_at` is unset, logs that the app is in **first-run setup mode** and where to point a browser — not fatal, it's the expected first boot.

---

## 6. Scanning & Indexing

The indexing model is deliberately layered: a live watcher for responsiveness, a scheduled rescan as a safety net, and manual trigger always available. This combination matters because no single mechanism is reliable alone — inotify can miss events on network mounts, and scheduled-only scanning means new downloads don't appear until the next run.

### 6.1 Incremental scan logic

For each file under the mounted roots, compare `(size_bytes, mtime)` against the stored row:

- **Unchanged** → skip, no re-probe.
- **New path** → run `ffprobe`, insert.
- **Changed** (size or mtime differs) → re-probe, update.
- **In DB but gone from disk** → see §6.3 (rename detection) before marking missing.

A **"Force full rescan"** action bypasses the diff and re-probes everything — useful after an `ffprobe` upgrade or a codec-detection fix.

`ffprobe` calls run concurrently up to `PROBE_CONCURRENCY` (a buffered-channel semaphore), so the initial 20k-file scan parallelizes without spawning thousands of processes at once.

### 6.2 Live watching (fsnotify)

A watcher monitors the mounted roots for create/modify/move/delete:

- On create/modify of a media file: **debounce ~30s** (so a file still being written by Sonarr/Radarr during import isn't probed mid-write), then queue an incremental probe.
- On delete/move-out: handled via rename detection (§6.3).
- **Network-mount caveat:** if mounts are NFS/SMB, inotify support depends on the host filesystem under the share and can drop events. This is exactly why the scheduled rescan (§6.4) exists as a backstop rather than relying on the watcher alone.

### 6.3 Rename / move detection (fingerprinting)

Rather than treating every vanished path as a delete (which would trigger a needless re-probe when the file simply moved — common with *arr renames), each file carries a lightweight **fingerprint**. A practical choice: hash of `(size_bytes + first N KB + last N KB)`, which is cheap to compute and stable across a pure rename/move.

When a path disappears and a new path appears with a matching fingerprint, it's recorded as a **move** (update the path, keep all probe data and job history) rather than a delete-plus-readd. Only paths that disappear with no fingerprint match are marked `status = missing` (soft delete, so job history stays valid).

### 6.4 Scheduled rescan

A full diff-based rescan (per §6.1) runs on the configurable `SCAN_INTERVAL` (default nightly) as a safety net for anything the watcher missed. It's independent of the encode window (§2.5) — scanning is cheap and read-only, so it doesn't need that gating.

### 6.5 Manual rescan

Always available from the UI regardless of schedule, running the same diff-based logic.

---

## 7. Package Structure

```
/cmd/reclaim          # main: wiring, startup checks, config
/internal/api         # REST handlers + WebSocket
/internal/scanner     # walk + incremental diff + fsnotify watcher
/internal/worker      # job execution within the encode window
/internal/jobs        # queue, job lifecycle/state
/internal/db          # SQLite access, migrations
/internal/media       # candidate scoring, stats aggregation
/internal/ffprobe     # typed probe wrapper
/internal/ffmpeg      # typed encode wrapper + progress parsing
/web                  # Next.js app (built to static export, embedded)
```

The frontend static build is embedded into the Go binary (e.g. `embed.FS`) so the single container ships one self-contained executable plus the bundled ffmpeg binaries.

---

## 8. Data Model (SQLite)

### `media_files`

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | |
| `path` | TEXT, unique | Absolute path inside container |
| `library_type` | TEXT | `movie` \| `tv` |
| `size_bytes` | INTEGER | |
| `mtime` | INTEGER | Unix ts — incremental-scan diffing |
| `fingerprint` | TEXT | For rename detection (§6.3) |
| `video_codec` | TEXT | e.g. `h264`, `hevc`, `mpeg2video` |
| `video_codec_profile` | TEXT | e.g. `Main 10`, nullable |
| `width` | INTEGER | |
| `height` | INTEGER | |
| `duration_seconds` | REAL | |
| `bitrate_kbps` | INTEGER | |
| `audio_codec` | TEXT | Primary audio stream |
| `audio_channels` | INTEGER | |
| `container_format` | TEXT | e.g. `matroska`, `mp4` |
| `is_already_hevc` | BOOLEAN | Computed at probe time (§10.1) |
| `predicted_savings_bytes` | INTEGER | Computed estimate (§10.2) |
| `last_probed_at` | INTEGER | Unix ts |
| `probe_error` | TEXT | Nullable — set if `ffprobe` failed |
| `status` | TEXT | `active` \| `missing` |

### `transcode_profiles`

Named, reusable encode settings so you can keep e.g. an "archive/high-quality" profile and an "aggressive space-saving" profile and choose per job.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | |
| `name` | TEXT | e.g. "Archive HQ", "Space Saver" |
| `crf` | INTEGER | e.g. 20 / 23 / 26 |
| `preset` | TEXT | e.g. `slow`, `medium` |
| `extra_args` | TEXT | Nullable — escape hatch for advanced flags |
| `is_default` | BOOLEAN | One profile flagged as default |

### `transcode_jobs`

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | |
| `media_file_id` | INTEGER FK | |
| `profile_id` | INTEGER FK | Which profile this job used |
| `status` | TEXT | `queued` \| `running` \| `verifying` \| `completed` \| `failed` \| `cancelled` |
| `queued_at` | INTEGER | |
| `started_at` | INTEGER | Nullable |
| `completed_at` | INTEGER | Nullable |
| `original_size_bytes` | INTEGER | Snapshot at queue time |
| `output_size_bytes` | INTEGER | Nullable until complete |
| `progress_percent` | REAL | Live during encode, for WS push |
| `output_path` | TEXT | Temp path before swap |
| `error_message` | TEXT | Nullable |
| `verification_result` | TEXT | JSON blob (§9.3) |

### `scan_runs`

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | |
| `trigger` | TEXT | `manual` \| `scheduled` \| `watcher` |
| `started_at` / `completed_at` | INTEGER | |
| `files_scanned` / `_added` / `_updated` / `_moved` / `_removed` | INTEGER | |
| `errors` | INTEGER | |

### `settings`

Single row (`id = 1`), seeded empty on first migrate. Holds the auth credential and the session-signing secret (§4.2). Kept deliberately small in v1 — runtime knobs (encode window, concurrency, scan interval) still come from env; only auth lives here because it must be set *after* deploy, not before.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PK | Always `1` (enforced single row) |
| `auth_username` | TEXT | Nullable until first-run setup |
| `auth_password_hash` | TEXT | bcrypt; nullable until setup. **Never plaintext** |
| `session_secret` | TEXT | Random, generated on first boot; signs session cookies |
| `setup_completed_at` | INTEGER | Unix ts; `NULL` = first-run setup pending |

---

## 9. Transcode Workflow

The heart of the safety model. Encoding is manual to trigger, gated by the time window for execution, and **never touches the original until a verified replacement exists**.

### 9.1 Triggering

- From the candidate browser: select one or more files, choose a **profile** (§8), → "Queue for re-encode."
- No bulk auto-queue without an explicit selection step. The selection is always visible and confirmable before queuing, even for large batches.

### 9.2 Encoding

- `ffmpeg` with `libx265` (CPU-only — no NVENC/QSV in v1).
- Stream handling, important to avoid data loss on remux:
  - `-c:v libx265` — re-encode video
  - `-c:a copy` — pass audio through untouched
  - `-c:s copy` — **pass subtitle streams through untouched** (prevents silently dropping embedded subs)
- CRF/preset come from the chosen profile, not hardcoded.
- Output is written to a **temp path** alongside the original (e.g. `original.mkv.reclaim-tmp.mkv`) — never overwriting in place during the encode.
- The worker pulls jobs only inside `ENCODE_WINDOW_START`–`ENCODE_WINDOW_END`. A job queued at 2pm sits `queued` until the window opens.
- Live `progress_percent` (parsed from ffmpeg progress output) pushed over WebSocket so the dashboard updates without polling.

### 9.3 Verification (before touching the original)

Once the encode finishes, before any deletion, the temp output must pass:

1. **Duration match** — within a small tolerance (e.g. ±1s) of the source.
2. **Playability** — `ffprobe` reads the output's streams without error.
3. **Stream-count match** — expected number of video/audio/subtitle streams present (catches truncated/partial outputs where ffmpeg exited early).
4. **Resolution match** — output dimensions match source (catches an accidental scale/misconfiguration).

Result stored as JSON in `verification_result`:
```json
{ "duration_match": true, "duration_delta_seconds": 0.3, "playable": true,
  "stream_count_match": true, "resolution_match": true, "passed": true }
```

If **any** check fails: status → `failed`, the temp output is **kept** (not deleted) for inspection, the original is left untouched, and the failure is surfaced clearly in the UI — not just logged.

### 9.4 Replace (only after verification passes)

1. Move the original to a short-lived `.reclaim-backup` suffix in the same directory.
2. Move the verified temp output into the original filename.
3. Delete the `.reclaim-backup`.
4. Update the `media_files` row (new `size_bytes`, `video_codec = hevc`, new `fingerprint`, etc.) — which also makes it satisfy §10.1 and drop out of future candidate lists automatically.

Steps 1–3 keep a brief window where, if step 2 fails (disk full, permissions), the backup can be restored rather than leaving you with no original. Small but meaningful given principle §2.4.

### 9.5 Cancellation

Queued or running jobs can be cancelled from the UI. Cancelling a running job kills the `ffmpeg` process, deletes the temp output, leaves the original untouched, status → `cancelled`.

### 9.6 Orphan cleanup

A startup sweep (and periodic check) removes stale `.reclaim-tmp` / `.reclaim-backup` files left behind by a container that died mid-job, so a crash can't silently litter the library. Backups are only removed once their corresponding original is confirmed present.

---

## 10. Candidate Logic

### 10.1 "Already HEVC" exclusion

A file is flagged `is_already_hevc = true` when `video_codec == 'hevc'` and is **auto-excluded** from the candidate list (it still appears in stats/browsing, just filtered out of "candidates"). Per the explicit decision, exclusion is on codec match alone, not on how well it was encoded — a later toggle could reconsider poorly-encoded HEVC if it ever matters.

### 10.2 Candidate scoring

Each non-HEVC file gets a `predicted_savings_bytes` estimate for ranking — **for sorting, not as a recommendation**. You decide what to queue.

```
predicted_savings_bytes = size_bytes * (1 - expected_hevc_ratio[source_codec])
```

`expected_hevc_ratio` starts as a rough per-codec lookup (conservative rule-of-thumb values) and is refined over time from your own completed `transcode_jobs` (actual `output_size_bytes / original_size_bytes` per source codec, once there's enough history to beat the seed values). Surfaced in the UI explicitly as an estimate, never a guarantee.

### 10.3 Sort / filter dimensions

Largest file first (biggest absolute win), source codec, resolution (e.g. deprioritize SD), library type, file age (`mtime`), and already-queued/already-completed status (so the same candidates don't keep reappearing).

---

## 11. UI Surface (v1)

- **Library Overview** — totals, codec breakdown (chart), resolution breakdown, and **total estimated recoverable space** if all candidates were re-encoded.
- **Candidate Browser** — virtualized, server-paginated, sortable/filterable table (§10.3) with multi-select, profile pick, and "Queue selected."
- **Dry-run savings view** — pick a set (or a filter) and see projected total space saved *before* committing anything, using the §10.2 estimates. Pure decision support, queues nothing.
- **Job Queue / History** — current queue with position, running job(s) with live progress, completed/failed history with verification results.
- **Settings** — encode window, `PROBE_CONCURRENCY`, scan interval, **transcode profile management** (CRUD), and **account credentials** (change username/password; the change re-bcrypts server-side). (Mount paths are display-only, set via env.)
- **Setup & Login** — a one-time first-run setup page (§4.2) and a login page; a logout action clears the session.

---

## 12. Performance Notes

The "fast page load" goal is met by three things working together: **server-side pagination** (never fetch 20k rows), **table virtualization** (never render more than ~50 DOM rows), and **precomputed library stats** (the overview aggregates are maintained incrementally on scan/encode, not recomputed by scanning the whole table on every dashboard open). Go's zero-warmup startup handles the cold-start half of "loads slowly"; these three handle the steady-state half.

---

## 13. Out of Scope (v1)

Deliberate cuts, not oversights:

- Autonomous/unattended queuing of any kind (see §2.1)
- GPU-accelerated encoding (CPU x265 only)
- ML-based quality scoring
- Sonarr/Radarr/Plex API integration (filesystem-only)
- Plex-aware / stream-aware scheduling (encode window is purely time-based)
- Multi-user accounts / RBAC (single user + a first-run-provisioned session login, §4.2)
- Cross-machine / distributed encoding workers
- Audio/subtitle *transformation* (they're passed through, not transcoded or stripped)

---

## 14. Target Image Size

**100–250 MB** is the goal. Reasonable, but not automatic: a static `ffmpeg` build is the main weight and can be a large fraction of that depending on how it's compiled. Worth measuring rather than assuming — if it runs over, a slimmer ffmpeg build (only the needed encoders/decoders/muxers) is the first lever.

---

## 15. Open Questions for Build Phase

- Exact `ffmpeg` audio behavior beyond `-c:a copy` — any cases where a re-encode of ancient audio codecs is wanted, or always pass through?
- Fingerprint tuning (§6.3): how many KB head/tail balances collision-resistance against probe cost on 20k files.
- Whether to add a `go test -race` CI run scoped to the scanner/worker, since that's the most concurrency-sensitive code even given the weekly-use profile.
- Bundled vs. system ffmpeg: bundling keeps the image self-contained (chosen default) but pins the version — acceptable, just noting the tradeoff.
