package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
)

// Webhook body shapes the notifier can speak. The URL alone doesn't say what a
// receiver expects, so the operator picks one.
const (
	WebhookFormatJSON    = "json"
	WebhookFormatDiscord = "discord"
	WebhookFormatSlack   = "slack"
	WebhookFormatNtfy    = "ntfy"
)

// Notification batching bounds. The delay is the quiet period a batch waits out
// before it is sent, so a whole season landing at once becomes one notification.
const (
	DefaultNotifyDelaySeconds = 900 // 15 minutes
	MaxNotifyDelaySeconds     = 24 * 60 * 60
)

// NotifySettings is the persisted notification configuration. Unlike the
// env-seeded knobs in config.Live these live in the settings row: a webhook URL
// typed into the UI must survive a restart, and there is no env var behind it.
type NotifySettings struct {
	Enabled       bool
	DelaySeconds  int
	WebhookURL    string
	WebhookFormat string
}

// DefaultNotifySettings is what a row that predates the columns — or holds
// anything unrecognised — reads back as.
func DefaultNotifySettings() NotifySettings {
	return NotifySettings{
		Enabled:       true,
		DelaySeconds:  DefaultNotifyDelaySeconds,
		WebhookFormat: WebhookFormatJSON,
	}
}

// ValidWebhookFormat reports whether v is a body shape the notifier can build.
func ValidWebhookFormat(v string) bool {
	switch v {
	case WebhookFormatJSON, WebhookFormatDiscord, WebhookFormatSlack, WebhookFormatNtfy:
		return true
	}
	return false
}

// ValidateWebhookURL accepts an empty URL (webhook off) or an absolute http(s)
// one. Anything else is rejected at write time rather than failing silently in
// the background sender, where nobody would see it.
func ValidateWebhookURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("must be a valid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("must start with http:// or https:// (got %q)", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("must include a host (got %q)", raw)
	}
	return nil
}

// Notify returns the stored notification settings, falling back to the defaults
// for a row that predates the columns or holds an unrecognised format.
func (s *Settings) Notify(ctx context.Context) NotifySettings {
	out := DefaultNotifySettings()

	var (
		enabled sql.NullInt64
		delay   sql.NullInt64
		hookURL sql.NullString
		format  sql.NullString
	)
	err := s.r.QueryRowContext(ctx, `
		SELECT notify_enabled, notify_delay_seconds, notify_webhook_url, notify_webhook_format
		FROM settings WHERE id = 1`,
	).Scan(&enabled, &delay, &hookURL, &format)
	if err != nil {
		return out
	}

	if enabled.Valid {
		out.Enabled = enabled.Int64 != 0
	}
	if delay.Valid && delay.Int64 >= 0 && delay.Int64 <= MaxNotifyDelaySeconds {
		out.DelaySeconds = int(delay.Int64)
	}
	if hookURL.Valid {
		out.WebhookURL = hookURL.String
	}
	if format.Valid && ValidWebhookFormat(format.String) {
		out.WebhookFormat = format.String
	}
	return out
}

// SetNotify validates and stores the notification settings.
func (s *Settings) SetNotify(ctx context.Context, n NotifySettings) error {
	if n.DelaySeconds < 0 || n.DelaySeconds > MaxNotifyDelaySeconds {
		return fmt.Errorf("notify_delay_seconds must be between 0 and %d", MaxNotifyDelaySeconds)
	}
	if !ValidWebhookFormat(n.WebhookFormat) {
		return fmt.Errorf("notify_webhook_format must be one of %q, %q, %q, %q",
			WebhookFormatJSON, WebhookFormatDiscord, WebhookFormatSlack, WebhookFormatNtfy)
	}
	hookURL := strings.TrimSpace(n.WebhookURL)
	if err := ValidateWebhookURL(hookURL); err != nil {
		return fmt.Errorf("notify_webhook_url: %w", err)
	}

	_, err := s.w.ExecContext(ctx, `
		UPDATE settings SET
			notify_enabled = ?, notify_delay_seconds = ?,
			notify_webhook_url = ?, notify_webhook_format = ?
		WHERE id = 1`,
		btoi(n.Enabled), n.DelaySeconds, hookURL, n.WebhookFormat,
	)
	return err
}
