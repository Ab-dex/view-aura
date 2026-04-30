package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/modules/user/domain"
	"github.com/Ab-dex/view-aura/internal/modules/user/service"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// UserHandler exposes user identity and preference routes.
//
// Auth routes (register, login, refresh, logout, change-password, sessions)
// are owned by auth/api/http.AuthHandler and mounted under /auth.
type UserHandler struct {
	svc service.UserService
}

func NewUserHandler(svc service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// RegisterRoutes mounts routes that do not require authentication.
// Currently empty — all user routes require an authenticated session.
func (h *UserHandler) RegisterRoutes(_ gin.IRouter) {}

// RegisterProtectedRoutes mounts routes that require a valid access token.
// The auth middleware must be applied by the caller before this group.
func (h *UserHandler) RegisterProtectedRoutes(r gin.IRouter) {
	r.GET("/me", h.GetMe)
	r.GET("/me/account", h.GetAccountDetails)
	r.PATCH("/me/account", h.UpdateAccountDetails)
	r.GET("/me/preferences", h.GetPreferences)
	r.PATCH("/me/preferences", h.UpdatePreferences)
	r.DELETE("/me", h.DeleteAccount)
	r.GET("/me/profile", h.GetProfile)
	r.PATCH("/me/profile", h.UpdateProfile)
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

func (h *UserHandler) GetMe(c *gin.Context) {
	userID := MustUserID(c)
	user, err := h.svc.GetByID(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toUserResponse(user))
}

func (h *UserHandler) GetAccountDetails(c *gin.Context) {
	userID := MustUserID(c)
	profile, err := h.svc.GetAccountDetails(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProfileResponse(profile))
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

func (h *UserHandler) UpdateAccountDetails(c *gin.Context) {
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}
	userID := MustUserID(c)
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
			respondError(c, apierror.Validation("birthdate must be YYYY-MM-DD", nil))
			return
		}
		cmd.Birthdate = &t
	}
	profile, err := h.svc.UpdateAccountDetails(c.Request.Context(), cmd)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProfileResponse(profile))
}

func (h *UserHandler) GetPreferences(c *gin.Context) {
	userID := MustUserID(c)
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
	userID := MustUserID(c)
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

// DeleteAccount soft-deletes the account.
// The handler also calls auth.AuthService.LogoutAll to revoke all sessions —
// the auth service is injected so the user handler stays auth-independent.
func (h *UserHandler) DeleteAccount(c *gin.Context) {
	userID := MustUserID(c)
	if err := h.svc.SoftDelete(c.Request.Context(), userID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── DTOs ─────────────────────────────────────────────────────────────────────

type updateProfileRequest struct {
	AvatarURL  *string `json:"avatar_url"`
	BannerURL  *string `json:"banner_url"`
	Bio        *string `json:"bio"`
	Website    *string `json:"website"`
	Birthdate  *string `json:"birthdate"`
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

// ─── Response helpers ─────────────────────────────────────────────────────────

// mustUserID extracts the authenticated user ID from the Gin context.
// It panics if the auth middleware was not applied (programming error).
func mustUserID(c *gin.Context) domain.UserID {
	v, exists := c.Get("user_id")
	if !exists {
		panic("auth middleware not applied — user_id not in context")
	}
	return domain.UserID(v.(string))
}

func toUserResponse(u *domain.User) userResponse {
	return userResponse{
		ID:            u.ID.String(),
		Email:         u.Email,
		EmailVerified: u.EmailVerified,
		Username:      u.UserName,
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

// MustUserID extracts the authenticated user ID from Gin context.
func MustUserID(c *gin.Context) domain.UserID {
	v, exists := c.Get("user_id")
	if !exists {
		panic("auth middleware not applied — user_id missing from context")
	}
	return domain.UserID(v.(string))
}

func respondError(c *gin.Context, err error) {
	log := logger.FromContext(c.Request.Context())
	ae, ok := apierror.As(err)
	if !ok {
		ae = apierror.Internal("an unexpected error occurred", err)
	}
	if ae.HTTPStatus >= 500 {
		log.Error().Err(err).Str("code", string(ae.Code)).Msg("user: internal error")
	}
	c.JSON(ae.HTTPStatus, gin.H{
		"error": gin.H{"code": ae.Code, "message": ae.Message, "details": ae.Details},
	})
}
