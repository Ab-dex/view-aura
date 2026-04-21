package resilience

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	// redisMailOutboxKey is the Redis list key for deferred emails.
	redisMailOutboxKey = "outbox:mail"

	// redisMailOutboxTTL — emails are retried for up to 24h.
	redisMailOutboxTTL = 24 * time.Hour

	maxRedisMailOutboxLen = 5_000

	// emailServiceTimeout is the HTTP call budget to the Node.js email service.
	emailServiceTimeout = 5 * time.Second
)

// MailPayload is what the Node.js email service expects on its HTTP endpoint.
type MailPayload struct {
	To         string         `json:"to"`
	Subject    string         `json:"subject"`
	TemplateID string         `json:"template_id"`
	Data       map[string]any `json:"data"`
	QueuedAt   string         `json:"queued_at,omitempty"`
}

// ResilientMailer implements auth/service.NotificationSender with three-tier
// degradation so the auth flow never fails because the email service is down.
//
//	Tier 1  POST to email-service HTTP endpoint
//	Tier 2  LPUSH to Redis outbox:mail (OutboxWorker retries)
//	Tier 3  FileOutbox JSONL append (ops replays via admin tooling)
//
// A nil rdb skips Tier 2.
// An empty emailServiceURL skips Tier 1 and starts at Tier 2.
type ResilientMailer struct {
	emailServiceURL string
	httpClient      *http.Client
	breaker         *InProcessBreaker
	rdb             goredis.UniversalClient
	outbox          *FileOutbox
}

// NewResilientMailer constructs the mailer.
//
//   - emailServiceURL: base URL of the Node.js email service, e.g. "http://email-service:7002"
//     Pass "" to disable Tier 1 (useful in local dev without the email service running).
//   - rdb: pass nil when Redis is not configured.
//   - logDir: directory for Tier 3 log files.
func NewResilientMailer(emailServiceURL string, rdb goredis.UniversalClient, logDir string) *ResilientMailer {
	return &ResilientMailer{
		emailServiceURL: emailServiceURL,
		httpClient:      &http.Client{Timeout: emailServiceTimeout},
		breaker:         NewInProcessBreaker("email-service", 3, 60*time.Second),
		rdb:             rdb,
		outbox:          NewFileOutbox(logDir, "mail"),
	}
}

// SendEmail implements auth/service.NotificationSender.
func (m *ResilientMailer) SendEmail(ctx context.Context, to, subject, templateID string, data map[string]any) error {
	payload := MailPayload{
		To:         to,
		Subject:    subject,
		TemplateID: templateID,
		Data:       data,
	}

	// ── Tier 1: email-service HTTP ────────────────────────────────────────────
	if m.emailServiceURL != "" && m.breaker.Allow() {
		if err := m.postToEmailService(ctx, payload); err == nil {
			m.breaker.RecordSuccess()
			return nil
		} else {
			m.breaker.RecordFailure()
			log.Warn().
				Err(err).
				Str("to", to).
				Str("template", templateID).
				Msg("resilient_mailer: email-service failed — falling back to redis outbox")
		}
	}

	// ── Tier 2: Redis outbox ──────────────────────────────────────────────────
	if m.rdb != nil {
		payload.QueuedAt = time.Now().UTC().Format(time.RFC3339)
		if err := m.pushToRedis(ctx, payload); err == nil {
			log.Info().Str("to", to).Str("template", templateID).
				Msg("resilient_mailer: email queued in redis outbox")
			return nil
		}
		log.Warn().Str("to", to).
			Msg("resilient_mailer: redis outbox failed — falling back to file outbox")
	}

	// ── Tier 3: File outbox ───────────────────────────────────────────────────
	m.outbox.Write("mail", to, payload)
	log.Error().
		Str("to", to).
		Str("template", templateID).
		Msg("resilient_mailer: email written to file outbox (email-service + redis both unavailable)")

	// Return nil — auth flows (register, password reset) must complete even
	// when email delivery is temporarily broken.
	return nil
}

func (m *ResilientMailer) postToEmailService(ctx context.Context, payload MailPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("resilient_mailer: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		m.emailServiceURL+"/send", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("resilient_mailer: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("resilient_mailer: http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("resilient_mailer: email-service status %d", resp.StatusCode)
	}
	return nil
}

func (m *ResilientMailer) pushToRedis(ctx context.Context, payload MailPayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("resilient_mailer: marshal for redis: %w", err)
	}

	tctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	pipe := m.rdb.Pipeline()
	pipe.LPush(tctx, redisMailOutboxKey, string(raw))
	pipe.LTrim(tctx, redisMailOutboxKey, 0, maxRedisMailOutboxLen-1)
	pipe.Expire(tctx, redisMailOutboxKey, redisMailOutboxTTL)

	if _, err := pipe.Exec(tctx); err != nil {
		return fmt.Errorf("resilient_mailer: redis pipeline: %w", err)
	}
	return nil
}
