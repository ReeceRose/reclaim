# Releasing Reclaim

## Prerequisites

- Push access to the repo
- `claude` CLI installed and authenticated (`npm install -g @anthropic-ai/claude-code`)
- `gh` CLI installed and authenticated (`gh auth login`)

## How to release

```bash
./scripts/release.sh          # bump patch:  v1.2.3 → v1.2.4
./scripts/release.sh minor    # bump minor:  v1.2.3 → v1.3.0
./scripts/release.sh major    # bump major:  v1.2.3 → v2.0.0
./scripts/release.sh v1.5.0   # explicit version
```

The script will show the current and new tag, ask for confirmation, then create an annotated tag and push it.

## What happens

The script runs entirely locally:

1. Collects commits since the last tag
2. Calls `claude -p` to generate changelog notes
3. Shows a preview and asks for confirmation
4. Creates an annotated tag, pushes it, and creates the GitHub Release via `gh`

Pushing the tag also triggers the **Docker** GitHub Actions workflow, which builds the container and pushes to `ghcr.io/<owner>/reclaim` with semver tags (`1.2.4`, `1.2`, `sha-abc1234`). The Docker image will be ready ~3–5 min after the tag is pushed.

## Pulling a released image

```bash
docker pull ghcr.io/reecerose/reclaim:1.2.4   # exact version (recommended)
docker pull ghcr.io/reecerose/reclaim:1.2      # latest patch of 1.2.x
```

Deployment steps (ports, volumes, env vars, Unraid): [`docs/DOCKER.md`](DOCKER.md).

## If the changelog generation fails

The script falls back to a raw commit list if `claude` isn't available or errors. You can also edit the release notes afterwards on GitHub (`Releases → Edit`).
