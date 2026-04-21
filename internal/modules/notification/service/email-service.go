package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type EmailServiceClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

// NotificationSender is a narrow interface satisfied by the notification module.
// Declared here to avoid importing the notification module directly.
type NotificationSender interface {
	SendEmail(ctx context.Context, to, subject, templateID string, data map[string]any) error
}

func NewEmailServiceClient(baseURL, apiKey string) *EmailServiceClient {
	return &EmailServiceClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *EmailServiceClient) SendEmail(
	ctx context.Context,
	to, subject, templateID string,
	data map[string]any,
) error {

	payload := map[string]any{
		"to":         to,
		"subject":    subject,
		"templateId": templateID,
		"data":       data,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/send-email",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("email service error: %s", string(b))
	}

	return nil
}
