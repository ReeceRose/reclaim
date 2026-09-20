# Changelog

Release notes for [Reclaim](https://github.com/ReeceRose/reclaim). This file is the
source of truth: `scripts/release.sh` writes each entry here, commits it, tags that
commit, and publishes the same text as the GitHub Release. The running binary embeds
this file, so the in-app release notes always match the build you are on.

## v0.0.44 — 2026-09-20

The queue page is now split into paginated Queued and History tabs, so a large backlog no longer buries completed jobs.

### What's Changed

#### Features

- The Queue page now has separate **Queued** and **History** tabs, each with its own numbered pager, so hundreds of pending jobs no longer push finished encodes out of reach.
- The currently running job stays pinned above the tabs while you page through either list.
- Your active tab and page are kept in the URL, so a queue view can be bookmarked, refreshed, or shared without losing your place.

#### Fixes

- Queue totals, per-status counts, and history summaries now describe the whole filtered set on every page instead of only the rows currently on screen — the page totals and the sidebar queue badge always agree.

#### Improvements

- `GET /api/jobs` returns summary counts and estimated queue time alongside any page of results; see `docs/API.md` for the new fields.
- Updated Go and frontend dependencies (including Next.js, React, and the SQLite driver) and moved builds to Node 24.

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.44
```

## v0.0.43 — 2026-09-20

Release notes now ship inside Reclaim — read what changed without leaving the app.

### What's Changed

#### Features

- The sidebar version number is now a button that opens release notes for every version, read straight from the changelog bundled with your build.
- After upgrading, a "What's New" indicator points at the notes for the version you just moved to. It clears once you open the panel.
- New `GET /api/releases` endpoint returns the parsed release history, and `POST /api/releases/seen` marks it read.

#### Improvements

- Release notes make no outbound requests, so they work on air-gapped installs and can never show notes for a different version than the one running.
- Fresh installs are stamped with their starting version at first boot, so a brand-new instance isn't shown a changelog for releases it never upgraded through.
- The release script now writes the changelog entry, commits it, tags that commit, and publishes the identical text to GitHub — the tag always points at a tree whose changelog already describes it.
- Notes render through a small purpose-built Markdown renderer covering exactly the formatting releases use.

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.43
```

## v0.0.42 — 2026-09-20

Documentation and polish for the AV1 encoding support added in v0.0.41.

### What's Changed

#### Improvements
- The landing page now covers AV1: a dedicated feature card explaining the HEVC (libx265) vs AV1 (SVT-AV1) choice, the roughly 20% size advantage of AV1, and the client-compatibility tradeoff worth checking before switching.
- Throughput tables now note that the listed figures are x265-specific, and explain the SVT-AV1 preset scale (`0`–`13`) plus the fact that encode-time estimates are tracked separately per codec.
- The verification description now mentions the codec check — Reclaim confirms the output is actually in the codec you asked for before swapping the original.
- Landing page dashboard mock shows an AV1 bucket alongside HEVC, matching what a mixed library actually looks like.

#### Features
- Codec badges on the dashboard's codec breakdown are now clickable — selecting one jumps straight to the Library view filtered to that codec.

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.42
```

## v0.0.41 — 2026-09-16

AV1 encoding lands alongside HEVC, and savings predictions now learn from your own completed encodes everywhere they appear.

### What's Changed

#### Features
- **AV1 encoding.** Each transcode profile now picks HEVC (`libx265`) or AV1 (SVT-AV1) under **Settings › Encoding**, with the right CRF range and preset vocabulary for each. Reclaim reads `ffmpeg -encoders` at boot and refuses AV1 profiles if your ffmpeg lacks `libsvtav1`.
- **Efficient-codec detection.** Files already in HEVC, AV1, or VVC are never candidates whichever codec you target — re-encoding between them costs quality for little or no space.
- **Sort by release date and air date** on Candidates, Library, and Browse, with a release-date column (theatrical release for movies, air date for episodes).
- **Queued files are shown separately from converted ones** in Browse progress bars, so a season part-way through the queue no longer looks finished.

#### Improvements
- Learned savings ratios now apply across the whole app — the dashboard, candidate rankings, and per-file estimates all price against your default profile's codec, and re-pricing happens automatically when you change it. Learned ratios and encode-time estimates are tracked per codec.
- Encode verification now checks the output really is in the profile's target codec.
- Dependency updates across the backend and both frontends.

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.41
```

## v0.0.40 — 2026-09-05

Table views are now yours to arrange, with richer detail on hover.

### What's Changed

#### Features
- **Customisable table columns** — the Candidates, Library, Browse › Movies, and Browse › TV episode tables each get a **Columns** button to toggle which columns appear and drag them into any order. Layouts are saved per browser (nothing is written to the server), and **Reset** restores the defaults.
- **New optional columns** — folder, length, bitrate, audio (codec + channel layout), and container are available on the tables above, off by default.
- **Tooltips across the media tables** — hover a truncated path, a date, or a badge for the full value, including exact timestamps behind the shortened dates.

#### Improvements
- Dates in tables now drop the year within the current year, so the column stays narrow; the full date and time is in the tooltip.
- Status badges use short labels (HEVC, Failed, Unknown) in fixed-width columns so they no longer wrap.
- Columns still respect window width: a column can be enabled and hidden on a narrow screen, and the Columns popover tells you which width each one needs.
- Columns added in future releases appear automatically in the right place without resetting a saved layout.

### Docker

```bash
docker pull ghcr.io/ReeceRose/reclaim:0.0.40
```

## v0.0.39 — 2026-08-31

Reclaim v0.0.39 corrects how the scanner handles a file that comes back unchanged, so the replacement ledger only counts real swaps.

### What's Changed

#### Fixes
- A file that reappears byte-for-byte identical under a new name — the same release downloaded again, or a copy restored from a backup — is now recognised as the original returning rather than being booked as a replacement. Its history is carried over to the new path instead of a new row being created.
- Replacements that neither freed nor cost any space are no longer written to the savings ledger, so the replacement count always agrees with the freed and given-back totals it sums. A migration removes rows already recorded this way, which will lower the replacement count shown on Insights (the byte totals are unaffected).

#### Improvements
- Insights now says "1 replacement" and "1 upgrade" instead of "1 replacements" and "1 upgrades".

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.39
```

## v0.0.38 — 2026-08-31

Release notes for v0.0.38:

---

Reclaim now measures the storage you win back by swapping a bloated release for a leaner one — not just by re-encoding it.

### What's Changed

#### Features
- **Delete-and-redownload tracking.** When a file disappears and another copy of the same episode or movie shows up, Reclaim now pairs the two instead of leaving an unrelated "missing" row and a fresh arrival. The size difference is recorded to the savings ledger, so replacing a 20 GB remux with an 8 GB encode counts toward your reclaimed total. Matching works on content identity, so resolution, codec, release group, and naming style can all change and still match — and it works whether the delete or the download lands first.
- **Replacements on Insights.** The page now reports net reclaimed alongside separate freed and given-back figures, since upgrading to a *larger* release costs disk and is shown as such. Includes per-library breakdowns and biggest wins.
- **`REPLACE_LOOKBACK` setting** (default 30 days, `0` to disable), adjustable from Settings → Library without a restart.

#### Improvements
- Matched pairs appear in the activity feed as a single event rather than two unrelated ones.
- Replacement data is excluded from the encode-savings model, so it can't train estimates on someone else's encode.
- Landing page gained an Insights section; frontend dependencies updated.

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.38
```

## v0.0.37 — 2026-08-28

Reclaim now tracks what your encodes actually reclaimed, not just what they were predicted to save.

### What's Changed

#### Features
- **New Insights page** — a full report on realized savings: lifetime bytes reclaimed, breakdowns by codec, resolution and library, a daily savings series, your biggest wins, and how accurate the size/time estimates have been.
- **Savings summary on the dashboard** — total space actually recovered now sits alongside the remaining-to-reclaim figures.
- **Every completed encode is recorded** in an append-only ledger capturing the original codec, resolution and duration before the swap overwrites them. Existing completed jobs are backfilled on upgrade.
- **Filter Browse by conversion progress** — narrow the TV shows and seasons views to fully converted, partly converted, not converted, or those with missing files, and filter the Movies tab by file state.

#### Fixes
- The savings model now learns correctly from finished encodes. It previously looked up the post-encode codec (always HEVC), so it never found the source codec's bucket and its predictions never actually improved.
- Lifetime reclaimed totals no longer shrink when a file is deleted from disk and pruned from the library.
- A completed job replayed after a crash can no longer be counted twice.

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.37
```

## v0.0.36 — 2026-08-17

Reclaim can now tell you when new re-encode candidates show up in your library.

### What's Changed

#### Features
- **New-candidate notifications** — when a scan or the file watcher indexes new files worth re-encoding, Reclaim raises a `candidates_added` event instead of leaving you to notice on your own.
- **Webhook delivery** — point notifications at Discord, Slack, ntfy, or a plain JSON endpoint. Configure it under Settings → Notifications, with a **Send test** button that reports the receiver's own error message if something's wrong.
- **One notification per title** — a TV series arrives as a single notification no matter how many episodes landed, and each movie gets its own. Bulk imports collapse into one rollup instead of hundreds of messages.
- **Quiet-period batching** — Reclaim waits until your library stops changing (default 15 minutes) before notifying, so a big copy doesn't spam you mid-transfer.

#### Improvements
- Notification settings persist across restarts, so a webhook URL typed into the UI survives a container restart.
- The very first scan on a new instance is treated as the baseline and never notifies.
- Renamed and out-of-band-transcoded files are no longer reported as new arrivals.
- Dependency updates across the backend, web UI, and landing site.

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.36
```

## v0.0.35 — 2026-08-01

Clock display is now configurable — pick 12-hour or 24-hour and every time in the UI follows it.

### What's Changed

**Features**
- New **Clock format** setting under Settings → Encoding: choose a 12-hour (`10 PM`) or 24-hour (`22:00`) clock. The choice is saved on the server, so it sticks across restarts and applies to every browser you open Reclaim in.
- The encode window pickers, the window status badge, and the Queue page timestamps all render in your chosen format.

**Fixes**
- The encode window hour picker no longer showed `:00` when the window was set to a non-whole hour — it now displays the actual minutes.

**Improvements**
- Time formatting is shared through a single helper, so the window badge and queue estimates stay consistent with each other and with the server's configured timezone.
- Existing installs default to the 12-hour clock, matching previous behaviour; no action needed after upgrading.

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.35
```

## v0.0.34 — 2026-08-01

The encode window now runs on a timezone you choose in the UI, instead of whatever the container's clock happened to be.

### What's Changed

#### Features
- **Timezone picker in Settings › Encoding** — pick the IANA zone your encode window and scan anchor are read in, and it applies immediately without a restart.
- New `TIMEZONE` environment variable, validated at boot so a typo fails loudly instead of silently shifting your window by hours. Falls back to `TZ`, then UTC. (Values are trimmed, so a stray trailing space in a NAS config field no longer breaks it.)
- Running jobs started with **Run now** are marked with a `forced` badge, making it clear why they're encoding outside the window.

#### Fixes
- The window open/closed badge and countdown are now computed on the server, in the configured zone. Previously they were derived from your browser clock, so viewing the app from a different timezone showed a window state that disagreed with when jobs actually ran.
- Scheduled rescans read `SCAN_ANCHOR` in the configured timezone rather than UTC, so a "00:00" scan runs at your midnight.

#### Improvements
- Window times display on a 12-hour clock with the active timezone shown alongside them.
- The Queue page refreshes window state every minute.
- Dependency updates across the Go backend, web app, and landing page.

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.34
```

## v0.0.33 — 2026-07-25

Files re-encoded outside Reclaim no longer show up as missing duplicates.

### What's Changed

#### Fixes
- When a file is replaced by the same title in a different container (e.g. `S07E01.mkv` becomes `S07E01.mp4` after an out-of-band transcode), the scanner now recognises the new file as a replacement instead of leaving a stale "missing" entry behind. The new file keeps its correct probe data and inherits the old file's encode history.
- Replacement matching is deliberately strict: it only applies when exactly one same-name file exists in the directory (`Movie.mkv` will never be claimed by `Movie.2160p.mkv`), and it is skipped while the old file still has a queued or running job.

#### Improvements
- New `file_superseded` event in the activity feed, so you can see when a file was absorbed by its replacement. Scans log one summary event; the live watcher logs one per file.
- Event filtering by `type=file_superseded` is documented in the API reference.

### Docker

```bash
docker pull ghcr.io/ReeceRose/reclaim:0.0.33
```

## v0.0.32 — 2026-07-25

Fixes episode paging inside TV seasons in the Library view.

### What's Changed

#### Fixes
- Loading more episodes in a TV season now pages within that season instead of across the entire series. Previously, seasons with many episodes could show empty or partial pages as the loader pulled in files from other seasons and then discarded them.
- The "load more" boundary now uses the season's real episode total, so scrolling stops at the right place instead of stalling early or requesting pages that don't exist.

#### Improvements
- Season episode queries now filter on the indexed series and season columns rather than matching file paths, making expanding a season noticeably faster on large libraries.
- Dropped an unused API client helper and added test coverage for season paging.

### Docker

```bash
docker pull ghcr.io/ReeceRose/reclaim:0.0.32
```

## v0.0.31 — 2026-07-25

Missing files no longer pile up forever — Reclaim can now clean up records for files that have vanished from disk.

### What's Changed

#### Features
- **Library cleanup in Settings.** A new panel shows how many files are currently missing, how much space they accounted for, and how long the oldest has been gone. Pick a retention period (7 / 14 / 30 / 60 / 90 days, or Never) and Reclaim removes those records after each scan.
- **Purge now.** A one-click button clears every missing record immediately, ignoring the retention setting. Files with an active or queued encode are skipped, and nothing on disk is ever touched.
- **`MISSING_RETENTION` env var** sets the default retention at startup (`0` = never prune). Adjustable at runtime from the UI without a restart.
- **Cleanup is audited.** Each prune writes a `missing_pruned` event to the activity log and pushes it live over WebSocket.

#### Improvements
- Missing files now record *when* they went missing, so a temporary NAS dropout or unmounted share doesn't restart the clock — and the timer resets if the file comes back.
- `docs/API.md` documents the new `missing_retention` setting, the `missing_files` summary on `GET /api/settings`, and `POST /api/settings/prune-missing`.

### Docker

```bash
docker pull ghcr.io/ReeceRose/reclaim:0.0.31
```

## v0.0.30 — 2026-07-25

Reclaim now spots bloated files in any codec — including HEVC — and ranks your biggest TV seasons.

### What's Changed

#### Features
- **Oversized-file detection** — every file gets a bitrate ratio comparing it against what a well-encoded file of the same codec and resolution should use. Files past the threshold get an "Oversized · 2.4×" badge in the Library, so you can find bloat even in files that are already HEVC and wouldn't otherwise show up as candidates.
- **Sort and filter by oversize** — new "Most oversized" sort in the Library view, plus an oversized-only filter.
- **Season leaderboard** — a new Seasons tab on Browse ranks every season across your whole TV library by total size or predicted savings, with search and grid/list views.
- **Configurable threshold** — set `OVERSIZE_THRESHOLD` (default `2.0`) via env or adjust it live in Settings → Encoding without a restart.

#### Fixes
- Signing in or completing first-run setup now refreshes the session properly instead of writing a stale value, fixing occasional redirects back to the login page.
- The version label no longer errors when build commit info is missing.
- Fixed a cache key collision between the queue list and other job views.

### Docker

```bash
docker pull ghcr.io/ReeceRose/reclaim:0.0.30
```

## v0.0.29 — 2026-07-12

Small, focused release with a single fix.

### What's Changed

**Fixes**
- Fixed the encode window countdown showing out-of-sync times between the Queue page and the app shell — both now share the same live-updating clock.

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.29
```

## v0.0.28 — 2026-07-12

Encode visibility improvements — see how estimates stack up against reality, and watch the encode window countdown update live.

### What's Changed

**Features**
- Job history now shows estimate-vs-actual deltas, so you can see how accurate the encode time predictions were for completed jobs
- The encode window countdown on the Queue page now updates live instead of only refreshing on page load

**Improvements**
- Job records now capture their initial time estimate at creation, so later comparisons reflect the original prediction rather than a recalculated one

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.28
```

## v0.0.27 — 2026-07-12

Reclaim's queue and history views now scale to large libraries with proper pagination.

### What's Changed

**Improvements**
- Queue and job history lists now load in pages instead of all at once, making them much faster and more responsive on libraries with a large backlog of jobs
- Refreshed the Queue page UI to support the new paginated data flow

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.27
```

## v0.0.26 — 2026-07-11

This release focuses on resilience — Reclaim now handles auth failures and network drops gracefully instead of leaving the UI in a broken state, plus routine dependency upgrades.

### What's Changed

**Improvements**
- Added a connection-lost indicator so the UI clearly communicates when it loses contact with the backend, instead of failing silently
- Improved error page handling for a clearer recovery path when something goes wrong client-side
- Hardened session/auth handling so expired or invalid sessions and network hiccups no longer surface as raw errors

**Fixes**
- Fixed edge cases in auth middleware around error responses (with added test coverage)

**Chores**
- Upgraded Go, web, and landing page dependencies to their latest compatible versions

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.26
```

## v0.0.25 — 2026-07-10

Reclaim v0.0.25 focuses on scan accuracy and TV browsing polish.

### What's Changed

**Fixes**
- Fixed files incorrectly showing as "missing" after being re-encoded
- Fixed misaligned columns in the TV episode browser

**Improvements**
- Preserved the `out` directory placeholder during builds

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.25
```

## v0.0.24 — 2026-07-04

Show version in UI, tighten up API auth, and standardize Tailwind class usage across the frontend.

### What's Changed

#### Features
- The app version is now displayed in the UI so you can quickly confirm what's running.

#### Improvements
- Cleaned up and standardized Tailwind class usage throughout the frontend (layout, settings, queue, library, and auth pages) for more consistent styling.
- Minor build/CI tweaks to support version injection into the frontend.

#### Fixes
- Small auth handler adjustment alongside the version display work.

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.24
```

## v0.0.23 — 2026-07-04

Ranking a stability-focused release: fixes a bug around deleted show tracking and adds per-item rescan control.

### What's Changed

#### Features
- Added the ability to rescan a single TV show or movie on demand, instead of triggering a full library rescan (0586b5d)

#### Fixes
- Fixed the "All converted" status incorrectly persisting after a TV show was deleted (4227f3d)

#### Improvements
- Updated dependencies, including `goose` (database migrations) and various frontend packages, for security and stability

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.23
```

## v0.0.22 — 2026-07-01

Encode time estimates on the Queue page now learn from your own hardware and history instead of relying on rough guesses.

### What's Changed

**Features**
- Added per-job encode time estimates on the Queue page that improve automatically as jobs complete on your instance
- Estimates fall back gracefully (profile + settings → preset/CRF → preset → global average → seed rate) so new profiles still get a reasonable estimate before any history exists
- Profile settings (preset, CRF, extra args) are now snapshotted on each job at queue time, so past estimates stay accurate even if you edit a profile later

**Improvements**
- Library and Candidates pages gained sortable column headers and expanded filtering options
- API responses for jobs now include `estimated_duration_seconds`, `estimate_source`, and `encode_duration_seconds`
- Expanded documentation covering the encode-time estimation design and updated API reference

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.22
```

## v0.0.21 — 2026-07-01

v0.0.21 tries the direct-play compatibility feature and then walks it back, while hardening scan/encode reliability and modernizing the frontend tooling.

### What's Changed

#### Features
- Added direct-play compatibility prediction, scoring files against built-in client profiles at probe time, with HDR compatibility rules and a way to queue re-encodes straight from the Compatibility view — then removed the feature again after further evaluation, along with the grouped candidates view and its cache.

#### Improvements
- Hardened scan and encode reliability, with better recovery and error handling in the scanner and worker.
- Added frontend connection and error resilience, including a WebSocket-disconnected banner and clearer query error states.
- Extended `ffprobe` and the scanner to persist stream and HDR metadata, plus a backfill coordinator to populate it for existing libraries.
- Switched the frontend from ESLint to Biome for linting and formatting.
- Sped up Docker image builds in CI.

#### Fixes
- Various stability fixes across job lifecycle handling, WebSocket events, and TV metadata parsing (covered by new tests).

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.21
```

## v0.0.20 — 2026-07-01

Faster selection when managing large libraries, plus a quieter CI pipeline.

### What's Changed

#### Features
- Added shift-click range selection for candidates, library, and queue views — select a contiguous range of files at once instead of clicking each one individually

#### Improvements
- Resolved a deprecation warning from the CI pipeline running on Node.js 20

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.20
```

## v0.0.19 — 2026-07-01

A focused update on managing job history and smoother page-to-page navigation.

### What's Changed

**Features**
- Add the ability to dismiss completed/failed jobs from the queue so your history stays clean

**Improvements**
- Improve navigation between browse, library, and queue pages, including shallow routing so pages don't fully reload
- Tidy up the queue page layout and file detail view
- Simplify media row/card components across movies and shows
- Update Docker and API documentation

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.19
```

## v0.0.18 — 2026-07-01

Here's the changelog:

---

This release makes the Docker container easier to run with non-root permissions and gives clearer visibility into why encodes fail.

### What's Changed

**Features**
- Added `PUID`/`PGID` environment variables so the container can run as your host user instead of root, avoiding permission issues on bind-mounted `/movies` and `/tv` libraries

**Fixes**
- The Queue page now shows the actual error message when a job fails instead of a generic "verification failed" message
- Fixed a stale output path being left behind after a failed encode, which could point the UI at a file that no longer exists

**Improvements**
- Failed jobs now log detailed error context (missing media file, missing profile, encode failure) to aid troubleshooting

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.18
```

## v0.0.17 — 2026-07-01

This release focuses on Docker usability and clearer error reporting for failed encode jobs.

### What's Changed

**Features**
- Added `PUID`/`PGID` environment variable support, so the container can run as a non-root user matching your host's file permissions

**Improvements**
- Failed jobs now surface the actual underlying error instead of a generic failure message, making it easier to diagnose why an encode failed

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.17
```

## v0.0.16 — 2026-06-30

Small routing improvement to avoid full page reloads when changing filters or sort options.

### What's Changed

#### Improvements
- Filter and sort changes now update the URL without triggering a full page navigation, making browsing candidates feel more responsive

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.16
```

## v0.0.15 — 2026-06-30

Patch release fixing a routing issue that caused `HEAD` requests to static routes to return 405.

### What's Changed

#### Fixes
- Static routes now correctly handle `HEAD` requests (previously returned 405 Method Not Allowed)

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.15
```

## v0.0.14 — 2026-06-30

UI simplification release — removes the grouped series view and improves default sort ordering.

### What's Changed

#### Improvements
- **Removed "By Series" view** from the Library page, simplifying navigation to a single flat list with less code to maintain
- **Default sort changed to "recently modified"** on the Browse and Candidates pages, surfacing newly added or changed files first

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.14
```

## v0.0.13 — 2026-06-30

Internal UI refactor — shared components extracted to improve consistency across all pages.

### What's Changed

#### Improvements
- Extracted reusable `MovieCard`, `MovieRow`, `ShowCard`, and `ShowRow` components, making media display consistent across the Browse, Library, and Candidates pages
- Added a dedicated `EncodeHealthBar` component for cleaner savings visualisation
- Introduced shared `PageHeader`, `EmptyState`, and `Detail` UI primitives, reducing duplication across pages
- Added a `codec.ts` utility to centralise codec display logic

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.13
```

## v0.0.12 — 2026-06-29

This release is a frontend refactor and documentation update with no breaking changes.

### What's Changed

**Improvements**
- Split the monolithic `app-shell` component into focused modules (sidebar, mobile nav, notification bell, scan banner, window arc) — smaller pages, easier to maintain
- Extracted shared media UI (codec badge, candidate state, queue confirm dialog, selection bar, flat row) into a dedicated `components/media/` directory
- Added TMDB API key configuration to docker-compose.yml and Docker docs

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.12
```

## v0.0.11 — 2026-06-29

TMDB metadata integration lands alongside browse navigation improvements and a package manager migration.

### What's Changed

#### Features
- **TMDB metadata**: Movies and TV shows now display posters, backdrops, and enriched detail pulled from The Movie Database. Configure your API key in Settings to enable automatic fetching after each scan.
- **File detail page**: Replaced the slide-out sheet with a dedicated `/browse/file` page, giving more room for stream info, encode history, and metadata.

#### Fixes
- Restored resolution band stats (SD/HD/UHD breakdown) that were dropped in v0.0.10.

#### Improvements
- Removed the dry-run page — it was incomplete and unused.
- Switched all packages (`web/`, `landing/`) from npm to pnpm; lock files and CI updated accordingly.
- Dependency upgrades across Go modules and frontend packages.

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.11
```

## v0.0.10 — 2026-06-29

This release is focused entirely on the new Browse section. Here's the changelog:

---

This release adds a full library browser — browse all your TV shows and movies, drill into seasons and episodes, and see encode health at a glance.

### What's Changed

#### Features
- **Browse page** — new top-level section in the sidebar with TV Shows and Movies tabs, search, and sort controls (A–Z, most savings, largest, most episodes, recently added)
- **TV show detail view** — click any show card to see a season-by-season breakdown with per-episode codec, resolution, size, and estimated savings
- **Movie detail view** — click any movie to see full file metadata (codec, resolution, size, directory, encode state)
- **Encode health bar** — each show card shows a progress bar indicating how many files have already been converted to HEVC
- **Colour-coded codec badges** — H.264, HEVC, MPEG-2, VC-1, and AV1 each get a distinct colour throughout the browser

#### Improvements
- Buttons across the app now show `cursor-pointer` on hover for clearer interactivity

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.10
```

## v0.0.9 — 2026-06-29

This release focuses on mobile polish and making large libraries more manageable with pagination and smarter filtering.

### What's Changed

**Features**
- Added "Newest file" sort option to the Candidates screen
- Grouped library views (TV series/seasons) are now paginated — large libraries no longer load all at once

**Fixes**
- Fixed layout overlap on full-height pages on mobile (Queue, Settings)
- Added error boundary pages (`error`, `global-error`, `not-found`) to both the app and landing site

**Improvements**
- Tightened mobile responsiveness across Overview, Queue, and Settings pages
- Refined resolution filter options (`sd`/`hd`/`uhd` bands) for more accurate candidate filtering
- Jobs store now tracks additional state for cleaner cancel/complete handling

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.9
```

## v0.0.8 — 2026-06-25

Here's the release changelog:

---

v0.0.8 ships a new Library page for browsing every file in your media collection, alongside the backend endpoint that powers it.

### What's Changed

#### Features
- **Library page** — new `/library` view lets you browse all scanned files (not just encode candidates), with filtering by library type, codec, resolution, and candidate state, plus five sort options and virtualized infinite scroll for large collections
- **Files API** — new `GET /api/files` endpoint backing the Library page, with full filter and sort support mirroring the candidates endpoint

#### Improvements
- Filter dropdowns on Candidates and Dry Run pages no longer show `Unknown` as a selectable codec option
- Active filter selections are now persisted in the URL via query parameters, so page state survives a refresh
- TypeScript type-checking added to the CI pipeline for the web package
- Landing page copy and layout refreshed

### Docker

```sh
docker pull ghcr.io/ReeceRose/reclaim:0.0.8
```

## v0.0.7 — 2026-06-25

Here's the changelog:

---

This release brings richer file inspection, smarter filter dropdowns, and percentage breakdowns across all dashboard stats.

### What's Changed

#### Features
- **File detail panel** — click any file to open a slide-out sheet showing codec, profile, resolution, bitrate, duration, audio channels, container format, file path, and probe errors
- **Stats percentages** — the dashboard now shows file count and size breakdowns by codec, resolution band (SD/HD/4K), and library type

#### Improvements
- Filter dropdowns for codec, resolution, and library type are now populated from real library stats instead of static lists, so only relevant options appear
- Added `GET /api/files/:id` endpoint to support fetching full file detail on demand

#### Fixes
- Fixed several dropdown components that were not rendering or behaving correctly
- Resolved minor TypeScript diagnostic warnings and errors across the frontend

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.7
```

## v0.0.6 — 2026-06-25

Here's the changelog:

---

Live scan progress is now visible in the sidebar while your library is being indexed.

### What's Changed

#### Features
- **Live scan progress in sidebar** — while a scan is running, the "Scanning library…" indicator now shows a live sub-line with files indexed, files found, and error count, updated in real time via WebSocket (`scan_progress` events)
- **Library stats refresh during scans** — stats and candidates now poll for updates every 2 seconds while a scan is in progress, so the dashboard reflects newly indexed files without waiting for the scan to finish

### Docker

```sh
docker pull ghcr.io/ReeceRose/reclaim:0.0.6
```

## v0.0.5 — 2026-06-25

Here's the changelog:

---

Startup scans now broadcast WebSocket events so the UI scanning banner appears correctly on page load, not just for manually triggered scans.

### What's Changed

**Improvements**

- **Startup scan now emits WS events** — the `scan_started` / `scan_completed` / `scan_failed` events are now fired for the initial startup scan, not only for manual or scheduled scans. Clients that connect mid-scan receive a retained `scan_started` message so they see the scanning banner regardless of when they join.
- **Scan lifecycle moved into the scanner** — scan event broadcasting was centralised inside `scanner.Scan()` rather than scattered across HTTP handlers, so all scan triggers (startup, scheduled, manual) share one consistent code path.
- **Overlapping scan safety** — the WebSocket hub now tracks concurrent in-flight scans (startup + manual can overlap) and clears the banner only when the last one finishes.

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.5
```

## v0.0.4 — 2026-06-25

v0.0.4 adds a marketing landing page for Reclaim, deployed separately to Vercel.

### What's Changed

**Features**
- Added a standalone landing page (`landing/`) with hero, nav, and feature sections — deployed to [reclaim.reecerose.com](https://reclaim.reecerose.com) via Vercel

**Fixes**
- Fixed a tag issue in the release workflow

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.4
```

## v0.0.3 — 2026-06-25

Focused on stability, mobile usability, and a polished UI with a new logo and improved event handling.

### What's Changed

**Features**
- Added a new Reclaim logo and branding assets (SVG, PNG, PWA icons, web manifest)
- Tooltips on encode preset options for clearer guidance
- Candidate list now uses paginated/keyset scrolling for large libraries
- New events store with a dedicated notification panel

**Fixes**
- WebSocket connection no longer breaks in the dev environment
- Fixed a missing `web/out/index.html` that caused the embedded frontend to 404 on fresh builds
- Resolved duplicate Docker image publishes in CI

**Improvements**
- All pages are now mobile-friendly with responsive layouts
- App shell and navigation reworked for smaller screens
- HTTP response caching headers added to static assets
- Scanner and worker internals cleaned up; dead code removed
- Docker Compose config simplified; new `docs/DOCKER.md` covers setup end-to-end
- API docs (`docs/API.md`) expanded to cover all current endpoints

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.3
```

## v0.0.2 — 2026-06-25

This release adds in-app event notifications and cleans up CI/CD pipeline noise.

### What's Changed

#### Features
- **Event notifications** — a new notification panel in the app shell surfaces real-time job and scan events (started, completed, failed, cancelled) via WebSocket, with a persistent event log stored in SQLite
- **Filter select component** — reusable filter dropdown added to the candidates and queue views

#### Improvements
- Scanner and worker now emit structured events consumed by the notification system
- Dry-run, candidates, queue, and settings pages refreshed with layout and UX improvements
- `_next` asset requests are excluded from middleware logging, reducing log noise in production

#### Fixes
- Docker images are no longer built and pushed on every commit to `main` — releases are now gated to tagged versions via a dedicated workflow

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.2
```

## v0.0.1 — 2026-06-24

First public release of Reclaim — a self-hosted tool for auditing and re-encoding media libraries to HEVC.

### What's Changed

**Features**
- Full scanner with `ffprobe`-backed library indexing, fsnotify watcher, and rename detection via file fingerprinting
- Candidate ranking by predicted HEVC savings, with filters for codec, resolution, and library type
- Encode worker with crash-safe atomic swap (`original → backup → tmp → original`) and post-encode ffprobe verification
- Learned savings ratios: completed jobs update per-codec savings estimates for more accurate future predictions
- Dry-run projection: estimate savings for any candidate set without queuing anything
- WebSocket hub broadcasting real-time job progress, completion, and scan events
- HMAC-signed session cookie auth with first-run setup flow; `DISABLE_AUTH=true` for dev

**Improvements**
- Echo v5 HTTP server with full REST API (`/api/candidates`, `/api/jobs`, `/api/settings`, `/api/dry-run`, and more)
- Embedded Next.js static frontend served directly from the Go binary — single container, no external runtime
- SQLite with WAL mode, goose migrations, and separate read/write pools
- Reduced Docker image size by excluding build artifacts from the final layer

### Docker

```
docker pull ghcr.io/ReeceRose/reclaim:0.0.1
```
