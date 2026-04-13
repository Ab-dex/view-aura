package apierror_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_CreatesCorrectFields(t *testing.T) {
	e := apierror.New(http.StatusTeapot, apierror.Code("TEAPOT"), "I am a teapot")
	assert.Equal(t, http.StatusTeapot, e.HTTPStatus)
	assert.Equal(t, apierror.Code("TEAPOT"), e.Code)
	assert.Equal(t, "I am a teapot", e.Message)
	assert.Nil(t, e.Details)
}

func TestSentinels_HaveCorrectHTTPStatus(t *testing.T) {
	tests := []struct {
		err    *apierror.APIError
		status int
	}{
		{apierror.ErrUserNotFound, http.StatusNotFound},
		{apierror.ErrEmailAlreadyExists, http.StatusConflict},
		{apierror.ErrUsernameAlreadyExists, http.StatusConflict},
		{apierror.ErrInvalidCredentials, http.StatusUnauthorized},
		{apierror.ErrAccountLocked, http.StatusForbidden},
		{apierror.ErrAccountSuspended, http.StatusForbidden},
		{apierror.ErrSessionNotFound, http.StatusUnauthorized},
		{apierror.ErrSessionExpired, http.StatusUnauthorized},
		{apierror.ErrSessionRevoked, http.StatusUnauthorized},
		{apierror.ErrTokenInvalid, http.StatusUnauthorized},
		{apierror.ErrTokenExpired, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(string(tt.err.Code), func(t *testing.T) {
			assert.Equal(t, tt.status, tt.err.HTTPStatus)
		})
	}
}

func TestWithCause_AttachesCause(t *testing.T) {
	cause := errors.New("underlying db error")
	e := apierror.ErrUserNotFound.WithCause(cause)

	// WithCause must NOT mutate the sentinel.
	assert.Nil(t, apierror.ErrUserNotFound.Unwrap())
	// Clone carries the cause.
	assert.Equal(t, cause, e.Unwrap())
}

func TestWithDetails_AttachesDetails(t *testing.T) {
	type fieldErr struct{ Field, Message string }
	details := []fieldErr{{"email", "invalid"}}

	e := apierror.Validation("bad input", details)
	assert.NotNil(t, e.Details)

	// WithDetails on an existing error also works.
	e2 := apierror.ErrUserNotFound.WithDetails(details)
	assert.NotNil(t, e2.Details)
	// Sentinel is unchanged.
	assert.Nil(t, apierror.ErrUserNotFound.Details)
}

func TestErrorsIs_MatchesBySentinel(t *testing.T) {
	cause := fmt.Errorf("pg error")
	wrapped := apierror.ErrUserNotFound.WithCause(cause)

	// errors.Is must match the sentinel even when wrapped.
	assert.True(t, errors.Is(wrapped, apierror.ErrUserNotFound))
	assert.False(t, errors.Is(wrapped, apierror.ErrEmailAlreadyExists))
}

func TestErrorsIs_WorksThroughWrapChain(t *testing.T) {
	inner := apierror.ErrInvalidCredentials.WithCause(errors.New("bcrypt mismatch"))
	outer := fmt.Errorf("login failed: %w", inner)

	assert.True(t, errors.Is(outer, apierror.ErrInvalidCredentials))
}

func TestAs_ExtractsAPIError(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", apierror.ErrUserNotFound)
	e, ok := apierror.As(err)
	require.True(t, ok)
	assert.Equal(t, apierror.CodeUserNotFound, e.Code)
}

func TestIsCode_ReturnsTrueForMatchingCode(t *testing.T) {
	err := apierror.ErrUserNotFound.WithCause(errors.New("pg: no rows"))
	assert.True(t, apierror.IsCode(err, apierror.CodeUserNotFound))
	assert.False(t, apierror.IsCode(err, apierror.CodeEmailAlreadyExists))
}

func TestIsCode_ReturnsFalseForNonAPIError(t *testing.T) {
	assert.False(t, apierror.IsCode(errors.New("plain error"), apierror.CodeInternal))
}

func TestDatabaseError_IsInternalCode(t *testing.T) {
	e := apierror.DatabaseError(errors.New("connection refused"))
	assert.Equal(t, apierror.CodeDatabase, e.Code)
	assert.Equal(t, http.StatusInternalServerError, e.HTTPStatus)
}

func TestInternal_ContainsCause(t *testing.T) {
	cause := errors.New("something exploded")
	e := apierror.Internal("oops", cause)
	assert.Equal(t, cause, e.Unwrap())
	assert.Equal(t, http.StatusInternalServerError, e.HTTPStatus)
}

func TestError_IncludesCauseInString(t *testing.T) {
	cause := errors.New("db connection refused")
	e := apierror.ErrUserNotFound.WithCause(cause)
	assert.Contains(t, e.Error(), "db connection refused")
}

func TestError_NoCauseString(t *testing.T) {
	e := apierror.ErrUserNotFound
	s := e.Error()
	assert.Contains(t, s, "USER_NOT_FOUND")
	assert.Contains(t, s, "user not found")
}

func TestValidation_SetsStatusBadRequest(t *testing.T) {
	e := apierror.Validation("field error", nil)
	assert.Equal(t, http.StatusBadRequest, e.HTTPStatus)
	assert.Equal(t, apierror.CodeValidation, e.Code)
}
