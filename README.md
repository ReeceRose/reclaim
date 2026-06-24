# Reclaim

Self-hosted media codec audit and re-encode tool. Scans your Plex/NAS libraries via `ffprobe`, ranks files by predicted HEVC savings, and lets you manually queue re-encodes that run through `ffmpeg` in a configurable overnight window.

Single self-contained binary: Go backend + embedded Next.js frontend. No external runtime dependencies.

---

## Building from source

The Dockerfile is a four-stage build: Next.js frontend → static ffmpeg binaries → Go binary (with the frontend embedded) → minimal distroless image.

**Requirements:** Docker 24+ (BuildKit enabled by default).

```bash
# Build the image locally
docker build -t reclaim:latest .

# Or let Compose build and start in one step
docker compose up --build
```

The compose file (`docker-compose.yml`) in the repo root is configured for self-hosting. Edit the volume paths and `TZ` environment variable before running.

---

## Deployment

### Docker Compose (recommended)

```yaml
services:
  reclaim:
    image: ghcr.io/you/reclaim:latest
    ports:
      - "8080:8080"
    volumes:
      - /mnt/movies:/movies:rw
      - /mnt/tv:/tv:rw
      - reclaim-data:/data
    environment:
      MOVIES_PATH: /movies
      TV_PATH: /tv
      DB_PATH: /data/reclaim.db
      ENCODE_WINDOW_START: "00:00"
      ENCODE_WINDOW_END: "06:00"
      SCAN_INTERVAL: 24h
      PROBE_CONCURRENCY: 4

volumes:
  reclaim-data:
```

The media mounts **must be read-write** — Reclaim replaces files in-place after encoding.

### Standalone binary

```bash
export MOVIES_PATH=/mnt/movies
export TV_PATH=/mnt/tv
export DB_PATH=/var/lib/reclaim/reclaim.db
./reclaim
```

---

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `MOVIES_PATH` | yes | — | Absolute path to the movies root (rw) |
| `TV_PATH` | yes | — | Absolute path to the TV root (rw) |
| `DB_PATH` | yes | — | SQLite database file path |
| `ENCODE_WINDOW_START` | no | `00:00` | Start of the encode window (HH:MM, 24h local time) |
| `ENCODE_WINDOW_END` | no | `06:00` | End of the encode window |
| `SCAN_INTERVAL` | no | `24h` | How often a diff-based rescan runs (Go duration string) |
| `PROBE_CONCURRENCY` | no | `4` | Max parallel ffprobe calls during a scan |
| `DISABLE_AUTH` | no | `false` | Bypass login entirely — **trusted LAN use only** |
| `RESET_AUTH` | no | `false` | Clear stored credentials on boot, re-triggering first-run setup |

---

## First-run setup

On first boot, navigate to `http://<host>:8080`. You will be prompted to create a username and password. This is stored bcrypt-hashed in the database; the plaintext never leaves your browser.

After setup, login is via a signed HTTP-only session cookie (same model as Sonarr/Radarr Forms auth). Credentials can be changed at any time from the Settings page without a restart.

### Auth escape hatches

- **`DISABLE_AUTH=true`** — skips the login gate entirely. Useful on a fully trusted LAN where you don't want the overhead. Don't expose port 8080 to the internet with this set.
- **`RESET_AUTH=true`** — clears credentials on boot, sending you back to the first-run setup page. Use this if you've lost your password. Remove the variable after resetting.

---

## How it works

1. **Scan** — walks `MOVIES_PATH` and `TV_PATH` with `ffprobe`, recording codec, resolution, duration, bitrate, and a fingerprint (sha256 of size + first/last 64 KB) per file. Subsequent scans are diff-based: files with unchanged `(size, mtime)` are skipped. Renames are detected via fingerprint and recorded as moves, preserving job history.

2. **Rank** — files are ranked by predicted HEVC savings (`size × (1 − expected_output_ratio[codec])`). The seed ratios are conservative rule-of-thumb constants per codec. After enough completed jobs accumulate for a given codec (≥ 10 by default), Reclaim replaces the seed ratio with the observed `mean(output_size / original_size)` from your own encodes, labelled "learned" in the UI.

3. **Queue** — you browse the candidate list, multi-select files, pick an encode profile (CRF + preset), and confirm the resolved selection before anything is queued. No silent bulk operations.

4. **Encode** — the worker picks up queued jobs inside the encode window only. It encodes to a `.reclaim-tmp` temp file, then:
   - Verifies the output (duration ±1 s, stream counts, resolution match)
   - On pass: atomically swaps original → `.reclaim-backup`, temp → original, deletes backup
   - On fail: marks the job failed, keeps the temp for inspection, leaves the original untouched

5. **Crash recovery** — on boot, Reclaim sweeps for orphaned `.reclaim-tmp` (deleted) and `.reclaim-backup` files (restored if the original is missing). Jobs stuck in `running`/`verifying` are reconciled to `failed`.

---

## Throughput expectations

CPU x265 is slow by design — it trades CPU time for the best compression. Realistic throughput on modern hardware:

| Preset | Typical speed | 1-hour HD file |
|---|---|---|
| `medium` | ~0.5–1× realtime | 1–2 hours |
| `fast` | ~2–3× realtime | 20–30 min |
| `ultrafast` | ~8–10× realtime | 6–8 min |

**A 20 000-file library at `medium` preset will take months of overnight encode windows.** That is expected and by design — this is a background task that runs while you sleep, not a batch job. The window gate (`ENCODE_WINDOW_START`/`END`) exists precisely so it never competes with daytime playback.

A running job is never interrupted when the window closes — it finishes and no new job is pulled.

---

## Operational notes

### ffmpeg/ffprobe

The binary ships with pinned static builds of ffmpeg and ffprobe. Versions are logged at startup. To upgrade, replace the bundled binaries and rebuild the image.

### Network mounts (NFS/SMB)

`fsnotify` does not work reliably over NFS or SMB — kernel inotify events are not propagated from the remote side. Reclaim logs a warning at startup when it detects a network mount. The scheduled `SCAN_INTERVAL` rescan will still catch new files; you just won't get the ~30 s watcher-triggered probe.

On Linux, large libraries (20 000+ directories) can hit the default inotify watch limit (`/proc/sys/fs/inotify/max_user_watches`). Increase it if the startup log reports watch failures:

```bash
echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

### SQLite WAL files

The database runs in WAL mode. Two auxiliary files (`reclaim.db-wal`, `reclaim.db-shm`) will appear next to the database file on the volume. This is normal. Do not delete them while Reclaim is running. They are safe to delete when Reclaim is stopped (they will be recreated on next boot).

### In-place replace safety

Reclaim never deletes the original file before verifying the encode. The replace sequence is:

```
original → original.reclaim-backup   (rename, atomic)
tmp      → original                  (rename, atomic)
delete original.reclaim-backup
```

Both renames are within the same directory, so they are atomic on any single filesystem. A crash between steps is recovered on next boot.

**Do not store media on a filesystem that does not support atomic rename** (some network shares under certain configurations). If in doubt, test with a single file before queuing your library.
