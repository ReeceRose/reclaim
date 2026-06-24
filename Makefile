.PHONY: dev build test test-race clean help migrate-new migrate-up migrate-status

GOOSE := go run github.com/pressly/goose/v3/cmd/goose@v3.26.0
MIGRATIONS_DIR := internal/store/migrations

# Local dev directories — created on first run, gitignored
DEV_DIR     := .dev
DEV_MOVIES  := $(DEV_DIR)/movies
DEV_TV      := $(DEV_DIR)/tv
DEV_DATA    := $(DEV_DIR)/data

$(DEV_DIR):
	mkdir -p $(DEV_MOVIES) $(DEV_TV) $(DEV_DATA)

## dev: run Go backend + Next.js dev server concurrently
dev: $(DEV_DIR)
	MOVIES_PATH=$(abspath $(DEV_MOVIES)) \
	TV_PATH=$(abspath $(DEV_TV)) \
	DB_PATH=$(abspath $(DEV_DATA))/reclaim.db \
	ENCODE_WINDOW_START=00:00 \
	ENCODE_WINDOW_END=06:00 \
	SCAN_INTERVAL=24h \
	PROBE_CONCURRENCY=4 \
	go run ./cmd/reclaim & \
	cd web && npm run dev & \
	wait

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

## migrate-new NAME=foo: scaffold a new goose SQL migration
migrate-new:
	@test -n "$(NAME)" || (echo "usage: make migrate-new NAME=description" && exit 1)
	$(GOOSE) -dir $(MIGRATIONS_DIR) create $(NAME) sql

## migrate-up: apply pending migrations to .dev DB (also runs automatically on app boot)
migrate-up: $(DEV_DIR)
	$(GOOSE) -dir $(MIGRATIONS_DIR) sqlite3 $(abspath $(DEV_DATA))/reclaim.db up

## migrate-status: show migration status for .dev DB
migrate-status: $(DEV_DIR)
	$(GOOSE) -dir $(MIGRATIONS_DIR) sqlite3 $(abspath $(DEV_DATA))/reclaim.db status

## help: list targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
