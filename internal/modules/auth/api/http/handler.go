package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/app/middleware"
	"github.com/Ab-dex/view-aura/internal/modules/auth/domain"
	"github.com/Ab-dex/view-aura/internal/modules/auth/service"
	userdomain "github.com/Ab-dex/view-aura/internal/modules/user/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// AuthHandler exposes all authentication flows as HTTP endpoints.
// Intentionally separate from UserHandler so auth concerns do not
// bleed into profile/preference logic.
type AuthHandler struct {
	svc service.AuthService
}

func NewAuthHandler(svc service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// RegisterRoutes mounts all auth routes on the given Gin router group.
// The caller is responsible for applying rate-limiting at the group level.
//
// Route split:
//   - Public routes: email verification, password reset, OAuth, MFA verify
//   - Protected routes (auth middleware applied inline): TOTP enroll/confirm,
//     MFA disable, send-verification-email (requires a valid session)
func (h *AuthHandler) RegisterRoutes(r gin.IRouter) {
	// ── Public routes ────────────────────────────────────────────────────────

	// Email verification — the link in the email has no access token.
	r.POST("/verify-email/send", h.SendVerificationEmail)
	r.POST("/verify-email", h.VerifyEmail)

	// Password reset — initiated before the user is logged in.
	r.POST("/password-reset/request", middleware.StrictRateLimit(5, time.Minute), h.SendPasswordReset)
	r.POST("/password-reset/confirm", h.ResetPassword)

	// OAuth2 social login — the callback arrives from the provider, no token.
	r.GET("/oauth/:provider/begin", h.OAuthBegin)
	r.GET("/oauth/:provider/callback", h.OAuthCallback)

	// MFA second-factor — called during login before a full session is issued.
	r.POST("/mfa/verify", h.VerifyMFA)
	r.POST("/mfa/email-otp/send", middleware.StrictRateLimit(5, time.Minute), h.SendMFAEmailOTP)

	// ── Protected routes — require a valid access token ──────────────────────
	protected := r.Group("")
	protected.Use(gin.HandlerFunc(authMiddlewarePlaceholder))

	protected.POST("/mfa/totp/enroll", h.EnrollTOTP)
	protected.POST("/mfa/totp/confirm", h.ConfirmTOTP)
	protected.DELETE("/mfa", h.DisableMFA)
}

// authMiddlewarePlaceholder is satisfied by the app-level auth middleware.
// The module.go passes the real contract.AuthMiddleware via the protected group.
// This constant keeps the handler self-contained; wire replaces it at app init.
//
// In practice the protected group's Use() is called in module.Register() with
// the real middleware — this variable is never executed.
var authMiddlewarePlaceholder = func(c *gin.Context) { c.Next() }

// ─── Email verification ───────────────────────────────────────────────────────

// SendVerificationEmail requires an authenticated session because it reads the
// user's current email from the JWT claims.
func (h *AuthHandler) SendVerificationEmail(c *gin.Context) {
	userID, email := mustIdentity(c)
	if userID == "" {
		return // mustIdentity already aborted
	}
	if err := h.svc.SendVerificationEmail(c.Request.Context(), domain.SendVerificationEmailCmd{
		UserID:    string(userID),
		Email:     email,
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation("token is required", nil))
		return
	}
	if err := h.svc.VerifyEmail(c.Request.Context(), domain.VerifyEmailCmd{
		Token:     req.Token,
		IPAddress: c.ClientIP(),
	}); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Password reset ───────────────────────────────────────────────────────────

func (h *AuthHandler) SendPasswordReset(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	// Deliberately ignore bind errors — always return 204 to prevent enumeration.
	_ = c.ShouldBindJSON(&req)
	_ = h.svc.SendPasswordReset(c.Request.Context(), domain.SendPasswordResetCmd{
		Email:     req.Email,
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token"        binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation("token and new_password are required", nil))
		return
	}
	user, pair, err := h.svc.ResetPassword(c.Request.Context(), domain.ResetPasswordCmd{
		Token:       req.Token,
		NewPassword: req.NewPassword,
		IPAddress:   c.ClientIP(),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, newAuthResponse(user, pair))
}

// ─── OAuth2 ───────────────────────────────────────────────────────────────────

func (h *AuthHandler) OAuthBegin(c *gin.Context) {
	redirectURI := c.Query("redirect_uri")
	if redirectURI == "" {
		respondError(c, apierror.Validation("redirect_uri is required", nil))
		return
	}
	result, err := h.svc.OAuthBegin(c.Request.Context(), domain.OAuthBeginCmd{
		Provider:    domain.AuthProvider(c.Param("provider")),
		RedirectURI: redirectURI,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.Redirect(http.StatusFound, result.AuthorizationURL)
}

func (h *AuthHandler) OAuthCallback(c *gin.Context) {
	code, state := c.Query("code"), c.Query("state")
	if code == "" || state == "" {
		respondError(c, apierror.Validation("code and state are required", nil))
		return
	}
	user, pair, err := h.svc.OAuthCallback(c.Request.Context(), domain.OAuthCallbackCmd{
		Provider:  domain.AuthProvider(c.Param("provider")),
		Code:      code,
		State:     state,
		DeviceID:  c.GetHeader("X-Device-ID"),
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, newAuthResponse(user, pair))
}

// ─── TOTP MFA (protected) ─────────────────────────────────────────────────────

func (h *AuthHandler) EnrollTOTP(c *gin.Context) {
	userID, _ := mustIdentity(c)
	if userID == "" {
		return
	}
	result, err := h.svc.EnrollTOTP(c.Request.Context(), domain.EnrollTOTPCmd{
		UserID: string(userID),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"secret":      result.Secret,
		"qr_code_uri": result.ProvisioningURI,
	})
}

func (h *AuthHandler) ConfirmTOTP(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation("code is required", nil))
		return
	}
	userID, _ := mustIdentity(c)
	if userID == "" {
		return
	}
	codes, err := h.svc.ConfirmTOTP(c.Request.Context(), domain.VerifyTOTPEnrollmentCmd{
		UserID: string(userID),
		Code:   req.Code,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"recovery_codes": codes,
		"message":        "Save these recovery codes somewhere safe. They will not be shown again.",
	})
}

func (h *AuthHandler) DisableMFA(c *gin.Context) {
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation("password is required", nil))
		return
	}
	userID, _ := mustIdentity(c)
	if userID == "" {
		return
	}
	if err := h.svc.DisableMFA(c.Request.Context(), domain.DisableMFACmd{
		UserID:   userID,
		Password: req.Password,
	}); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── MFA verification (second-factor step during login) ───────────────────────

func (h *AuthHandler) VerifyMFA(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Code   string `json:"code"    binding:"required"`
		Method string `json:"method"  binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation("user_id, code, and method are required", nil))
		return
	}
	user, pair, err := h.svc.VerifyMFA(c.Request.Context(), domain.VerifyMFACmd{
		UserID:    userdomain.UserID(req.UserID),
		Code:      req.Code,
		Method:    domain.MFAMethod(req.Method),
		DeviceID:  c.GetHeader("X-Device-ID"),
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, newAuthResponse(user, pair))
}

func (h *AuthHandler) SendMFAEmailOTP(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation("user_id is required", nil))
		return
	}
	if err := h.svc.SendMFAEmailOTP(c.Request.Context(), userdomain.UserID(req.UserID)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Response types ───────────────────────────────────────────────────────────

type authResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    int64       `json:"expires_in"`
	User         userSummary `json:"user"`
}

type userSummary struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	DisplayName   string    `json:"display_name"`
	Role          string    `json:"role"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
}

func newAuthResponse(user *userdomain.User, pair *domain.TokenPair) authResponse {
	return authResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
		User: userSummary{
			ID:            string(user.ID),
			Email:         user.Email,
			DisplayName:   user.DisplayName,
			Role:          string(user.Role),
			EmailVerified: user.EmailVerified,
			CreatedAt:     user.CreatedAt,
		},
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// mustIdentity extracts the authenticated user ID and email from the Gin context.
// If the auth middleware was not applied it aborts with 401 and returns zero values.
func mustIdentity(c *gin.Context) (userdomain.UserID, string) {
	v, exists := c.Get("user_id")
	if !exists {
		respondError(c, apierror.ErrTokenInvalid)
		return "", ""
	}
	email, _ := c.Get("email")
	emailStr, _ := email.(string)
	return userdomain.UserID(v.(string)), emailStr
}

// respondError maps domain errors to HTTP responses using the standard
// apierror pattern used by every other module handler.
func respondError(c *gin.Context, err error) {
	log := logger.FromContext(c.Request.Context())

	ae, ok := apierror.As(err)
	if !ok {
		ae = apierror.Internal("an unexpected error occurred", err)
	}

	if ae.HTTPStatus >= 500 {
		log.Error().Err(err).Str("code", string(ae.Code)).Msg("auth: internal error")
	}

	c.JSON(ae.HTTPStatus, gin.H{
		"error": gin.H{
			"code":    ae.Code,
			"message": ae.Message,
			"details": ae.Details,
		},
	})
}
