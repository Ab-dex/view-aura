package events

import "time"

// ─── Playback events ──────────────────────────────────────────────────────────

// PlaybackSessionStartedEvent is published to TopicPlaybackSessionStarted when
// a user presses play.  It is the entry point of the viewing funnel.
type PlaybackSessionStartedEvent struct {
	SessionID       string    `json:"session_id"`
	UserID          string    `json:"user_id"`
	ProfileID       string    `json:"profile_id,omitempty"` // empty until profiles are enabled
	MovieID         string    `json:"movie_id"`
	AssetID         string    `json:"asset_id"`
	DeviceType      string    `json:"device_type"`      // "web","ios","android","tv","stb"
	ManifestVariant string    `json:"manifest_variant"` // "4k_hdr","1080p","720p",…
	StartedAt       time.Time `json:"started_at"`
	SchemaVersion   int       `json:"schema_version"` // always 1 for this type
}

// PlaybackHeartbeatEvent is published to TopicPlaybackHeartbeat every 10 s.
// High volume — kept intentionally small.
type PlaybackHeartbeatEvent struct {
	SessionID     string    `json:"session_id"`
	UserID        string    `json:"user_id"`
	MovieID       string    `json:"movie_id"`
	PositionMS    int64     `json:"position_ms"`
	BufferHealth  float64   `json:"buffer_health"` // 0.0–1.0 (seconds_buffered / 30s)
	BitrateKbps   int       `json:"bitrate_kbps"`
	Timestamp     time.Time `json:"timestamp"`
	SchemaVersion int       `json:"schema_version"`
}

// PlaybackAbandonedEvent is published to TopicPlaybackAbandoned when a session
// ends before 90 % of runtime has been reached.
type PlaybackAbandonedEvent struct {
	SessionID     string        `json:"session_id"`
	UserID        string        `json:"user_id"`
	ProfileID     string        `json:"profile_id,omitempty"`
	MovieID       string        `json:"movie_id"`
	PositionMS    int64         `json:"position_ms"`
	RuntimeMS     int64         `json:"runtime_ms"`
	WatchPct      float64       `json:"watch_pct"`                // 0.0–1.0
	Duration      time.Duration `json:"duration"`                 // time spent in this session
	AbandonReason string        `json:"abandon_reason,omitempty"` // "user_exit","network","error"
	AbandonedAt   time.Time     `json:"abandoned_at"`
	SchemaVersion int           `json:"schema_version"`
}

// PlaybackCompletedEvent is published to TopicPlaybackCompleted when
// watch_pct >= 0.9.
type PlaybackCompletedEvent struct {
	SessionID     string        `json:"session_id"`
	UserID        string        `json:"user_id"`
	ProfileID     string        `json:"profile_id,omitempty"`
	MovieID       string        `json:"movie_id"`
	RuntimeMS     int64         `json:"runtime_ms"`
	Duration      time.Duration `json:"duration"`
	CompletedAt   time.Time     `json:"completed_at"`
	SchemaVersion int           `json:"schema_version"`
}

// PlaybackQoEEvent is published to TopicPlaybackQoE by the telemetry endpoint.
// Aggregated per-ISP and per-region by the Flink job.
type PlaybackQoEEvent struct {
	SessionID       string    `json:"session_id"`
	UserID          string    `json:"user_id"`
	MovieID         string    `json:"movie_id"`
	ISP             string    `json:"isp,omitempty"`
	Region          string    `json:"region,omitempty"` // ISO 3166-1 alpha-2
	StartupTimeMS   int       `json:"startup_time_ms"`
	BufferingMS     int       `json:"buffering_ms"` // total buffering in this window
	BitrateSwitches int       `json:"bitrate_switches"`
	ErrorCode       string    `json:"error_code,omitempty"`
	ManifestVariant string    `json:"manifest_variant"`
	ReportedAt      time.Time `json:"reported_at"`
	SchemaVersion   int       `json:"schema_version"`
}

// ─── Search behavioural events ────────────────────────────────────────────────

// SearchQueryEvent is published to TopicSearchQuery.
type SearchQueryEvent struct {
	QueryID       string    `json:"query_id"`
	UserID        string    `json:"user_id"`
	ProfileID     string    `json:"profile_id,omitempty"`
	Query         string    `json:"query"`
	ResultCount   int       `json:"result_count"`
	Filters       []string  `json:"filters,omitempty"` // e.g. ["genre:action","year:2020"]
	SearchedAt    time.Time `json:"searched_at"`
	SchemaVersion int       `json:"schema_version"`
}

// SearchClickEvent is published to TopicSearchClick.
type SearchClickEvent struct {
	QueryID       string    `json:"query_id"`
	UserID        string    `json:"user_id"`
	ProfileID     string    `json:"profile_id,omitempty"`
	MovieID       string    `json:"movie_id"`
	Position      int       `json:"position"` // 0-based rank in results
	ClickedAt     time.Time `json:"clicked_at"`
	SchemaVersion int       `json:"schema_version"`
}

// SearchAbandonedEvent is published to TopicSearchAbandoned when a search
// session ends with zero clicks.
type SearchAbandonedEvent struct {
	QueryID       string    `json:"query_id"`
	UserID        string    `json:"user_id"`
	Query         string    `json:"query"`
	ResultCount   int       `json:"result_count"`
	AbandonedAt   time.Time `json:"abandoned_at"`
	SchemaVersion int       `json:"schema_version"`
}

// ─── Content performance events ───────────────────────────────────────────────

// TrendingScoreUpdatedEvent is published by the Flink analytics pipeline to
// TopicTrendingScoreUpdated when a movie's trending score changes significantly.
type TrendingScoreUpdatedEvent struct {
	MovieID       string    `json:"movie_id"`
	Score         float64   `json:"score"`
	Window        string    `json:"window"` // "1h", "24h", "7d"
	Rank          int       `json:"rank"`   // position in the global trending list
	UpdatedAt     time.Time `json:"updated_at"`
	SchemaVersion int       `json:"schema_version"`
}

// ─── Experiment events ────────────────────────────────────────────────────────

// ExperimentExposureEvent is published to TopicExperimentExposure when a user
// is assigned to a treatment.  Required for intent-to-treat analysis.
type ExperimentExposureEvent struct {
	ExperimentID  string    `json:"experiment_id"`
	UserID        string    `json:"user_id"`
	ProfileID     string    `json:"profile_id,omitempty"`
	Variant       string    `json:"variant"` // e.g. "control", "treatment_a"
	Surface       string    `json:"surface"` // e.g. "home_feed", "search", "player"
	ExposedAt     time.Time `json:"exposed_at"`
	SchemaVersion int       `json:"schema_version"`
}

// ─── Entitlement events ───────────────────────────────────────────────────────

// EntitlementGrantedEvent is published to TopicEntitlementGranted.
type EntitlementGrantedEvent struct {
	UserID        string     `json:"user_id"`
	ProfileID     string     `json:"profile_id,omitempty"`
	GrantType     string     `json:"grant_type"` // "subscription","ppv","promo","trial"
	Plan          string     `json:"plan,omitempty"`
	MovieID       string     `json:"movie_id,omitempty"` // non-empty for PPV
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	GrantedAt     time.Time  `json:"granted_at"`
	SchemaVersion int        `json:"schema_version"`
}

// EntitlementRevokedEvent is published to TopicEntitlementRevoked.
type EntitlementRevokedEvent struct {
	UserID        string    `json:"user_id"`
	ProfileID     string    `json:"profile_id,omitempty"`
	GrantType     string    `json:"grant_type"`
	Reason        string    `json:"reason"` // "cancelled","expired","refunded","admin"
	RevokedAt     time.Time `json:"revoked_at"`
	SchemaVersion int       `json:"schema_version"`
}
