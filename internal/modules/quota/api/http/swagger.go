package httpapi

// ─── Quota ───────────────────────────────────────────────────────────────────

// GetStatus godoc
//
//	@Summary     Get quota status
//	@Description Returns a full usage snapshot including storage usage, daily uploads,
//	             tier information, and system limits.
//	@Tags        Quota
//	@Security    BearerAuth
//	@Produce     json
//	@Success     200 {object} quotaStatusResponse
//	@Failure     403 {object} errorResponse
//	@Router      /me/quota [get]
func (h *QuotaHandler) swaggerGetStatus() {}

// GetTier godoc
//
//	@Summary     Get quota tier
//	@Description Returns the user's current tier and associated limits.
//	@Tags        Quota
//	@Security    BearerAuth
//	@Produce     json
//	@Success     200 {object} quotaTierResponse
//	@Failure     403 {object} errorResponse
//	@Router      /me/quota/tier [get]
func (h *QuotaHandler) swaggerGetTier() {}

// ─── Response schema stubs ────────────────────────────────────────────────────

type quotaStatusResponse struct {
	Tier string `json:"tier" example:"pro" enums:"free,pro,studio"`

	Storage struct {
		UsedBytes int64   `json:"used_bytes" example:"1073741824"`
		MaxBytes  int64   `json:"max_bytes" example:"107374182400"`
		UsedPct   float64 `json:"used_pct" example:"1.0"`
	} `json:"storage"`

	DailyUploads struct {
		Used    int     `json:"used" example:"5"`
		Max     int     `json:"max" example:"50"`
		UsedPct float64 `json:"used_pct" example:"10.0"`
	} `json:"daily_uploads"`

	Limits tierLimitsResponse `json:"limits"`

	AsOf string `json:"as_of" example:"2026-04-14T12:00:00Z"`
}

type quotaTierResponse struct {
	Tier   string             `json:"tier" example:"pro" enums:"free,pro,studio"`
	Limits tierLimitsResponse `json:"limits"`
}

type tierLimitsResponse struct {
	Tier             string `json:"tier" example:"pro" enums:"free,pro,studio"`
	MaxStorageBytes  int64  `json:"max_storage_bytes" example:"107374182400"`
	MaxDailyUploads  int    `json:"max_daily_uploads" example:"50"`
	MaxFileSizeBytes int64  `json:"max_file_size_bytes" example:"10737418240"`
	TranscodeSpeed   string `json:"transcode_speed" example:"priority" enums:"standard,priority,express"`
	RetentionDays    int    `json:"retention_days" example:"365"`
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
