package store

import (
	"database/sql"
	"embed"
	"sync"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

var (
	gooseOnce sync.Once
	gooseErr  error
)

// initGoose configures goose globals exactly once. Concurrent callers block
// until initialization completes, then all read the same gooseErr.
func initGoose() error {
	gooseOnce.Do(func() {
		goose.SetBaseFS(migrationFS)
		gooseErr = goose.SetDialect("sqlite3")
	})
	return gooseErr
}

func runMigrations(db *sql.DB) error {
	if err := initGoose(); err != nil {
		return err
	}
	return goose.Up(db, "migrations")
}

func migrationVersion(db *sql.DB) (int64, error) {
	if err := initGoose(); err != nil {
		return 0, err
	}
	return goose.GetDBVersion(db)
}
