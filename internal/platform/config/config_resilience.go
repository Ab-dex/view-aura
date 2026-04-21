package config

// ResilienceConfig controls three-tier degradation for every external dependency.
//
// Degradation order for all components:
//
//	Tier 1  External service / provider      (full capability)
//	Tier 2  Redis-backed in-process queue    (reduced capability)
//	Tier 3  Local JSONL log file             (zero capability, fully observable)
//
// Redis is optional — when Redis.Addr is empty or Redis is unreachable, all
// components skip Tier 2 and fall directly to Tier 3.
// The database is the only hard startup requirement.
type ResilienceConfig struct {
	// LogDir is the directory where Tier 3 JSONL outbox files are written.
	// Files rotate daily: {kind}_outbox_YYYY-MM-DD.jsonl
	// Defaults to "logs" relative to the working directory.
	LogDir string `mapstructure:"log_dir"`

	// EmailServiceURL is the base URL of the Node.js email service (Tier 1).
	// Leave empty to skip Tier 1 — the mailer degrades directly to Redis/file.
	// Example: "http://email-service:7002"
	EmailServiceURL string `mapstructure:"email_service_url"`

	// TemporalDeferredKey is the Redis list key used when Temporal is unavailable.
	// OutboxWorker drains this list and starts the deferred workflows on recovery.
	// Defaults to "outbox:temporal:deferred".
	TemporalDeferredKey string `mapstructure:"temporal_deferred_key"`

	// StripePendingKey is the Redis list key for unprocessed Stripe write ops
	// (create/update/cancel subscription).  OutboxWorker retries these on recovery.
	// Defaults to "outbox:payment:pending".
	StripePendingKey string `mapstructure:"stripe_pending_key"`

	// PushOutboxKeyPrefix is the Redis key prefix for per-user push notification
	// outboxes.  Full key: {prefix}{user_id}  (e.g. "outbox:push:abc123").
	// Defaults to "outbox:push:".
	PushOutboxKeyPrefix string `mapstructure:"push_outbox_key_prefix"`

	// R2TempUploadDir is the local directory used as a Tier 2 fallback when R2
	// is unavailable.  Uploads here are served by the monolith's temp-upload
	// endpoint until R2 recovers.
	// Defaults to "/tmp/viewaura-uploads".
	R2TempUploadDir string `mapstructure:"r2_temp_upload_dir"`

	// R2TempMaxBytes caps total bytes stored in R2TempUploadDir.
	// Uploads that would exceed the cap return a clean 503 immediately.
	// Defaults to 500 MB.
	R2TempMaxBytes int64 `mapstructure:"r2_temp_max_bytes"`
}
