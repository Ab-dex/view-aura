package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/modules/movie/domain"
	"github.com/Ab-dex/view-aura/internal/modules/movie/service"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

type MovieHandler struct {
	svc service.MovieService
}

func NewMovieHandler(svc service.MovieService) *MovieHandler {
	return &MovieHandler{svc: svc}
}

func (h *MovieHandler) RegisterRoutes(r gin.IRouter) {
	r.GET("", h.List)
	r.GET("/genres", h.ListGenres)
	r.GET("/:id", h.Get)
	r.GET("/:id/credits", h.ListCredits)
	r.GET("/:id/streaming", h.ListStreamingLinks)
	r.GET("/:id/locations", h.ListFilmingLocations)
	r.GET("/people/:id", h.GetPerson)
	r.GET("/people/:id/filmography", h.GetFilmography)
	r.GET("/people/search", h.SearchPeople)
}

func (h *MovieHandler) RegisterAdminRoutes(r gin.IRouter) {
	r.POST("", h.Create)
	r.PATCH("/:id", h.Update)
	r.DELETE("/:id", h.Delete)
	r.POST("/:id/credits", h.AddCredit)
	r.DELETE("/:id/credits/:credit_id", h.RemoveCredit)
	r.PUT("/:id/streaming", h.UpsertStreamingLink)
	r.DELETE("/:id/streaming/:link_id", h.DeleteStreamingLink)
	r.POST("/:id/locations", h.AddFilmingLocation)
	r.DELETE("/:id/locations/:loc_id", h.DeleteFilmingLocation)
}

// ─── Request / Response types ─────────────────────────────────────────────────

type createMovieRequest struct {
	Title         string   `json:"title"          binding:"required"`
	OriginalTitle string   `json:"original_title"`
	Synopsis      string   `json:"synopsis"`
	Tagline       string   `json:"tagline"`
	ReleaseDate   *string  `json:"release_date"` // YYYY-MM-DD
	RuntimeMins   int      `json:"runtime_mins"`
	ContentRating string   `json:"content_rating"`
	OriginalLang  string   `json:"original_lang"`
	Countries     []string `json:"countries"`
	Genres        []string `json:"genres"`
	PosterURL     string   `json:"poster_url"`
	BackdropURL   string   `json:"backdrop_url"`
	TrailerURL    string   `json:"trailer_url"`
	IMDbID        string   `json:"imdb_id"`
	TMDbID        int      `json:"tmdb_id"`
}

type updateMovieRequest struct {
	Title         *string  `json:"title"`
	OriginalTitle *string  `json:"original_title"`
	Synopsis      *string  `json:"synopsis"`
	Tagline       *string  `json:"tagline"`
	ReleaseDate   *string  `json:"release_date"`
	RuntimeMins   *int     `json:"runtime_mins"`
	ContentRating *string  `json:"content_rating"`
	OriginalLang  *string  `json:"original_lang"`
	Countries     []string `json:"countries"`
	Genres        []string `json:"genres"`
	PosterURL     *string  `json:"poster_url"`
	BackdropURL   *string  `json:"backdrop_url"`
	TrailerURL    *string  `json:"trailer_url"`
	Status        *string  `json:"status"`
}

type addCreditRequest struct {
	PersonID     string `json:"person_id"     binding:"required"`
	Role         string `json:"role"          binding:"required"`
	Character    string `json:"character"`
	BillingOrder int    `json:"billing_order"`
}

type upsertStreamingLinkRequest struct {
	Provider   string `json:"provider"    binding:"required"`
	LinkURL    string `json:"link_url"    binding:"required"`
	AccessType string `json:"access_type" binding:"required"`
	PriceCents *int   `json:"price_cents"`
	Region     string `json:"region"`
}

type addFilmingLocationRequest struct {
	Name        string   `json:"name"        binding:"required"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	Description string   `json:"description"`
}

// ─── Response mappers ─────────────────────────────────────────────────────────

type movieResponse struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	OriginalTitle   string   `json:"original_title,omitempty"`
	Slug            string   `json:"slug"`
	Synopsis        string   `json:"synopsis"`
	Tagline         string   `json:"tagline,omitempty"`
	ReleaseDate     *string  `json:"release_date,omitempty"`
	RuntimeMins     int      `json:"runtime_mins"`
	ContentRating   string   `json:"content_rating"`
	Status          string   `json:"status"`
	OriginalLang    string   `json:"original_lang"`
	Countries       []string `json:"countries"`
	Genres          []string `json:"genres"`
	PosterURL       string   `json:"poster_url,omitempty"`
	BackdropURL     string   `json:"backdrop_url,omitempty"`
	TrailerURL      string   `json:"trailer_url,omitempty"`
	IMDbID          string   `json:"imdb_id,omitempty"`
	TMDbID          int      `json:"tmdb_id,omitempty"`
	AvgRating       float64  `json:"avg_rating"`
	RatingCount     int      `json:"rating_count"`
	PopularityScore float64  `json:"popularity_score"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

type movieDetailResponse struct {
	movieResponse
	Credits          []creditResponse          `json:"credits"`
	StreamingLinks   []streamingLinkResponse   `json:"streaming_links"`
	FilmingLocations []filmingLocationResponse `json:"filming_locations"`
}

type creditResponse struct {
	ID           string       `json:"id"`
	Role         string       `json:"role"`
	Character    string       `json:"character,omitempty"`
	BillingOrder int          `json:"billing_order"`
	Person       *personBrief `json:"person,omitempty"`
}

type personBrief struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	ProfileURL string `json:"profile_url,omitempty"`
}

type personResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Slug       string  `json:"slug"`
	BirthDate  *string `json:"birth_date,omitempty"`
	DeathDate  *string `json:"death_date,omitempty"`
	Birthplace string  `json:"birthplace,omitempty"`
	Bio        string  `json:"bio,omitempty"`
	ProfileURL string  `json:"profile_url,omitempty"`
	IMDbID     string  `json:"imdb_id,omitempty"`
}

type streamingLinkResponse struct {
	ID         string `json:"id"`
	Provider   string `json:"provider"`
	LinkURL    string `json:"link_url"`
	AccessType string `json:"access_type"`
	PriceCents *int   `json:"price_cents,omitempty"`
	Region     string `json:"region,omitempty"`
	UpdatedAt  string `json:"updated_at"`
}

type filmingLocationResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
	Description string   `json:"description,omitempty"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

func (h *MovieHandler) List(c *gin.Context) {
	filter, err := domain.ParseMovieFilter(c)
	if err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	result, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		respondError(c, err)
		return
	}

	movies := make([]movieResponse, 0, len(result.Movies))
	for _, m := range result.Movies {
		movies = append(movies, toMovieResponse(m))
	}

	c.JSON(http.StatusOK, gin.H{
		"movies": movies,
		"total":  result.Total,
		"limit":  result.Limit,
		"offset": result.Offset,
	})
}

func (h *MovieHandler) Get(c *gin.Context) {
	idOrSlug := c.Param("id_or_slug")

	var (
		detail *service.MovieDetail
		err    error
	)

	// Try by ID first (UUID format), fall back to slug.
	if len(idOrSlug) == 36 {
		detail, err = h.svc.GetByID(c.Request.Context(), domain.MovieID(idOrSlug))
	} else {
		detail, err = h.svc.GetBySlug(c.Request.Context(), idOrSlug)
	}

	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, toMovieDetailResponse(detail))
}

func (h *MovieHandler) Create(c *gin.Context) {
	var req createMovieRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	cmd := domain.CreateMovieCmd{
		Title:         req.Title,
		OriginalTitle: req.OriginalTitle,
		Synopsis:      req.Synopsis,
		Tagline:       req.Tagline,
		RuntimeMins:   req.RuntimeMins,
		ContentRating: domain.ContentRating(req.ContentRating),
		OriginalLang:  req.OriginalLang,
		Countries:     req.Countries,
		Genres:        req.Genres,
		PosterURL:     req.PosterURL,
		BackdropURL:   req.BackdropURL,
		TrailerURL:    req.TrailerURL,
		IMDbID:        req.IMDbID,
		TMDbID:        req.TMDbID,
	}
	if req.ReleaseDate != nil {
		t, err := time.Parse("2006-01-02", *req.ReleaseDate)
		if err != nil {
			respondError(c, apierror.Validation("release_date must be YYYY-MM-DD", nil))
			return
		}
		cmd.ReleaseDate = &t
	}

	movie, err := h.svc.Create(c.Request.Context(), cmd)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toMovieResponse(movie))
}

func (h *MovieHandler) Update(c *gin.Context) {
	var req updateMovieRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	cmd := domain.UpdateMovieCmd{
		ID:            domain.MovieID(c.Param("id")),
		Title:         req.Title,
		OriginalTitle: req.OriginalTitle,
		Synopsis:      req.Synopsis,
		Tagline:       req.Tagline,
		RuntimeMins:   req.RuntimeMins,
		OriginalLang:  req.OriginalLang,
		Countries:     req.Countries,
		Genres:        req.Genres,
		PosterURL:     req.PosterURL,
		BackdropURL:   req.BackdropURL,
		TrailerURL:    req.TrailerURL,
	}
	if req.ReleaseDate != nil {
		t, err := time.Parse("2006-01-02", *req.ReleaseDate)
		if err != nil {
			respondError(c, apierror.Validation("release_date must be YYYY-MM-DD", nil))
			return
		}
		cmd.ReleaseDate = &t
	}
	if req.ContentRating != nil {
		cr := domain.ContentRating(*req.ContentRating)
		cmd.ContentRating = &cr
	}
	if req.Status != nil {
		s := domain.MovieStatus(*req.Status)
		cmd.Status = &s
	}

	movie, err := h.svc.Update(c.Request.Context(), cmd)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toMovieResponse(movie))
}

func (h *MovieHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), domain.MovieID(c.Param("id"))); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MovieHandler) ListGenres(c *gin.Context) {
	genres, err := h.svc.ListGenres(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	type genreResp struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	out := make([]genreResp, 0, len(genres))
	for _, g := range genres {
		out = append(out, genreResp{ID: string(g.ID), Name: g.Name, Slug: g.Slug})
	}
	c.JSON(http.StatusOK, gin.H{"genres": out})
}

func (h *MovieHandler) ListCredits(c *gin.Context) {
	idOrSlug := c.Param("id_or_slug")

	var movieID domain.MovieID
	if len(idOrSlug) == 36 {
		movieID = domain.MovieID(idOrSlug)
	} else {
		detail, err := h.svc.GetBySlug(c.Request.Context(), idOrSlug)
		if err != nil {
			respondError(c, err)
			return
		}
		movieID = detail.Movie.ID
	}

	credits, err := h.svc.ListCredits(c.Request.Context(), movieID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"credits": toCreditResponses(credits)})
}

func (h *MovieHandler) AddCredit(c *gin.Context) {
	var req addCreditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	credit, err := h.svc.AddCredit(c.Request.Context(), domain.AddCreditCmd{
		MovieID:      domain.MovieID(c.Param("id")),
		PersonID:     domain.PersonID(req.PersonID),
		Role:         domain.CreditRole(req.Role),
		Character:    req.Character,
		BillingOrder: req.BillingOrder,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, creditResponse{
		ID:           credit.ID,
		Role:         string(credit.Role),
		Character:    credit.Character,
		BillingOrder: credit.BillingOrder,
	})
}

func (h *MovieHandler) RemoveCredit(c *gin.Context) {
	if err := h.svc.RemoveCredit(c.Request.Context(), c.Param("credit_id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MovieHandler) ListStreamingLinks(c *gin.Context) {
	idOrSlug := c.Param("id_or_slug")
	region := c.DefaultQuery("region", "")

	detail, err := h.getMovieByIDOrSlug(c, idOrSlug)
	if err != nil {
		respondError(c, err)
		return
	}

	links, err := h.svc.ListStreamingLinks(c.Request.Context(), detail.Movie.ID, region)
	if err != nil {
		respondError(c, err)
		return
	}

	out := make([]streamingLinkResponse, 0, len(links))
	for _, l := range links {
		out = append(out, toStreamingLinkResponse(l))
	}
	c.JSON(http.StatusOK, gin.H{"streaming_links": out})
}

func (h *MovieHandler) UpsertStreamingLink(c *gin.Context) {
	var req upsertStreamingLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	link, err := h.svc.UpsertStreamingLink(c.Request.Context(), domain.UpsertStreamingLinkCmd{
		MovieID:    domain.MovieID(c.Param("id")),
		Provider:   req.Provider,
		LinkURL:    req.LinkURL,
		AccessType: req.AccessType,
		PriceCents: req.PriceCents,
		Region:     req.Region,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toStreamingLinkResponse(link))
}

func (h *MovieHandler) DeleteStreamingLink(c *gin.Context) {
	if err := h.svc.DeleteStreamingLink(c.Request.Context(), c.Param("link_id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MovieHandler) ListFilmingLocations(c *gin.Context) {
	detail, err := h.getMovieByIDOrSlug(c, c.Param("id_or_slug"))
	if err != nil {
		respondError(c, err)
		return
	}

	locs, err := h.svc.ListFilmingLocations(c.Request.Context(), detail.Movie.ID)
	if err != nil {
		respondError(c, err)
		return
	}

	out := make([]filmingLocationResponse, 0, len(locs))
	for _, l := range locs {
		out = append(out, filmingLocationResponse{
			ID:          l.ID,
			Name:        l.Name,
			Latitude:    l.Latitude,
			Longitude:   l.Longitude,
			Description: l.Description,
		})
	}
	c.JSON(http.StatusOK, gin.H{"locations": out})
}

func (h *MovieHandler) AddFilmingLocation(c *gin.Context) {
	var req addFilmingLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	loc, err := h.svc.AddFilmingLocation(c.Request.Context(), &domain.FilmingLocation{
		MovieID:     domain.MovieID(c.Param("id")),
		Name:        req.Name,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		Description: req.Description,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, filmingLocationResponse{
		ID:          loc.ID,
		Name:        loc.Name,
		Latitude:    loc.Latitude,
		Longitude:   loc.Longitude,
		Description: loc.Description,
	})
}

func (h *MovieHandler) DeleteFilmingLocation(c *gin.Context) {
	if err := h.svc.DeleteFilmingLocation(c.Request.Context(), c.Param("loc_id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MovieHandler) GetPerson(c *gin.Context) {
	idOrSlug := c.Param("id_or_slug")
	var (
		person *domain.Person
		err    error
	)
	if len(idOrSlug) == 36 {
		person, err = h.svc.GetPerson(c.Request.Context(), domain.PersonID(idOrSlug))
	} else {
		person, err = h.svc.GetPersonBySlug(c.Request.Context(), idOrSlug)
	}
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPersonResponse(person))
}

func (h *MovieHandler) GetFilmography(c *gin.Context) {
	idOrSlug := c.Param("id_or_slug")
	var (
		person *domain.Person
		err    error
	)
	if len(idOrSlug) == 36 {
		person, err = h.svc.GetPerson(c.Request.Context(), domain.PersonID(idOrSlug))
	} else {
		person, err = h.svc.GetPersonBySlug(c.Request.Context(), idOrSlug)
	}
	if err != nil {
		respondError(c, err)
		return
	}

	credits, err := h.svc.GetFilmography(c.Request.Context(), person.ID)
	if err != nil {
		respondError(c, err)
		return
	}

	type filmographyItem struct {
		CreditID  string `json:"credit_id"`
		MovieID   string `json:"movie_id"`
		Role      string `json:"role"`
		Character string `json:"character,omitempty"`
	}
	out := make([]filmographyItem, 0, len(credits))
	for _, cr := range credits {
		out = append(out, filmographyItem{
			CreditID:  cr.ID,
			MovieID:   cr.MovieID.String(),
			Role:      string(cr.Role),
			Character: cr.Character,
		})
	}
	c.JSON(http.StatusOK, gin.H{"person": toPersonResponse(person), "filmography": out})
}

func (h *MovieHandler) SearchPeople(c *gin.Context) {
	people, err := h.svc.SearchPeople(c.Request.Context(), c.Query("q"))
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]personResponse, 0, len(people))
	for _, p := range people {
		out = append(out, toPersonResponse(p))
	}
	c.JSON(http.StatusOK, gin.H{"people": out})
}

// ─── Private helpers ──────────────────────────────────────────────────────────

func (h *MovieHandler) getMovieByIDOrSlug(c *gin.Context, idOrSlug string) (*service.MovieDetail, error) {
	if len(idOrSlug) == 36 {
		return h.svc.GetByID(c.Request.Context(), domain.MovieID(idOrSlug))
	}
	return h.svc.GetBySlug(c.Request.Context(), idOrSlug)
}

func toMovieResponse(m *domain.Movie) movieResponse {
	resp := movieResponse{
		ID:              m.ID.String(),
		Title:           m.Title,
		OriginalTitle:   m.OriginalTitle,
		Slug:            m.Slug,
		Synopsis:        m.Synopsis,
		Tagline:         m.Tagline,
		RuntimeMins:     m.RuntimeMins,
		ContentRating:   string(m.ContentRating),
		Status:          string(m.Status),
		OriginalLang:    m.OriginalLang,
		Countries:       orSlice(m.Countries),
		Genres:          orSlice(m.Genres),
		PosterURL:       m.PosterURL,
		BackdropURL:     m.BackdropURL,
		TrailerURL:      m.TrailerURL,
		IMDbID:          m.IMDbID,
		TMDbID:          m.TMDbID,
		AvgRating:       m.AvgRating,
		RatingCount:     m.RatingCount,
		PopularityScore: m.PopularityScore,
		CreatedAt:       m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       m.UpdatedAt.Format(time.RFC3339),
	}
	if m.ReleaseDate != nil {
		s := m.ReleaseDate.Format("2006-01-02")
		resp.ReleaseDate = &s
	}
	return resp
}

func toMovieDetailResponse(d *service.MovieDetail) movieDetailResponse {
	return movieDetailResponse{
		movieResponse:    toMovieResponse(d.Movie),
		Credits:          toCreditResponses(d.Credits),
		StreamingLinks:   toStreamingLinkResponses(d.StreamingLinks),
		FilmingLocations: toFilmingLocationResponses(d.FilmingLocations),
	}
}

func toCreditResponses(credits []*service.CreditWithPerson) []creditResponse {
	out := make([]creditResponse, 0, len(credits))
	for _, c := range credits {
		r := creditResponse{
			ID:           c.Credit.ID,
			Role:         string(c.Credit.Role),
			Character:    c.Credit.Character,
			BillingOrder: c.Credit.BillingOrder,
		}
		if c.Person != nil {
			r.Person = &personBrief{
				ID:         c.Person.ID.String(),
				Name:       c.Person.Name,
				Slug:       c.Person.Slug,
				ProfileURL: c.Person.ProfileURL,
			}
		}
		out = append(out, r)
	}
	return out
}

func toStreamingLinkResponse(l *domain.StreamingLink) streamingLinkResponse {
	return streamingLinkResponse{
		ID:         l.ID,
		Provider:   l.Provider,
		LinkURL:    l.LinkURL,
		AccessType: l.AccessType,
		PriceCents: l.PriceCents,
		Region:     l.Region,
		UpdatedAt:  l.UpdatedAt.Format(time.RFC3339),
	}
}

func toStreamingLinkResponses(links []*domain.StreamingLink) []streamingLinkResponse {
	out := make([]streamingLinkResponse, 0, len(links))
	for _, l := range links {
		out = append(out, toStreamingLinkResponse(l))
	}
	return out
}

func toFilmingLocationResponses(locs []*domain.FilmingLocation) []filmingLocationResponse {
	out := make([]filmingLocationResponse, 0, len(locs))
	for _, l := range locs {
		out = append(out, filmingLocationResponse{
			ID:          l.ID,
			Name:        l.Name,
			Latitude:    l.Latitude,
			Longitude:   l.Longitude,
			Description: l.Description,
		})
	}
	return out
}

func toPersonResponse(p *domain.Person) personResponse {
	resp := personResponse{
		ID:         p.ID.String(),
		Name:       p.Name,
		Slug:       p.Slug,
		Birthplace: p.Birthplace,
		Bio:        p.Bio,
		ProfileURL: p.ProfileURL,
		IMDbID:     p.IMDbID,
	}
	if p.BirthDate != nil {
		s := p.BirthDate.Format("2006-01-02")
		resp.BirthDate = &s
	}
	if p.DeathDate != nil {
		s := p.DeathDate.Format("2006-01-02")
		resp.DeathDate = &s
	}
	return resp
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

func orSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
