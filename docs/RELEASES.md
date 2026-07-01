# Releasing Reclaim

## Prerequisites

- Push access to the repo
- `claude` CLI installed and authenticated (`npm install -g @anthropic-ai/claude-code`)
- `gh` CLI installed and authenticated (`gh auth login`)

## How to release

```bash
./scripts/release.sh          # bump patch:  v0.0.20 → v0.0.21
./scripts/release.sh minor    # bump minor:  v0.0.20 → v0.1.0
./scripts/release.sh major    # bump major:  v0.0.20 → v1.0.0
./scripts/release.sh v0.0.21  # explicit version
```

The script will show the current and new tag, ask for confirmation, then create an annotated tag and push it.

## What happens

The script runs entirely locally:

1. Collects commits since the last tag
2. Calls `claude -p` to generate changelog notes
3. Shows a preview and asks for confirmation
4. Creates an annotated tag, pushes it, and creates the GitHub Release via `gh`

Pushing the tag triggers the **CI** workflow:

1. **Go** — vet, unit tests, and race detector (`scanner`, `worker`, `jobs`, `api`, `store`)
2. **Frontend** — lint, type-check, and static export build
3. **Landing** — lint, type-check, and build (marketing site on Vercel)
4. **Docker** — runs only after Go and frontend pass; on tags, builds and pushes the container

The image is pushed to `ghcr.io/<owner>/reclaim` with a single semver tag matching the release version (e.g. tag `v0.0.21` → image `ghcr.io/reecerose/reclaim:0.0.21`). The image will be ready ~3–5 min after the tag is pushed.

## Pulling a released image

```bash
docker pull ghcr.io/reecerose/reclaim:0.0.21   # exact version (recommended)
```

Pin to the full `x.y.z` tag — CI does not publish floating `x.y` or `latest` tags.

Deployment steps (ports, volumes, env vars, Unraid): [`docs/DOCKER.md`](DOCKER.md).

## If the changelog generation fails

The script falls back to a raw commit list if `claude` isn't available or errors. You can also edit the release notes afterwards on GitHub (`Releases → Edit`).
