# syntax=docker/dockerfile:1

# ── Stage 1: Build the Next.js frontend ──────────────────────────────────────
FROM node:22-alpine AS frontend-build
WORKDIR /build
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build
# Output lands in /build/out — copied into the Go source tree before compile.

# ── Stage 2: Build the Go binary ─────────────────────────────────────────────
FROM golang:1.26-alpine AS go-build
WORKDIR /src

RUN apk add --no-cache upx

# Fetch deps first so this layer is cached until go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Overlay the compiled frontend so the embed.FS picks it up at build time.
COPY --from=frontend-build /build/out ./web/out

RUN CGO_ENABLED=0 GOOS=linux \
    go build \
      -ldflags="-s -w" \
      -trimpath \
      -o /reclaim \
      ./cmd/reclaim && \
    upx --best /reclaim

# ── Stage 3: Minimal final image ─────────────────────────────────────────────
# Pin Alpine minor + ffmpeg package so encode behavior stays stable across rebuilds.
FROM alpine:3.21

ARG FFMPEG_VERSION=6.1.2-r1
RUN apk add --no-cache "ffmpeg=${FFMPEG_VERSION}" su-exec && \
    adduser -D -H -u 1000 reclaim

LABEL org.opencontainers.image.source="https://github.com/ReeceRose/reclaim" \
      org.reclaim.ffmpeg.version="${FFMPEG_VERSION}"

COPY --from=go-build /reclaim /reclaim
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Stays root at startup so the entrypoint can create a uid/gid matching
# PUID/PGID and drop to it via su-exec before exec'ing the binary.
EXPOSE 8080
ENTRYPOINT ["/docker-entrypoint.sh"]
