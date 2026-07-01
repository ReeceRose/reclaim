# syntax=docker/dockerfile:1

# ── Stage 1: Build the Next.js frontend ──────────────────────────────────────
FROM node:22-alpine AS frontend-build
WORKDIR /build
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build
# Output lands in /build/out — copied into the Go source tree before compile.

# ── Shared Go compile dependencies ─────────────────────────────────────────────
FROM golang:1.26-alpine AS go-base
WORKDIR /src

# Fetch deps first so this layer is cached until go.mod/go.sum change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd/ cmd/
COPY internal/ internal/
COPY web/embed.go web/embed.go

# ── Go build with frontend compiled inside Docker (default / release tags) ─────
FROM go-base AS go-build
COPY --from=frontend-build /build/out web/out
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build \
      -ldflags="-s -w" \
      -trimpath \
      -o /reclaim \
      ./cmd/reclaim

# ── Go build with pre-built frontend (CI — web/out supplied as build context) ──
FROM go-base AS go-build-prebuilt
COPY web/out web/out
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build \
      -ldflags="-s -w" \
      -trimpath \
      -o /reclaim \
      ./cmd/reclaim

# ── Minimal final image ────────────────────────────────────────────────────────
# Pin Alpine minor + ffmpeg package so encode behavior stays stable across rebuilds.
FROM alpine:3.21 AS runtime-base

ARG FFMPEG_VERSION=6.1.2-r1
RUN apk add --no-cache "ffmpeg=${FFMPEG_VERSION}" su-exec && \
    adduser -D -H -u 1000 reclaim

LABEL org.opencontainers.image.source="https://github.com/ReeceRose/reclaim" \
      org.reclaim.ffmpeg.version="${FFMPEG_VERSION}"

COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Stays root at startup so the entrypoint can create a uid/gid matching
# PUID/PGID and drop to it via su-exec before exec'ing the binary.
EXPOSE 8080
ENTRYPOINT ["/docker-entrypoint.sh"]

# CI validate — reuses web/out artifact from the Frontend job (see ci.yml).
FROM runtime-base AS release-prebuilt
COPY --from=go-build-prebuilt /reclaim /reclaim

# Default image — full build from source (docker compose, local, release tags).
FROM runtime-base AS release
COPY --from=go-build /reclaim /reclaim
