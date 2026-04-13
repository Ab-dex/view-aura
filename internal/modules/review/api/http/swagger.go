package httpapi

// Create godoc
//
//	@Summary     Create review
//	@Description Writes a review for a movie. A user can have at most one review
//	             per movie. Short reviews are capped at 280 characters.
//	             Video reviews require a `video_url`.
//	@Tags        Reviews
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       movie_id path string true "Movie UUID"
//	@Param       body body createReviewRequest true "Review payload"
//	@Success     201 {object} reviewResponse
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Failure     409 {object} errorResponse "User already reviewed this movie"
//	@Router      /movies/{movie_id}/reviews [post]
func (h *ReviewHandler) swaggerCreate() {}

// List godoc
//
//	@Summary     List reviews for a movie
//	@Description Returns paginated published reviews for a movie.
//	             Spoiler reviews are excluded by default.
//	@Tags        Reviews
//	@Produce     json
//	@Param       movie_id         path  string false "Movie UUID (required for this route)"
//	@Param       type             query string false "Filter by type: short, long_form, spoiler, video"
//	@Param       include_spoilers query bool   false "Include spoiler-tagged reviews" default(false)
//	@Param       sort_by          query string false "Sort field: created_at, helpful_count, credibility_score" default(created_at)
//	@Param       limit            query int    false "Page size" default(20)
//	@Param       offset           query int    false "Pagination offset" default(0)
//	@Success     200 {object} reviewListResponse
//	@Router      /movies/{movie_id}/reviews [get]
func (h *ReviewHandler) swaggerList() {}

// GetByID godoc
//
//	@Summary     Get review
//	@Description Returns a single review by ID.
//	@Tags        Reviews
//	@Produce     json
//	@Param       id path string true "Review UUID"
//	@Success     200 {object} reviewResponse
//	@Failure     404 {object} errorResponse
//	@Router      /reviews/{id} [get]
func (h *ReviewHandler) swaggerGetByID() {}

// Update godoc
//
//	@Summary     Update review
//	@Description Updates a review's body, spoiler flag, or publish state.
//	             Only the owner can edit. Once published, body edits are locked.
//	@Tags        Reviews
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       id   path string true "Review UUID"
//	@Param       body body updateReviewRequest true "Fields to update"
//	@Success     200 {object} reviewResponse
//	@Failure     401 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /reviews/{id} [patch]
func (h *ReviewHandler) swaggerUpdate() {}

// Delete godoc
//
//	@Summary     Delete review
//	@Description Permanently deletes the review. Only the owner can delete.
//	@Tags        Reviews
//	@Security    BearerAuth
//	@Param       id path string true "Review UUID"
//	@Success     204 "Deleted"
//	@Failure     401 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Router      /reviews/{id} [delete]
func (h *ReviewHandler) swaggerDelete() {}

// React godoc
//
//	@Summary     React to a review
//	@Description Adds or changes the authenticated user's reaction on a review.
//	             Valid types: like, helpful, insightful, funny.
//	             Reacting again with the same type is idempotent.
//	             Changing reaction type atomically swaps the counters.
//	@Tags        Reviews
//	@Security    BearerAuth
//	@Accept      json
//	@Param       id   path string true "Review UUID"
//	@Param       body body reactRequest true "Reaction type"
//	@Success     204 "Reaction recorded"
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Router      /reviews/{id}/reactions [post]
func (h *ReviewHandler) swaggerReact() {}

// UnReact godoc
//
//	@Summary     Remove reaction
//	@Description Removes the authenticated user's reaction from a review.
//	             Idempotent if no reaction exists.
//	@Tags        Reviews
//	@Security    BearerAuth
//	@Param       id path string true "Review UUID"
//	@Success     204 "Reaction removed"
//	@Failure     401 {object} errorResponse
//	@Router      /reviews/{id}/reactions [delete]
func (h *ReviewHandler) swaggerUnReact() {}

// Report godoc
//
//	@Summary     Report a review
//	@Description Submits an abuse report. A user can only report a review once.
//	             Valid reasons: spam, hate_speech, unmarked_spoiler, other.
//	@Tags        Reviews
//	@Security    BearerAuth
//	@Accept      json
//	@Param       id   path string true "Review UUID"
//	@Param       body body reportRequest true "Report details"
//	@Success     204 "Report submitted"
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Failure     409 {object} errorResponse "Already reported"
//	@Router      /reviews/{id}/reports [post]
func (h *ReviewHandler) swaggerReport() {}

// ─── Schema stubs ─────────────────────────────────────────────────────────────

type reviewListResponse struct {
	Reviews []reviewResponse `json:"reviews"`
	Total   int              `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}

type errorResponse struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}
