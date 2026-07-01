# Reclaim — Build Plan (phased, layered)

> **Status:** v1 shipped. This document is a historical build log from initial
> development. For current product behavior see [`README.md`](../README.md) and
> [`docs/API.md`](API.md).

Companion to `RECLAIM_SPEC.md`. Full v1, in build order. Each phase carries planning
detail **and** paste-ready Claude Code task prompts.

**Audience:** comfortable in Go — so this is dense on sequencing, decisions, and the bugs
that actually bite, light on language mechanics.

## Progress

| Phase | Status | Notes |
|---|---|---|
| **P0** Foundation | ✅ done (1 box deferred) | `docker compose up` end-to-end check deferred to packaging (P9); all other gates pass. |
| **P1** Data layer | ✅ done | Migrations, repos, single-writer pool, auth/session secret. |
| **P2** ffprobe wrapper | ✅ done | Typed probe + fingerprint. |
| **P3** Scanner | ✅ done | Walk, incremental diff, rename detection, fsnotify, scheduled/manual. |
| **P4** Candidates + Stats | ✅ done | Seeded savings model, `Candidates()` query (sort/filter/keyset), incremental `library_stats` + `Recompute()`. |
| **P5** REST API + WS | ✅ done | Auth + data + jobs + settings endpoints, keyset candidate paging, WS hub (cookie-gated on upgrade), live config holder. |
| **P6** Queue + worker | ✅ done | FSM (`internal/jobs`), window-gated single-worker loop, libx265 wrapper + progress, process-group cancel, WS progress (DB-throttled ~1/s). Wired into `main.go` + API cancel. |
| **P7** Safety (verify/swap/orphan) | ✅ done | §9.3 verify (re-probes source), §9.4 atomic backup→swap→row update (drops from candidates + stats), §9.6 startup+hourly orphan sweep & crash reconcile. Real-ffmpeg integration test + `-race` clean. |
| **P8** Frontend | ✅ done | Overview, candidate browser (virtualized keyset), Library view, job queue/history (live WS progress, verification detail, kept-temp path), settings (profile CRUD, credentials). Auth gating + 401 interceptor. |
| **P9** Hardening + packaging | ✅ done | Self-refining savings model (≥10 jobs/codec, clamped [0.30–0.95], labeled `seed`/`learned` in API + UI). `-race` suite green. Drift guard: scheduled rescans now call `Stats.Recompute`. README + Dockerfile (Alpine 3.21, pinned apk ffmpeg) + docker-compose.yml. |
| **P10** Polish + audit trail | ✅ done | Persistent events API (`/api/events`), notification panel, TMDB metadata + Library grouped views, force-encode, job dismiss. |

> **Note:** P4–P10 are complete. Post-v1 features (TMDB metadata, Library grouped
> views, events audit trail, force-encode) landed during P10 polish.

## How to read this

Each phase has:
- **Goal / Depends on** — what it delivers and what must exist first.
- **Sub-tasks** — the work, ordered.
- **Decisions & gotchas** — the value; where data-loss or "database is locked" lives.
- **Acceptance** — a checklist; the phase isn't done until these pass.
- **CC prompts** — discrete units you can paste into Claude Code one at a time.

**Demoable checkpoints** (even though we're going top-to-bottom):
- After **P4** → you can scan + analyze + browse + rank candidates. No encoding. Genuinely useful already.
- After **P7** → the full encode→verify→swap pipeline is safe and crash-proof. The risky half is done.
- After **P8** → it's a pleasant app, not curl commands.

## Resolved §15 / open defaults (override in the doc, not in your head)

- **Audio:** always `-c:a copy` in v1. Per-profile audio re-encode is a noted future flag, not built.
- **Subtitles:** always `-c:s copy` (never dropped). Non-negotiable per §9.2.
- **Fingerprint:** `hash(size_bytes ‖ first 64 KB ‖ last 64 KB)`, sha256, hex. Tunable via one const.
- **ffmpeg/ffprobe:** bundled static, **version-pinned**, versions logged at startup (§5.2).
- **`-race` CI:** yes, scoped to `scanner`, `worker`, `jobs` packages (P9).
- **SQLite concurrency:** WAL + `busy_timeout`, **single serialized writer**, read pool separate (P1 — see gotcha).
- **Window edge:** a job already *running* when the encode window closes **finishes**; the worker only stops *pulling new* jobs. The window gates pulls, not in-flight encodes.
- **Auth:** no pre-hashed env var. Credentials are set on **first-run setup**, bcrypt-stored in the `settings` table; the wire mechanism is a **login page + signed HTTP-only session cookie** (matches *arr Forms auth), not HTTP Basic. See spec §4.2.
- **Host assumption:** amd64 Linux alongside your *arr stack. If arm64, the static ffmpeg build in P0/P9 changes.

## Dependency spine

```
P0 Foundation ─┬─ P1 Data layer ─ P2 ffprobe ─ P3 Scanner ─ P4 Candidates+Stats ─ P5 API/WS ─┐
               │                                                                              ├─ P8 Frontend ─ P9 Hardening/Packaging
               └────────────────────────────── P6 Queue+Worker ── P7 Safety (verify/swap/orphan) ─┘
```
P6/P7 depend on P1 (jobs tables) and P2 (ffmpeg wrapper shares the subprocess plumbing), not on P3/P4. You can build the encode pipeline in parallel with the read side once the data layer exists — but the plan keeps them in sequence as requested.

---

## P0 — Foundation & runnable container

**Goal:** an image that boots, fails fast on a missing dependency, serves a placeholder page, and runs migrations, with the auth-gate middleware in place (concrete login wired in P5). Nothing domain-specific yet — just the skeleton everything hangs off.

**Depends on:** nothing.

**Sub-tasks**
1. `go mod init`, repo layout per spec §7 (`/cmd/reclaim`, `/internal/{api,scanner,worker,jobs,db,media,ffprobe,ffmpeg}`, `/web`).
2. Config: load all §4.1 env vars into a typed `Config` struct; validate at startup (parse `ENCODE_WINDOW_*` as `HH:MM`, durations via `time.ParseDuration`, ints, non-empty paths). Fail fast on anything malformed.
3. Startup checks (§5), in order, each fatal with a clear message: (a) `ffmpeg`/`ffprobe` present + executable, (b) log their `-version` output, (c) DB path writable + migrate, (d) each media mount exists + readable.
4. `chi` router; structured logging (`slog`); `GET /healthz`.
5. **Auth middleware + setup gate**, built against an injected `AuthStore` interface (`IsSetupComplete()`, `ValidateLogin(user, pass) bool`, `SessionSecret() []byte`) so it's testable before the DB exists. Behavior: setup incomplete → allow only the setup route + SPA shell, redirect everything else to `/setup`; setup complete → require a valid **signed, HTTP-only session cookie**, else 401 for `/api/*` and redirect to `/login` for the SPA. Honor `DISABLE_AUTH`. Wrap everything except `/healthz`, `/login`, `/setup`. The concrete settings-backed `AuthStore` + session-secret persistence land in **P1**; the `/api/setup`, `/api/login`, `/api/logout` endpoints land in **P5** — here, build the middleware against a fake store and unit-test the gate.
6. `embed.FS` for `/web/out`; serve a committed placeholder `index.html` so the embed path is real from day one.
7. Multi-stage `Dockerfile` (Go build → final image with bundled ffmpeg) + `docker-compose.yml` per §4.1. Image-size optimization deferred to P9 — just get it running now.

**Decisions & gotchas**
- Keep config parsing in one place; pass `Config` by value into constructors. No global state, makes the worker/scanner testable.
- Bundle ffmpeg now even if fat — proving the startup version check and the bundled-binary path early de-risks P9.
- `embed.FS` of a not-yet-built frontend: commit a static placeholder so `go build` never breaks on a missing `/web/out`.
- Auth wire details to settle now so P5/P8 inherit them: cookie is `HttpOnly`, `SameSite=Lax`, `Secure` only when serving HTTPS (LAN HTTP omits it — documented in §4.2); login compare is constant-time `bcrypt.CompareHashAndPassword`; setup mode must be un-skippable (no protected route reachable until `setup_completed_at` is set).

**Acceptance**
- [ ] `docker compose up` → container serves the placeholder page.
- [x] Auth-gate unit tests (fake store): setup-incomplete → protected route redirects to `/setup`; setup-complete + valid cookie → 200; missing/invalid cookie → 401 on `/api/*`, redirect to `/login` on the SPA; `DISABLE_AUTH=true` → always pass.
- [x] Remove ffmpeg from the image → startup exits non-zero with a named error (not a mid-run panic).
- [x] Bad `ENCODE_WINDOW_END` → fatal at boot with a clear message.
- [x] Migrations run idempotently on second boot.

**CC prompts**
- *"Scaffold a Go module `reclaim` with the package layout from RECLAIM_SPEC.md §7. Add a typed `Config` in `/internal/config` loaded from the env vars in §4.1, with validation: HH:MM windows, `time.Duration` for SCAN_INTERVAL, ints for concurrency, non-empty mount/DB paths. Return a descriptive error per invalid field."*
- *"In `/cmd/reclaim`, implement ordered fatal startup checks per §5: ffmpeg/ffprobe executable, log their versions, DB writable+migrate hook (stub), media mounts readable. Each failure logs and exits non-zero."*
- *"Add a chi server with slog structured logging and a `/healthz` endpoint. Implement an auth middleware over an `AuthStore` interface (IsSetupComplete, ValidateLogin, SessionSecret): setup-incomplete redirects all but /setup to /setup; setup-complete requires a valid signed HttpOnly SameSite session cookie (401 for /api/*, redirect to /login for the SPA); DISABLE_AUTH bypasses. Exclude /healthz, /login, /setup. Unit-test the gate against a fake store."*
- *"Add an embed.FS-backed static file server mounted at `/` serving `/web/out`, with a committed placeholder index.html. Write a multi-stage Dockerfile bundling pinned ffmpeg/ffprobe and a docker-compose.yml matching §4.1."*

---

## P1 — Data layer (SQLite, the careful part)

**Goal:** schema, migrations, and a query layer that survives concurrent scan + encode without `database is locked`.

**Depends on:** P0.

**Sub-tasks**
1. `modernc.org/sqlite`. On open: `PRAGMA journal_mode=WAL`, `PRAGMA busy_timeout=5000`, `PRAGMA foreign_keys=ON`, `PRAGMA synchronous=NORMAL`.
2. **Single-writer architecture.** One dedicated `*sql.DB` (or single conn) for writes with `SetMaxOpenConns(1)`, funnel all writes through it; a separate read `*sql.DB` pool for queries. Or: one writer goroutine consuming a command channel. Pick the connection-pool split — simpler, and WAL gives readers-don't-block-writer.
3. Embedded, versioned migrations (`embed.FS` of `*.sql`, a `schema_migrations` table, apply-in-order). All five tables from §8: `media_files`, `transcode_profiles`, `transcode_jobs`, `scan_runs`, `settings` (single-row, `id=1`).
4. Indexes that earn their keep: `media_files(path)` unique; partial/filtered index for the candidate query (`status`, `is_already_hevc`); index on `(predicted_savings_bytes DESC, id)` for the default sort's keyset pagination; `media_files(fingerprint)` for rename lookups; `transcode_jobs(status)`, `transcode_jobs(media_file_id)`.
5. Repository layer per aggregate (`MediaRepo`, `JobRepo`, `ProfileRepo`, `ScanRepo`, `SettingsRepo`) — typed methods, no raw SQL leaking into handlers.
6. Seed a default `transcode_profiles` row on first migrate (e.g. "Space Saver", crf 26, preset medium, `is_default=true`), and seed the empty `settings` row (`id=1`, nulls).
7. **`AuthStore` implementation + session secret (the concrete half of P0's gate).** `SettingsRepo` satisfies the P0 `AuthStore` interface: `IsSetupComplete` reads `setup_completed_at`; `ValidateLogin` does the bcrypt compare; `SessionSecret` returns the stored secret, **generating + persisting 32 random bytes on first boot if null**. Also implement `CompleteSetup(user, plaintext)` (bcrypt + store + stamp) and `ChangeCredentials(...)` for P5. Honor `RESET_AUTH` (null the three auth columns on boot) as a startup step.

**Decisions & gotchas**
- **This is the phase most likely to cause heisenbugs later.** `modernc` is pure-Go and concurrency-safe *per connection*, but SQLite still allows one writer. Without the single-writer funnel + WAL, the scanner updating rows while the worker flips a job to `running` will throw `SQLITE_BUSY`. Build it right here, not after you hit it.
- WAL files (`-wal`, `-shm`) live next to `reclaim.db` on the mounted volume — fine, just don't be surprised.
- Store all timestamps as Unix int (matches §8); `verification_result` as a TEXT JSON blob.
- Generate the `session_secret` **once** and persist it — regenerating on each boot would silently invalidate every existing session cookie on restart.

**Acceptance**
- [x] Migrations apply from empty and are idempotent.
- [x] Concurrency smoke test: N goroutines reading + 1 writer hammering for a few seconds → zero `database is locked`.
- [x] Default profile present after first boot; second boot doesn't duplicate it.
- [x] Empty `settings` row exists; `session_secret` is generated on first boot and **stable across restarts**; `RESET_AUTH=true` clears the auth columns.

**CC prompts**
- *"Implement `/internal/db`: open modernc.org/sqlite with WAL, busy_timeout=5000, foreign_keys on. Provide a write handle with MaxOpenConns(1) and a separate read pool. Add an embedded versioned migration runner with a schema_migrations table."*
- *"Write migration 0001 creating media_files, transcode_profiles, transcode_jobs, scan_runs, and a single-row settings table exactly per RECLAIM_SPEC.md §8, plus the indexes listed in RECLAIM_PLAN.md P1 sub-task 4. Seed one default transcode profile and the empty settings row."*
- *"Implement typed repositories (MediaRepo, JobRepo, ProfileRepo, ScanRepo, SettingsRepo) over the read/write handles. Add a test that spins up N reader goroutines + 1 writer for 3s against a temp DB and asserts no SQLITE_BUSY errors."*
- *"Make SettingsRepo satisfy the P0 AuthStore interface: IsSetupComplete, ValidateLogin (bcrypt compare), SessionSecret (generate+persist 32 random bytes if null), plus CompleteSetup(user, plaintext) and ChangeCredentials. Add a RESET_AUTH boot step that nulls the auth columns. Test secret stability across reopen and the setup/login round-trip."*

---

## P2 — ffprobe wrapper & probe model

**Goal:** turn a file path into a typed probe result mapped to `media_files`, robustly.

**Depends on:** P0 (subprocess plumbing), P1 (target schema).

**Sub-tasks**
1. `/internal/ffprobe`: wrap `ffprobe -v quiet -print_format json -show_format -show_streams <path>`. Structs for format + streams; `Probe(ctx, path) (*ProbeResult, error)`.
2. Map: `video_codec`, `video_codec_profile` (nullable), `width`, `height`, `duration_seconds`, `bitrate_kbps`, primary `audio_codec` + `audio_channels`, `container_format`. Pick the first video stream as canonical; primary audio = first audio stream.
3. Compute `is_already_hevc` (`hevc`/`h265`).
4. `/internal/media` fingerprint helper: sha256 of `size ‖ first 64KB ‖ last 64KB`. Use one `os.File` with two `ReadAt`s; handle files smaller than 128KB (hash whole file).
5. **Typed `ProbeError`** on non-zero exit or unparseable output — never panic. Caller stores it in `probe_error` and moves on.

**Decisions & gotchas**
- Duration sometimes lives on `format`, sometimes only on the stream — read `format.duration`, fall back to video stream. Same for bitrate (`format.bit_rate` vs stream).
- Some files have no audio (extras, samples) — nullable, don't assume `streams[1]`.
- Always pass a context with timeout; a pathological file shouldn't hang a scan worker forever.

**Acceptance**
- [x] Fixture set (h264/mp4, hevc/mkv, mpeg2, no-audio file) probes to correct values.
- [x] Deliberately corrupt file → `ProbeError`, no panic, scan continues.
- [x] Fingerprint stable across a `mv` of the same bytes; differs when content changes.

**CC prompts**
- *"Implement `/internal/ffprobe` per RECLAIM_PLAN.md P2: typed wrapper around `ffprobe -v quiet -print_format json -show_format -show_streams`, struct mapping to the media_files probe columns, first-video/first-audio selection, duration/bitrate format-then-stream fallback, context timeout, and a typed ProbeError on failure. Table-driven test over committed fixture JSON."*
- *"In `/internal/media`, add Fingerprint(path) computing sha256 of size ‖ first 64KB ‖ last 64KB via ReadAt, hashing the whole file when <128KB. Unit-test stability across a rename and sensitivity to content change."*

---

## P3 — Scanner & indexing

**Goal:** keep the DB in sync with disk — incrementally, concurrently, and treating *arr renames as moves not churn.

**Depends on:** P1, P2.

**Sub-tasks**
1. Walk mounted roots, filter to media extensions (`.mkv .mp4 .avi .m4v .ts .wmv ...`). Tag `library_type` from which root matched.
2. **Incremental diff (§6.1):** per file compare `(size, mtime)` to stored row → skip / insert(probe) / update(re-probe). Collect a set of seen paths for the vanished-path pass.
3. **Concurrency:** a single **scanner-wide** buffered-channel semaphore sized to `PROBE_CONCURRENCY` — the walk, the fsnotify watcher, and scheduled/manual/overlapping scans all acquire the *same* semaphore before probing; fan out probes, fan in writes through the single writer. Never more than `PROBE_CONCURRENCY` concurrent ffprobe across overlapping scan + watcher activity.
4. **Rename detection (§6.3):** after the walk, for paths in DB not seen on disk → look up by fingerprint among newly-inserted paths; match → record as **move** (update path, keep probe data + job history); no match → `status = missing` (soft delete).
5. `scan_runs` bookkeeping: trigger, timestamps, counts (scanned/added/updated/moved/removed/errors).
6. **Force full rescan:** bypass the diff, re-probe everything (post ffprobe upgrade / detection fix).
7. **fsnotify watcher (§6.2):** create/modify → **30s debounce** (coalesce per-path) → incremental probe; delete/move → feed the rename-detection path. Log the network-mount caveat at startup if mounts look like NFS/SMB.
8. **Scheduled rescan (§6.4):** ticker on `SCAN_INTERVAL`, diff-based, independent of the encode window.
9. **Manual rescan (§6.5):** same diff logic, callable (wired to API in P5).

**Decisions & gotchas**
- Order matters: do inserts/updates *before* the vanished-path pass so a moved file's new path is already in the DB to fingerprint-match against.
- Debounce is per-path with a reset timer — Sonarr writes then renames; you want one probe after it settles, not three mid-write.
- The watcher and the scheduled scan can race on the same file. The single writer + "compare (size,mtime) before re-probe" makes a double-trigger idempotent — lean on that rather than locking paths.
- Don't recursively re-add watches naively on huge trees; add roots and rely on fsnotify recursion behavior of your platform, or walk-and-add once. Note the inode/watch limits on Linux for 20k+ dirs.

**Acceptance**
- [x] Second scan of an unchanged tree probes ~0 files.
- [x] `mv` a file → next scan records a **move**, job history preserved, no delete+readd.
- [x] Delete a file → `status=missing`, history intact.
- [x] Drop a new file in → watcher probes it ~30s later without a manual scan.
- [x] `scan_runs` counts match what actually happened.

**CC prompts**
- *"Implement `/internal/scanner` walk + incremental diff per §6.1 with a PROBE_CONCURRENCY buffered-channel semaphore, fanning probe writes through the single DB writer. Record a scan_runs row with full counts."*
- *"Add rename detection per §6.3: after the walk, match vanished DB paths to new paths by fingerprint → update as a move (keep probe + job history); unmatched → status=missing. Order inserts before the vanished pass. Test: rename, delete, and new-file scenarios."*
- *"Add an fsnotify watcher per §6.2 with a per-path 30s debounce queuing incremental probes that acquire the shared scanner-wide PROBE_CONCURRENCY semaphore (same as the walk), routing deletes/moves into rename detection, plus a SCAN_INTERVAL scheduled diff-rescan and a manual rescan entrypoint. Log a network-mount warning when mounts are NFS/SMB."*

---

## P4 — Candidate logic & stats aggregation

**Goal:** rank what's worth re-encoding, and make the overview load instantly regardless of library size.

**Depends on:** P1–P3.

**Sub-tasks**
1. **Seed savings model (§10.2):** `expected_hevc_ratio[source_codec]` lookup (conservative rule-of-thumb constants). `predicted_savings_bytes = size * (1 - ratio)`. Computed at probe time and stored. *Self-refinement from job history is P9 — seed values ship.*
2. **Candidate query:** exclude `is_already_hevc`, exclude `status=missing`, exclude files with an active/completed job (so they don't reappear). Support sort/filter dims (§10.3): size desc (default), source codec, resolution band (deprioritize SD), library_type, mtime, queued/completed status.
3. **Incremental stats (§12):** a `library_stats` aggregates table (or in-memory cache rebuilt on boot) holding totals, per-codec counts/bytes, per-resolution counts/bytes, total predicted recoverable bytes. Maintain it transactionally on insert/update/move/remove **and** on encode completion — never `SELECT … GROUP BY` the whole 20k table on dashboard open. Provide a "recompute from scratch" path as the source of truth / repair tool.

**Decisions & gotchas**
- Keep the savings ratio table in one place with a comment that P9 will start overriding entries from observed `output/original` ratios once history is sufficient. Make the seed-vs-learned source explicit in the value so the UI can label "estimate".
- "Already queued/completed excluded" needs care with the move logic: a moved file keeps its job history, so it correctly stays excluded. A re-added (no-match) file is a fresh row — correct to re-include.
- Incremental stats are easy to drift. Guard them: a cheap nightly assertion (or the force-rescan) recomputes and logs a warning on mismatch.

**Acceptance**
- [x] Candidate list ranks largest predicted savings first; HEVC + missing + queued excluded.
- [x] Stats endpoint result equals a brute-force `GROUP BY` recompute.
- [x] Completing an encode updates stats without a full recompute.

**CC prompts**
- *"Implement candidate scoring in `/internal/media`: a seeded expected_hevc_ratio lookup, predicted_savings_bytes computed at probe time, and a CandidateQuery supporting the sort/filter dimensions in §10.3 with exclusions for is_already_hevc, missing, and files having an active/completed job."*
- *"Implement incremental library stats per §12: a library_stats table updated transactionally on every media_files change and on encode completion, plus a Recompute() that rebuilds from scratch. Test that incremental == recompute after a mixed sequence of inserts/updates/removes."*

---

## P5 — REST API + WebSocket

**Goal:** expose everything the frontend needs, paginated and live.

**Depends on:** P1–P4 (read side); P6/P7 fill in job endpoints' behavior, but the routes exist here.

**Sub-tasks**
1. Handlers (all behind the session gate from P0, except the auth endpoints below):
   - `POST /api/setup` — first-run only: username + plaintext password → `CompleteSetup` (bcrypt + store + stamp). 409 if setup already complete.
   - `POST /api/login` — username + password → bcrypt compare → set signed session cookie. Optional light rate-limit.
   - `POST /api/logout` — clear the session cookie.
   - `GET /api/session` — whoami / is-setup-complete, so the SPA can route to setup vs login vs app on load.
   - `GET /api/stats` — overview aggregates.
   - `GET /api/candidates` — **server-side pagination** + sort + filter. Default sort (savings desc) uses **keyset pagination** on `(predicted_savings_bytes, id)`; offset+limit acceptable for the other sorts. Page size 50–100.
   - `GET /api/files/:id`.
   - `POST /api/scan`, `POST /api/scan/full`.
   - `GET/POST/PUT/DELETE /api/profiles` (CRUD §11).
   - `POST /api/jobs` — body = file IDs + profile ID → enqueue (§9.1). **Echo the resolved selection in the response** so the UI's confirm step is honest.
   - `GET /api/jobs` — queue + history, filter by status, with position for queued.
   - `POST /api/jobs/:id/cancel` (§9.5).
   - `GET /api/dry-run` — projected total savings for a set or filter, queues nothing (§11).
   - `GET/PUT /api/settings` — encode window, probe concurrency, scan interval. **Credential change is its own endpoint** (`PUT /api/settings/credentials` → `ChangeCredentials`, re-bcrypts; never returns the hash). Mount paths read-only.
2. `GET /api/ws` (gorilla/websocket): broadcast job progress + scan progress. Hub pattern (register/unregister/broadcast), JSON envelopes typed by `event`. **Authenticated via the same session cookie** (validate on upgrade; reject unauthenticated upgrades).

**Decisions & gotchas**
- Keyset beats offset at 20k rows for the default view (no deep-offset scan), and it pairs with the virtualized infinite-scroll table in P8. Worth the small extra contract complexity (`?after_savings=&after_id=`).
- WS is push-only for progress; keep all *commands* on REST. Simpler reconnection story.
- Settings that change runtime behavior (probe concurrency, window) should apply without a restart — have the worker/scanner read them from a live config holder, not a snapshot taken at boot. **Credential changes also take effect immediately** (next login validates against the new hash) — no restart, unlike the old env-var approach.
- The WS upgrade must check the session cookie itself — middleware that only guards `/api/*` JSON routes can miss the upgrade handshake. Validate in the handler before upgrading.

**Acceptance**
- [x] First boot: `POST /api/setup` succeeds once, then returns 409; `POST /api/login` issues a working session cookie; `/api/logout` invalidates it.
- [x] Every `/api/*` (and the WS upgrade) returns 401 without a valid cookie; `DISABLE_AUTH=true` lets them through.
- [x] Credential change via `PUT /api/settings/credentials` takes effect on the next login with no restart.
- [x] `/api/candidates` keyset paging walks the full list without dupes/gaps. (20k-row perf rests on the P1 index + keyset query; verified correctness in tests.)
- [x] `POST /api/jobs` returns the exact set it queued (echoes resolved selection + skipped, with reasons).
- [x] WS client receives a tick during a scan (`scan_started`/`scan_completed`) and on job enqueue (`jobs_queued`); cookie validated on upgrade. (Live encode-progress ticks arrive with the P6 worker.)

**CC prompts**
- *"Implement the auth endpoints in `/internal/api`: POST /api/setup (first-run only, 409 if done), POST /api/login (bcrypt compare → signed HttpOnly SameSite session cookie, light rate-limit), POST /api/logout, GET /api/session (setup-state + whoami), and PUT /api/settings/credentials (ChangeCredentials, never returns the hash). Use the P1 SettingsRepo/AuthStore."*
- *"Implement the data handlers for stats, candidates (keyset pagination on (predicted_savings_bytes,id) for default sort, offset for others, page size 50–100), file detail, scan/full-scan, profile CRUD, jobs create/list/cancel, dry-run, and settings get/put per RECLAIM_SPEC.md §11. POST /api/jobs echoes the resolved selection. All behind the session gate."*
- *"Add a gorilla/websocket hub at /api/ws broadcasting typed JSON events for job + scan progress, validating the session cookie on upgrade and rejecting unauthenticated handshakes. Command actions stay on REST."*
- *"Wire runtime-mutable settings (probe concurrency, encode window, scan interval) through a live config holder the scanner and worker read on each use, so PUT /api/settings takes effect without a restart."*

---

## P6 — Job queue & worker (encode execution)

**Goal:** pull queued jobs *only* inside the window, run ffmpeg to a temp file, stream progress, cancel cleanly. **No verification or swapping yet — that's P7.**

**Depends on:** P1 (jobs tables), P2 (subprocess plumbing), P5 (job endpoints).

**Sub-tasks**
1. `/internal/jobs`: state machine `queued → running → verifying → completed/failed/cancelled`. Transitions go through the single writer; reject illegal transitions.
2. `/internal/worker`: a single worker loop. Sleep/wait outside `ENCODE_WINDOW_START–END`; inside the window, pull the oldest `queued` job and run it. (Window check on each pull; a running job is never interrupted by the window closing — see §9.2 default.)
3. `/internal/ffmpeg`: typed encode wrapper. Command: `-c:v libx265` with CRF/preset from the job's profile, `-c:a copy`, `-c:s copy`, then `extra_args`. Output to `<original>.reclaim-tmp.<ext>` in the same directory.
4. **Progress:** run ffmpeg with `-progress pipe:1 -nostats`; parse `out_time_ms`/`out_time` vs known `duration_seconds` → `progress_percent`; persist + push over WS.
5. **Cancellation (§9.5):** context-kill the ffmpeg process group, delete the temp output, leave the original untouched, status → `cancelled`. Works for both queued (just drop it) and running (kill).

**Decisions & gotchas**
- Use a process **group** kill (`Setpgid` + negative PID, or `CommandContext` carefully) — ffmpeg can spawn children; a bare `Process.Kill()` can orphan them.
- Compute percent from the profile's *known source duration*, not by trusting ffmpeg's own ETA. You already have `duration_seconds` from the probe.
- One worker in v1 (spec is single-host, CPU x265). Structure the loop so bumping to N workers later is a config change, but don't build the pool now.
- Persisting `progress_percent` every parse tick hammers the writer — throttle DB writes to ~1/sec; push every tick to WS, write to DB sparsely.

**Acceptance**
- [x] Job queued outside the window stays `queued`; opens-window → runs. (`withinWindow` gates pulls; `TestWithinWindow`.)
- [x] Progress streams over WS during an encode; temp file appears, original untouched. (`job_progress` per tick, DB throttled; `TestProcessJobHappyPath`.)
- [x] Cancel a running job → ffmpeg (and children) dead (process-group kill), temp gone, original intact, status `cancelled`. (`TestCancelRunningJob`.)
- [x] Window closes mid-encode → running job finishes; no new job pulled. (Window checked only on pull; in-flight encode uses its own context.)

**CC prompts**
- *"Implement `/internal/jobs` state machine (queued→running→verifying→completed/failed/cancelled) with illegal-transition rejection, all writes via the single writer."*
- *"Implement `/internal/ffmpeg` encode wrapper: build `-c:v libx265 -crf <profile> -preset <profile> -c:a copy -c:s copy <extra_args>` to `<original>.reclaim-tmp.<ext>`, run with `-progress pipe:1`, parse out_time against known duration into a percent, and support context cancellation via process-group kill."*
- *"Implement `/internal/worker`: single loop that only pulls queued jobs inside ENCODE_WINDOW_START–END, runs them via the ffmpeg wrapper, pushes progress to the WS hub (DB-throttled to ~1/sec), and on cancel deletes the temp + leaves the original. Running jobs are not interrupted when the window closes."*

---

## P7 — Safety: verification, atomic replace, orphan cleanup (the heart)

**Goal:** never lose a source file. Verify before touching the original; swap atomically; survive a crash mid-job. This is where the spec's whole value lives — build it test-first.

**Depends on:** P6.

**Sub-tasks**
1. **Verification (§9.3)** on `verifying`, against the temp output, all four checks:
   - Duration match within ±1s of source.
   - Playability: ffprobe reads the output's streams without error.
   - Stream-count match (video/audio/subtitle counts vs source) — catches truncated output.
   - Resolution match (output WxH == source) — catches accidental scaling.
   Store the §9.3 JSON in `verification_result`. **Any** failure → status `failed`, **keep** the temp, original untouched, surface in the UI (not just logs).
2. **Replace (§9.4)**, only on pass: (a) move original → `<original>.reclaim-backup`; (b) move temp → original filename; (c) delete the backup; (d) update the `media_files` row (size, `video_codec=hevc`, new fingerprint, recompute `is_already_hevc` + savings, refresh stats). Both moves are same-directory renames → atomic on one filesystem.
3. **Orphan cleanup (§9.6):** a startup sweep + periodic check. Remove stale `.reclaim-tmp`. For each `.reclaim-backup`: if the original is present → delete the backup; if the original is **missing** → restore the backup (a crash between step a and b). Reconcile jobs stuck in `running`/`verifying` after a crash → mark `failed`, clean their temp.

**Decisions & gotchas**
- **This is the only place data loss can happen.** Treat every step as "what if the process dies right here?" — the backup-suffix window in §9.4 exists precisely so step-b failure (disk full, perms) is recoverable. Don't optimize it away.
- The replace's row update is what makes the file *drop out of candidates* (it's now HEVC) — verify that side effect, it closes the loop with §10.1.
- Stream-count match must compare *counts by type*, not total — a remux that merges/splits nothing should preserve each.
- Guard against same-name collisions: if a `.reclaim-tmp`/`.reclaim-backup` already exists from a prior run for that path, the orphan sweep must run (and win) *before* a new job for that file starts.

**Acceptance**
- [x] Forced verification failure (truncate the temp) → status `failed`, temp kept, original untouched, surfaced via `job_failed` + `verification_result`. (`TestProcessJobVerificationFailureKeepsTemp`.)
- [x] Happy path → atomic swap, row updated to HEVC, file drops from candidate list, stats reflect the saved bytes. (`ReplaceWithEncoded`; `TestReplaceWithEncodedUpdatesStatsAndDropsCandidate` + real-ffmpeg `TestEncodeVerifyReplaceReal`.)
- [x] Kill the container mid-encode → on restart, orphan `.reclaim-tmp` removed, job reconciled to `failed`, library intact. (`reconcileInterrupted` + `sweepOrphans`; `TestReconcileInterrupted`.)
- [x] Kill between backup-move and temp-move → on restart, backup restored, original present. (`sweepOrphans` restores backup iff original missing; `TestSweepOrphans`.)

**CC prompts**
- *"Implement verification in `/internal/worker` per §9.3: duration ±1s, ffprobe playability, per-type stream-count match, resolution match; persist the §9.3 JSON to verification_result; any failure → status failed, keep temp, original untouched. Tests for each failing check using crafted/truncated outputs."*
- *"Implement the §9.4 replace: original→.reclaim-backup, temp→original, delete backup, then update the media_files row (size, video_codec=hevc, new fingerprint, recomputed is_already_hevc/savings) and refresh stats. Assert the file leaves the candidate query afterward."*
- *"Implement §9.6 orphan cleanup as a startup sweep + periodic check: delete stale .reclaim-tmp; for each .reclaim-backup restore it iff the original is missing else delete it; reconcile running/verifying jobs to failed and clean their temp. Add a crash-simulation test that interrupts between each replace step and asserts the library is always recoverable."*

---

## P8 — Frontend (Next.js static export)

**Goal:** the five surfaces from §11, fast at 20k files. *When implementing UI, follow the `frontend-design` skill's tokens/conventions for this environment.*

**Depends on:** P5 (API/WS). (Plumb the embed from P0.)

**Sub-tasks**
1. `output: 'export'`, all client components, TanStack Query for fetch/cache, Tailwind, `@tanstack/react-virtual`. Build output → `/web/out`, embedded by Go (P0).
2. **Setup + Login pages + app gating.** On load, call `GET /api/session`: setup incomplete → **Setup page** (create username/password, one time); not logged in → **Login page**; else the app. A 401 from any call drops the user back to Login. A logout control clears the session. (These satisfy spec §4.2 / §11 "Setup & Login".)
3. **Library Overview** — totals, codec breakdown chart, resolution breakdown, total estimated recoverable space.
4. **Candidate Browser** — virtualized + server-paginated (keyset infinite scroll) sortable/filterable table (§10.3), multi-select, profile picker, and **"Queue selected"** with a **visible confirm step showing the resolved selection** before anything is queued (§2.1, §9.1) — even for big multi-selects.
5. **Dry-run savings view** — pick a set or filter → projected total saved, queues nothing.
6. **Job Queue / History** — current queue with position, running job(s) with live WS progress, completed/failed history with verification results (and the kept-temp note on failures).
7. **Settings** — encode window, probe concurrency, scan interval, profile CRUD, and a **change-credentials** form (calls `PUT /api/settings/credentials`); mount paths display-only.
8. WS client: subscribe for live job + scan progress; reconcile into TanStack Query caches. The cookie rides the upgrade automatically (same origin) — no token plumbing needed.

**Decisions & gotchas**
- Virtualization + keyset infinite scroll must agree: the table holds ~30–50 DOM rows and asks for the next keyset page at the scroll threshold. This pairing is the whole "feels fast" story (§12) — don't fall back to rendering full pages.
- The confirm-before-queue step is a *safety principle*, not UX polish — it's the human-in-the-loop gate from §2.1/§2.2. Make the selection count + sample explicit.
- Surface verification failures prominently (§9.3 says "not just logged") — a failed job with "temp kept for inspection" needs to be obvious, with the path.

**Acceptance**
- [x] First visit with no credentials → Setup page; after setup → Login; after login → app. A forced 401 returns the user to Login.
- [x] Overview and candidate browser first paint stays flat as the library grows (virtualized, one page fetched).
- [x] Queueing always shows the resolved selection and a confirm before POST.
- [x] Running job progress updates live without polling; failed jobs show verification detail + kept-temp path.

**CC prompts**
- *"Set up the Next.js app in /web with output:'export', client-only components, TanStack Query, Tailwind, and @tanstack/react-virtual, building to /web/out for Go embedding. Add an API client wrapping the §11 endpoints + a WS hook, with a 401 interceptor that routes to the Login page."*
- *"Build the auth flow: a GET /api/session bootstrap that routes to a Setup page (first run), a Login page, or the app; plus a logout control and a change-credentials form in Settings (PUT /api/settings/credentials)."*
- *"Build the Candidate Browser: virtualized table over keyset-paginated /api/candidates with the §10.3 sort/filter controls, multi-select, profile picker, and a Queue-selected flow that shows the resolved selection and requires explicit confirmation before POST /api/jobs (§2.1)."*
- *"Build Library Overview (codec/resolution charts + total recoverable), the Dry-run savings view (queues nothing), the Job Queue/History view (live WS progress, verification results, kept-temp note on failures), and Settings (window, concurrency, scan interval, profile CRUD, read-only mount paths)."*

---

## P9 — Self-refinement, hardening, packaging

**Goal:** the deferred refinements, the concurrency safety net, and hitting the image-size target.

**Depends on:** everything.

**Sub-tasks**
1. **Self-refining savings (§10.2):** once a source codec has enough completed jobs, override its seed `expected_hevc_ratio` with the observed mean `output_size/original_size`. Keep it labeled as a learned estimate in the API so the UI can say so.
2. **`-race` CI (§15):** `go test -race ./internal/scanner/... ./internal/worker/... ./internal/jobs/...` in CI — the concurrency-sensitive packages.
3. **Image size:** Alpine 3.21 runtime with pinned apk ffmpeg (`6.1.2-r1`). Three-stage build: Next.js static export → Go binary (embedded frontend) → Alpine runtime.
4. **Ops pass:** README with the *throughput reality* — CPU x265 `slow`/`medium` on 20k files is enormous wall-clock; the encode window paces it but plan for months, and that's fine for a weekly-touched tool. Note the bundled-ffmpeg version pin, the network-mount inotify caveat, and the WAL files on the volume.
5. **Drift guard:** wire the stats recompute-assertion from P4 into the scheduled rescan so incremental stats can't silently drift.

**Decisions & gotchas**
- Don't let the learned ratio swing on tiny samples — require a minimum N per codec before overriding the seed, and clamp to a sane range.
- Measure the image before optimizing — a slim ffmpeg is real work; only spend it if you're over 250 MB.

**Acceptance**
- [x] After enough jobs, a codec's predicted ratio reflects your actual results, labeled as learned.
- [x] `-race` suite is green (`make test-race`).
- [x] Final image uses Alpine 3.21 with pinned apk ffmpeg — see [`Dockerfile`](../Dockerfile).
- [x] README covers throughput expectations + the operational caveats.

**CC prompts**
- *"Add self-refining expected_hevc_ratio per §10.2: compute observed output/original means per source codec from completed transcode_jobs, override seeds only above a minimum sample size, clamp to a sane range, and expose seed-vs-learned in the API."*
- *"Add a CI workflow running `go test -race` scoped to internal/scanner, internal/worker, internal/jobs."*
- *"Produce a slim multi-stage Dockerfile: build a static ffmpeg with only the encoders/decoders/muxers Reclaim needs (x265 + the source demuxers + matroska/mp4 muxers + audio/subtitle copy), strip the Go binary, distroless/scratch final stage. Report the final image size against the 100–250 MB target."*
- *"Write the README: deployment, the env vars, the first-run setup flow (§4.2) and DISABLE_AUTH/RESET_AUTH escape hatches, throughput expectations for CPU x265 across a 20k library, and operational caveats (version-pinned ffmpeg, NFS/SMB inotify limits, WAL files on the volume)."*

---

## P10 — Polish & audit trail

**Status:** ✅ done.

**Goal:** replace ephemeral toasts with a durable event log, and tighten any rough edges that surface during real use. The app should feel finished — nothing "beta" about the UX, and users have a real record of what happened and when.

**Depends on:** P8, P9.

**Sub-tasks**
1. **`events` table:** `id`, `type` (scan_completed | scan_failed | job_completed | job_failed | job_cancelled | orphan_swept | stats_drift), `severity` (info | warn | error), `message` TEXT, `metadata` JSON, `created_at` Unix int. Written by the scanner, worker, and orphan sweep — never by the API handler itself. Indexed on `(created_at DESC)` for the feed query.
2. **`GET /api/events`** — paginated (keyset on `id`), optional `?severity=` and `?type=` filters. `DELETE /api/events` and `DELETE /api/events/:id` clear entries from the UI feed (rows remain for learned-ratio accounting).
3. **WS `event_created` broadcast** — the hub emits this whenever a row is inserted, so the frontend feed updates in real time without polling.
4. **Frontend notification center** — a bell/feed icon in the sidebar with an unread badge (count since last visit, stored in `localStorage`). Clicking opens a slide-over or dedicated `/events` page showing the feed with severity coloring, relative timestamps, and expandable metadata (e.g. verification check breakdown on a `job_failed` event). Toasts remain for immediate confirmation; the log is the durable surface.
5. **UI polish pass** — fix any rough edges found during real use: empty states, loading edge cases, mobile layout gaps, error messages that say "request failed (500)" instead of something useful.

**Decisions & gotchas**
- Events are written inside the same transaction as the state change where possible (e.g. job row → `completed` + event insert in one write) so the log never diverges from actual state.
- Don't surface internal/debug events to the user — only the five types above. The distinction matters: an orphan sweep that finds nothing is not worth logging; one that restores a backup is.
- Unread count via `localStorage` is intentionally simple — no server-side read state, no per-device sync. This is a single-user tool; the complexity isn't worth it.
- The polish pass should be driven by real use, not speculation. Don't invent polish items — fix things that actually feel broken.

**Acceptance**
- [x] Encode completion, failure, cancellation, scan, and orphan restore events are written to the audit log.
- [x] `GET /api/events` paginates correctly; WS `event_created` fires on insert and the frontend feed updates live.
- [x] Unread badge reflects events since last visit; clears on open.
- [x] Notification panel shows job, scan, and recovery events with severity coloring.

**CC prompts**
- *"Add an `events` table (migration) and an `EventsRepo` with an `Insert(type, severity, message, metadata)` method that writes within the caller's transaction. Wire inserts into the worker (job completed/failed/cancelled, orphan restored), scanner (scan completed/failed), and stats drift guard. Add `GET /api/events` with keyset pagination and optional severity/type filters. Broadcast `event_created` from the WS hub on each insert."*
- *"Build the frontend notification center: a bell icon in the sidebar with an unread badge (localStorage watermark), a slide-over or `/events` page with a live-updating feed (WS `event_created` + TanStack Query), severity coloring, relative timestamps, and expandable metadata for job_failed events (verification checks). Toasts stay; this is the durable layer."*
- *"UI polish pass: audit every empty state, every error path, and the mobile layout. Fix any 'request failed (NNN)' messages with real descriptions. No speculative polish — only things that feel broken in actual use."*

---

## Cross-cutting threads (don't let these fall between phases)

- **Human-in-the-loop gate** (§2.1) — surfaces in P5 (echo selection) and P8 (confirm step). Verify both.
- **Non-destructive guarantee** (§2.4) — lives in P7; every acceptance test there is really testing this one principle.
- **Live config** — set up in P5, consumed by P3 (probe concurrency, scan interval) and P6 (encode window). Test that a settings change takes effect without restart.
- **Stats integrity** — created P4, defended P9. Incremental aggregates are a classic drift source; the recompute-assertion is the cheap insurance.
- **Crash recovery** — P7's orphan/reconcile sweep is the catch-all; make sure P6's job states and P3's `.reclaim-tmp` awareness line up with it.
- **Auth (§4.2)** — spans four phases: middleware + setup gate (P0), settings-backed `AuthStore` + session secret (P1), setup/login/logout endpoints + WS cookie check (P5), Setup/Login pages + 401 routing (P8). The seam to watch is the `AuthStore` interface: P0 defines it, P1 implements it — keep the signature stable so they don't drift.
