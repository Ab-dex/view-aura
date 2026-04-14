package error

import "net/http"

// ─── Payment domain ───────────────────────────────────────────────────────────

const (
	CodePaymentRequired      Code = "PAYMENT_REQUIRED"
	CodeSubscriptionActive   Code = "SUBSCRIPTION_ALREADY_ACTIVE"
	CodeSubscriptionNotFound Code = "SUBSCRIPTION_NOT_FOUND"
	CodeStripeError          Code = "STRIPE_ERROR"
	CodeWebhookInvalid       Code = "WEBHOOK_SIGNATURE_INVALID"
)

var (
	ErrPaymentRequired      = New(http.StatusPaymentRequired, CodePaymentRequired, "a paid subscription is required for this action")
	ErrSubscriptionActive   = New(http.StatusConflict, CodeSubscriptionActive, "an active subscription already exists")
	ErrSubscriptionNotFound = New(http.StatusNotFound, CodeSubscriptionNotFound, "no active subscription found")
)

// ─── Quota domain ─────────────────────────────────────────────────────────────

const (
	CodeQuotaStorageExceeded Code = "QUOTA_STORAGE_EXCEEDED"
	CodeQuotaDailyExceeded   Code = "QUOTA_DAILY_LIMIT_EXCEEDED"
	CodeQuotaFileTooLarge    Code = "QUOTA_FILE_TOO_LARGE"
)

var (
	ErrQuotaStorageExceeded = New(http.StatusForbidden, CodeQuotaStorageExceeded, "storage quota exceeded for your plan")
	ErrQuotaDailyExceeded   = New(http.StatusTooManyRequests, CodeQuotaDailyExceeded, "daily upload limit reached for your plan")
	ErrQuotaFileTooLarge    = New(http.StatusRequestEntityTooLarge, CodeQuotaFileTooLarge, "file exceeds the maximum size for your plan")
)

// ─── Upload domain ────────────────────────────────────────────────────────────

const (
	CodeUploadNotFound   Code = "UPLOAD_NOT_FOUND"
	CodeUploadExpired    Code = "UPLOAD_SESSION_EXPIRED"
	CodeUploadInProgress Code = "UPLOAD_ALREADY_IN_PROGRESS"
)

var (
	ErrUploadNotFound   = New(http.StatusNotFound, CodeUploadNotFound, "upload session not found")
	ErrUploadExpired    = New(http.StatusGone, CodeUploadExpired, "upload session has expired")
	ErrUploadInProgress = New(http.StatusConflict, CodeUploadInProgress, "an upload is already in progress for this asset")
)

// ─── Moderation domain ────────────────────────────────────────────────────────

const (
	CodeModerationCaseNotFound   Code = "MODERATION_CASE_NOT_FOUND"
	CodeModerationAlreadyDecided Code = "MODERATION_ALREADY_DECIDED"
	CodeModerationLocked         Code = "MODERATION_CASE_LOCKED"
)

var (
	ErrModerationCaseNotFound   = New(http.StatusNotFound, CodeModerationCaseNotFound, "moderation case not found")
	ErrModerationAlreadyDecided = New(http.StatusConflict, CodeModerationAlreadyDecided, "a decision has already been recorded for this case")
	ErrModerationLocked         = New(http.StatusConflict, CodeModerationLocked, "this case is currently being reviewed by another moderator")
)

// ─── Additional constructors ──────────────────────────────────────────────────

// PaymentRequired is a shortcut for the 402 response.
func PaymentRequired(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusPaymentRequired, Code: CodePaymentRequired, Message: message}
}

// UpstreamError wraps a third-party service error (Stripe, R2, Temporal).
func UpstreamError(service string, cause error) *APIError {
	return &APIError{
		HTTPStatus: http.StatusBadGateway,
		Code:       CodeUpstream,
		Message:    service + " service error",
		cause:      cause,
	}
}
