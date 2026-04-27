package api

// Register godoc
//
//	@Summary     Register a new account
//	@Description Creates an email/password account and returns a JWT pair.
//	             The account starts in `pending_verification` status; email
//	             verification is required before login is permitted.
//	@Tags        Auth
//	@Accept      json
//	@Produce     json
//	@Param       body body registerRequest true "Registration payload"
//	@Success     201 {object} authResponse
//	@Failure     400 {object} errorResponse "Validation error"
//	@Failure     409 {object} errorResponse "Email already registered"
//	@Router      /users/register [post]
func (h *UserHandler) swaggerRegister() {}

// Login godoc
//
//	@Summary     Login
//	@Description Authenticates with email and password, returns a JWT pair and
//	             session details.
//	@Tags        Auth
//	@Accept      json
//	@Produce     json
//	@Param       body body loginRequest true "Login credentials"
//	@Success     200 {object} authResponse
//	@Failure     401 {object} errorResponse "Invalid credentials"
//	@Failure     403 {object} errorResponse "Account locked or suspended"
//	@Router      /users/login [post]
func (h *UserHandler) swaggerLogin() {}

// RefreshTokens godoc
//
//	@Summary     Refresh tokens
//	@Description Exchanges a valid refresh token for a new access+refresh pair.
//	             The old refresh token is revoked immediately (rotation).
//	@Tags        Auth
//	@Accept      json
//	@Produce     json
//	@Param       body body refreshRequest true "Refresh token"
//	@Success     200 {object} tokenResponse
//	@Failure     401 {object} errorResponse "Token invalid or expired"
//	@Router      /users/refresh [post]
func (h *UserHandler) swaggerRefreshTokens() {}

// Logout godoc
//
//	@Summary     Logout
//	@Description Revokes the current session and adds the JTI to the blocklist.
//	@Tags        Auth
//	@Security    BearerAuth
//	@Success     204 "Session revoked"
//	@Failure     401 {object} errorResponse
//	@Router      /users/logout [post]
func (h *UserHandler) swaggerLogout() {}

// LogoutAll godoc
//
//	@Summary     Logout all devices
//	@Description Revokes every active session for the authenticated user.
//	@Tags        Auth
//	@Security    BearerAuth
//	@Success     204 "All sessions revoked"
//	@Failure     401 {object} errorResponse
//	@Router      /users/logout/all [post]
func (h *UserHandler) swaggerLogoutAll() {}

// ─── Account Details ───────────────────────────────────────────────────────────────

// GetMe godoc
//
//	@Summary     Get current user
//	@Description Returns the authenticated user's account record.
//	@Tags        Users
//	@Security    BearerAuth
//	@Produce     json
//	@Success     200 {object} userResponse
//	@Failure     401 {object} errorResponse
//	@Router      /users/me [get]
func (h *UserHandler) swaggerGetMe() {}

// GetAccountDetails godoc
//
//	@Summary     Get Account Details
//	@Description Returns the authenticated user's public account details (avatar, bio, etc.).
//	@Tags        Users
//	@Security    BearerAuth
//	@Produce     json
//	@Success     200 {object} profileResponse
//	@Failure     401 {object} errorResponse
//	@Router      /users/me/account [get]
func (h *UserHandler) swaggerGetAccountDetails() {}

// UpdateAccountDetails godoc
//
//	@Summary     Update account details
//	@Description Applies partial updates to the authenticated user's account details.
//	             All fields are optional — only provided fields are changed.
//	@Tags        Users
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       body body updateProfileRequest true "Account fields to update"
//	@Success     200 {object} profileResponse
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Router      /users/me/account [patch]
func (h *UserHandler) swaggerUpdateAccountDetails() {}

// GetPreferences godoc
//
//	@Summary     Get preferences
//	@Description Returns genre, language, and UI preferences for the user.
//	@Tags        Users
//	@Security    BearerAuth
//	@Produce     json
//	@Success     200 {object} preferencesResponse
//	@Failure     401 {object} errorResponse
//	@Router      /users/me/preferences [get]
func (h *UserHandler) swaggerGetPreferences() {}

// UpdatePreferences godoc
//
//	@Summary     Update preferences
//	@Description Applies partial updates to the user's preferences.
//	@Tags        Users
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       body body updatePreferencesRequest true "Preference fields to update"
//	@Success     200 {object} preferencesResponse
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Router      /users/me/preferences [patch]
func (h *UserHandler) swaggerUpdatePreferences() {}

// ChangePassword godoc
//
//	@Summary     Change password
//	@Description Verifies the old password then replaces it. All sessions are
//	             revoked on success, requiring re-login on all devices.
//	@Tags        Users
//	@Security    BearerAuth
//	@Accept      json
//	@Success     204 "Password updated"
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse "Wrong old password"
//	@Router      /users/me/password [put]
func (h *UserHandler) swaggerChangePassword() {}

// DeleteAccount godoc
//
//	@Summary     Delete account
//	@Description Soft-deletes the authenticated user's account and revokes all sessions.
//	@Tags        Users
//	@Security    BearerAuth
//	@Success     204 "Account deleted"
//	@Failure     401 {object} errorResponse
//	@Router      /users/me [delete]
func (h *UserHandler) swaggerDeleteAccount() {}

// ListSessions godoc
//
//	@Summary     List active sessions
//	@Description Returns all non-expired, non-revoked sessions for the user.
//	@Tags        Users
//	@Security    BearerAuth
//	@Produce     json
//	@Success     200 {object} map[string]interface{} "sessions array"
//	@Failure     401 {object} errorResponse
//	@Router      /users/me/sessions [get]
func (h *UserHandler) swaggerListSessions() {}

// RevokeSession godoc
//
//	@Summary     Revoke a session
//	@Description Revokes a single session by ID (logout from one device).
//	@Tags        Users
//	@Security    BearerAuth
//	@Param       session_id path string true "Session ID"
//	@Success     204 "Session revoked"
//	@Failure     401 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /users/me/sessions/{session_id} [delete]
func (h *UserHandler) swaggerRevokeSession() {}

// ─── Shared response schema stubs (so swag picks them up) ────────────────────

// errorResponse is the standard error envelope returned on all 4xx/5xx responses.
type errorResponse struct {
	Error struct {
		Code    string      `json:"code"    example:"VALIDATION_ERROR"`
		Message string      `json:"message" example:"email must be a valid email address"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Locale      string `json:"locale"`
	Country     string `json:"country"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type authResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	User         struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		DisplayName   string `json:"display_name"`
		Role          string `json:"role"`
		EmailVerified bool   `json:"email_verified"`
	} `json:"user"`
}
