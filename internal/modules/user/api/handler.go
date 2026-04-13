package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/modules/user/domain"
	"github.com/Ab-dex/view-aura/internal/modules/user/service"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// UserHandler wires HTTP routes to the UserService.
type UserHandler struct {
	svc service.UserService
}

// NewUserHandler constructs the handler. Wire calls this.
func NewUserHandler(svc service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// RegisterRoutes attaches all User routes to the given Gin router group.
// Prefix is typically /api/v1/users  (applied by the caller).
func (h *UserHandler) RegisterRoutes(r gin.IRouter) {
	// Public
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)
	r.POST("/refresh", h.RefreshTokens)
}

func (h *UserHandler) RegisterProtectedRoutes(r gin.IRouter) {
	r.POST("/logout", h.Logout)
	r.POST("/logout/all", h.LogoutAll)
	r.GET("/me", h.GetMe)
	r.GET("/me/profile", h.GetProfile)
	r.PATCH("/me/profile", h.UpdateProfile)
	r.GET("/me/preferences", h.GetPreferences)
	r.PATCH("/me/preferences", h.UpdatePreferences)
	r.PUT("/me/password", h.ChangePassword)
	r.DELETE("/me", h.DeleteAccount)
	r.GET("/me/sessions", h.ListSessions)
	r.DELETE("/me/sessions/:session_id", h.RevokeSession)
}

// ─── Request / Response DTOs ──────────────────────────────────────────────────

type registerRequest struct {
	Email       string `json:"email"        binding:"required,email"`
	Password    string `json:"password"     binding:"required,min=8"`
	DisplayName string `json:"display_name" binding:"required,min=2"`
	Locale      string `json:"locale"`
	Country     string `json:"country"`
}

type loginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type updateProfileRequest struct {
	AvatarURL  *string `json:"avatar_url"`
	BannerURL  *string `json:"banner_url"`
	Bio        *string `json:"bio"`
	Website    *string `json:"website"`
	Birthdate  *string `json:"birthdate"` // RFC3339 date
	Gender     *string `json:"gender"`
	Visibility *string `json:"visibility"`
}

type updatePreferencesRequest struct {
	PreferredGenres    *[]string `json:"preferred_genres"`
	DislikedGenres     *[]string `json:"disliked_genres"`
	PreferredLanguages *[]string `json:"preferred_languages"`
	AdultContent       *bool     `json:"adult_content"`
	DarkMode           *bool     `json:"dark_mode"`
	AutoplayTrailers   *bool     `json:"autoplay_trailers"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// userResponse is the safe external representation of a user (no hash etc.).
type userResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Username      string `json:"username,omitempty"`
	DisplayName   string `json:"display_name"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	Locale        string `json:"locale"`
	Country       string `json:"country"`
	CreatedAt     string `json:"created_at"`
}

type authResponse struct {
	User         userResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int64        `json:"expires_in"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type profileResponse struct {
	UserID     string `json:"user_id"`
	AvatarURL  string `json:"avatar_url"`
	BannerURL  string `json:"banner_url"`
	Bio        string `json:"bio"`
	Website    string `json:"website"`
	Birthdate  string `json:"birthdate,omitempty"`
	Gender     string `json:"gender"`
	Visibility string `json:"visibility"`
	UpdatedAt  string `json:"updated_at"`
}

type preferencesResponse struct {
	PreferredGenres    []string `json:"preferred_genres"`
	DislikedGenres     []string `json:"disliked_genres"`
	PreferredLanguages []string `json:"preferred_languages"`
	AdultContent       bool     `json:"adult_content"`
	DarkMode           bool     `json:"dark_mode"`
	AutoplayTrailers   bool     `json:"autoplay_trailers"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

func (h *UserHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	user, pair, err := h.svc.Register(c.Request.Context(), domain.RegisterCmd{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Locale:      req.Locale,
		Country:     req.Country,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, authResponse{
		User:         toUserResponse(user),
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
	})
}

func (h *UserHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	user, pair, err := h.svc.Login(c.Request.Context(), domain.LoginCmd{
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

	c.JSON(http.StatusOK, authResponse{
		User:         toUserResponse(user),
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
	})
}

func (h *UserHandler) RefreshTokens(c *gin.Context) {
	var req refreshRequest
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

	c.JSON(http.StatusOK, tokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
	})
}

func (h *UserHandler) Logout(c *gin.Context) {
	jti, _ := c.Get("jwt_jti")
	sessionID, _ := c.Get("session_id")
	remainingTTL, _ := c.Get("jwt_remaining_ttl")

	ttl, _ := remainingTTL.(time.Duration)
	err := h.svc.Logout(
		c.Request.Context(),
		jti.(string),
		sessionID.(string),
		ttl,
	)
	if err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *UserHandler) LogoutAll(c *gin.Context) {
	userID := mustUserID(c)
	if err := h.svc.LogoutAll(c.Request.Context(), userID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *UserHandler) GetMe(c *gin.Context) {
	userID := mustUserID(c)
	user, err := h.svc.GetByID(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toUserResponse(user))
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := mustUserID(c)
	profile, err := h.svc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProfileResponse(profile))
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	userID := mustUserID(c)
	cmd := domain.UpdateProfileCmd{
		UserID:     userID,
		AvatarURL:  req.AvatarURL,
		BannerURL:  req.BannerURL,
		Bio:        req.Bio,
		Website:    req.Website,
		Gender:     req.Gender,
		Visibility: req.Visibility,
	}
	if req.Birthdate != nil {
		t, err := time.Parse("2006-01-02", *req.Birthdate)
		if err != nil {
			respondError(c, apierror.Validation("birthdate must be in YYYY-MM-DD format", nil))
			return
		}
		cmd.Birthdate = &t
	}

	profile, err := h.svc.UpdateProfile(c.Request.Context(), cmd)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProfileResponse(profile))
}

func (h *UserHandler) GetPreferences(c *gin.Context) {
	userID := mustUserID(c)
	prefs, err := h.svc.GetPreferences(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPreferencesResponse(prefs))
}

func (h *UserHandler) UpdatePreferences(c *gin.Context) {
	var req updatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	userID := mustUserID(c)
	prefs, err := h.svc.UpdatePreferences(c.Request.Context(), domain.UpdatePreferencesCmd{
		UserID:             userID,
		PreferredGenres:    req.PreferredGenres,
		DislikedGenres:     req.DislikedGenres,
		PreferredLanguages: req.PreferredLanguages,
		AdultContent:       req.AdultContent,
		DarkMode:           req.DarkMode,
		AutoplayTrailers:   req.AutoplayTrailers,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPreferencesResponse(prefs))
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	userID := mustUserID(c)
	if err := h.svc.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *UserHandler) DeleteAccount(c *gin.Context) {
	userID := mustUserID(c)
	if err := h.svc.SoftDelete(c.Request.Context(), userID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *UserHandler) ListSessions(c *gin.Context) {
	userID := mustUserID(c)
	sessions, err := h.svc.ListSessions(c.Request.Context(), userID)
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

func (h *UserHandler) RevokeSession(c *gin.Context) {
	userID := mustUserID(c)
	sessionID := c.Param("session_id")

	if err := h.svc.RevokeSession(c.Request.Context(), userID, sessionID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Middleware helpers ────────────────────────────────────────────────────────

// mustUserID extracts the authenticated user ID from the Gin context.
// It panics if the auth middleware was not applied (programming error).
func mustUserID(c *gin.Context) domain.UserID {
	v, exists := c.Get("user_id")
	if !exists {
		panic("auth middleware not applied — user_id not in context")
	}
	return domain.UserID(v.(string))
}

// ─── Response helpers ──────────────────────────────────────────────────────────

func respondError(c *gin.Context, err error) {
	log := logger.FromContext(c.Request.Context())

	ae, ok := apierror.As(err)
	if !ok {
		// Unexpected error — wrap it.
		ae = apierror.Internal("an unexpected error occurred", err)
	}

	if ae.HTTPStatus >= 500 {
		log.Error().Err(err).Str("code", string(ae.Code)).Msg("internal error")
	}

	c.JSON(ae.HTTPStatus, gin.H{
		"error": gin.H{
			"code":    ae.Code,
			"message": ae.Message,
			"details": ae.Details,
		},
	})
}

func toUserResponse(u *domain.User) userResponse {
	return userResponse{
		ID:            u.ID.String(),
		Email:         u.Email,
		EmailVerified: u.EmailVerified,
		Username:      u.Username,
		DisplayName:   u.DisplayName,
		Role:          string(u.Role),
		Status:        string(u.Status),
		Locale:        u.Locale,
		Country:       u.Country,
		CreatedAt:     u.CreatedAt.Format(time.RFC3339),
	}
}

func toProfileResponse(p *domain.UserProfile) profileResponse {
	resp := profileResponse{
		UserID:     p.UserID.String(),
		AvatarURL:  p.AvatarURL,
		BannerURL:  p.BannerURL,
		Bio:        p.Bio,
		Website:    p.Website,
		Gender:     p.Gender,
		Visibility: string(p.Visibility),
		UpdatedAt:  p.UpdatedAt.Format(time.RFC3339),
	}
	if p.Birthdate != nil {
		resp.Birthdate = p.Birthdate.Format("2006-01-02")
	}
	return resp
}

func toPreferencesResponse(p *domain.UserPreferences) preferencesResponse {
	return preferencesResponse{
		PreferredGenres:    orSlice(p.PreferredGenres),
		DislikedGenres:     orSlice(p.DislikedGenres),
		PreferredLanguages: orSlice(p.PreferredLanguages),
		AdultContent:       p.AdultContent,
		DarkMode:           p.DarkMode,
		AutoplayTrailers:   p.AutoplayTrailers,
	}
}

func orSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// Suppress unused import warning from strings.
var _ = strings.TrimSpace
