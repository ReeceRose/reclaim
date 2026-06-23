package store

import (
	"context"
	"fmt"
)

func (m *Media) touch(ctx context.Context, id int64) error {
	_, err := m.w.ExecContext(ctx,
		"UPDATE media_files SET mtime = mtime WHERE id = ?", id,
	)
	return err
}

func (m *Media) insertTestRow(ctx context.Context, path string) (int64, error) {
	return m.Insert(ctx, &MediaFile{
		Path:        path,
		LibraryType: "movie",
		SizeBytes:   1000,
		Mtime:       1,
		Fingerprint: fmt.Sprintf("fp-%s", path),
		Status:      "active",
	})
}
