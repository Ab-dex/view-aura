package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/review/domain"
)

// ReviewRepository manages review persistence.
type ReviewRepository interface {
	Create(ctx context.Context, review *domain.Review) (*domain.Review, error)
	GetByID(ctx context.Context, id domain.ReviewID) (*domain.Review, error)
	Update(ctx context.Context, review *domain.Review) (*domain.Review, error)
	Delete(ctx context.Context, id domain.ReviewID, userID string) error
	List(ctx context.Context, filter domain.ReviewFilter) ([]*domain.Review, int, error)
	// GetByUserAndMovie returns the user's review for a specific movie, if any.
	GetByUserAndMovie(ctx context.Context, userID, movieID string) (*domain.Review, error)
}

// ReviewReactionRepository manages per-user reactions on reviews.
type ReviewReactionRepository interface {
	// Upsert inserts or replaces a user's reaction on a review.
	Upsert(ctx context.Context, reaction *domain.ReviewReaction) (*domain.ReviewReaction, error)
	// Delete removes a user's reaction from a review.
	Delete(ctx context.Context, reviewID domain.ReviewID, userID string) error
	// GetByUserAndReview fetches the user's current reaction, if any.
	GetByUserAndReview(ctx context.Context, userID string, reviewID domain.ReviewID) (*domain.ReviewReaction, error)
	// IncrementCounter atomically increments the named reaction counter on the
	// review row (like_count, helpful_count, etc.) by delta (+1 or -1).
	IncrementCounter(ctx context.Context, reviewID domain.ReviewID, reactionType domain.ReviewReactionType, delta int) error
}

// ReviewReportRepository manages abuse reports.
type ReviewReportRepository interface {
	Create(ctx context.Context, report *domain.ReviewReport) (*domain.ReviewReport, error)
	// HasUserReported returns true if the user already filed a report for this review.
	HasUserReported(ctx context.Context, userID string, reviewID domain.ReviewID) (bool, error)
}
