package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/modules/profile/domain"
	"github.com/Ab-dex/view-aura/internal/modules/profile/service"
	userapi "github.com/Ab-dex/view-aura/internal/modules/user/api"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// ProfileHandler exposes the household profiles API.
// Routes are registered under /me/profiles by UserHandler.RegisterProtectedRoutes.
type ProfileHandler struct {
	svc service.ProfileService
}

func NewProfileHandler(svc service.ProfileService) *ProfileHandler {
	return &ProfileHandler{svc: svc}
}

// RegisterProfileRoutes attaches all profile routes under the given router group.
// Expected to be called from UserHandler.RegisterProtectedRoutes with a
// /me/profiles group prefix.
//
//	GET    /me/profiles          — list all profiles for the authenticated user
//	POST   /me/profiles          — create a new profile
//	GET    /me/profiles/:id      — get a single profile
//	PATCH  /me/profiles/:id      — update a profile
//	DELETE /me/profiles/:id      — delete a profile
//	POST   /me/profiles/:id/switch — switch active profile (validates PIN)
//	PUT    /me/profiles/:id/default — set a profile as the account default
func (h *ProfileHandler) RegisterProfileRoutes(r gin.IRouter) {
	g := r.Group("/me/profiles")
	{
		g.GET("", h.List)
		g.POST("", h.Create)
		g.GET("/:id", h.Get)
		g.PATCH("/:id", h.Update)
		g.DELETE("/:id", h.Delete)
		g.POST("/:id/switch", h.Switch)
		g.PUT("/:id/default", h.SetDefault)
	}
}

// ─── List ─────────────────────────────────────────────────────────────────────

func (h *ProfileHandler) List(c *gin.Context) {
	userID := userapi.MustUserID(c)
	profiles, err := h.svc.List(c.Request.Context(), domain.UserID(userID))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"profiles": toProfileResponses(profiles)})
}

// ─── Create ───────────────────────────────────────────────────────────────────

type createProfileRequest struct {
	Name      string             `json:"name"       binding:"required,max=50"`
	AvatarURL string             `json:"avatar_url"`
	Type      domain.ProfileType `json:"type"       binding:"required"`
	PIN       string             `json:"pin"` // optional; must be 4 digits if provided
	MaxRating *domain.MPAARating `json:"max_rating"`
}

func (h *ProfileHandler) Create(c *gin.Context) {
	var req createProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}
	userID := userapi.MustUserID(c)
	p, err := h.svc.Create(c.Request.Context(), domain.CreateProfileCmd{
		UserID:    domain.UserID(userID),
		Name:      req.Name,
		AvatarURL: req.AvatarURL,
		Type:      req.Type,
		PIN:       req.PIN,
		MaxRating: req.MaxRating,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toProfileResponse(p))
}

// ─── Get ──────────────────────────────────────────────────────────────────────

func (h *ProfileHandler) Get(c *gin.Context) {
	userID := userapi.MustUserID(c)
	p, err := h.svc.GetByID(c.Request.Context(),
		domain.UserID(userID), domain.ProfileID(c.Param("id")))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProfileResponse(p))
}

// ─── Update ───────────────────────────────────────────────────────────────────

type updateProfileRequest struct {
	Name      *string            `json:"name"`
	AvatarURL *string            `json:"avatar_url"`
	PIN       *string            `json:"pin"`
	MaxRating *domain.MPAARating `json:"max_rating"`
	IsDefault *bool              `json:"is_default"`
	SortOrder *int               `json:"sort_order"`
}

func (h *ProfileHandler) Update(c *gin.Context) {
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}
	userID := userapi.MustUserID(c)
	p, err := h.svc.Update(c.Request.Context(), domain.UpdateProfileCmd{
		UserID:    domain.UserID(userID),
		ProfileID: domain.ProfileID(c.Param("id")),
		Name:      req.Name,
		AvatarURL: req.AvatarURL,
		PIN:       req.PIN,
		MaxRating: req.MaxRating,
		IsDefault: req.IsDefault,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProfileResponse(p))
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func (h *ProfileHandler) Delete(c *gin.Context) {
	userID := userapi.MustUserID(c)
	if err := h.svc.Delete(c.Request.Context(), domain.DeleteProfileCmd{
		UserID:    domain.UserID(userID),
		ProfileID: domain.ProfileID(c.Param("id")),
	}); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Switch ───────────────────────────────────────────────────────────────────

type switchProfileRequest struct {
	PIN string `json:"pin"` // required only when the profile has a PIN
}

func (h *ProfileHandler) Switch(c *gin.Context) {
	var req switchProfileRequest
	_ = c.ShouldBindJSON(&req) // PIN is optional
	userID := userapi.MustUserID(c)

	p, err := h.svc.Switch(c.Request.Context(), domain.SwitchProfileCmd{
		UserID:    domain.UserID(userID),
		ProfileID: domain.ProfileID(c.Param("id")),
		PIN:       req.PIN,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	// Return the profile so the client can update its active profile state
	// and re-fetch personalised content.
	c.JSON(http.StatusOK, gin.H{
		"active_profile": toProfileResponse(p),
		"message":        "profile switched successfully",
	})
}

// ─── SetDefault ───────────────────────────────────────────────────────────────

func (h *ProfileHandler) SetDefault(c *gin.Context) {
	userID := userapi.MustUserID(c)
	if err := h.svc.SetDefault(c.Request.Context(),
		domain.UserID(userID), domain.ProfileID(c.Param("id")),
	); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "default profile updated"})
}

// ─── Response DTOs ────────────────────────────────────────────────────────────

type profileResponse struct {
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	AvatarURL          string             `json:"avatar_url"`
	Type               domain.ProfileType `json:"type"`
	HasPIN             bool               `json:"has_pin"`
	MaxRating          *domain.MPAARating `json:"max_rating,omitempty"`
	PreferredLanguages []string           `json:"preferred_languages,omitempty"`
	IsDefault          bool               `json:"is_default"`
	SortOrder          int                `json:"sort_order"`
}

func toProfileResponse(p *domain.Profile) profileResponse {
	return profileResponse{
		ID:                 p.ID.String(),
		Name:               p.Name,
		AvatarURL:          p.AvatarURL,
		Type:               p.Type,
		HasPIN:             p.PINHash != "",
		MaxRating:          p.MaxRating,
		PreferredLanguages: p.PreferredLanguages,
		IsDefault:          p.IsDefault,
		SortOrder:          p.SortOrder,
	}
}

func toProfileResponses(profiles []*domain.Profile) []profileResponse {
	out := make([]profileResponse, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, toProfileResponse(p))
	}
	return out
}

func respondError(c *gin.Context, err error) {
	log := logger.FromContext(c.Request.Context())
	ae, ok := apierror.As(err)
	if !ok {
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
