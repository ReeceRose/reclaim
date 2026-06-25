<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="web/public/logo.svg">
    <img alt="Reclaim" src="web/public/logo-light.svg" width="340">
  </picture>
</p>

Self-hosted codec audit and re-encode tool for homelabs. Point it at the same movie and TV folders Plex, Jellyfin, or Emby already use, rank files by predicted HEVC savings, and manually queue overnight `ffmpeg` jobs.

Single container: Go API + embedded web UI + ffmpeg/ffprobe. No database server, Redis, or sidecar services.

---

## What it is

Reclaim is for large libraries with mixed codecs where you want a safe, manual-first way to find the biggest space wins.

| Does | Does not |
|---|---|
| Scans mounted library folders directly | Integrate with Sonarr, Radarr, Plex, Jellyfin, or Emby APIs |
| Ranks candidates by estimated savings | Auto-encode your whole library |
| Replaces files in-place after verification | Use GPU/NVENC hardware encoding (CPU `libx265` only) |
| Runs encodes in a configurable overnight window | Pause for active streams (time window only) |

---

## Quick start (Docker)

Full deployment guide: [`docs/DOCKER.md`](docs/DOCKER.md).

```bash
# Edit media paths and TZ in docker-compose.yml first
docker compose up --build -d
```

Open `http://<nas-ip>:8080`, create your login, and let the first scan run.

Media mounts must be **read-write** because Reclaim replaces files in-place after verification. The included Compose file uses a named DB volume; NAS users may prefer a host `appdata` path as shown in [`docs/DOCKER.md`](docs/DOCKER.md).

### Building the image

Three-stage Dockerfile: Next.js static export → Go binary (frontend embedded) → Alpine 3.21 with pinned ffmpeg.

```bash
docker build -t ghcr.io/reecerose/reclaim:latest .
```

### Standalone binary

If you run outside Docker, you need `ffmpeg` and `ffprobe` on `PATH`:

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
| `SCAN_ANCHOR` | no | `00:00` | Daily scan anchor time (`HH:MM`, local) |
| `TZ` | no | — | Container timezone — **set this in Docker** so the encode window matches your clock |
| `DISABLE_AUTH` | no | `false` | Bypass login entirely — **trusted LAN use only** |
| `RESET_AUTH` | no | `false` | Clear stored credentials on boot, re-triggering first-run setup |

See [`.env.example`](.env.example) for a copy-paste template.

---

## First-run setup

On first boot, open `http://<host>:8080` and create a username/password. The password is bcrypt-hashed in SQLite; login uses a signed HTTP-only cookie.

### Auth escape hatches

- **`DISABLE_AUTH=true`** — skips login; trusted LAN only.
- **`RESET_AUTH=true`** — clears credentials on boot; remove after resetting.

### Behind a reverse proxy

For HTTPS reverse proxies, forward `X-Forwarded-Proto: https` so cookies get the `Secure` flag.

---

## How it works

1. **Scan** — walks `MOVIES_PATH` and `TV_PATH`, probes video files with `ffprobe`, and records codec, resolution, bitrate, size, mtime, and fingerprint. Later scans skip unchanged files and detect renames.

2. **Rank** — files are sorted by predicted HEVC savings. After enough completed jobs for a codec, estimates switch from seed values to your observed results.

3. **Queue** — select files, pick a profile, and confirm before jobs are created.

4. **Encode** — queued jobs run inside the encode window unless forced. Reclaim writes a `.reclaim-tmp` file, then:
   - Verifies the output (duration ±1 s, stream counts, resolution match)
   - On pass: atomically swaps original → `.reclaim-backup`, temp → original, deletes backup
   - On fail: marks the job failed, keeps the temp for inspection, leaves the original untouched

5. **Recover** — on boot, temp files are cleaned up, interrupted backups are restored, and stuck jobs are marked failed.

---

## Throughput expectations

CPU x265 is slow by design. Rough expectations:

| Preset | Typical speed | 1-hour HD file |
|---|---|---|
| `medium` | ~0.5–1× realtime | 1–2 hours |
| `fast` | ~2–3× realtime | 20–30 min |
| `ultrafast` | ~8–10× realtime | 6–8 min |

**A 20 000-file library at `medium` can take months of overnight windows.** Reclaim is meant to chip away safely, not batch-convert everything at once.

A running job is never interrupted when the window closes — it finishes and no new job is pulled.

---

## Operational notes

### ffmpeg/ffprobe

The Docker image includes pinned Alpine ffmpeg/ffprobe. Rebuild to bump ffmpeg deliberately.

### Network mounts (NFS/SMB)

`fsnotify` is unreliable over NFS/SMB. Reclaim falls back to scheduled `SCAN_INTERVAL` rescans for remote shares.

On Linux, large libraries (20 000+ directories) can hit the default inotify watch limit (`/proc/sys/fs/inotify/max_user_watches`). Increase it if the startup log reports watch failures:

```bash
echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

### SQLite WAL files

SQLite WAL sidecars (`reclaim.db-wal`, `reclaim.db-shm`) next to the DB are normal. Do not delete them while Reclaim is running.

### In-place replace safety

Reclaim never deletes the original file before verifying the encode. The replace sequence is:

```
original → original.reclaim-backup   (rename, atomic)
tmp      → original                  (rename, atomic)
delete original.reclaim-backup
```

Both renames happen in the same directory and are recovered on next boot if interrupted. Avoid filesystems that do not support atomic rename.

---

## Further reading

| Doc | Audience |
|---|---|
| [`docs/DOCKER.md`](docs/DOCKER.md) | Homelab deployment (Unraid, Synology, Compose, `docker run`) |
| [`docs/RELEASES.md`](docs/RELEASES.md) | Pulling versioned images from GHCR |
| [`docs/API.md`](docs/API.md) | REST + WebSocket reference for scripting and integrations |
