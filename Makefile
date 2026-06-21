.PHONY: dev build test test-race clean help

# Local dev directories — created on first run, gitignored
DEV_DIR     := .dev
DEV_MOVIES  := $(DEV_DIR)/movies
DEV_TV      := $(DEV_DIR)/tv
DEV_DATA    := $(DEV_DIR)/data

$(DEV_DIR):
	mkdir -p $(DEV_MOVIES) $(DEV_TV) $(DEV_DATA)

## dev: run locally against .dev/ dirs (requires ffmpeg/ffprobe in PATH)
dev: $(DEV_DIR)
	MOVIES_PATH=$(abspath $(DEV_MOVIES)) \
	TV_PATH=$(abspath $(DEV_TV)) \
	DB_PATH=$(abspath $(DEV_DATA))/reclaim.db \
	ENCODE_WINDOW_START=00:00 \
	ENCODE_WINDOW_END=06:00 \
	SCAN_INTERVAL=24h \
	PROBE_CONCURRENCY=4 \
	DISABLE_AUTH=true \
	go run ./cmd/reclaim

## build: compile binary to bin/reclaim
build:
	mkdir -p bin
	go build -o bin/reclaim ./cmd/reclaim

## test: run all tests
test:
	go test ./...

## test-race: race detector on concurrency-sensitive packages (P9 CI gate)
test-race:
	go test -race ./internal/scanner/... ./internal/worker/... ./internal/jobs/...

## clean: remove build output and dev dirs
clean:
	rm -rf bin/ $(DEV_DIR)

## help: list targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
