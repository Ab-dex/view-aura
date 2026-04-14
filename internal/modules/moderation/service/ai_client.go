package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Ab-dex/view-aura/internal/platform/config"
)

// AIScreeningResult is the response from the Python moderation-ai service.
type AIScreeningResult struct {
	Signals        []string           `json:"signals"`
	Confidence     map[string]float64 `json:"confidence"`
	Recommendation string             `json:"recommendation"` // "approve" | "reject" | "escalate_to_human"
}

// AIClient is the interface for calling the Python moderation-ai service.
// Keeping it as an interface lets the Go module call the Python service
// without coupling to HTTP or gRPC — tests inject a mock.
type AIClient interface {
	Screen(ctx context.Context, contentType, contentID string) (*AIScreeningResult, error)
}

// httpAIClient calls the Python moderation-ai service over HTTP/JSON.
type httpAIClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewAIClient(cfg config.ModerationAIConfig) AIClient {
	return &httpAIClient{
		baseURL: cfg.Endpoint,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *httpAIClient) Screen(ctx context.Context, contentType, contentID string) (*AIScreeningResult, error) {
	body := fmt.Sprintf(`{"content_type":%q,"content_id":%q}`, contentType, contentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/screen", strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai_client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai_client: http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ai_client: unexpected status %d", resp.StatusCode)
	}

	var result AIScreeningResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ai_client: decode response: %w", err)
	}
	return &result, nil
}

// NoopAIClient returns an "approve" recommendation for everything.
// Used in local dev and tests when the Python service is unavailable.
type NoopAIClient struct{}

func (NoopAIClient) Screen(_ context.Context, _, _ string) (*AIScreeningResult, error) {
	return &AIScreeningResult{
		Signals:        []string{},
		Confidence:     map[string]float64{},
		Recommendation: "approve",
	}, nil
}
