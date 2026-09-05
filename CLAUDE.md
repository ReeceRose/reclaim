# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

Reclaim is a self-hosted media codec audit and re-encode tool. It scans Plex/NAS libraries via `ffprobe`, ranks files by predicted HEVC savings, and lets the user manually queue re-encodes that run through `ffmpeg` in a configurable overnight window. The Go binary serves both the REST API and the embedded Next.js static frontend as a single container with no external runtime dependencies.

## Testing UI changes

Never run e2e/browser automation yourself (Playwright, chromium-cli, or similar) to verify a change — don't launch a headless browser, click through the app, or take screenshots as a substitute for real verification. Rely on `make lint`, `pnpm run build`, and `go test`/`go build` instead, and ask the user to check the UI in their own browser when visual/interactive confirmation is needed.

## Commands

### Go backend

```bash
make dev          # run locally against .dev/ dirs (sets all env vars, DISABLE_AUTH=true)
make build        # compile to bin/reclaim
make test         # go test ./...
make test-race    # race detector on scanner/worker/jobs (CI gate)
make lint         # go vet + web/landing lint + type-check
make clean        # remove bin/ and .dev/

# Single package test
go test ./internal/store/...
go test -run TestWorker ./internal/worker/...

# Migrations
make migrate-new NAME=add_something   # scaffold a new SQL migration
make migrate-up                       # apply pending to .dev DB
make migrate-status
```

### Frontend (web/)

```bash
cd web
pnpm install      # or pnpm install --frozen-lockfile in CI
pnpm run dev      # Next.js dev server on :3000, proxies /api/* to :8080
pnpm run build    # static export to web/out/ (used by embed.go)
pnpm run lint
pnpm run type-check
```

Or from the repo root: `make lint` runs both web/ and landing/ lint + type-check alongside `go vet`.

### Landing page (landing/)

Separate marketing site deployed to Vercel (not embedded in the Go binary):

```bash
cd landing
pnpm run dev      # Next.js dev server on :3000
pnpm run build
```

Production: https://reclaim.reecerose.com — config in `landing/lib/site.ts`.

### Generating fake media for dev

Populate `.dev/tv/` and `.dev/movies/` with tiny ffmpeg test-source videos:

```bash
# TV — 3 shows × 3 seasons × 6 episodes (54 files, ~5s each)
for show in "Breaking Bad" "The Wire" "Severance"; do
  for season in 1 2 3; do
    dir=".dev/tv/${show}/Season ${season}"
    mkdir -p "$dir"
    for ep in $(seq -w 1 6); do
      ffmpeg -y -f lavfi -i "testsrc=duration=5:size=1280x720:rate=24" \
        -f lavfi -i "sine=frequency=440:duration=5" \
        -c:v libx264 -preset ultrafast -crf 28 -c:a aac \
        "${dir}/S0${season}E${ep}.mkv" -loglevel error
    done
  done
done

# Movies — 6 files at 1080p
for title in "Inception (2010)" "Dune (2021)" "The Godfather (1972)" "Oppenheimer (2023)" "Mad Max Fury Road (2015)" "Interstellar (2014)"; do
  mkdir -p ".dev/movies/${title}"
  ffmpeg -y -f lavfi -i "testsrc=duration=5:size=1920x1080:rate=24" \
    -f lavfi -i "sine=frequency=440:duration=5" \
    -c:v libx264 -preset ultrafast -crf 28 -c:a aac \
    ".dev/movies/${title}/${title}.mkv" -loglevel error
done
```

Swap `-c:v libx264` for `-c:v mpeg4` on some files to get non-H.264 entries that rank higher for re-encode savings.

### Dev workflow

`make dev` runs the Go backend and the Next.js dev server concurrently (`& wait`). The frontend dev server (`pnpm run dev` in `web/`) proxies `/api/*` to the Go server. The Go static handler serves the embedded `web/out/` in production; the rewrite proxy in `next.config.ts` handles dev.

## Required environment variables

`MOVIES_PATH`, `TV_PATH`, `DB_PATH` are required and have no defaults. See `.env.example`. `make dev` sets them to `.dev/{movies,tv,data}`.

## Architecture

### Startup sequence (cmd/reclaim/main.go)

1. `config.Load()` — validates env vars
2. `startup.CheckBinaries()` — asserts `ffprobe`/`ffmpeg` on PATH
3. `store.Open()` — opens SQLite (WAL mode, two pools: 1 writer / 25 readers), runs goose migrations, bootstraps defaults
4. `config.NewLive(cfg)` — creates the runtime-mutable settings holder (encode window, scan interval, probe concurrency); read by the scanner and worker on every use so PUT `/api/settings` takes effect without a restart
5. `scanner.New()` + `sc.Start(ctx)` — runs startup scan, starts fsnotify watcher, schedules periodic rescans; `notify.New()` + `nt.Run(ctx)` batches the new candidates it finds
6. `api.New()` — wires routes on Echo v5; full route list: `/healthz`, `/api/{setup,login,logout,session}`, `/api/{stats,files,candidates}{,/grouped,/grouped/seasons,/grouped/episodes}`, `/api/stats/savings`, `/api/files/:id`, `/api/scan{,/full}`, `/api/profiles{,/:id}`, `/api/jobs{,/:id/cancel,/:id/force,/:id}`, `/api/events{,/:id}`, `/api/settings{,/credentials,/prune-missing,/notify-test}`, `/api/metadata{,/search,/refresh}`, `/api/ws`
7. `worker.New()` + `wk.Run(ctx)` — encode loop; polls for queued jobs inside the window

### Package map

| Package | Role |
|---|---|
| `internal/config` | Env parsing (`Config`) + runtime-mutable holder (`Live`) |
| `internal/store` | SQLite access — typed sub-stores: `Media`, `Jobs`, `Profiles`, `Scans`, `Settings`, `Stats`, `Metadata`, `Savings` |
| `internal/scanner` | Walk+ffprobe indexer, fsnotify watcher, rename detection via fingerprint |
| `internal/worker` | Encode loop: claim job → ffmpeg → verify → atomic swap |
| `internal/ffprobe` | Thin `ffprobe -v quiet -print_format json -show_streams -show_format` wrapper |
| `internal/ffmpeg` | Thin `ffmpeg` wrapper with progress parsing |
| `internal/media` | Fingerprinting (sha256 of size + first/last 64 KB), savings estimation (`savings.go`), encode-time estimation (`encodetime.go`), and content identity for replacement matching (`replacekey.go`) |
| `internal/jobs` | Pure state machine — legal transitions for the job lifecycle |
| `internal/api` | Echo v5 HTTP server, WebSocket hub, auth middleware |
| `internal/startup` | Pre-flight checks (binaries, mounts) |
| `internal/tmdb` | Rate-limited TMDB API client (3 req/s) — movie/TV search, detail fetching, image URL helpers |
| `internal/metadata` | Background fetcher: runs after each scan, populates `media_metadata` with staleness rules (14/30/90 days by status) |
| `internal/notify` | Batches newly-added re-encode candidates into one `candidates_added` event + optional webhook |
| `web/` | Next.js 16 static export embedded into the binary via `web/embed.go` |

### Store

`store.Open` returns a single `*Store` with typed sub-stores as fields. The write pool is `MaxOpenConns=1` (SQLite single-writer); the read pool is `MaxOpenConns=25`. Migrations run via goose embedded SQL in `internal/store/migrations/`.

### Worker safety model

The worker encodes to a `.reclaim-tmp.<ext>` temp file, verifies it with ffprobe (duration ±1s, stream counts, resolution), then atomically swaps: `original → .reclaim-backup`, `tmp → original`, delete backup. A crash between steps is recovered by `sweepOrphans` on next boot: a backup present with its original missing means the swap was interrupted and the backup is restored.

### Missing-file lifecycle

A file that vanishes from disk is soft-deleted: `status='missing'`, `missing_since` stamped (migration `00012`), and its `library_stats` contribution removed. The stamp is set with `COALESCE` so repeat marks don't restart the clock, and cleared when the file returns (re-probe, rename match, or post-encode swap).

Before a vanished file is marked missing, the scanner tries two reconciliations. First fingerprint matching (`GetByFingerprintOtherThan`) — identical bytes at a new path means a rename, handled by `RecordMove` (old row kept, path rewritten). Then, since a re-encode changes the bytes and so can never match a fingerprint, `FindSuperseder`/`Supersede`: a surviving active row in the same directory with the same name up to the final extension (`S07E01.mkv` → `S07E01.mp4`, an out-of-band transcode that changed container) absorbs the old row. Unlike a move, the *new* row survives — it holds the correct probe data — inheriting the old row's `transcode_jobs` while the old row is hard-deleted. The match requires exactly one same-stem candidate (`Movie.mkv` never claims `Movie.2160p.mkv`) and is refused while the old row has a live job. Scans emit one aggregated `file_superseded` event; the watcher emits one per file.

### Delete-and-redownload tracking

The other way a library shrinks is out of band: the user deletes a bloated
release and downloads a leaner one. `internal/scanner/replace.go` reconciles
that into a single event with a measured byte delta, rather than leaving an
unrelated `missing` row and a fresh arrival.

Matching is on **content identity**, not path: `media.ReplaceKey` reduces a file
to `tv|<title>|s%03d|e%04d` or `movie|<title><year>` (letters and digits only,
lowercased), so a redownload can change resolution, codec, release group, and
separators and still match. An underivable identity is `""`, which never matches
anything — two unparseable files are not thereby the same file. The key is
stamped on `media_files.replace_key` by `probeAndStore` on every insert and
re-probe, by `MarkMissing` when a row disappears, by `RecordMove` when a rename
gives the surviving row a new path (the row that held the correct key is the
duplicate being deleted, and a rename changes neither size nor mtime, so nothing
would ever re-probe the file and fix it), and by `Media.BackfillReplaceKeys` at
boot so an existing library participates without waiting for a full re-probe.

The movie half truncates at the **last release year** (`19xx`/`20xx` as a run of
exactly four digits, which is what excludes `1080p`, `2160p`, and `x264`), so
`Inception (2010)`, `Inception (2010) [Bluray-1080p]`, and a folderless
`Inception.2010.2160p.WEB.x265-GRP.mkv` all key on `inception2010`. Taking the
*last* year is what lets a title carry one of its own (`2001 A Space Odyssey
(1968)`). With no year there is nothing dependable to truncate at, so the whole
first path segment is used — which for a flat, yearless library means two
releases of one film simply fail to match. That is the deliberate direction:
guessing where the title stops would fold two unrelated movies into one row.

Both orderings are handled, because both happen:
- **Delete first** (the manual case, and any scan that sees both halves at
  once): `scanner.matchReplacements` runs over the scan's inserts *after* the
  vanished-path loop has stamped this pass's disappearances, and the watcher's
  `probeSingleFile` runs the same check on each arrival. `Media.FindReplacement`
  bounds it on `missing_since`.
- **Import first** (the *arr upgrade case): the remove debounce deliberately
  trails the probe debounce, so by the time `checkVanishedFile` fires the
  replacement is already active. `matchArrivedReplacement` searches from that
  side via `Media.FindActiveReplacement`, bounding it on the candidate's own
  `media_files.first_seen_at` (migration `00016`, seeded from `last_probed_at`
  for pre-existing rows).

`REPLACE_LOOKBACK` (default `720h`, `0` = off) is that bound in both directions,
and the arrival-side half of it is load-bearing rather than cosmetic: a library
that deliberately keeps two cuts of one title gives both rows the same key, so
deleting either leaves *exactly one* survivor and the ambiguity guard below
never fires. Only the survivor's arrival time separates "the file that just
replaced this" from "the copy that has sat here for a year", which would
otherwise book a plain deletion as a replacement.

A match whose two halves are byte-identical is not a replacement at all: the
same file has come back under a new name — the same release downloaded twice, a
copy restored from a backup. `matchReplacement` compares fingerprints before
folding and, when they agree, calls `RecordMove` instead, reviving the original
row at the new path with its history. The vanished-path reconciliations cannot
catch these — they compare fingerprints only against rows that are still
*active*, and the original has been missing for days by the time the file
returns. `recordReplacementTx` refuses an equal-sized pair for the same reason
it refuses a zero-byte one: the fold is still right, but a ledger of bytes has
no entry to make, and a zero row would only make the replacement count disagree
with the reclaimed and given-back figures it sums (migration `00017` drops the
rows booked before this).

Either direction resolves to `Media.RecordReplacement`, which shares
`Media.foldInto` with `Supersede`: the ledger row is written first (it reads
both rows, and the old one is deleted by the end of the transaction), job
history moves to the survivor, and the old row is hard-deleted. Both refuse
ambiguity — more than one candidate for the same key returns `ErrNotFound`
rather than a guess, since a fabricated pair would corrupt a lifetime total —
and both refuse while the old row has a live job. A matched arrival is
`Discard`ed from the candidate notifier for the same reason a supersede
survivor is: it is half of a swap, not an arrival.

`MISSING_RETENTION` (default `0` = never) drives the post-scan cleanup in `scanner.pruneMissing`: rows past the cutoff are hard-deleted along with their `transcode_jobs` history, skipping files with a `queued`/`running`/`verifying` job. `POST /api/settings/prune-missing` does the same ignoring the cutoff. Both write a `missing_pruned` event. Stats need no adjustment — the contribution left when the row went missing. Pruning forfeits any future replacement match for those rows, whatever `REPLACE_LOOKBACK` says.

### Timezone model

`main.go` sets `time.Local = time.UTC` before anything else, so every bare `time.Now()` — log stamps above all — is UTC regardless of the container's `TZ`. Wall-clock decisions are made explicitly against `live.Location()`: the worker's `withinWindow` and the scanner's `nextScanDelay` both do `time.Now().In(loc)`. The zone comes from `TIMEZONE` (validated at boot; an unloadable value is fatal), falling back to `TZ` (warn + UTC if unloadable), then UTC. Both are trimmed — a trailing space in a NAS template field otherwise makes `LoadLocation` fail and silently shifts the window by the whole UTC offset.

`config.WindowState(now, start, end)` is the single implementation of "is the window open, and when does it next flip". The worker gates on it and `GET /api/settings` reports it (`window_open`, `window_changes_at`), so the UI badge renders server truth instead of recomputing from the browser clock.

### Live settings

`config.Live` is a `sync.RWMutex`-guarded struct seeded from env at boot. The scanner and worker read it on each tick, so PUT `/api/settings` takes effect immediately. Settings overrides are in-memory only — a restart re-seeds from env (this includes `missing_retention` and `replace_lookback`, so a value set in the UI reverts to `MISSING_RETENTION` / `REPLACE_LOOKBACK` on restart).

`clock_format` is the exception: it has no env var and is persisted to the `settings` row (`clock_format` column, migration `00013`, `"12h"` default). It is display-only — the API always speaks 24-hour `HH:MM` — and instance-wide, since sessions are single-user. The frontend reads it off the cached settings query via `web/hooks/use-clock-format.ts`, so `formatClock`, `formatZoneClock`, and `windowInfo` all render on the chosen clock.

### New-candidate notifications

`internal/notify` announces newly-indexed re-encode candidates. The scanner feeds it row IDs —
one `Add` per watcher insert, one batched `Add` per scan after the rename/supersede
reconciliation has run — and the notifier holds them until the library goes quiet for
`notify_delay_seconds` (default 900), or 4× that if files keep trickling in. Its `Run` loop
ticks every 15s and re-reads the settings each tick, so a change applies without a restart.

IDs are collected at insert time and only judged at send time, via `Media.CandidatesByID`: a
rename's duplicate row has been deleted by `RecordMove`, a file may have been queued or
re-encoded, and the candidate query is the same one the Candidates page uses. The surviving
half of a supersede is explicitly `Discard`ed — same content, new container, not an arrival.
The first scan an instance ever completes (`Scans.CompletedBefore(runID) == 0`) is the library
baseline and never notifies; later scans, including the startup scan after a restart, do.

A flush is then split **one notification per title** (`notify.Split`): a TV series is one
`candidates_added` event however many episodes/seasons of it arrived, and every movie is its own
— a message that mixes two shows is unactionable. Past `maxTitlesPerFlush` (10) titles the flush
collapses into a single rollup (`notify.RollUp`) so a bulk import doesn't emit 200 events, and
consecutive webhook posts are spaced `webhookSpacing` (500ms) apart to stay under chat-service
rate limits. Each event is POSTed, if `notify_webhook_url` is set, in the shape
`notify_webhook_format` names (`json`/`discord`/`slack`/`ntfy`). Runtime delivery failures are
logged only — `POST /api/settings/notify-test` is where a webhook error is visible, so it
returns the receiver's own message. The `notify_*` settings live in the `settings` row
(migration `00014`), not `config.Live`: there is no env var behind a webhook URL typed into the
UI, so it has to survive restarts.

### Authentication

HMAC-signed session cookie (`reclaim_session`). First-run setup creates credentials in the DB. `DISABLE_AUTH=true` bypasses the middleware entirely. `RESET_AUTH=true` clears credentials on boot.

### Frontend

`web/` is a Next.js 16 static export (`output: 'export'`). The built `web/out/` is embedded into the binary via `web/embed.go` and served by the Go static handler as a catch-all after all API routes. In dev, `next.config.ts` rewrites `/api/*` to the Go backend at `RECLAIM_BACKEND` (default `http://localhost:8080`).

Key frontend pieces:
- `web/lib/api.ts` — typed API client; all types mirror Go DTOs in `internal/api/dto.go`
- `web/hooks/use-ws.ts` — WebSocket hook for live job progress
- `web/components/app-shell.tsx` — root shell with auth gate

`web/components/ui/tooltip-layer.tsx` is the app's tooltip: one instance
mounted in `AppShell`, driven by delegated `pointerover`/`focusin` on
`document`. Any element opts in with a `data-tooltip` attribute (innermost
under the pointer wins) — prefer it over the native `title`, which the media
tables cannot use at scale: a Radix trigger per cell would mount hundreds of
stateful components inside a virtualised list.

Table columns on the Candidates, Library, Browse › Movies, and Browse › TV
episode tables are user-configurable (visibility + drag order). `web/lib/table-columns.ts`
holds the pure `ColumnDef`/`TableLayout` model, `web/lib/table-layout-store.ts` is
the `localStorage` store behind `useSyncExternalStore` (key `reclaim:columns:<tableId>`,
`EMPTY_LAYOUT` as the server snapshot so a static export hydrates cleanly), and
`web/hooks/use-table-columns.ts` joins the two. Column definitions come from the
shared `web/components/media/media-columns.tsx` registry, composed per table in
`components/{candidates,library}/columns.tsx` and `components/browse/{movie,episode}-columns.tsx`.
Saved layouts store only what the user changed: a column absent from `visible`
falls back to its `defaultVisible`, and a column absent from `order` is spliced
in beside its registry neighbour, so adding a column in a later release does not
invalidate a saved layout. Breakpoint hiding is independent of user visibility —
`ColumnDef.breakpoint` still hides a column on narrow windows. Nothing is
persisted server-side; see the README for the user-facing contract.

The frontend uses the **Next.js App Router** (`web/app/`). **Important:** `web/AGENTS.md` warns that this is Next.js 16 with breaking changes from prior versions. Read `node_modules/next/dist/docs/` before writing Next.js code.

`docs/API.md` is the authoritative REST API reference. Encode time estimation design: `docs/ENCODE-TIME-PLAN.md`.

### WebSocket events

The hub broadcasts: `job_started`, `job_progress` (with `percent`), `job_completed`, `job_failed`, `job_cancelled`, `jobs_queued`, `event_created`. The scanner broadcasts `scan_started`, `scan_completed`, and `scan_failed` during scans, and `event_created` for its `file_superseded` / `file_replaced` reconciliations. The notifier broadcasts `event_created` for its `candidates_added` batches.

### Candidate pagination & filtering

`GET /api/candidates` supports 8 sort options via `?sort=`: `savings_desc` (default), `size_desc`, `size_asc`, `codec`, `resolution`, `mtime_desc`, `mtime_asc`, `library_type`. Filters: `library_type` (`movies`|`tv`), `video_codec`, `height` (`uhd8k`|`uhd`|`qhd`|`fhd`|`hd`|`sd`|`unknown`, or legacy numeric heights), `search` (path substring).

`GET /api/files` is the Library view — same filters plus `status` (`active`|`missing`) and `candidate_state` (`candidate`|`already_hevc`|`probe_failed`|`unknown_codec`|`queued`|`completed`|`missing`). Sort options: `path_asc` (default), `size_desc`, `size_asc`, `codec`, `resolution`, `mtime_desc`, `mtime_asc`, `library_type`.

The Browse page's grouped TV views (`GET /api/files/grouped`, `GET /api/seasons`)
take a `progress` filter — `converted` | `partial` | `unconverted` | `missing` —
applied as a `HAVING` clause over the group aggregates, so page and count queries
agree. The Movies tab has no groups, so it reuses the per-file `candidate_state`
filter instead.

Pagination: the default `savings_desc` sort uses keyset cursors (`after_savings` + `after_id`) for gap-free infinite scroll over large libraries. All other sorts fall back to `offset` pagination.

### Realized savings ledger

`library_stats` only ever holds *predictions*: `ReplaceWithEncodedTx` zeroes a
file's `predicted_savings_bytes` and rewrites `video_codec` to `hevc` the moment
an encode lands, so the library aggregates can say what is left to reclaim but
never what was actually reclaimed. `savings_ledger` (migration `00015`) is the
append-only record that fills that gap — one row per completed encode, written
by `Savings.RecordTx` inside the same `CommitEncodeSwap` transaction as the job
completion and the `job_completed` event.

The insert runs *before* `ReplaceWithEncodedTx` because it captures the
pre-encode `video_codec`, dimensions, and duration that the swap destroys. It
also snapshots the queue-time predictions (`predicted_savings_bytes`,
`initial_estimated_duration_seconds`) alongside the measured result, which is
what powers the estimate-accuracy figures. Both completion paths — `replace`
and the crash-recovery `tryCompletePostSwap` — go through `CommitEncodeSwap`,
so neither can miss a row. `MarkCompletedTx` guards `verifying → completed`, so
a replayed commit rolls the whole transaction back rather than double-counting.

The ledger has no foreign key to `media_files` on purpose: `PruneMissing` hard-
deletes a pruned file's `transcode_jobs` history, and lifetime reclaimed bytes
must not shrink when a file is later deleted from disk. The migration backfills
from existing completed jobs, recording a null `source_codec` rather than
guessing one: the swap has already overwritten it, and while the seed ratio can
be inverted out of `transcode_jobs.predicted_savings_bytes`, the seed table is
not injective (`h264` and `avc` both sit at 0.60), so any single codec picked
back out would be a fabrication. `Savings.ByCodec` groups those rows under
`unknown` so the buckets still sum to the lifetime total; `LearnedRatios`
excludes them.

`Jobs.LearnedRatios` reads the ledger rather than joining `transcode_jobs` to
`media_files`. The old query grouped by the post-encode `media_files.video_codec`,
which is always `hevc`, so `refineRatioIfReady` could never find the source
codec's bucket and the savings model never actually refined.

The ledger's `source` column (migration `00016`) widened it from "bytes this
encoder reclaimed" to "bytes reclaimed by any means": `encode` rows come from
`Savings.RecordTx`, `replace` rows from `recordReplacementTx` inside the fold
described under *Delete-and-redownload tracking*. `job_id` went nullable for
that (partial unique index on non-null, so `RecordTx`'s `INSERT OR IGNORE` stays
idempotent), and every pre-existing query gained `WHERE source = 'encode'` —
including `Jobs.LearnedRatios`, where a replacement's size ratio would otherwise
train the savings model on someone else's encode.

A replacement's delta is signed. An upgrade to a larger release costs disk and
is recorded as such, so `ReplacementSummary` reports the net alongside
`BytesReclaimed`/`BytesAdded` rather than a single figure that nets a win
against a cost.

`GET /api/stats` carries `savings` and `replacements` summary blocks; `GET
/api/stats/savings` returns the full report (breakdowns, daily series, top wins,
job outcomes, plus a `replacements` block) that the Insights page renders. The
encode-side aggregates stay encode-only — they double as the estimate-accuracy
record, which a replacement has no prediction to be scored against — so the
Insights headline is `bytes_saved + replacements.bytes_delta`. Day buckets are
shifted by the `TIMEZONE` offset so the series matches the clock the UI shows.

### Encode time estimates

Per-job encode duration estimates on the Queue page learn from completed jobs on this instance, bucketed by **profile first** with fallbacks (preset+CRF → preset → global → seed rates). Settings are snapshotted on the job row at queue time (`encode_preset`, `encode_crf`, `encode_extra_args`, migration `00009`); the worker still reads the live profile when encoding. `GET /api/jobs` returns `estimated_duration_seconds` / `estimate_source` for queued and running jobs, `encode_duration_seconds` for completed jobs, and `queue_total_estimated_seconds` on the first page. See `docs/ENCODE-TIME-PLAN.md` and `docs/API.md` § Jobs.

