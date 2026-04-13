package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/movie/domain"
)

// MovieRepository is the primary persistence interface for the Movie aggregate.
type MovieRepository interface {
	Create(ctx context.Context, movie *domain.Movie) (*domain.Movie, error)
	GetByID(ctx context.Context, id domain.MovieID) (*domain.Movie, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Movie, error)
	Update(ctx context.Context, movie *domain.Movie) (*domain.Movie, error)
	Delete(ctx context.Context, id domain.MovieID) error
	List(ctx context.Context, filter domain.MovieFilter) ([]*domain.Movie, int, error)
	// UpdateAggregates atomically writes pre-computed avg_rating, rating_count,
	// and popularity_score — called by the ratings worker, not inline.
	UpdateAggregates(ctx context.Context, id domain.MovieID, avgRating float64, ratingCount int, popularity float64) error
	ExistsByIMDbID(ctx context.Context, imdbID string) (bool, error)
}

// GenreRepository manages the genre taxonomy.
type GenreRepository interface {
	Create(ctx context.Context, genre *domain.Genre) (*domain.Genre, error)
	GetByID(ctx context.Context, id domain.GenreID) (*domain.Genre, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Genre, error)
	List(ctx context.Context) ([]*domain.Genre, error)
	Delete(ctx context.Context, id domain.GenreID) error
}

// PersonRepository manages cast and crew.
type PersonRepository interface {
	Create(ctx context.Context, person *domain.Person) (*domain.Person, error)
	GetByID(ctx context.Context, id domain.PersonID) (*domain.Person, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Person, error)
	// Search returns people whose name matches the query (for credit linking).
	Search(ctx context.Context, query string, limit int) ([]*domain.Person, error)
	Update(ctx context.Context, person *domain.Person) (*domain.Person, error)
}

// CreditRepository links people to movies.
type CreditRepository interface {
	Add(ctx context.Context, credit *domain.Credit) (*domain.Credit, error)
	Remove(ctx context.Context, creditID string) error
	ListByMovie(ctx context.Context, movieID domain.MovieID) ([]*domain.Credit, error)
	// ListByPerson returns a person's full filmography.
	ListByPerson(ctx context.Context, personID domain.PersonID) ([]*domain.Credit, error)
}

// StreamingLinkRepository manages where-to-watch data.
type StreamingLinkRepository interface {
	// Upsert inserts or replaces the link for (movie, provider, region).
	Upsert(ctx context.Context, link *domain.StreamingLink) (*domain.StreamingLink, error)
	ListByMovie(ctx context.Context, movieID domain.MovieID, region string) ([]*domain.StreamingLink, error)
	Delete(ctx context.Context, id string) error
}

// FilmingLocationRepository manages shoot locations.
type FilmingLocationRepository interface {
	Add(ctx context.Context, loc *domain.FilmingLocation) (*domain.FilmingLocation, error)
	ListByMovie(ctx context.Context, movieID domain.MovieID) ([]*domain.FilmingLocation, error)
	Delete(ctx context.Context, id string) error
}
