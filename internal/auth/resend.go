package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ResendClient struct {
	apiKey     string
	from       string
	httpClient *http.Client
}

type resendPayload struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
	HTML    string   `json:"html"`
}

func NewResendClient(apiKey, from string) *ResendClient {
	return &ResendClient{
		apiKey:     strings.TrimSpace(apiKey),
		from:       strings.TrimSpace(from),
		httpClient: &http.Client{Timeout: 12 * time.Second},
	}
}

func (c *ResendClient) SendOTP(ctx context.Context, email, code string) error {
	if c == nil || c.apiKey == "" || c.from == "" {
		return fmt.Errorf("resend is not configured")
	}

	payload := resendPayload{
		From:    c.from,
		To:      []string{strings.TrimSpace(email)},
		Subject: "Your NX Sandbox verification code",
		Text:    fmt.Sprintf("Your NX Sandbox verification code is %s. It expires in 10 minutes.", code),
		HTML:    fmt.Sprintf("<p>Your NX Sandbox verification code is <strong>%s</strong>.</p><p>It expires in 10 minutes.</p>", code),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal resend payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send resend request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("resend API returned status %d", resp.StatusCode)
	}

	return nil
}
