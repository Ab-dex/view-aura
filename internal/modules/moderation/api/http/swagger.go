package httpapi

// ─── Moderation (Admin) ───────────────────────────────────────────────────────

// ListQueue godoc
//
//	@Summary     List moderation queue
//	@Description Returns a paginated list of moderation cases pending review.
//	             Supports filtering by status and content type.
//	@Tags        Moderation
//	@Security    BearerAuth
//	@Produce     json
//	@Param       status        query  string false "Filter by case status"
//	@Param       content_type  query  string false "Filter by content type"
//	@Param       limit         query  int    false "Page size" default(20)
//	@Param       offset        query  int    false "Pagination offset" default(0)
//	@Success     200 {object} moderationCaseListResponse
//	@Failure     403 {object} errorResponse
//	@Router      /moderation/queue [get]
func (h *ModerationHandler) swaggerListQueue() {}

// GetCase godoc
//
//	@Summary     Get moderation case
//	@Description Returns full detail of a moderation case.
//	@Tags        Moderation
//	@Security    BearerAuth
//	@Produce     json
//	@Param       id path string true "Case ID"
//	@Success     200 {object} caseResponse
//	@Failure     404 {object} errorResponse
//	@Router      /moderation/{id} [get]
func (h *ModerationHandler) swaggerGetCase() {}

// AcquireLock godoc
//
//	@Summary     Acquire moderation lock
//	@Description Locks a case for exclusive moderation by the current user.
//	@Tags        Moderation
//	@Security    BearerAuth
//	@Param       id path string true "Case ID"
//	@Success     204 "Lock acquired"
//	@Failure     409 {object} errorResponse "Already locked"
//	@Failure     403 {object} errorResponse
//	@Router      /moderation/{id}/lock [post]
func (h *ModerationHandler) swaggerAcquireLock() {}

// ReleaseLock godoc
//
//	@Summary     Release moderation lock
//	@Description Releases the lock on a moderation case.
//	@Tags        Moderation
//	@Security    BearerAuth
//	@Param       id path string true "Case ID"
//	@Success     204 "Lock released"
//	@Failure     403 {object} errorResponse
//	@Router      /moderation/{id}/lock [delete]
func (h *ModerationHandler) swaggerReleaseLock() {}

// Approve godoc
//
//	@Summary     Approve content
//	@Description Approves content associated with a moderation case.
//	@Tags        Moderation
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       id   path string         true "Case ID"
//	@Param       body body decideRequest false "Optional decision reason"
//	@Success     200 {object} caseResponse
//	@Failure     403 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /moderation/{id}/approve [post]
func (h *ModerationHandler) swaggerApprove() {}

// Reject godoc
//
//	@Summary     Reject content
//	@Description Rejects content associated with a moderation case.
//	@Tags        Moderation
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       id   path string         true "Case ID"
//	@Param       body body decideRequest false "Optional rejection reason"
//	@Success     200 {object} caseResponse
//	@Failure     403 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /moderation/{id}/reject [post]
func (h *ModerationHandler) swaggerReject() {}

// GetAppeal godoc
//
//	@Summary     Get appeal
//	@Description Returns the appeal associated with a moderation case, if any.
//	@Tags        Moderation
//	@Security    BearerAuth
//	@Produce     json
//	@Param       id path string true "Case ID"
//	@Success     200 {object} appealWrapperResponse
//	@Failure     404 {object} errorResponse
//	@Router      /moderation/{id}/appeal [get]
func (h *ModerationHandler) swaggerGetAppeal() {}

// DecideAppeal godoc
//
//	@Summary     Decide appeal
//	@Description Decides an appeal (upheld or overturned). Requires senior moderator.
//	@Tags        Moderation
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       id   path string               true "Case ID"
//	@Param       body body decideAppealRequest true "Appeal decision"
//	@Success     200 {object} appealResponse
//	@Failure     400 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /moderation/{id}/appeal/decide [post]
func (h *ModerationHandler) swaggerDecideAppeal() {}

// ─── User Moderation ──────────────────────────────────────────────────────────

// MyCases godoc
//
//	@Summary     List my moderation cases
//	@Description Returns moderation cases related to the authenticated user.
//	@Tags        Moderation
//	@Security    BearerAuth
//	@Produce     json
//	@Param       limit  query int false "Page size" default(20)
//	@Param       offset query int false "Pagination offset" default(0)
//	@Success     200 {object} moderationCaseListResponse
//	@Failure     403 {object} errorResponse
//	@Router      /me/moderation [get]
func (h *ModerationHandler) swaggerMyCases() {}

// SubmitAppeal godoc
//
//	@Summary     Submit appeal
//	@Description Submits an appeal for a rejected moderation case.
//	@Tags        Moderation
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       id   path string        true "Case ID"
//	@Param       body body appealRequest true "Appeal statement"
//	@Success     201 {object} appealResponse
//	@Failure     400 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /me/moderation/{id}/appeal [post]
func (h *ModerationHandler) swaggerSubmitAppeal() {}

// ─── Response schema stubs ────────────────────────────────────────────────────

type moderationCaseListResponse struct {
	Cases  []caseResponse `json:"cases"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type appealWrapperResponse struct {
	Appeal *appealResponse `json:"appeal"`
}
