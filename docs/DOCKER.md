# Docker deployment

Reclaim runs as one container: Go API + web UI + ffmpeg/ffprobe. Mount the same movie and TV folders your media server uses, plus persistent storage for SQLite.

It reads the filesystem directly. No Sonarr, Radarr, Plex, Jellyfin, or Emby API setup is required.

---

## Quick reference

| Item                 | Value                                                          |
| -------------------- | -------------------------------------------------------------- |
| **Web UI**           | `http://<host>:8080`                                           |
| **Host port**        | `8080` → container `8080`                                      |
| **Health check**     | `GET /healthz` (no auth)                                       |
| **Image (releases)** | `ghcr.io/reecerose/reclaim:<version>`                          |
| **Compose file**     | [`docker-compose.yml`](../docker-compose.yml) in the repo root |

### Required volumes

All three mounts are required. Media paths **must be read-write**.

| Container path | Purpose            | Example host path (Unraid)  |
| -------------- | ------------------ | --------------------------- |
| `/movies`      | Movie library root | `/mnt/user/media/movies`    |
| `/tv`          | TV library root    | `/mnt/user/media/tv`        |
| `/data`        | SQLite database    | `/mnt/user/appdata/reclaim` |

On Unraid, map `/data` to `appdata`. The repo Compose file uses a named volume for quick trials; NAS users may prefer a host path like `/mnt/user/appdata/reclaim:/data`.

### Required environment variables

| Variable      | Example            | Description                        |
| ------------- | ------------------ | ---------------------------------- |
| `MOVIES_PATH` | `/movies`          | Must match the movies volume mount |
| `TV_PATH`     | `/tv`              | Must match the TV volume mount     |
| `DB_PATH`     | `/data/reclaim.db` | SQLite file on the data volume     |

### Recommended environment variables

| Variable              | Default           | Description                                                                             |
| --------------------- | ----------------- | --------------------------------------------------------------------------------------- |
| `TZ`                  | container default | **Set this.** Encode window times use local time                                        |
| `PUID`                | `1000`            | Set to the uid that owns your media library so the container can write re-encoded files |
| `PGID`                | `1000`            | Set to the gid that owns your media library                                             |
| `ENCODE_WINDOW_START` | `00:00`           | Start of overnight encode window (`HH:MM`, 24h)                                         |
| `ENCODE_WINDOW_END`   | `06:00`           | End of encode window                                                                    |
| `SCAN_INTERVAL`       | `24h`             | Diff-based rescan interval (Go duration)                                                |
| `PROBE_CONCURRENCY`   | `4`               | Parallel `ffprobe` calls during scans                                                   |
| `SCAN_ANCHOR`         | `00:00`           | Daily scan anchor time (`HH:MM`)                                                        |
| `TMDB_API_KEY`        | unset             | Optional TMDB API key for movie/TV posters, backdrops, and metadata                     |

`PUID`/`PGID` matter because the container writes re-encoded files back into `/movies` and `/tv`. If they don't match the uid/gid that owns your library on the host, encodes fail with `Permission denied` (verification never even runs). On Unraid, find the right values with `id nobody` on the host (typically `99`/`100`); on most other Linux hosts, `id <your-user>`.

To enable artwork and metadata fetching, create a TMDB API key at [themoviedb.org/settings/api](https://www.themoviedb.org/settings/api) and set `TMDB_API_KEY` on the container. Restart Reclaim after changing it, then run a scan or refresh metadata from the UI.

### Optional (recovery only)

| Variable       | Default | Description                                                 |
| -------------- | ------- | ----------------------------------------------------------- |
| `DISABLE_AUTH` | `false` | Skip login — trusted LAN only, never expose to the internet |
| `RESET_AUTH`   | `false` | Clear credentials on boot → first-run setup again           |

---

## First run

1. Start the container (see options below).
2. Open `http://<nas-ip>:8080`.
3. Create a username and password on the setup page.
4. Let the startup scan finish, then queue files from **Candidates**.
5. Encodes run only inside the configured window unless you use **Force** on a queued job.

For HTTPS reverse proxies, forward `X-Forwarded-Proto: https`.

---

## Option A — Docker Compose (recommended)

Edit paths and timezone in [`docker-compose.yml`](../docker-compose.yml), then:

```bash
# Build from source (or pull a release image — see below)
docker compose up --build -d

# Check health
docker compose ps
curl -s http://localhost:8080/healthz
```

### Using a release image

Tagged releases publish to GitHub Container Registry. See [`docs/RELEASES.md`](RELEASES.md).

```bash
docker pull ghcr.io/reecerose/reclaim:1.0.0   # pin a version
```

In `docker-compose.yml`, comment out `build: .` and set `image:` to the tag you pulled.

---

## Option B — Unraid

### Add Container (manual template)

| Field             | Value                                                         |
| ----------------- | ------------------------------------------------------------- |
| **Name**          | `reclaim`                                                     |
| **Repository**    | `ghcr.io/reecerose/reclaim:latest` (or a pinned `:1.0.0` tag) |
| **Network Type**  | `bridge`                                                      |
| **WebUI**         | `8080`                                                        |
| **Console shell** | `sh` (for debugging only)                                     |

**Path mappings** (adjust host paths to your shares):

| Container | Host                        | Access     |
| --------- | --------------------------- | ---------- |
| `/movies` | `/mnt/user/media/movies`    | Read/Write |
| `/tv`     | `/mnt/user/media/tv`        | Read/Write |
| `/data`   | `/mnt/user/appdata/reclaim` | Read/Write |

**Environment variables:**

```
MOVIES_PATH=/movies
TV_PATH=/tv
DB_PATH=/data/reclaim.db
TZ=America/New_York
ENCODE_WINDOW_START=00:00
ENCODE_WINDOW_END=06:00
SCAN_INTERVAL=24h
PROBE_CONCURRENCY=4
# Optional: enables TMDB posters, backdrops, and metadata
TMDB_API_KEY=your_api_key_here
```

**Extra Parameters** (optional, adds Docker health check):

```
--health-cmd="wget -qO- http://127.0.0.1:8080/healthz || exit 1" --health-interval=30s --health-timeout=5s --health-retries=3 --health-start-period=20s
```

### Building on Unraid

If no release image is available, build on any Docker host and import the image into Unraid.

---

## Option C — `docker run` (generic NAS / CLI)

Replace host paths and timezone:

```bash
docker run -d \
  --name reclaim \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /mnt/movies:/movies:rw \
  -v /mnt/tv:/tv:rw \
  -v /mnt/appdata/reclaim:/data:rw \
  -e MOVIES_PATH=/movies \
  -e TV_PATH=/tv \
  -e DB_PATH=/data/reclaim.db \
  -e TZ=America/New_York \
  -e ENCODE_WINDOW_START=00:00 \
  -e ENCODE_WINDOW_END=06:00 \
  -e SCAN_INTERVAL=24h \
  -e PROBE_CONCURRENCY=4 \
  -e TMDB_API_KEY=your_api_key_here \
  --health-cmd="wget -qO- http://127.0.0.1:8080/healthz || exit 1" \
  --health-interval=30s \
  --health-timeout=5s \
  --health-retries=3 \
  --health-start-period=20s \
  ghcr.io/reecerose/reclaim:latest
```

Synology, TrueNAS, and Proxmox follow the same pattern: one port, three volumes, same env vars.

---

## Upgrading

1. Pull the new image tag (or rebuild from source).
2. Recreate the container with the **same volume mounts** — the database lives on `/data`.
3. Open the UI and confirm the dashboard loads.

```bash
docker compose pull    # if using a registry image
docker compose up -d   # recreates with new image
```

Credentials and job history are in `reclaim.db`; do not delete the data volume unless you intend to start fresh.

---

## Operational notes

### Network mounts (NFS / SMB)

`fsnotify` is unreliable over NFS/SMB, so remote shares rely on `SCAN_INTERVAL` rescans.

### Large libraries (inotify limit)

On Linux hosts with 20,000+ directories, raise the inotify watch limit if startup logs report watch failures:

```bash
echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

### Encode throughput

CPU `libx265` only; no GPU/NVENC. Large libraries can take months of overnight windows.

### Lost password

Set `RESET_AUTH=true`, restart once, complete setup again, then **remove** `RESET_AUTH`.

### ffmpeg version

The image ships ffmpeg from Alpine 3.21 (currently `6.1.2-r1`), pinned in the [`Dockerfile`](../Dockerfile). Rebuild the image to pick up a deliberate ffmpeg bump.

---

## Building from source

```bash
git clone https://github.com/ReeceRose/reclaim.git
cd reclaim
docker build -t ghcr.io/reecerose/reclaim:latest .
```

The Dockerfile is a three-stage build: Next.js static export → Go binary (frontend embedded) → Alpine runtime with pinned ffmpeg.
