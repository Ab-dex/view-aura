package http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/app/middleware"
	authdomain "github.com/Ab-dex/view-aura/internal/modules/auth/domain"
	"github.com/Ab-dex/view-aura/internal/modules/auth/service"
	userdomain "github.com/Ab-dex/view-aura/internal/modules/user/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// AuthHandler exposes every authentication and session flow over HTTP.
//
// Route ownership:
//   - Public:    register, login, refresh, verify-email, password-reset,
//     oauth begin/callback, mfa/verify, mfa/email-otp/send
//   - Protected: logout, logout-all, change-password, verify-email/send,
//     mfa totp enroll/confirm, mfa disable, sessions list/revoke
type AuthHandler struct {
	svc service.AuthService
}

// NewAuthHandler constructs the handler.
// authMW is the real JWT validation middleware from contract.AuthMiddleware.
// It is injected here so the handler is self-contained and testable.
func NewAuthHandler(svc service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// RegisterRoutes mounts all auth routes on the given group.
// The caller applies rate-limiting at the group level before calling this.
func (h *AuthHandler) RegisterRoutes(r gin.IRouter) {
	// ── Public ───────────────────────────────────────────────────────────────
	r.POST("/register", middleware.StrictRateLimit(10, time.Minute), h.Register)
	r.POST("/login", middleware.StrictRateLimit(10, time.Minute), h.Login)
	r.POST("/refresh", middleware.StrictRateLimit(10, time.Minute), h.RefreshTokens)

	r.POST("/verify-email", h.VerifyEmail)
	r.POST("/password-reset/request", middleware.StrictRateLimit(5, time.Minute), h.SendPasswordReset)
	r.POST("/password-reset/confirm", h.ResetPassword)

	r.GET("/oauth/:provider/begin", h.OAuthBegin)
	r.GET("/oauth/:provider/callback", h.OAuthCallback)

	r.POST("/mfa/verify", h.VerifyMFA)
	r.POST("/mfa/email-otp/send", middleware.StrictRateLimit(5, time.Minute), h.SendMFAEmailOTP)

}

func (h *AuthHandler) RegisterProtectedRoutes(r gin.IRouter) {

	r.POST("/logout", h.Logout)
	r.POST("/logout/all", h.LogoutAll)
	r.POST("/verify-email/send", h.SendVerificationEmail)
	r.PUT("/me/password", h.ChangePassword)

	r.POST("/mfa/totp/enroll", h.EnrollTOTP)
	r.POST("/mfa/totp/confirm", h.ConfirmTOTP)
	r.DELETE("/mfa", h.DisableMFA)

	r.GET("/me/sessions", h.ListSessions)
	r.DELETE("/me/sessions/:id", h.RevokeSession)
}

// ─── Credential-based auth ────────────────────────────────────────────────────

func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Email       string `json:"email"        binding:"required,email"`
		Password    string `json:"password"     binding:"required,min=8"`
		DisplayName string `json:"display_name" binding:"required,min=2"`
		UserName    string `json:"user_name" binding:"required,min=2"`
		OtherNames  string `json:"other_names"`
		Locale      string `json:"locale"`
		Country     string `json:"country"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}
	user, pair, err := h.svc.Register(c.Request.Context(), authdomain.RegisterCmd{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		UserName:    req.UserName,
		Locale:      req.Locale,
		Country:     req.Country,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, newAuthResponse(user, pair))
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"    binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), "Invalid Request body"))
		return
	}

	user, pair, err := h.svc.Login(c.Request.Context(), authdomain.LoginCmd{
		Email:     req.Email,
		Password:  req.Password,
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

func (h *AuthHandler) RefreshTokens(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}
	pair, err := h.svc.RefreshTokens(
		c.Request.Context(),
		req.RefreshToken,
		c.GetHeader("X-Device-ID"),
		c.ClientIP(),
		c.Request.UserAgent(),
	)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	jti, _ := c.Get(middleware.ContextKeyJTI)
	sessionID, _ := c.Get(middleware.ContextKeySessionID)
	remainingTTL, _ := c.Get(middleware.ContextKeyRemainingTTL)

	jtiStr, _ := jti.(string)
	sessionStr, _ := sessionID.(string)
	ttl, _ := remainingTTL.(time.Duration)

	if err := h.svc.Logout(c.Request.Context(), jtiStr, sessionStr, ttl); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) LogoutAll(c *gin.Context) {
	userID := mustUserID(c)
	if userID == "" {
		return
	}
	if err := h.svc.LogoutAll(c.Request.Context(), userdomain.UserID(userID)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}
	userID := mustUserID(c)
	if userID == "" {
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), authdomain.ChangePasswordCmd{
		UserID:      userID,
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	}); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Email verification ───────────────────────────────────────────────────────

func (h *AuthHandler) SendVerificationEmail(c *gin.Context) {
	userID, email := mustIdentity(c)
	if userID == "" {
		return
	}
	if err := h.svc.SendVerificationEmail(c.Request.Context(), authdomain.SendVerificationEmailCmd{
		UserID:    userID,
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
		Token  string  `json:"token" binding:"required"`
		UserID *string `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation("token is required", nil))
		return
	}

	// Try to get userID from auth context first (if logged in)
	var userID *string

	if uid, exists := c.Get("user_id"); exists {
		if uidStr, ok := uid.(string); ok {
			userID = &uidStr
		}
	}
	fmt.Printf("user id: %v", userID)

	// fallback to request body if not authenticated
	if userID == nil {
		userID = req.UserID
	}

	cmd := authdomain.VerifyEmailCmd{
		Token:     req.Token,
		UserID:    userID,
		IPAddress: c.ClientIP(),
	}

	if err := h.svc.VerifyEmail(c.Request.Context(), cmd); err != nil {
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
	// Always 204 — never reveal whether the email is registered.
	_ = c.ShouldBindJSON(&req)
	_ = h.svc.SendPasswordReset(c.Request.Context(), authdomain.SendPasswordResetCmd{
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
	user, pair, err := h.svc.ResetPassword(c.Request.Context(), authdomain.ResetPasswordCmd{
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
	result, err := h.svc.OAuthBegin(c.Request.Context(), authdomain.OAuthBeginCmd{
		Provider:    authdomain.AuthProvider(c.Param("provider")),
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
	user, pair, err := h.svc.OAuthCallback(c.Request.Context(), authdomain.OAuthCallbackCmd{
		Provider:  authdomain.AuthProvider(c.Param("provider")),
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
	userID := mustUserID(c)
	if userID == "" {
		return
	}
	result, err := h.svc.EnrollTOTP(c.Request.Context(), authdomain.EnrollTOTPCmd{
		UserID: userID, // EnrollTOTPCmd.UserID is string
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
	userID := mustUserID(c)
	if userID == "" {
		return
	}
	codes, err := h.svc.ConfirmTOTP(c.Request.Context(), authdomain.VerifyTOTPEnrollmentCmd{
		UserID: userID,
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
	userID := mustUserID(c)
	if userID == "" {
		return
	}
	// DisableMFACmd.UserID is string in auth/domain
	if err := h.svc.DisableMFA(c.Request.Context(), authdomain.DisableMFACmd{
		UserID:   userID,
		Password: req.Password,
	}); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── MFA verify (public — second factor before session is issued) ─────────────

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
	user, pair, err := h.svc.VerifyMFA(c.Request.Context(), authdomain.VerifyMFACmd{
		// VerifyMFACmd in auth/domain uses userdomain.UserID
		UserID:    (req.UserID),
		Code:      req.Code,
		Method:    authdomain.MFAMethod(req.Method),
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

// ─── Sessions (protected) ─────────────────────────────────────────────────────

func (h *AuthHandler) ListSessions(c *gin.Context) {
	userID := mustUserID(c)
	if userID == "" {
		return
	}
	sessions, err := h.svc.ListSessions(c.Request.Context(), userdomain.UserID(userID))
	if err != nil {
		respondError(c, err)
		return
	}
	type sessionItem struct {
		ID        string `json:"id"`
		DeviceID  string `json:"device_id"`
		IPAddress string `json:"ip_address"`
		UserAgent string `json:"user_agent"`
		CreatedAt string `json:"created_at"`
		ExpiresAt string `json:"expires_at"`
	}
	out := make([]sessionItem, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, sessionItem{
			ID:        s.ID,
			DeviceID:  s.DeviceID,
			IPAddress: s.IPAddress,
			UserAgent: s.UserAgent,
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
			ExpiresAt: s.ExpiresAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"sessions": out})
}

func (h *AuthHandler) RevokeSession(c *gin.Context) {
	userID := mustUserID(c)
	if userID == "" {
		return
	}
	if err := h.svc.RevokeSession(c.Request.Context(), userdomain.UserID(userID), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Response types ───────────────────────────────────────────────────────────

type authResponse struct {
	AccessToken  *string     `json:"access_token,omitempty"`
	RefreshToken *string     `json:"refresh_token,omitempty"`
	ExpiresIn    *int64      `json:"expires_in,omitempty"`
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

// newAuthResponse builds the auth response from the service return values.
// user is *userdomain.User; pair is *authdomain.TokenPair.
func newAuthResponse(user *userdomain.User, pair *authdomain.TokenPair) authResponse {
	var accessToken *string
	var refreshToken *string
	var expiresIn *int64

	if pair != nil {
		accessToken = &pair.AccessToken
		refreshToken = &pair.RefreshToken
		expiresIn = &pair.ExpiresIn
	}

	return authResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		User: userSummary{
			ID:            user.ID.String(),
			Email:         user.Email,
			DisplayName:   user.DisplayName,
			Role:          string(user.Role),
			EmailVerified: user.EmailVerified,
			CreatedAt:     user.CreatedAt,
		},
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// mustUserID extracts the authenticated user ID string from the Gin context.
// Returns "" and aborts with 401 when the auth middleware was not applied.
// Returns a plain string — callers cast to userdomain.UserID where needed.
func mustUserID(c *gin.Context) string {
	v, exists := c.Get(middleware.ContextKeyUserID)
	if !exists {
		respondError(c, apierror.ErrTokenInvalid)
		return ""
	}
	id, _ := v.(string)
	return id
}

// mustIdentity returns (userID string, email string) from the Gin context.
// Aborts with 401 and returns ("", "") when the token is missing.
func mustIdentity(c *gin.Context) (string, string) {
	v, exists := c.Get(middleware.ContextKeyUserID)
	if !exists {
		respondError(c, apierror.ErrTokenInvalid)
		return "", ""
	}
	userID, _ := v.(string)
	email, _ := c.Get("email")
	emailStr, _ := email.(string)
	return userID, emailStr
}

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
