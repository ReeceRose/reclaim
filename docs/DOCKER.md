# Docker deployment

Reclaim runs as a single container: Go API + embedded web UI + ffmpeg/ffprobe. Point it at your movie and TV libraries, keep the database on persistent storage, and open the web UI to scan and queue encodes.

**Full deployment reference** — use this page for Unraid, Synology, TrueNAS, or any Docker host.

---

## Quick reference

| Item | Value |
|---|---|
| **Web UI** | `http://<host>:8080` |
| **Host port** | `8080` → container `8080` |
| **Health check** | `GET /healthz` (no auth) |
| **Image (releases)** | `ghcr.io/reecerose/reclaim:<version>` |
| **Compose file** | [`docker-compose.yml`](../docker-compose.yml) in the repo root |

### Required volumes

All three mounts are required. Media paths **must be read-write** — Reclaim replaces files in-place after a successful encode.

| Container path | Purpose | Example host path (Unraid) |
|---|---|---|
| `/movies` | Movie library root | `/mnt/user/media/movies` |
| `/tv` | TV library root | `/mnt/user/media/tv` |
| `/data` | SQLite database | `/mnt/user/appdata/reclaim` |

On Unraid, map `/data` to an `appdata` folder (not `system`). The DB file is `reclaim.db` inside that folder; WAL sidecar files (`reclaim.db-wal`, `reclaim.db-shm`) are normal.

### Required environment variables

| Variable | Example | Description |
|---|---|---|
| `MOVIES_PATH` | `/movies` | Must match the movies volume mount |
| `TV_PATH` | `/tv` | Must match the TV volume mount |
| `DB_PATH` | `/data/reclaim.db` | SQLite file on the data volume |

### Recommended environment variables

| Variable | Default | Description |
|---|---|---|
| `TZ` | container default | **Set this.** Encode window times use local time |
| `ENCODE_WINDOW_START` | `00:00` | Start of overnight encode window (`HH:MM`, 24h) |
| `ENCODE_WINDOW_END` | `06:00` | End of encode window |
| `SCAN_INTERVAL` | `24h` | Diff-based rescan interval (Go duration) |
| `PROBE_CONCURRENCY` | `4` | Parallel `ffprobe` calls during scans |
| `SCAN_ANCHOR` | `00:00` | Daily scan anchor time (`HH:MM`) |

### Optional (recovery only)

| Variable | Default | Description |
|---|---|---|
| `DISABLE_AUTH` | `false` | Skip login — trusted LAN only, never expose to the internet |
| `RESET_AUTH` | `false` | Clear credentials on boot → first-run setup again |

---

## First run

1. Start the container (see options below).
2. Open `http://<nas-ip>:8080`.
3. Create a username and password on the setup page.
4. Reclaim scans your libraries on startup. Browse **Candidates**, select files, pick a profile, and queue encodes.
5. Encodes run only inside `ENCODE_WINDOW_START`–`ENCODE_WINDOW_END` (local time per `TZ`). A job already running when the window closes is allowed to finish.

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

| Field | Value |
|---|---|
| **Name** | `reclaim` |
| **Repository** | `ghcr.io/reecerose/reclaim:latest` (or a pinned `:1.0.0` tag) |
| **Network Type** | `bridge` |
| **WebUI** | `8080` |
| **Console shell** | `sh` (for debugging only) |

**Path mappings** (adjust host paths to your shares):

| Container | Host | Access |
|---|---|---|
| `/movies` | `/mnt/user/media/movies` | Read/Write |
| `/tv` | `/mnt/user/media/tv` | Read/Write |
| `/data` | `/mnt/user/appdata/reclaim` | Read/Write |

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
```

**Extra Parameters** (optional, adds Docker health check):

```
--health-cmd="wget -qO- http://127.0.0.1:8080/healthz || exit 1" --health-interval=30s --health-timeout=5s --health-retries=3 --health-start-period=20s
```

### Building on Unraid

If no release image is available yet, clone the repo on any machine with Docker, run `docker compose build`, then load/save the image, or build directly on Unraid if the Docker folder is on a share with the source.

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
  --health-cmd="wget -qO- http://127.0.0.1:8080/healthz || exit 1" \
  --health-interval=30s \
  --health-timeout=5s \
  --health-retries=3 \
  --health-start-period=20s \
  ghcr.io/reecerose/reclaim:latest
```

Synology **Container Manager**, TrueNAS **Apps**, and Proxmox LXC-with-Docker follow the same mapping: one published port, three volumes, same env vars.

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

`fsnotify` does not work reliably over NFS or SMB. Reclaim logs a warning and relies on `SCAN_INTERVAL` to pick up new files. This is expected — you will not get instant watcher-triggered probes on remote shares.

### Large libraries (inotify limit)

On Linux hosts with 20,000+ directories, raise the inotify watch limit if startup logs report watch failures:

```bash
echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

### Encode throughput

CPU x265 is slow by design. A large library at the `medium` preset can take months of overnight windows. See the throughput table in [`README.md`](../README.md#throughput-expectations).

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
