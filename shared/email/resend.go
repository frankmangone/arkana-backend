package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const resendBaseURL = "https://api.resend.com"

// ResendSender sends email through Resend's REST API.
type ResendSender struct {
	apiKey     string
	from       string
	baseURL    string
	httpClient *http.Client
}

// NewResendSender creates a Sender backed by Resend.
func NewResendSender(apiKey, from string) *ResendSender {
	return &ResendSender{
		apiKey:     apiKey,
		from:       from,
		baseURL:    resendBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *ResendSender) Send(ctx context.Context, msg Message) error {
	payload, err := json.Marshal(map[string]string{
		"from":    s.from,
		"to":      msg.To,
		"subject": msg.Subject,
		"html":    msg.HTMLBody,
	})
	if err != nil {
		return fmt.Errorf("failed to encode email payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/emails", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-side close, nothing actionable on failure

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
