package httpapi

// List godoc
//
//	@Summary     List / search movies
//	@Description Returns a paginated, filterable list of published movies.
//	             Supports full-text search, genre, year, runtime, content rating,
//	             and minimum rating filters.
//	@Tags        Movies
//	@Produce     json
//	@Param       q          query  string  false "Full-text search query"
//	@Param       genre      query  []string false "Filter by genre name (repeatable)" collectionFormat(multi)
//	@Param       year_from  query  int     false "Minimum release year"
//	@Param       year_to    query  int     false "Maximum release year"
//	@Param       runtime_max query int     false "Maximum runtime in minutes"
//	@Param       min_rating query  number  false "Minimum average rating (1.0–5.0)"
//	@Param       sort_by    query  string  false "Sort field: popularity, release_date, avg_rating, title" default(popularity)
//	@Param       sort_dir   query  string  false "Sort direction: asc, desc" default(desc)
//	@Param       limit      query  int     false "Page size (max 100)" default(20)
//	@Param       offset     query  int     false "Pagination offset" default(0)
//	@Success     200 {object} movieListResponse
//	@Failure     400 {object} errorResponse
//	@Router      /movies [get]
func (h *MovieHandler) swaggerList() {}

// Get godoc
//
//	@Summary     Get movie detail
//	@Description Returns a movie's full detail including credits, streaming links,
//	             and filming locations. Accepts either a UUID or a URL slug.
//	@Tags        Movies
//	@Produce     json
//	@Param       id_or_slug path string true "Movie UUID or slug (e.g. the-godfather-1972)"
//	@Success     200 {object} movieDetailResponse
//	@Failure     404 {object} errorResponse
//	@Router      /movies/{id_or_slug} [get]
func (h *MovieHandler) swaggerGet() {}

// Create godoc
//
//	@Summary     Create movie
//	@Description Creates a new movie record in `draft` status. Requires admin or producer role.
//	@Tags        Movies
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       body body createMovieRequest true "Movie fields"
//	@Success     201 {object} movieResponse
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Failure     409 {object} errorResponse "Duplicate IMDb ID"
//	@Router      /movies [post]
func (h *MovieHandler) swaggerCreate() {}

// Update godoc
//
//	@Summary     Update movie
//	@Description Applies partial updates to a movie. All fields are optional.
//	             Requires admin or producer role.
//	@Tags        Movies
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       id   path string true "Movie UUID"
//	@Param       body body updateMovieRequest true "Fields to update"
//	@Success     200 {object} movieResponse
//	@Failure     400 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /movies/{id} [patch]
func (h *MovieHandler) swaggerUpdate() {}

// Delete godoc
//
//	@Summary     Archive movie
//	@Description Sets the movie status to `archived` (soft delete).
//	             Requires admin or producer role.
//	@Tags        Movies
//	@Security    BearerAuth
//	@Param       id path string true "Movie UUID"
//	@Success     204 "Archived"
//	@Failure     403 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /movies/{id} [delete]
func (h *MovieHandler) swaggerDelete() {}

// ─── Genres ───────────────────────────────────────────────────────────────────

// ListGenres godoc
//
//	@Summary     List genres
//	@Description Returns the full genre taxonomy ordered alphabetically.
//	@Tags        Genres
//	@Produce     json
//	@Success     200 {object} genreListResponse
//	@Router      /movies/genres [get]
func (h *MovieHandler) swaggerListGenres() {}

// ─── Credits ──────────────────────────────────────────────────────────────────

// ListCredits godoc
//
//	@Summary     List credits
//	@Description Returns all cast and crew credits for a movie, each hydrated
//	             with the person's profile data.
//	@Tags        Movies
//	@Produce     json
//	@Param       id_or_slug path string true "Movie UUID or slug"
//	@Success     200 {object} creditListResponse
//	@Failure     404 {object} errorResponse
//	@Router      /movies/{id_or_slug}/credits [get]
func (h *MovieHandler) swaggerListCredits() {}

// AddCredit godoc
//
//	@Summary     Add credit
//	@Description Links a person to a movie with a role. Requires admin or producer role.
//	@Tags        Movies
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       id   path string true "Movie UUID"
//	@Param       body body addCreditRequest true "Credit details"
//	@Success     201 {object} creditResponse
//	@Failure     400 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Failure     404 {object} errorResponse "Movie or person not found"
//	@Router      /movies/{id}/credits [post]
func (h *MovieHandler) swaggerAddCredit() {}

// RemoveCredit godoc
//
//	@Summary     Remove credit
//	@Description Removes a credit link. Requires admin or producer role.
//	@Tags        Movies
//	@Security    BearerAuth
//	@Param       id        path string true "Movie UUID"
//	@Param       credit_id path string true "Credit UUID"
//	@Success     204 "Removed"
//	@Failure     403 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /movies/{id}/credits/{credit_id} [delete]
func (h *MovieHandler) swaggerRemoveCredit() {}

// ─── Streaming links ──────────────────────────────────────────────────────────

// ListStreamingLinks godoc
//
//	@Summary     List streaming links
//	@Description Returns where-to-watch data for a movie, filtered by region.
//	@Tags        Movies
//	@Produce     json
//	@Param       id_or_slug path   string true  "Movie UUID or slug"
//	@Param       region     query  string false "ISO-3166 alpha-2 region code (e.g. US, NG)"
//	@Success     200 {object} streamingLinkListResponse
//	@Failure     404 {object} errorResponse
//	@Router      /movies/{id_or_slug}/streaming [get]
func (h *MovieHandler) swaggerListStreamingLinks() {}

// UpsertStreamingLink godoc
//
//	@Summary     Upsert streaming link
//	@Description Inserts or replaces the streaming link for (movie, provider, region).
//	             Requires admin or producer role.
//	@Tags        Movies
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       id   path string true "Movie UUID"
//	@Param       body body upsertStreamingLinkRequest true "Link details"
//	@Success     200 {object} streamingLinkResponse
//	@Failure     400 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Router      /movies/{id}/streaming [put]
func (h *MovieHandler) swaggerUpsertStreamingLink() {}

// ─── People ───────────────────────────────────────────────────────────────────

// GetPerson godoc
//
//	@Summary     Get person
//	@Description Returns a cast or crew member's profile. Accepts UUID or slug.
//	@Tags        People
//	@Produce     json
//	@Param       id_or_slug path string true "Person UUID or slug"
//	@Success     200 {object} personResponse
//	@Failure     404 {object} errorResponse
//	@Router      /movies/people/{id_or_slug} [get]
func (h *MovieHandler) swaggerGetPerson() {}

// GetFilmography godoc
//
//	@Summary     Get filmography
//	@Description Returns all movie credits for a person.
//	@Tags        People
//	@Produce     json
//	@Param       id_or_slug path string true "Person UUID or slug"
//	@Success     200 {object} filmographyResponse
//	@Failure     404 {object} errorResponse
//	@Router      /movies/people/{id_or_slug}/filmography [get]
func (h *MovieHandler) swaggerGetFilmography() {}

// SearchPeople godoc
//
//	@Summary     Search people
//	@Description Full-text search over cast and crew names.
//	@Tags        People
//	@Produce     json
//	@Param       q query string true "Name search query"
//	@Success     200 {object} peopleListResponse
//	@Failure     400 {object} errorResponse
//	@Router      /movies/people/search [get]
func (h *MovieHandler) swaggerSearchPeople() {}

// ─── Response schema stubs ────────────────────────────────────────────────────

type movieListResponse struct {
	Movies []movieResponse `json:"movies"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

type genreListResponse struct {
	Genres []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"genres"`
}

type creditListResponse struct {
	Credits []creditResponse `json:"credits"`
}

type streamingLinkListResponse struct {
	StreamingLinks []streamingLinkResponse `json:"streaming_links"`
}

type filmographyResponse struct {
	Person      personResponse `json:"person"`
	Filmography []struct {
		CreditID  string `json:"credit_id"`
		MovieID   string `json:"movie_id"`
		Role      string `json:"role"`
		Character string `json:"character,omitempty"`
	} `json:"filmography"`
}

type peopleListResponse struct {
	People []personResponse `json:"people"`
}

type errorResponse struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}
