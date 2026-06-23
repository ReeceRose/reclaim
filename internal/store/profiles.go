package store

import (
	"context"
	"database/sql"
	"errors"
)

type TranscodeProfile struct {
	ID        int64
	Name      string
	CRF       int
	Preset    string
	ExtraArgs *string
	IsDefault bool
}

type Profiles struct {
	r, w *sql.DB
}

func (p *Profiles) List(ctx context.Context) ([]TranscodeProfile, error) {
	rows, err := p.r.QueryContext(ctx, profileQ+" ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TranscodeProfile
	for rows.Next() {
		prof, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *prof)
	}
	return out, rows.Err()
}

func (p *Profiles) GetByID(ctx context.Context, id int64) (*TranscodeProfile, error) {
	return scanProfile(p.r.QueryRowContext(ctx, profileQ+" WHERE id = ?", id))
}

func (p *Profiles) GetDefault(ctx context.Context) (*TranscodeProfile, error) {
	return scanProfile(p.r.QueryRowContext(ctx, profileQ+" WHERE is_default = 1 LIMIT 1"))
}

func (p *Profiles) Create(ctx context.Context, prof *TranscodeProfile) (int64, error) {
	res, err := p.w.ExecContext(ctx, `
		INSERT INTO transcode_profiles (name, crf, preset, extra_args, is_default)
		VALUES (?, ?, ?, ?, ?)`,
		prof.Name, prof.CRF, prof.Preset, prof.ExtraArgs, btoi(prof.IsDefault),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (p *Profiles) Update(ctx context.Context, prof *TranscodeProfile) error {
	_, err := p.w.ExecContext(ctx, `
		UPDATE transcode_profiles
		SET name = ?, crf = ?, preset = ?, extra_args = ?, is_default = ?
		WHERE id = ?`,
		prof.Name, prof.CRF, prof.Preset, prof.ExtraArgs, btoi(prof.IsDefault), prof.ID,
	)
	return err
}

func (p *Profiles) Delete(ctx context.Context, id int64) error {
	_, err := p.w.ExecContext(ctx, "DELETE FROM transcode_profiles WHERE id = ?", id)
	return err
}

const profileQ = `SELECT id, name, crf, preset, extra_args, is_default FROM transcode_profiles`

func scanProfile(s rowScanner) (*TranscodeProfile, error) {
	var p TranscodeProfile
	var isDefault int
	err := s.Scan(&p.ID, &p.Name, &p.CRF, &p.Preset, &p.ExtraArgs, &isDefault)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.IsDefault = isDefault != 0
	return &p, nil
}
