# syntax=docker/dockerfile:1

# ── Stage 1: Build the Next.js frontend ──────────────────────────────────────
FROM node:22-alpine AS frontend-build
WORKDIR /build
COPY web/package*.json ./
RUN npm ci --prefer-offline
COPY web/ ./
RUN npm run build
# Output lands in /build/out — copied into the Go source tree before compile.

# ── Stage 2: Static ffmpeg/ffprobe binaries ───────────────────────────────────
# Pin by tag; bump deliberately when upgrading ffmpeg.
FROM mwader/static-ffmpeg:7.1 AS ffmpeg-bins

# ── Stage 3: Build the Go binary ─────────────────────────────────────────────
FROM golang:1.26-alpine AS go-build
WORKDIR /src

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
      ./cmd/reclaim

# ── Stage 4: Minimal final image ─────────────────────────────────────────────
# distroless/static has no shell and no package manager — attack surface is
# just the Go binary + the two statically-linked ffmpeg executables.
# The nonroot variant runs as uid 65532 by default.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=go-build   /reclaim               /reclaim
COPY --from=ffmpeg-bins /ffmpeg               /usr/local/bin/ffmpeg
COPY --from=ffmpeg-bins /ffprobe              /usr/local/bin/ffprobe

EXPOSE 8080
ENTRYPOINT ["/reclaim"]
