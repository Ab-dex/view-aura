package httpapi

// Upsert godoc
//
//	@Summary     Rate a movie
//	@Description Submits or updates the authenticated user's rating for a movie.
//	             `overall` (1.0–5.0) is required; all dimension scores are optional.
//	             `reaction` is independent of the score and can be set on its own.
//	@Tags        Ratings
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       movie_id path string true "Movie UUID"
//	@Param       body body upsertRatingRequest true "Rating payload"
//	@Success     200 {object} ratingResponse
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Router      /movies/{movie_id}/rating [put]
func (h *RatingHandler) swaggerUpsert() {}

// Delete godoc
//
//	@Summary     Delete rating
//	@Description Removes the authenticated user's rating for a movie.
//	@Tags        Ratings
//	@Security    BearerAuth
//	@Param       movie_id path string true "Movie UUID"
//	@Success     204 "Rating deleted"
//	@Failure     401 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /movies/{movie_id}/rating [delete]
func (h *RatingHandler) swaggerDelete() {}

// GetMine godoc
//
//	@Summary     Get my rating
//	@Description Returns the authenticated user's rating for a specific movie,
//	             or `{"rating": null}` if they have not rated it yet.
//	@Tags        Ratings
//	@Security    BearerAuth
//	@Produce     json
//	@Param       movie_id path string true "Movie UUID"
//	@Success     200 {object} map[string]interface{} "rating object or null"
//	@Failure     401 {object} errorResponse
//	@Router      /movies/{movie_id}/rating [get]
func (h *RatingHandler) swaggerGetMine() {}

// ListByMovie godoc
//
//	@Summary     List ratings for a movie
//	@Description Returns paginated ratings submitted by all users for a movie.
//	@Tags        Ratings
//	@Produce     json
//	@Param       movie_id path  string true  "Movie UUID"
//	@Param       limit    query int    false "Page size" default(20)
//	@Param       offset   query int    false "Pagination offset" default(0)
//	@Success     200 {object} ratingListResponse
//	@Router      /movies/{movie_id}/ratings [get]
func (h *RatingHandler) swaggerListByMovie() {}

// GetStats godoc
//
//	@Summary     Get rating stats
//	@Description Returns aggregated dimensional averages and star-distribution
//	             histogram for a movie.
//	@Tags        Ratings
//	@Produce     json
//	@Param       movie_id path string true "Movie UUID"
//	@Success     200 {object} statsResponse
//	@Router      /movies/{movie_id}/ratings/stats [get]
func (h *RatingHandler) swaggerGetStats() {}

// ListByUser godoc
//
//	@Summary     List a user's ratings
//	@Description Returns all movies a given user has rated, paginated.
//	@Tags        Ratings
//	@Produce     json
//	@Param       user_id path  string true  "User UUID"
//	@Param       limit   query int    false "Page size" default(20)
//	@Param       offset  query int    false "Pagination offset" default(0)
//	@Success     200 {object} ratingListResponse
//	@Router      /users/{user_id}/ratings [get]
func (h *RatingHandler) swaggerListByUser() {}

// ─── Schema stubs ─────────────────────────────────────────────────────────────

type ratingListResponse struct {
	Ratings []ratingResponse `json:"ratings"`
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
