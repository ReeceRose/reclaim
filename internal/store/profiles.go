package store

import (
	"context"
	"database/sql"
	"errors"
)

type TranscodeProfile struct {
	ID   int64
	Name string
	// Codec is the target codec the profile encodes to ("hevc" or "av1"); see
	// media.EncoderFor for the encoder and the CRF/preset vocabulary it implies.
	Codec     string
	CRF       int
	Preset    string
	ExtraArgs *string
	IsDefault bool
}

type Profiles struct {
	r, w *sql.DB
}

func (p *Profiles) List(ctx context.Context) ([]TranscodeProfile, error) {
	rows, err := p.r.QueryContext(ctx, profileQ+" ORDER BY is_default DESC, id")
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
	tx, err := p.w.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if prof.IsDefault {
		if _, err := tx.ExecContext(ctx, "UPDATE transcode_profiles SET is_default = 0"); err != nil {
			return 0, err
		}
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO transcode_profiles (name, codec, crf, preset, extra_args, is_default)
		VALUES (?, ?, ?, ?, ?, ?)`,
		prof.Name, profileCodec(prof.Codec), prof.CRF, prof.Preset, prof.ExtraArgs, btoi(prof.IsDefault),
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (p *Profiles) Update(ctx context.Context, prof *TranscodeProfile) error {
	tx, err := p.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if prof.IsDefault {
		if _, err := tx.ExecContext(ctx, "UPDATE transcode_profiles SET is_default = 0 WHERE id <> ?", prof.ID); err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE transcode_profiles
		SET name = ?, codec = ?, crf = ?, preset = ?, extra_args = ?, is_default = ?
		WHERE id = ?`,
		prof.Name, profileCodec(prof.Codec), prof.CRF, prof.Preset, prof.ExtraArgs, btoi(prof.IsDefault), prof.ID,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (p *Profiles) Delete(ctx context.Context, id int64) error {
	_, err := p.w.ExecContext(ctx, "DELETE FROM transcode_profiles WHERE id = ?", id)
	return err
}

const profileQ = `SELECT id, name, codec, crf, preset, extra_args, is_default FROM transcode_profiles`

// profileCodec stores an unset codec as the default rather than an empty
// string, so every row names the codec it encodes to.
func profileCodec(codec string) string {
	if codec == "" {
		return defaultProfileCodec
	}
	return codec
}

const defaultProfileCodec = "hevc"

func scanProfile(s rowScanner) (*TranscodeProfile, error) {
	var p TranscodeProfile
	var isDefault int
	err := s.Scan(&p.ID, &p.Name, &p.Codec, &p.CRF, &p.Preset, &p.ExtraArgs, &isDefault)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.IsDefault = isDefault != 0
	return &p, nil
}
