# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

Reclaim is a self-hosted media codec audit and re-encode tool. It scans Plex/NAS libraries via `ffprobe`, ranks files by predicted HEVC savings, and lets the user manually queue re-encodes that run through `ffmpeg` in a configurable overnight window. The Go binary serves both the REST API and the embedded Next.js static frontend as a single container with no external runtime dependencies.

## Commands

### Go backend

```bash
make dev          # run locally against .dev/ dirs (sets all env vars, DISABLE_AUTH=true)
make build        # compile to bin/reclaim
make test         # go test ./...
make test-race    # race detector on scanner/worker/jobs (CI gate)
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
```

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
5. `scanner.New()` + `sc.Start(ctx)` — runs startup scan, starts fsnotify watcher, schedules periodic rescans
6. `api.New()` — wires routes on Echo v5; full route list: `/healthz`, `/api/{setup,login,logout,session}`, `/api/{stats,candidates,candidates/grouped,files/:id}`, `/api/scan{,/full}`, `/api/profiles{,/:id}`, `/api/jobs{,/:id/cancel}`, `/api/settings{,/credentials}`, `/api/files/grouped{,/seasons,/episodes}`, `/api/metadata{/search,/refresh}`, `/api/ws`
7. `worker.New()` + `wk.Run(ctx)` — encode loop; polls for queued jobs inside the window

### Package map

| Package | Role |
|---|---|
| `internal/config` | Env parsing (`Config`) + runtime-mutable holder (`Live`) |
| `internal/store` | SQLite access — typed sub-stores: `Media`, `Jobs`, `Profiles`, `Scans`, `Settings`, `Stats`, `Metadata` |
| `internal/scanner` | Walk+ffprobe indexer, fsnotify watcher, rename detection via fingerprint |
| `internal/worker` | Encode loop: claim job → ffmpeg → verify → atomic swap |
| `internal/ffprobe` | Thin `ffprobe -v quiet -print_format json -show_streams -show_format` wrapper |
| `internal/ffmpeg` | Thin `ffmpeg` wrapper with progress parsing |
| `internal/media` | Fingerprinting (sha256 of size + first/last 64 KB) and savings estimation |
| `internal/jobs` | Pure state machine — legal transitions for the job lifecycle |
| `internal/api` | Echo v5 HTTP server, WebSocket hub, auth middleware |
| `internal/startup` | Pre-flight checks (binaries, mounts) |
| `internal/tmdb` | Rate-limited TMDB API client (3 req/s) — movie/TV search, detail fetching, image URL helpers |
| `internal/metadata` | Background fetcher: runs after each scan, populates `media_metadata` with staleness rules (14/30/90 days by status) |
| `web/` | Next.js 16 static export embedded into the binary via `web/embed.go` |

### Store

`store.Open` returns a single `*Store` with typed sub-stores as fields. The write pool is `MaxOpenConns=1` (SQLite single-writer); the read pool is `MaxOpenConns=25`. Migrations run via goose embedded SQL in `internal/store/migrations/`.

### Worker safety model

The worker encodes to a `.reclaim-tmp.<ext>` temp file, verifies it with ffprobe (duration ±1s, stream counts, resolution), then atomically swaps: `original → .reclaim-backup`, `tmp → original`, delete backup. A crash between steps is recovered by `sweepOrphans` on next boot: a backup present with its original missing means the swap was interrupted and the backup is restored.

### Live settings

`config.Live` is a `sync.RWMutex`-guarded struct seeded from env at boot. The scanner and worker read it on each tick, so PUT `/api/settings` takes effect immediately. Settings overrides are in-memory only — a restart re-seeds from env.

### Authentication

HMAC-signed session cookie (`reclaim_session`). First-run setup creates credentials in the DB. `DISABLE_AUTH=true` bypasses the middleware entirely. `RESET_AUTH=true` clears credentials on boot.

### Frontend

`web/` is a Next.js 16 static export (`output: 'export'`). The built `web/out/` is embedded into the binary via `web/embed.go` and served by the Go static handler as a catch-all after all API routes. In dev, `next.config.ts` rewrites `/api/*` to the Go backend at `RECLAIM_BACKEND` (default `http://localhost:8080`).

Key frontend pieces:
- `web/lib/api.ts` — typed API client; all types mirror Go DTOs in `internal/api/dto.go`
- `web/hooks/use-ws.ts` — WebSocket hook for live job progress
- `web/components/app-shell.tsx` — root shell with auth gate

The frontend uses the **Next.js App Router** (`web/app/`). **Important:** `web/AGENTS.md` warns that this is Next.js 16 with breaking changes from prior versions. Read `node_modules/next/dist/docs/` before writing Next.js code.

`docs/API.md` is the authoritative REST API reference.

### WebSocket events

The hub broadcasts: `job_started`, `job_progress` (with `percent`), `job_completed`, `job_failed`, `job_cancelled`, `jobs_queued`, `event_created`. The scanner broadcasts `scan_started`, `scan_completed`, and `scan_failed` during scans.

### Candidate pagination & filtering

`GET /api/candidates` supports 8 sort options via `?sort=`: `savings_desc` (default), `size_desc`, `size_asc`, `codec`, `resolution`, `mtime_desc`, `mtime_asc`, `library_type`. Filters: `library_type` (`movies`|`tv`), `video_codec`, `resolution_band` (sd|hd|uhd), `search` (path substring).

Pagination: the default `savings_desc` sort uses keyset cursors (`after_savings` + `after_id`) for gap-free infinite scroll over large libraries. All other sorts fall back to `offset` pagination.

`GET /api/candidates/grouped` returns series+season hierarchy for TV and a flat list for movies (loads all matching rows in one pass via `AllCandidates`).
