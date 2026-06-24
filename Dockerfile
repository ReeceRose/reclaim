# syntax=docker/dockerfile:1

# ── Stage 1: Build the Next.js frontend ──────────────────────────────────────
FROM node:22-alpine AS frontend-build
WORKDIR /build
COPY web/package*.json ./
RUN npm ci --prefer-offline
COPY web/ ./
RUN npm run build
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
FROM alpine:3

RUN apk add --no-cache ffmpeg && \
    adduser -D -H -u 1000 reclaim

COPY --from=go-build /reclaim /reclaim

USER reclaim
EXPOSE 8080
ENTRYPOINT ["/reclaim"]
