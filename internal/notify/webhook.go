package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"reclaim/internal/store"
)

// WebhookPayload is the generic ("json") body. The summary is embedded, so its
// fields sit at the top level next to the message — a receiver that only wants a
// count doesn't have to reach into a nested object.
type WebhookPayload struct {
	Event      string `json:"event"`
	Message    string `json:"message"`
	OccurredAt int64  `json:"occurred_at"`
	Summary
}

// discordEmbedColor is Reclaim's brand green, as the integer Discord expects.
const discordEmbedColor = 0x4ADE80

// maxErrorBody bounds how much of a failing receiver's response is quoted back.
const maxErrorBody = 400

// buildBody renders the summary in the shape the configured receiver expects and
// returns it with its content type. Every receiver gets the same information;
// only the envelope differs.
func buildBody(format string, s Summary, occurredAt int64) (body []byte, contentType string, err error) {
	message := s.Message()
	details := s.Details()

	switch format {
	case store.WebhookFormatDiscord:
		embed := map[string]any{
			"title":       message,
			"color":       discordEmbedColor,
			"description": bulletList(details),
			"footer":      map[string]any{"text": "Reclaim"},
			"fields": []map[string]any{
				{"name": "Files", "value": fmt.Sprintf("%d", s.Count), "inline": true},
				{"name": "Recoverable", "value": FormatBytes(s.SavingsBytes), "inline": true},
			},
		}
		body, err = json.Marshal(map[string]any{"embeds": []any{embed}})
		return body, "application/json", err

	case store.WebhookFormatSlack:
		text := message
		if len(details) > 0 {
			text += "\n" + bulletList(details)
		}
		body, err = json.Marshal(map[string]any{"text": text})
		return body, "application/json", err

	case store.WebhookFormatNtfy:
		// ntfy takes the message as the raw body; the title rides in a header,
		// which the caller sets.
		text := strings.Join(details, "\n")
		if text == "" {
			text = message
		}
		return []byte(text), "text/plain; charset=utf-8", nil

	default:
		body, err = json.Marshal(WebhookPayload{
			Event:      store.EventCandidatesAdded,
			Message:    message,
			OccurredAt: occurredAt,
			Summary:    s,
		})
		return body, "application/json", err
	}
}

func bulletList(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return "• " + strings.Join(lines, "\n• ")
}

// postWebhook delivers one batch to the configured receiver. A non-2xx response
// is an error carrying a truncated body, so the "send test" button in Settings
// can show the operator exactly what the receiver said.
func postWebhook(
	ctx context.Context,
	client *http.Client,
	cfg store.NotifySettings,
	s Summary,
	occurredAt int64,
) error {
	body, contentType, err := buildBody(cfg.WebhookFormat, s, occurredAt)
	if err != nil {
		return fmt.Errorf("build webhook body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "reclaim")
	if cfg.WebhookFormat == store.WebhookFormatNtfy {
		req.Header.Set("Title", s.Message())
		req.Header.Set("Tags", "film_projector")
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		msg := strings.TrimSpace(string(snippet))
		if msg == "" {
			return fmt.Errorf("webhook returned %s", resp.Status)
		}
		return fmt.Errorf("webhook returned %s: %s", resp.Status, msg)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBody))
	return nil
}
