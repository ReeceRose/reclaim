package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Settings implements api.AuthStore and manages the singleton settings row.
type Settings struct {
	r, w *sql.DB

	mu     sync.RWMutex
	secret []byte
}

// IsSetupComplete reports whether first-run setup has been completed.
// Takes no context to satisfy the api.AuthStore interface.
func (s *Settings) IsSetupComplete() bool {
	var ts sql.NullInt64
	err := s.r.QueryRowContext(context.Background(),
		"SELECT setup_completed_at FROM settings WHERE id = 1",
	).Scan(&ts)
	return err == nil && ts.Valid
}

// ValidateLogin checks credentials. A dummy bcrypt compare runs on username
// mismatch to prevent timing-based username enumeration.
func (s *Settings) ValidateLogin(username, password string) bool {
	var storedUser, hash sql.NullString
	err := s.r.QueryRowContext(context.Background(),
		"SELECT auth_username, auth_password_hash FROM settings WHERE id = 1",
	).Scan(&storedUser, &hash)
	if err != nil || !storedUser.Valid || !hash.Valid {
		return false
	}
	if storedUser.String != strings.ToLower(username) {
		bcrypt.CompareHashAndPassword([]byte(hash.String), []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash.String), []byte(password)) == nil
}

// SessionSecret returns the decoded session signing key, loading and caching
// it from the database on first call.
func (s *Settings) SessionSecret() []byte {
	s.mu.RLock()
	if s.secret != nil {
		defer s.mu.RUnlock()
		return s.secret
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.secret != nil {
		return s.secret
	}

	var enc sql.NullString
	if err := s.r.QueryRowContext(context.Background(),
		"SELECT session_secret FROM settings WHERE id = 1",
	).Scan(&enc); err != nil || !enc.Valid || enc.String == "" {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(enc.String)
	if err != nil {
		return nil
	}
	s.secret = decoded
	return s.secret
}

// CompleteSetup stores bcrypt-hashed credentials and marks setup as done.
func (s *Settings) CompleteSetup(ctx context.Context, username, plaintext string) error {
	if s.IsSetupComplete() {
		return ErrSetupAlreadyComplete
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.w.ExecContext(ctx, `
		UPDATE settings SET auth_username = ?, auth_password_hash = ?, setup_completed_at = ?
		WHERE id = 1`,
		strings.ToLower(username), string(hash), time.Now().Unix(),
	)
	return err
}

// ChangeCredentials replaces username and password on an already-configured instance.
func (s *Settings) ChangeCredentials(ctx context.Context, username, plaintext string) error {
	if !s.IsSetupComplete() {
		return ErrSetupNotComplete
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.w.ExecContext(ctx, `
		UPDATE settings SET auth_username = ?, auth_password_hash = ? WHERE id = 1`,
		strings.ToLower(username), string(hash),
	)
	return err
}

// ResetAuth clears credentials so first-run setup is required again.
func (s *Settings) ResetAuth(ctx context.Context) error {
	_, err := s.w.ExecContext(ctx, `
		UPDATE settings
		SET auth_username = NULL, auth_password_hash = NULL, setup_completed_at = NULL
		WHERE id = 1`,
	)
	return err
}

// DefaultClientProfile returns the user's sticky default client profile for
// the compatibility view (internal/compatibility), e.g. "apple_tv_4k". Persisted in
// the DB (unlike config.Live) so the choice survives a restart.
func (s *Settings) DefaultClientProfile(ctx context.Context) (string, error) {
	var profile string
	err := s.r.QueryRowContext(ctx,
		"SELECT default_client_profile FROM settings WHERE id = 1",
	).Scan(&profile)
	return profile, err
}

// SetDefaultClientProfile updates the sticky default client profile. Callers
// are responsible for validating profile against the known built-in IDs
// (internal/compatibility.BuiltinProfiles) before calling this.
func (s *Settings) SetDefaultClientProfile(ctx context.Context, profile string) error {
	_, err := s.w.ExecContext(ctx,
		"UPDATE settings SET default_client_profile = ? WHERE id = 1", profile,
	)
	return err
}

// ensureSecret generates and persists a 32-byte random session secret if one
// does not already exist. Called once from Open after repos are initialized.
func (s *Settings) ensureSecret(ctx context.Context) error {
	var enc sql.NullString
	if err := s.w.QueryRowContext(ctx,
		"SELECT session_secret FROM settings WHERE id = 1",
	).Scan(&enc); err != nil {
		return err
	}
	if enc.Valid && enc.String != "" {
		return nil
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("generate secret: %w", err)
	}
	_, err := s.w.ExecContext(ctx,
		"UPDATE settings SET session_secret = ? WHERE id = 1",
		base64.StdEncoding.EncodeToString(buf),
	)
	return err
}
