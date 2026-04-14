package api

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/modules/payment/domain"
	"github.com/Ab-dex/view-aura/internal/modules/payment/service"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

type PaymentHandler struct {
	svc     service.PaymentService
	webhook service.WebhookService
}

func NewPaymentHandler(svc service.PaymentService, webhook service.WebhookService) *PaymentHandler {
	return &PaymentHandler{svc: svc, webhook: webhook}
}

// RegisterPublicRoutes registers the Stripe webhook endpoint.
// This route must NOT be behind the auth middleware — Stripe calls it directly.
// Authentication is provided by verifying the Stripe-Signature header.
func (h *PaymentHandler) RegisterPublicRoutes(r gin.IRouter) {
	r.POST("/webhook/stripe", h.StripeWebhook)
}

// RegisterProtectedRoutes registers authenticated payment endpoints.
//
//	POST   /me/subscription          — subscribe to a plan
//	GET    /me/subscription          — get current subscription
//	PATCH  /me/subscription          — upgrade plan
//	DELETE /me/subscription          — cancel subscription
//	GET    /me/invoices              — billing history
//	POST   /me/pay-per-view          — purchase a screener license
//	GET    /me/licenses              — list active screener licenses
//	GET    /movies/:movie_id/access  — check if user has access to a movie
func (h *PaymentHandler) RegisterProtectedRoutes(r gin.IRouter) {
	r.POST("/me/subscription", h.Subscribe)
	r.GET("/me/subscription", h.GetSubscription)
	r.PATCH("/me/subscription", h.UpgradePlan)
	r.DELETE("/me/subscription", h.CancelSubscription)
	r.GET("/me/invoices", h.ListInvoices)
	r.POST("/me/pay-per-view", h.PayPerView)
	r.GET("/me/licenses", h.GetLicenses)
	r.GET("/movies/:movie_id/access", h.CheckAccess)
}

// ─── DTOs ─────────────────────────────────────────────────────────────────────

type subscribeRequest struct {
	Plan          string `json:"plan"           binding:"required"`
	PaymentMethod string `json:"payment_method" binding:"required"`
}

type upgradePlanRequest struct {
	Plan string `json:"plan" binding:"required"`
}

type payPerViewRequest struct {
	MovieID       string `json:"movie_id"       binding:"required"`
	PaymentMethod string `json:"payment_method" binding:"required"`
	AmountCents   int64  `json:"amount_cents"   binding:"required,min=1"`
	Currency      string `json:"currency"       binding:"required"`
}

// ─── Response types ───────────────────────────────────────────────────────────

type subscriptionResponse struct {
	ID               string `json:"id"`
	Plan             string `json:"plan"`
	Status           string `json:"status"`
	CurrentPeriodEnd string `json:"current_period_end"`
	CancelAtEnd      bool   `json:"cancel_at_end"`
}

type invoiceResponse struct {
	ID          string  `json:"id"`
	AmountCents int64   `json:"amount_cents"`
	Currency    string  `json:"currency"`
	Status      string  `json:"status"`
	PaidAt      *string `json:"paid_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

type licenseResponse struct {
	ID          string `json:"id"`
	MovieID     string `json:"movie_id"`
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
	ExpiresAt   string `json:"expires_at"`
	CreatedAt   string `json:"created_at"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

func (h *PaymentHandler) StripeWebhook(c *gin.Context) {
	// Must read raw body before binding — Stripe signature verification
	// requires the exact original payload bytes.
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := h.webhook.Handle(c.Request.Context(), payload, c.GetHeader("Stripe-Signature")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusOK)
}

func (h *PaymentHandler) Subscribe(c *gin.Context) {
	var req subscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	// Email and name are injected by the auth middleware from JWT claims.
	email, _ := c.Get("user_email")
	name, _ := c.Get("user_name")

	sub, err := h.svc.Subscribe(c.Request.Context(), domain.SubscribeCmd{
		UserID:        mustUserID(c),
		Plan:          domain.Plan(req.Plan),
		PaymentMethod: req.PaymentMethod,
		Email:         stringOrEmpty(email),
		Name:          stringOrEmpty(name),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toSubResponse(sub))
}

func (h *PaymentHandler) GetSubscription(c *gin.Context) {
	sub, err := h.svc.GetSubscription(c.Request.Context(), mustUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toSubResponse(sub))
}

func (h *PaymentHandler) UpgradePlan(c *gin.Context) {
	var req upgradePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}
	sub, err := h.svc.UpgradePlan(c.Request.Context(), domain.UpgradePlanCmd{
		UserID:  mustUserID(c),
		NewPlan: domain.Plan(req.Plan),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toSubResponse(sub))
}

func (h *PaymentHandler) CancelSubscription(c *gin.Context) {
	if err := h.svc.CancelSubscription(c.Request.Context(), mustUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PaymentHandler) ListInvoices(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	invoices, total, err := h.svc.GetInvoices(c.Request.Context(), mustUserID(c), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]invoiceResponse, 0, len(invoices))
	for _, inv := range invoices {
		out = append(out, toInvoiceResponse(inv))
	}
	c.JSON(http.StatusOK, gin.H{
		"invoices": out,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *PaymentHandler) PayPerView(c *gin.Context) {
	var req payPerViewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}
	lic, err := h.svc.PayPerView(c.Request.Context(), domain.PayPerViewCmd{
		UserID:        mustUserID(c),
		MovieID:       req.MovieID,
		PaymentMethod: req.PaymentMethod,
		AmountCents:   req.AmountCents,
		Currency:      req.Currency,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toLicenseResponse(lic))
}

func (h *PaymentHandler) GetLicenses(c *gin.Context) {
	licenses, err := h.svc.GetLicenses(c.Request.Context(), mustUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]licenseResponse, 0, len(licenses))
	for _, l := range licenses {
		out = append(out, toLicenseResponse(l))
	}
	c.JSON(http.StatusOK, gin.H{"licenses": out})
}

func (h *PaymentHandler) CheckAccess(c *gin.Context) {
	has, err := h.svc.HasAccess(c.Request.Context(), mustUserID(c), c.Param("movie_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"has_access": has})
}

// ─── Mappers ──────────────────────────────────────────────────────────────────

func toSubResponse(s *domain.Subscription) subscriptionResponse {
	return subscriptionResponse{
		ID:               s.ID,
		Plan:             string(s.Plan),
		Status:           string(s.Status),
		CurrentPeriodEnd: s.CurrentPeriodEnd.Format(time.RFC3339),
		CancelAtEnd:      s.CancelAtEnd,
	}
}

func toInvoiceResponse(inv *domain.Invoice) invoiceResponse {
	r := invoiceResponse{
		ID:          inv.ID,
		AmountCents: inv.AmountCents,
		Currency:    inv.Currency,
		Status:      string(inv.Status),
		CreatedAt:   inv.CreatedAt.Format(time.RFC3339),
	}
	if inv.PaidAt != nil {
		s := inv.PaidAt.Format(time.RFC3339)
		r.PaidAt = &s
	}
	return r
}

func toLicenseResponse(l *domain.ScreenerLicense) licenseResponse {
	return licenseResponse{
		ID:          l.ID,
		MovieID:     l.MovieID,
		AmountCents: l.AmountCents,
		Currency:    l.Currency,
		ExpiresAt:   l.ExpiresAt.Format(time.RFC3339),
		CreatedAt:   l.CreatedAt.Format(time.RFC3339),
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func mustUserID(c *gin.Context) string {
	v, _ := c.Get("user_id")
	return v.(string)
}

func stringOrEmpty(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func respondError(c *gin.Context, err error) {
	log := logger.FromContext(c.Request.Context())
	ae, ok := apierror.As(err)
	if !ok {
		ae = apierror.Internal("unexpected error", err)
	}
	if ae.HTTPStatus >= 500 {
		log.Error().Err(err).Str("code", string(ae.Code)).Msg("payment: internal error")
	}
	c.JSON(ae.HTTPStatus, gin.H{"error": gin.H{
		"code":    ae.Code,
		"message": ae.Message,
		"details": ae.Details,
	}})
}
