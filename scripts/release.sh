#!/usr/bin/env bash
# Usage: ./scripts/release.sh [major|minor|patch|vX.Y.Z]
set -euo pipefail

BUMP=${1:-patch}

# Get the latest semver tag (default to v0.0.0 if none exist)
LATEST=$(git tag --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1 || true)
LATEST=${LATEST:-v0.0.0}

# Parse components
IFS='.' read -r MAJOR MINOR PATCH <<< "${LATEST#v}"

case "$BUMP" in
  major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
  minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
  patch) PATCH=$((PATCH + 1)) ;;
  v*)    MAJOR="" ;;  # explicit version — skip bump
  *)     echo "Usage: $0 [major|minor|patch|vX.Y.Z]" >&2; exit 1 ;;
esac

if [[ -n "$MAJOR" ]]; then
  NEW_TAG="v${MAJOR}.${MINOR}.${PATCH}"
else
  NEW_TAG="$BUMP"
fi

REPO=$(git remote get-url origin | sed 's/.*github.com[:/]//' | sed 's/\.git$//')

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Warning: you have uncommitted changes. They will NOT be included in this release."
  git status --short
  echo ""
  read -r -p "Continue anyway? [y/N] " DIRTY
  [[ "$DIRTY" =~ ^[Yy]$ ]] || { echo "Aborted. Commit or stash your changes first."; exit 0; }
  echo ""
fi

echo "Current tag : ${LATEST}"
echo "New tag     : ${NEW_TAG}"
echo ""
read -r -p "Create and push ${NEW_TAG}? [y/N] " CONFIRM
[[ "$CONFIRM" =~ ^[Yy]$ ]] || { echo "Aborted."; exit 0; }

# Collect commits since the last tag
if [[ "$LATEST" != "v0.0.0" ]]; then
  RANGE="${LATEST}..HEAD"
else
  RANGE="HEAD"
fi

COMMITS=$(git log "$RANGE" --pretty=format:"- %s (%h)" --no-merges)
DIFF_STAT=$(git diff --stat "${LATEST}..HEAD" 2>/dev/null || git diff --stat HEAD)

echo ""
echo "Generating changelog with Claude..."

PROMPT="You are writing a GitHub release changelog for Reclaim — a self-hosted media codec audit and re-encode tool for Plex/NAS libraries.

Release: ${NEW_TAG}
Previous release: ${LATEST}

Commits in this release:
${COMMITS}

Changed files summary:
${DIFF_STAT}

Write a concise, user-focused release changelog in GitHub-flavoured Markdown. Format it as:
- A short opening sentence describing the overall theme of this release (one line, no heading)
- A \"## What's Changed\" section with bullet points grouped by type (Features, Fixes, Improvements). Use plain English, not commit message jargon. Skip merge commits and version bump commits.
- A \"## Docker\" section with the exact pull command: \`docker pull ghcr.io/${REPO}:${NEW_TAG#v}\`

Keep it tight — no filler, no 'this release includes' boilerplate. Max ~200 words."

NOTES=$(claude -p "$PROMPT" 2>/dev/null || true)

if [[ -z "$NOTES" ]]; then
  echo "Claude not available — using raw commit list."
  NOTES="## What's Changed

${COMMITS}

## Docker

\`\`\`
docker pull ghcr.io/${REPO}:${NEW_TAG#v}
\`\`\`"
fi

echo ""
echo "--- Release notes preview ---"
echo "$NOTES"
echo "-----------------------------"
echo ""
read -r -p "Proceed with these notes? [y/N] " CONFIRM2
[[ "$CONFIRM2" =~ ^[Yy]$ ]] || { echo "Aborted."; exit 0; }

# Tag, push, and create the GitHub release
git tag -a "$NEW_TAG" -m "Release ${NEW_TAG}"
git push origin "$NEW_TAG"

gh release create "$NEW_TAG" \
  --title "Reclaim ${NEW_TAG}" \
  --notes "$NOTES"

echo ""
echo "Released: https://github.com/${REPO}/releases/tag/${NEW_TAG}"
echo "Docker image building at: https://github.com/${REPO}/actions"
