package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/Ab-dex/view-aura/internal/modules/rating/domain"
	"github.com/Ab-dex/view-aura/internal/modules/rating/repository"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// RatingService defines all business operations for the rating module.
type RatingService interface {
	// Upsert submits or updates a user's rating for a movie.
	Upsert(ctx context.Context, cmd domain.UpsertRatingCmd) (*domain.Rating, error)

	// Delete removes a user's rating for a movie.
	Delete(ctx context.Context, userID, movieID string) error

	// GetByUser returns a specific user's rating for a movie, or nil.
	GetByUser(ctx context.Context, userID, movieID string) (*domain.Rating, error)

	// ListByMovie returns paginated ratings for a movie.
	ListByMovie(ctx context.Context, filter domain.RatingFilter) ([]*domain.Rating, int, error)

	// ListByUser returns all movies a user has rated.
	ListByUser(ctx context.Context, filter domain.RatingFilter) ([]*domain.Rating, int, error)

	// GetStats returns aggregated dimensional stats for a movie.
	GetStats(ctx context.Context, movieID string) (*domain.MovieRatingStats, error)
}

type ratingService struct {
	ratings repository.RatingRepository
}

func NewRatingService(ratings repository.RatingRepository) RatingService {
	return &ratingService{ratings: ratings}
}

func (s *ratingService) Upsert(ctx context.Context, cmd domain.UpsertRatingCmd) (*domain.Rating, error) {
	if err := validateRating(cmd); err != nil {
		return nil, err
	}

	rt := &domain.Rating{
		ID:             domain.RatingID(uuid.New().String()),
		MovieID:        cmd.MovieID,
		UserID:         cmd.UserID,
		Overall:        cmd.Overall,
		Acting:         cmd.Acting,
		Direction:      cmd.Direction,
		Writing:        cmd.Writing,
		Cinematography: cmd.Cinematography,
		Soundtrack:     cmd.Soundtrack,
		Reaction:       cmd.Reaction,
	}

	result, err := s.ratings.Upsert(ctx, rt)
	if err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info().
		Str("user_id", cmd.UserID).
		Str("movie_id", cmd.MovieID).
		Float64("overall", cmd.Overall).
		Msg("rating upserted")

	return result, nil
}

func (s *ratingService) Delete(ctx context.Context, userID, movieID string) error {
	return s.ratings.Delete(ctx, userID, movieID)
}

func (s *ratingService) GetByUser(ctx context.Context, userID, movieID string) (*domain.Rating, error) {
	rt, err := s.ratings.GetByUserAndMovie(ctx, userID, movieID)
	if err != nil {
		// Return nil (no rating) rather than a 404 error — callers treat
		// "no rating" as a valid state (the user hasn't rated yet).
		if apierror.IsCode(err, apierror.CodeNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return rt, nil
}

func (s *ratingService) ListByMovie(ctx context.Context, f domain.RatingFilter) ([]*domain.Rating, int, error) {
	return s.ratings.ListByMovie(ctx, f)
}

func (s *ratingService) ListByUser(ctx context.Context, f domain.RatingFilter) ([]*domain.Rating, int, error) {
	return s.ratings.ListByUser(ctx, f)
}

func (s *ratingService) GetStats(ctx context.Context, movieID string) (*domain.MovieRatingStats, error) {
	return s.ratings.GetStats(ctx, movieID)
}

// ─── Validation ───────────────────────────────────────────────────────────────

func validateRating(cmd domain.UpsertRatingCmd) error {
	type fe struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}
	var errs []fe

	if cmd.Overall < 1.0 || cmd.Overall > 5.0 {
		errs = append(errs, fe{"overall", "overall must be between 1.0 and 5.0"})
	}
	for _, dim := range []*float64{cmd.Acting, cmd.Direction, cmd.Writing, cmd.Cinematography, cmd.Soundtrack} {
		if dim != nil && (*dim < 1.0 || *dim > 5.0) {
			errs = append(errs, fe{"dimension", "all dimension scores must be between 1.0 and 5.0"})
			break
		}
	}
	if cmd.MovieID == "" {
		errs = append(errs, fe{"movie_id", "movie_id is required"})
	}
	if cmd.UserID == "" {
		errs = append(errs, fe{"user_id", "user_id is required"})
	}

	if len(errs) > 0 {
		return apierror.Validation("rating input is invalid", errs)
	}
	return nil
}
