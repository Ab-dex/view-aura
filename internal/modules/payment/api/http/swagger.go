package httpapi

// ─── Webhook ──────────────────────────────────────────────────────────────────

// StripeWebhook godoc
//
//	@Summary     Stripe webhook
//	@Description Receives Stripe webhook events. This endpoint must not be behind authentication.
//	             Signature verification is performed using the Stripe-Signature header.
//	@Tags        Payments
//	@Accept      json
//	@Produce     json
//	@Param       Stripe-Signature header string true "Stripe signature header"
//	@Success     200 "OK"
//	@Failure     400 "Invalid payload"
//	@Failure     400 {object} errorResponse
//	@Router      /webhook/stripe [post]
func (h *PaymentHandler) swaggerStripeWebhook() {}

// ─── Subscription ─────────────────────────────────────────────────────────────

// Subscribe godoc
//
//	@Summary     Subscribe to a plan
//	@Description Creates a new subscription for the authenticated user.
//	@Tags        Payments
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       body body subscribeRequest true "Subscription details"
//	@Success     201 {object} subscriptionResponse
//	@Failure     400 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Router      /me/subscription [post]
func (h *PaymentHandler) swaggerSubscribe() {}

// GetSubscription godoc
//
//	@Summary     Get current subscription
//	@Description Returns the authenticated user's active subscription.
//	@Tags        Payments
//	@Security    BearerAuth
//	@Produce     json
//	@Success     200 {object} subscriptionResponse
//	@Failure     404 {object} errorResponse
//	@Router      /me/subscription [get]
func (h *PaymentHandler) swaggerGetSubscription() {}

// UpgradePlan godoc
//
//	@Summary     Upgrade subscription plan
//	@Description Changes the user's subscription to a new plan.
//	@Tags        Payments
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       body body upgradePlanRequest true "New plan"
//	@Success     200 {object} subscriptionResponse
//	@Failure     400 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Router      /me/subscription [patch]
func (h *PaymentHandler) swaggerUpgradePlan() {}

// CancelSubscription godoc
//
//	@Summary     Cancel subscription
//	@Description Cancels the current subscription (at period end).
//	@Tags        Payments
//	@Security    BearerAuth
//	@Success     204 "Cancelled"
//	@Failure     403 {object} errorResponse
//	@Router      /me/subscription [delete]
func (h *PaymentHandler) swaggerCancelSubscription() {}

// ─── Billing ──────────────────────────────────────────────────────────────────

// ListInvoices godoc
//
//	@Summary     List invoices
//	@Description Returns paginated billing history for the authenticated user.
//	@Tags        Payments
//	@Security    BearerAuth
//	@Produce     json
//	@Param       limit  query int false "Page size" default(20)
//	@Param       offset query int false "Pagination offset" default(0)
//	@Success     200 {object} invoiceListResponse
//	@Failure     403 {object} errorResponse
//	@Router      /me/invoices [get]
func (h *PaymentHandler) swaggerListInvoices() {}

// ─── Pay-per-view ─────────────────────────────────────────────────────────────

// PayPerView godoc
//
//	@Summary     Purchase pay-per-view access
//	@Description Purchases a screener license for a specific movie.
//	@Tags        Payments
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       body body payPerViewRequest true "Payment details"
//	@Success     201 {object} licenseResponse
//	@Failure     400 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Router      /me/pay-per-view [post]
func (h *PaymentHandler) swaggerPayPerView() {}

// GetLicenses godoc
//
//	@Summary     List licenses
//	@Description Returns all active screener licenses for the authenticated user.
//	@Tags        Payments
//	@Security    BearerAuth
//	@Produce     json
//	@Success     200 {object} licenseListResponse
//	@Failure     403 {object} errorResponse
//	@Router      /me/licenses [get]
func (h *PaymentHandler) swaggerGetLicenses() {}

// ─── Access ───────────────────────────────────────────────────────────────────

// CheckAccess godoc
//
//	@Summary     Check movie access
//	@Description Returns whether the user has access to a specific movie.
//	@Tags        Payments
//	@Security    BearerAuth
//	@Produce     json
//	@Param       movie_id path string true "Movie ID"
//	@Success     200 {object} accessResponse
//	@Failure     403 {object} errorResponse
//	@Router      /movies/{movie_id}/access [get]
func (h *PaymentHandler) swaggerCheckAccess() {}

// ─── Response schema stubs ────────────────────────────────────────────────────

type invoiceListResponse struct {
	Invoices []invoiceResponse `json:"invoices"`
	Total    int               `json:"total"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
}

type licenseListResponse struct {
	Licenses []licenseResponse `json:"licenses"`
}

type accessResponse struct {
	HasAccess bool `json:"has_access"`
}

type errorResponse struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}
