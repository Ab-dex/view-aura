package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/Ab-dex/view-aura/internal/modules/movie/domain"
	"github.com/Ab-dex/view-aura/internal/modules/movie/repository"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// MovieService defines all business operations for the Movie module.
type MovieService interface {
	// Catalog
	Create(ctx context.Context, cmd domain.CreateMovieCmd) (*domain.Movie, error)
	GetByID(ctx context.Context, id string) (*MovieDetail, error)
	GetBySlug(ctx context.Context, slug string) (*MovieDetail, error)
	Update(ctx context.Context, cmd domain.UpdateMovieCmd) (*domain.Movie, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter domain.MovieFilter) (*MovieListResult, error)

	// Credits
	AddCredit(ctx context.Context, cmd domain.AddCreditCmd) (*domain.Credit, error)
	RemoveCredit(ctx context.Context, creditID string) error
	ListCredits(ctx context.Context, movieID string) ([]*CreditWithPerson, error)

	// Streaming availability
	UpsertStreamingLink(ctx context.Context, cmd domain.UpsertStreamingLinkCmd) (*domain.StreamingLink, error)
	DeleteStreamingLink(ctx context.Context, linkID string) error
	ListStreamingLinks(ctx context.Context, movieID string, region string) ([]*domain.StreamingLink, error)

	// Filming locations
	AddFilmingLocation(ctx context.Context, loc *domain.FilmingLocation) (*domain.FilmingLocation, error)
	DeleteFilmingLocation(ctx context.Context, id string) error
	ListFilmingLocations(ctx context.Context, movieID string) ([]*domain.FilmingLocation, error)

	// People / filmography
	GetPerson(ctx context.Context, id domain.PersonID) (*domain.Person, error)
	GetPersonBySlug(ctx context.Context, slug string) (*domain.Person, error)
	SearchPeople(ctx context.Context, query string) ([]*domain.Person, error)
	GetFilmography(ctx context.Context, personID domain.PersonID) ([]*domain.Credit, error)

	// Genres
	ListGenres(ctx context.Context) ([]*domain.Genre, error)
}

// ─── Composite response types ─────────────────────────────────────────────────

// MovieDetail is the enriched view returned on a single-movie GET — it includes
// streaming links, credits, and filming locations in one response to avoid
// N+1 round trips from the client.
type MovieDetail struct {
	Movie            *domain.Movie
	Credits          []*CreditWithPerson
	StreamingLinks   []*domain.StreamingLink
	FilmingLocations []*domain.FilmingLocation
}

type CreditWithPerson struct {
	Credit *domain.Credit
	Person *domain.Person
}

type MovieListResult struct {
	Movies []*domain.Movie
	Total  int
	Limit  int
	Offset int
}

// ─── Implementation ───────────────────────────────────────────────────────────

type movieService struct {
	movies    repository.MovieRepository
	genres    repository.GenreRepository
	persons   repository.PersonRepository
	credits   repository.CreditRepository
	links     repository.StreamingLinkRepository
	locations repository.FilmingLocationRepository
}

func NewMovieService(
	movies repository.MovieRepository,
	genres repository.GenreRepository,
	persons repository.PersonRepository,
	credits repository.CreditRepository,
	links repository.StreamingLinkRepository,
	locations repository.FilmingLocationRepository,
) MovieService {
	return &movieService{
		movies:    movies,
		genres:    genres,
		persons:   persons,
		credits:   credits,
		links:     links,
		locations: locations,
	}
}

// ─── Catalog ──────────────────────────────────────────────────────────────────

func (s *movieService) Create(ctx context.Context, cmd domain.CreateMovieCmd) (*domain.Movie, error) {
	if err := validateCreateMovie(cmd); err != nil {
		return nil, err
	}

	// Guard against exact IMDb duplicates.
	if cmd.IMDbID != "" {
		exists, err := s.movies.ExistsByIMDbID(ctx, cmd.IMDbID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, apierror.Conflict("a movie with this IMDb ID already exists")
		}
	}

	movie := &domain.Movie{
		ID:            domain.MovieID(uuid.New()),
		Title:         strings.TrimSpace(cmd.Title),
		OriginalTitle: strings.TrimSpace(cmd.OriginalTitle),
		Slug:          generateSlug(cmd.Title, time.Now().Year()),
		Synopsis:      strings.TrimSpace(cmd.Synopsis),
		Tagline:       strings.TrimSpace(cmd.Tagline),
		ReleaseDate:   cmd.ReleaseDate,
		RuntimeMins:   cmd.RuntimeMins,
		ContentRating: cmd.ContentRating,
		Status:        domain.MovieStatusDraft,
		OriginalLang:  orDefault(cmd.OriginalLang, "en"),
		Countries:     cmd.Countries,
		Genres:        cmd.Genres,
		PosterURL:     cmd.PosterURL,
		BackdropURL:   cmd.BackdropURL,
		TrailerURL:    cmd.TrailerURL,
		IMDbID:        cmd.IMDbID,
		TMDbID:        cmd.TMDbID,
	}

	if cmd.ReleaseDate != nil {
		movie.Slug = generateSlug(cmd.Title, cmd.ReleaseDate.Year())
	}

	created, err := s.movies.Create(ctx, movie)
	if err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info().
		Str("movie_id", created.ID.String()).
		Str("title", created.Title).
		Msg("movie created")

	return created, nil
}

func (s *movieService) GetByID(ctx context.Context, id string) (*MovieDetail, error) {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return nil, apierror.New(400, apierror.CodeValidation, "invalid movie id")
	}
	movie, err := s.movies.GetByID(ctx, domain.MovieID(parsedID))
	if err != nil {
		return nil, err
	}
	return s.enrichMovie(ctx, movie)
}

func (s *movieService) GetBySlug(ctx context.Context, slug string) (*MovieDetail, error) {
	movie, err := s.movies.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	return s.enrichMovie(ctx, movie)
}

// enrichMovie fans out to credits, streaming links, and locations in sequence.
// In production these can be parallelised with errgroup — sequential is correct
// and simpler for the scaffold.
func (s *movieService) enrichMovie(ctx context.Context, movie *domain.Movie) (*MovieDetail, error) {
	rawCredits, err := s.credits.ListByMovie(ctx, movie.ID)
	if err != nil {
		return nil, err
	}

	// Hydrate each credit with the person record.
	creditsWithPerson := make([]*CreditWithPerson, 0, len(rawCredits))
	for _, c := range rawCredits {
		person, err := s.persons.GetByID(ctx, c.PersonID)
		if err != nil {
			// Non-fatal: return the credit without person data rather than failing.
			creditsWithPerson = append(creditsWithPerson, &CreditWithPerson{Credit: c})
			continue
		}
		creditsWithPerson = append(creditsWithPerson, &CreditWithPerson{Credit: c, Person: person})
	}

	// Region is not available in this layer — "" returns worldwide + all links.
	streamingLinks, err := s.links.ListByMovie(ctx, movie.ID, "")
	if err != nil {
		return nil, err
	}

	locations, err := s.locations.ListByMovie(ctx, movie.ID)
	if err != nil {
		return nil, err
	}

	return &MovieDetail{
		Movie:            movie,
		Credits:          creditsWithPerson,
		StreamingLinks:   streamingLinks,
		FilmingLocations: locations,
	}, nil
}

func (s *movieService) Update(ctx context.Context, cmd domain.UpdateMovieCmd) (*domain.Movie, error) {
	movie, err := s.movies.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	applyStringPtr(cmd.Title, &movie.Title)
	applyStringPtr(cmd.OriginalTitle, &movie.OriginalTitle)
	applyStringPtr(cmd.Synopsis, &movie.Synopsis)
	applyStringPtr(cmd.Tagline, &movie.Tagline)
	applyStringPtr(cmd.PosterURL, &movie.PosterURL)
	applyStringPtr(cmd.BackdropURL, &movie.BackdropURL)
	applyStringPtr(cmd.TrailerURL, &movie.TrailerURL)
	applyStringPtr(cmd.OriginalLang, &movie.OriginalLang)

	if cmd.ReleaseDate != nil {
		movie.ReleaseDate = cmd.ReleaseDate
	}
	if cmd.RuntimeMins != nil {
		movie.RuntimeMins = *cmd.RuntimeMins
	}
	if cmd.ContentRating != nil {
		movie.ContentRating = *cmd.ContentRating
	}
	if cmd.Status != nil {
		movie.Status = *cmd.Status
	}
	if cmd.Countries != nil {
		movie.Countries = cmd.Countries
	}
	if cmd.Genres != nil {
		movie.Genres = cmd.Genres
	}

	return s.movies.Update(ctx, movie)
}

func (s *movieService) Delete(ctx context.Context, id string) error {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return apierror.New(400, apierror.CodeValidation, "invalid movie id")
	}
	return s.movies.Delete(ctx, domain.MovieID(parsedID))
}

func (s *movieService) List(ctx context.Context, filter domain.MovieFilter) (*MovieListResult, error) {
	movies, total, err := s.movies.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &MovieListResult{
		Movies: movies,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}, nil
}

// ─── Credits ──────────────────────────────────────────────────────────────────

func (s *movieService) AddCredit(ctx context.Context, cmd domain.AddCreditCmd) (*domain.Credit, error) {
	// Verify the movie and person both exist before inserting.
	if _, err := s.movies.GetByID(ctx, cmd.MovieID); err != nil {
		return nil, err
	}
	if _, err := s.persons.GetByID(ctx, cmd.PersonID); err != nil {
		return nil, err
	}

	credit := &domain.Credit{
		ID:           uuid.New().String(),
		MovieID:      cmd.MovieID,
		PersonID:     cmd.PersonID,
		Role:         cmd.Role,
		Character:    cmd.Character,
		BillingOrder: cmd.BillingOrder,
	}
	return s.credits.Add(ctx, credit)
}

func (s *movieService) RemoveCredit(ctx context.Context, creditID string) error {
	return s.credits.Remove(ctx, creditID)
}

func (s *movieService) ListCredits(ctx context.Context, movieID string) ([]*CreditWithPerson, error) {
	parsedID, err := uuid.Parse(movieID)
	if err != nil {
		return nil, apierror.New(400, apierror.CodeValidation, "invalid movie id")
	}
	rawCredits, err := s.credits.ListByMovie(ctx, domain.MovieID(parsedID))
	if err != nil {
		return nil, err
	}
	out := make([]*CreditWithPerson, 0, len(rawCredits))
	for _, c := range rawCredits {
		person, _ := s.persons.GetByID(ctx, c.PersonID)
		out = append(out, &CreditWithPerson{Credit: c, Person: person})
	}
	return out, nil
}

// ─── Streaming links ──────────────────────────────────────────────────────────

func (s *movieService) UpsertStreamingLink(ctx context.Context, cmd domain.UpsertStreamingLinkCmd) (*domain.StreamingLink, error) {
	if _, err := s.movies.GetByID(ctx, cmd.MovieID); err != nil {
		return nil, err
	}
	link := &domain.StreamingLink{
		ID:         uuid.New().String(),
		MovieID:    cmd.MovieID,
		Provider:   cmd.Provider,
		LinkURL:    cmd.LinkURL,
		AccessType: cmd.AccessType,
		PriceCents: cmd.PriceCents,
		Region:     cmd.Region,
	}
	return s.links.Upsert(ctx, link)
}

func (s *movieService) DeleteStreamingLink(ctx context.Context, linkID string) error {
	return s.links.Delete(ctx, linkID)
}

func (s *movieService) ListStreamingLinks(ctx context.Context, movieID string, region string) ([]*domain.StreamingLink, error) {
	parsedID, err := uuid.Parse(movieID)
	if err != nil {
		return nil, apierror.New(400, apierror.CodeValidation, "invalid movie id")
	}
	return s.links.ListByMovie(ctx, domain.MovieID(parsedID), region)
}

// ─── Filming locations ────────────────────────────────────────────────────────

func (s *movieService) AddFilmingLocation(ctx context.Context, loc *domain.FilmingLocation) (*domain.FilmingLocation, error) {
	if _, err := s.movies.GetByID(ctx, loc.MovieID); err != nil {
		return nil, err
	}
	loc.ID = uuid.New().String()
	return s.locations.Add(ctx, loc)
}

func (s *movieService) DeleteFilmingLocation(ctx context.Context, id string) error {
	return s.locations.Delete(ctx, id)
}

func (s *movieService) ListFilmingLocations(ctx context.Context, movieID string) ([]*domain.FilmingLocation, error) {
	parsedID, err := uuid.Parse(movieID)
	if err != nil {
		return nil, apierror.New(400, apierror.CodeValidation, "invalid movie id")
	}
	return s.locations.ListByMovie(ctx, domain.MovieID(parsedID))
}

// ─── People ───────────────────────────────────────────────────────────────────

func (s *movieService) GetPerson(ctx context.Context, id domain.PersonID) (*domain.Person, error) {
	return s.persons.GetByID(ctx, id)
}

func (s *movieService) GetPersonBySlug(ctx context.Context, slug string) (*domain.Person, error) {
	return s.persons.GetBySlug(ctx, slug)
}

func (s *movieService) SearchPeople(ctx context.Context, query string) ([]*domain.Person, error) {
	if strings.TrimSpace(query) == "" {
		return nil, apierror.Validation("search query must not be empty", nil)
	}
	return s.persons.Search(ctx, query, 20)
}

func (s *movieService) GetFilmography(ctx context.Context, personID domain.PersonID) ([]*domain.Credit, error) {
	if _, err := s.persons.GetByID(ctx, personID); err != nil {
		return nil, err
	}
	return s.credits.ListByPerson(ctx, personID)
}

// ─── Genres ───────────────────────────────────────────────────────────────────

func (s *movieService) ListGenres(ctx context.Context) ([]*domain.Genre, error) {
	return s.genres.List(ctx)
}

// ─── Validation ───────────────────────────────────────────────────────────────

func validateCreateMovie(cmd domain.CreateMovieCmd) error {
	type fe struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}
	var errs []fe

	if strings.TrimSpace(cmd.Title) == "" {
		errs = append(errs, fe{"title", "title is required"})
	}
	if cmd.RuntimeMins < 0 {
		errs = append(errs, fe{"runtime_mins", "runtime must be non-negative"})
	}
	if len(errs) > 0 {
		return apierror.Validation("movie input is invalid", errs)
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// generateSlug produces a url-safe slug from a title and year, e.g.
// "The Godfather" + 1972 → "the-godfather-1972".
func generateSlug(title string, year int) string {
	slug := strings.ToLower(strings.TrimSpace(title))
	var b strings.Builder
	for _, r := range slug {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	base := strings.Trim(b.String(), "-")
	// Collapse multiple dashes.
	for strings.Contains(base, "--") {
		base = strings.ReplaceAll(base, "--", "-")
	}
	if year > 0 {
		return fmt.Sprintf("%s-%d", base, year)
	}
	return base
}

func applyStringPtr(src *string, dst *string) {
	if src != nil {
		*dst = *src
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
