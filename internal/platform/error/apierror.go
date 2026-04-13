package error

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is a machine-readable error identifier sent in the JSON response.
// Front-ends and other services key off this string, not the HTTP status.
type Code string

const (
	// 4xx — client errors
	CodeValidation      Code = "VALIDATION_ERROR"
	CodeUnauthorized    Code = "UNAUTHORIZED"
	CodeForbidden       Code = "FORBIDDEN"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT"
	CodeTooManyRequests Code = "TOO_MANY_REQUESTS"
	CodeGone            Code = "GONE" // soft-deleted resource

	// 5xx — server errors
	CodeInternal Code = "INTERNAL_ERROR"
	CodeDatabase Code = "DATABASE_ERROR"
	CodeCache    Code = "CACHE_ERROR"
	CodeUpstream Code = "UPSTREAM_ERROR"

	// domain-specific
	CodeUserNotFound          Code = "USER_NOT_FOUND"
	CodeEmailAlreadyExists    Code = "EMAIL_ALREADY_EXISTS"
	CodeUsernameAlreadyExists Code = "USERNAME_ALREADY_EXISTS"
	CodeInvalidCredentials    Code = "INVALID_CREDENTIALS"
	CodeAccountLocked         Code = "ACCOUNT_LOCKED"
	CodeAccountSuspended      Code = "ACCOUNT_SUSPENDED"
	CodeEmailNotVerified      Code = "EMAIL_NOT_VERIFIED"
	CodeSessionNotFound       Code = "SESSION_NOT_FOUND"
	CodeSessionExpired        Code = "SESSION_EXPIRED"
	CodeSessionRevoked        Code = "SESSION_REVOKED"
	CodeTokenInvalid          Code = "TOKEN_INVALID"
	CodeTokenExpired          Code = "TOKEN_EXPIRED"
	CodeTwoFactorRequired     Code = "TWO_FACTOR_REQUIRED"
)

// APIError is the single error type that crosses the service boundary.
// It carries an HTTP status, a stable Code, a human message, and an optional
// cause chain for structured logging (the cause is never serialised to the
// client).
type APIError struct {
	HTTPStatus int    `json:"-"`
	Code       Code   `json:"code"`
	Message    string `json:"message"`
	Details    any    `json:"details,omitempty"` // e.g. field validation failures

	// Internal — logged but NOT sent to clients.
	cause error
}

func (e *APIError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *APIError) Unwrap() error { return e.cause }

// Is satisfies errors.Is — two APIErrors are equal if their Codes match.
func (e *APIError) Is(target error) bool {
	var t *APIError
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

func IsCode(err error, code Code) bool {
	var e *APIError
	if errors.As(err, &e) {
		return e.Code == code
	}
	return false
}

// WithCause attaches an underlying error for logging without exposing it.
func (e *APIError) WithCause(err error) *APIError {
	clone := *e
	clone.cause = err
	return &clone
}

// WithDetails attaches structured details (e.g. validation field errors).
func (e *APIError) WithDetails(d any) *APIError {
	clone := *e
	clone.Details = d
	return &clone
}

// ─── constructors ────────────────────────────────────────────────────────────

func New(status int, code Code, message string) *APIError {
	return &APIError{HTTPStatus: status, Code: code, Message: message}
}

func Validation(message string, details any) *APIError {
	return &APIError{HTTPStatus: http.StatusBadRequest, Code: CodeValidation, Message: message, Details: details}
}

func Unauthorized(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusUnauthorized, Code: CodeUnauthorized, Message: message}
}

func Forbidden(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusForbidden, Code: CodeForbidden, Message: message}
}

func NotFound(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusNotFound, Code: CodeNotFound, Message: message}
}

func Conflict(message string) *APIError {
	return &APIError{HTTPStatus: http.StatusConflict, Code: CodeConflict, Message: message}
}

func Internal(message string, cause error) *APIError {
	return &APIError{HTTPStatus: http.StatusInternalServerError, Code: CodeInternal, Message: message, cause: cause}
}

func DatabaseError(cause error) *APIError {
	return &APIError{HTTPStatus: http.StatusInternalServerError, Code: CodeDatabase, Message: "a database error occurred", cause: cause}
}

// ─── sentinel instances ───────────────────────────────────────────────────────
// Use these with errors.Is() throughout the codebase.

var (
	ErrUserNotFound          = New(http.StatusNotFound, CodeUserNotFound, "user not found")
	ErrEmailAlreadyExists    = New(http.StatusConflict, CodeEmailAlreadyExists, "email address is already registered")
	ErrUsernameAlreadyExists = New(http.StatusConflict, CodeUsernameAlreadyExists, "username is already taken")
	ErrInvalidCredentials    = New(http.StatusUnauthorized, CodeInvalidCredentials, "invalid email or password")
	ErrAccountLocked         = New(http.StatusForbidden, CodeAccountLocked, "account is temporarily locked due to too many failed login attempts")
	ErrAccountSuspended      = New(http.StatusForbidden, CodeAccountSuspended, "account has been suspended")
	ErrEmailNotVerified      = New(http.StatusForbidden, CodeEmailNotVerified, "email address has not been verified")
	ErrSessionNotFound       = New(http.StatusUnauthorized, CodeSessionNotFound, "session not found")
	ErrSessionExpired        = New(http.StatusUnauthorized, CodeSessionExpired, "session has expired")
	ErrSessionRevoked        = New(http.StatusUnauthorized, CodeSessionRevoked, "session has been revoked")
	ErrTokenInvalid          = New(http.StatusUnauthorized, CodeTokenInvalid, "token is invalid")
	ErrTokenExpired          = New(http.StatusUnauthorized, CodeTokenExpired, "token has expired")
	ErrTwoFactorRequired     = New(http.StatusForbidden, CodeTwoFactorRequired, "two-factor authentication is required")
)

// As is a convenience wrapper around errors.As for *APIError.
func As(err error) (*APIError, bool) {
	var e *APIError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
