package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/rating/domain"
)

// RatingRepository manages user ratings for movies.
type RatingRepository interface {
	// Upsert inserts or updates the rating for (user_id, movie_id).
	// One user can hold exactly one rating per movie.
	Upsert(ctx context.Context, rating *domain.Rating) (*domain.Rating, error)

	// GetByUserAndMovie fetches a user's rating for a specific movie.
	GetByUserAndMovie(ctx context.Context, userID, movieID string) (*domain.Rating, error)

	// Delete removes a user's rating for a movie.
	Delete(ctx context.Context, userID, movieID string) error

	// ListByMovie returns all ratings for a movie, paginated.
	ListByMovie(ctx context.Context, filter domain.RatingFilter) ([]*domain.Rating, int, error)

	// ListByUser returns all ratings submitted by a user.
	ListByUser(ctx context.Context, filter domain.RatingFilter) ([]*domain.Rating, int, error)

	// GetStats returns aggregated dimensional stats for a movie.
	GetStats(ctx context.Context, movieID string) (*domain.MovieRatingStats, error)
}
