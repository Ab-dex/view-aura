package http

// ─── Auth routes ──────────────────────────────────────────────────────────────

// Register godoc
//
//	@Summary     Register
//	@Tags        Auth
//	@Accept      json
//	@Produce     json
//	@Param       body body authRegisterRequest true "Registration payload"
//	@Success     201 {object} authResponse
//	@Failure     400 {object} authErrorResponse
//	@Failure     409 {object} authErrorResponse "Email already registered"
//	@Router      /auth/register [post]
func (h *AuthHandler) swaggerRegister() {}

// Login godoc
//
//	@Summary     Login
//	@Tags        Auth
//	@Accept      json
//	@Produce     json
//	@Param       body body authLoginRequest true "Credentials"
//	@Success     200 {object} authResponse
//	@Failure     401 {object} authErrorResponse
//	@Failure     403 {object} authErrorResponse "Account locked or suspended"
//	@Router      /auth/login [post]
func (h *AuthHandler) swaggerLogin() {}

// RefreshTokens godoc
//
//	@Summary     Refresh tokens
//	@Tags        Auth
//	@Accept      json
//	@Produce     json
//	@Param       body body authRefreshRequest true "Refresh token"
//	@Success     200 {object} authTokenResponse
//	@Failure     401 {object} authErrorResponse
//	@Router      /auth/refresh [post]
func (h *AuthHandler) swaggerRefreshTokens() {}

// Logout godoc
//
//	@Summary     Logout
//	@Tags        Auth
//	@Security    BearerAuth
//	@Success     204
//	@Failure     401 {object} authErrorResponse
//	@Router      /auth/logout [post]
func (h *AuthHandler) swaggerLogout() {}

// LogoutAll godoc
//
//	@Summary     Logout all devices
//	@Tags        Auth
//	@Security    BearerAuth
//	@Success     204
//	@Failure     401 {object} authErrorResponse
//	@Router      /auth/logout/all [post]
func (h *AuthHandler) swaggerLogoutAll() {}

// ChangePassword godoc
//
//	@Summary     Change password
//	@Tags        Auth
//	@Security    BearerAuth
//	@Accept      json
//	@Param       body body authChangePasswordRequest true "Old and new password"
//	@Success     204
//	@Failure     401 {object} authErrorResponse
//	@Router      /auth/me/password [put]
func (h *AuthHandler) swaggerChangePassword() {}

// ─── Email verification ───────────────────────────────────────────────────────

// SendVerificationEmail godoc
//
//	@Summary     Send verification email
//	@Tags        Auth
//	@Security    BearerAuth
//	@Success     204
//	@Failure     401 {object} authErrorResponse
//	@Router      /auth/verify-email/send [post]
func (h *AuthHandler) swaggerSendVerificationEmail() {}

// VerifyEmail godoc
//
//	@Summary     Verify email
//	@Tags        Auth
//	@Accept      json
//	@Param       body body authTokenRequest true "Verification token"
//	@Success     204
//	@Failure     400 {object} authErrorResponse
//	@Router      /auth/verify-email [post]
func (h *AuthHandler) swaggerVerifyEmail() {}

// ─── Password reset ───────────────────────────────────────────────────────────

// SendPasswordReset godoc
//
//	@Summary     Request password reset
//	@Tags        Auth
//	@Accept      json
//	@Param       body body authEmailRequest true "Email address"
//	@Success     204
//	@Router      /auth/password-reset/request [post]
func (h *AuthHandler) swaggerSendPasswordReset() {}

// ResetPassword godoc
//
//	@Summary     Confirm password reset
//	@Tags        Auth
//	@Accept      json
//	@Produce     json
//	@Param       body body authResetPasswordRequest true "Token and new password"
//	@Success     200 {object} authResponse
//	@Failure     400 {object} authErrorResponse
//	@Router      /auth/password-reset/confirm [post]
func (h *AuthHandler) swaggerResetPassword() {}

// ─── OAuth2 ───────────────────────────────────────────────────────────────────

// OAuthBegin godoc
//
//	@Summary     Begin OAuth flow
//	@Tags        Auth
//	@Param       provider     path  string true "Provider: google | apple"
//	@Param       redirect_uri query string true "Callback URI"
//	@Success     302
//	@Failure     400 {object} authErrorResponse
//	@Router      /auth/oauth/{provider}/begin [get]
func (h *AuthHandler) swaggerOAuthBegin() {}

// OAuthCallback godoc
//
//	@Summary     OAuth callback
//	@Tags        Auth
//	@Param       provider path  string true "Provider: google | apple"
//	@Param       code     query string true "Authorization code"
//	@Param       state    query string true "CSRF state"
//	@Success     200 {object} authResponse
//	@Failure     400 {object} authErrorResponse
//	@Router      /auth/oauth/{provider}/callback [get]
func (h *AuthHandler) swaggerOAuthCallback() {}

// ─── MFA ─────────────────────────────────────────────────────────────────────

// EnrollTOTP godoc
//
//	@Summary     Enroll TOTP
//	@Tags        Auth
//	@Security    BearerAuth
//	@Produce     json
//	@Success     200 {object} authTOTPEnrollResponse
//	@Failure     401 {object} authErrorResponse
//	@Router      /auth/mfa/totp/enroll [post]
func (h *AuthHandler) swaggerEnrollTOTP() {}

// ConfirmTOTP godoc
//
//	@Summary     Confirm TOTP enrollment
//	@Tags        Auth
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       body body authTOTPConfirmRequest true "6-digit code"
//	@Success     200 {object} authRecoveryCodesResponse
//	@Failure     401 {object} authErrorResponse
//	@Router      /auth/mfa/totp/confirm [post]
func (h *AuthHandler) swaggerConfirmTOTP() {}

// DisableMFA godoc
//
//	@Summary     Disable MFA
//	@Tags        Auth
//	@Security    BearerAuth
//	@Accept      json
//	@Param       body body authDisableMFARequest true "Current password"
//	@Success     204
//	@Failure     401 {object} authErrorResponse
//	@Router      /auth/mfa [delete]
func (h *AuthHandler) swaggerDisableMFA() {}

// VerifyMFA godoc
//
//	@Summary     Verify MFA code
//	@Tags        Auth
//	@Accept      json
//	@Produce     json
//	@Param       body body authVerifyMFARequest true "User ID, code, method"
//	@Success     200 {object} authResponse
//	@Failure     401 {object} authErrorResponse
//	@Router      /auth/mfa/verify [post]
func (h *AuthHandler) swaggerVerifyMFA() {}

// SendMFAEmailOTP godoc
//
//	@Summary     Send email OTP
//	@Tags        Auth
//	@Accept      json
//	@Param       body body authUserIDRequest true "User ID"
//	@Success     204
//	@Failure     400 {object} authErrorResponse
//	@Router      /auth/mfa/email-otp/send [post]
func (h *AuthHandler) swaggerSendMFAEmailOTP() {}

// ─── Sessions ─────────────────────────────────────────────────────────────────

// ListSessions godoc
//
//	@Summary     List active sessions
//	@Tags        Auth
//	@Security    BearerAuth
//	@Produce     json
//	@Success     200 {object} authSessionListResponse
//	@Failure     401 {object} authErrorResponse
//	@Router      /auth/me/sessions [get]
func (h *AuthHandler) swaggerListSessions() {}

// RevokeSession godoc
//
//	@Summary     Revoke session
//	@Tags        Auth
//	@Security    BearerAuth
//	@Param       id path string true "Session ID"
//	@Success     204
//	@Failure     401 {object} authErrorResponse
//	@Router      /auth/me/sessions/{id} [delete]
func (h *AuthHandler) swaggerRevokeSession() {}

// ─── Request / response types ─────────────────────────────────────────────────

type authRegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	OtherNames  string `json:"other_names,omitempty"`
	Locale      string `json:"locale,omitempty"`
	Country     string `json:"country,omitempty"`
}

type authLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type authTokenRequest struct {
	Token string `json:"token"`
}

type authEmailRequest struct {
	Email string `json:"email"`
}

type authResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type authChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type authTOTPConfirmRequest struct {
	Code string `json:"code"`
}

type authDisableMFARequest struct {
	Password string `json:"password"`
}

type authVerifyMFARequest struct {
	UserID string `json:"user_id"`
	Code   string `json:"code"`
	Method string `json:"method"`
}

type authUserIDRequest struct {
	UserID string `json:"user_id"`
}

type authTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type authTOTPEnrollResponse struct {
	Secret    string `json:"secret"`
	QRCodeURI string `json:"qr_code_uri"`
}

type authRecoveryCodesResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
	Message       string   `json:"message"`
}

type authSessionItem struct {
	ID        string `json:"id"`
	DeviceID  string `json:"device_id"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
}

type authSessionListResponse struct {
	Sessions []authSessionItem `json:"sessions"`
}

type authErrorResponse struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}
