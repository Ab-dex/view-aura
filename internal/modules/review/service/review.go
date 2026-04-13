package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/Ab-dex/view-aura/internal/modules/review/domain"
	"github.com/Ab-dex/view-aura/internal/modules/review/repository"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// ReviewService defines all business operations for the review module.
type ReviewService interface {
	Create(ctx context.Context, cmd domain.CreateReviewCmd) (*domain.Review, error)
	Update(ctx context.Context, cmd domain.UpdateReviewCmd) (*domain.Review, error)
	Delete(ctx context.Context, id domain.ReviewID, userID string) error
	GetByID(ctx context.Context, id domain.ReviewID) (*domain.Review, error)
	List(ctx context.Context, filter domain.ReviewFilter) ([]*domain.Review, int, error)
	GetByUserAndMovie(ctx context.Context, userID, movieID string) (*domain.Review, error)

	// React adds or updates a user's reaction on a review.
	React(ctx context.Context, cmd domain.ReactToReviewCmd) error
	// UnReact removes a user's reaction from a review.
	UnReact(ctx context.Context, reviewID domain.ReviewID, userID string) error
	// GetMyReaction returns the calling user's current reaction, if any.
	GetMyReaction(ctx context.Context, userID string, reviewID domain.ReviewID) (*domain.ReviewReaction, error)

	// Report submits an abuse report for a review.
	Report(ctx context.Context, cmd domain.ReportReviewCmd) error
}

type reviewService struct {
	reviews   repository.ReviewRepository
	reactions repository.ReviewReactionRepository
	reports   repository.ReviewReportRepository
}

func NewReviewService(
	reviews repository.ReviewRepository,
	reactions repository.ReviewReactionRepository,
	reports repository.ReviewReportRepository,
) ReviewService {
	return &reviewService{reviews: reviews, reactions: reactions, reports: reports}
}

func (s *reviewService) Create(ctx context.Context, cmd domain.CreateReviewCmd) (*domain.Review, error) {
	if err := validateCreate(cmd); err != nil {
		return nil, err
	}

	// Enforce one review per user per movie.
	existing, err := s.reviews.GetByUserAndMovie(ctx, cmd.UserID, cmd.MovieID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apierror.Conflict("you have already reviewed this movie")
	}

	rev := &domain.Review{
		ID:          domain.ReviewID(uuid.New().String()),
		MovieID:     cmd.MovieID,
		UserID:      cmd.UserID,
		Type:        cmd.Type,
		Body:        strings.TrimSpace(cmd.Body),
		VideoURL:    cmd.VideoURL,
		IsSpoiler:   cmd.IsSpoiler,
		IsPublished: true, // published immediately; admin moderation handles removal
	}

	result, err := s.reviews.Create(ctx, rev)
	if err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info().
		Str("review_id", result.ID.String()).
		Str("movie_id", cmd.MovieID).
		Str("user_id", cmd.UserID).
		Msg("review created")

	return result, nil
}

func (s *reviewService) Update(ctx context.Context, cmd domain.UpdateReviewCmd) (*domain.Review, error) {
	rev, err := s.reviews.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	if rev.UserID != cmd.UserID {
		return nil, apierror.Forbidden("you can only edit your own reviews")
	}

	if cmd.Body != nil {
		rev.Body = strings.TrimSpace(*cmd.Body)
	}
	if cmd.VideoURL != nil {
		rev.VideoURL = *cmd.VideoURL
	}
	if cmd.IsSpoiler != nil {
		rev.IsSpoiler = *cmd.IsSpoiler
	}
	if cmd.Publish != nil {
		rev.IsPublished = *cmd.Publish
	}

	return s.reviews.Update(ctx, rev)
}

func (s *reviewService) Delete(ctx context.Context, id domain.ReviewID, userID string) error {
	return s.reviews.Delete(ctx, id, userID)
}

func (s *reviewService) GetByID(ctx context.Context, id domain.ReviewID) (*domain.Review, error) {
	return s.reviews.GetByID(ctx, id)
}

func (s *reviewService) List(ctx context.Context, filter domain.ReviewFilter) ([]*domain.Review, int, error) {
	return s.reviews.List(ctx, filter)
}

func (s *reviewService) GetByUserAndMovie(ctx context.Context, userID, movieID string) (*domain.Review, error) {
	return s.reviews.GetByUserAndMovie(ctx, userID, movieID)
}

// ─── Reactions ────────────────────────────────────────────────────────────────

func (s *reviewService) React(ctx context.Context, cmd domain.ReactToReviewCmd) error {
	// Check if the review exists.
	if _, err := s.reviews.GetByID(ctx, cmd.ReviewID); err != nil {
		return err
	}

	existing, err := s.reactions.GetByUserAndReview(ctx, cmd.UserID, cmd.ReviewID)
	if err != nil {
		return err
	}

	rx := &domain.ReviewReaction{
		ID:       domain.ReactionID(uuid.New().String()),
		ReviewID: cmd.ReviewID,
		UserID:   cmd.UserID,
		Type:     cmd.Type,
	}

	if existing != nil && existing.Type == cmd.Type {
		// Idempotent — same reaction already exists, nothing to do.
		return nil
	}

	if existing != nil {
		// Changing reaction type: decrement old counter, then upsert.
		if err := s.reactions.IncrementCounter(ctx, cmd.ReviewID, existing.Type, -1); err != nil {
			return err
		}
	}

	if _, err := s.reactions.Upsert(ctx, rx); err != nil {
		return err
	}
	return s.reactions.IncrementCounter(ctx, cmd.ReviewID, cmd.Type, 1)
}

func (s *reviewService) UnReact(ctx context.Context, reviewID domain.ReviewID, userID string) error {
	existing, err := s.reactions.GetByUserAndReview(ctx, userID, reviewID)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil // idempotent
	}

	if err := s.reactions.Delete(ctx, reviewID, userID); err != nil {
		return err
	}
	return s.reactions.IncrementCounter(ctx, reviewID, existing.Type, -1)
}

func (s *reviewService) GetMyReaction(ctx context.Context, userID string, reviewID domain.ReviewID) (*domain.ReviewReaction, error) {
	return s.reactions.GetByUserAndReview(ctx, userID, reviewID)
}

// ─── Reports ──────────────────────────────────────────────────────────────────

func (s *reviewService) Report(ctx context.Context, cmd domain.ReportReviewCmd) error {
	if _, err := s.reviews.GetByID(ctx, cmd.ReviewID); err != nil {
		return err
	}

	already, err := s.reports.HasUserReported(ctx, cmd.UserID, cmd.ReviewID)
	if err != nil {
		return err
	}
	if already {
		return apierror.Conflict("you have already reported this review")
	}

	report := &domain.ReviewReport{
		ID:       uuid.New().String(),
		ReviewID: cmd.ReviewID,
		UserID:   cmd.UserID,
		Reason:   cmd.Reason,
		Note:     strings.TrimSpace(cmd.Note),
	}
	_, err = s.reports.Create(ctx, report)
	return err
}

// ─── Validation ───────────────────────────────────────────────────────────────

func validateCreate(cmd domain.CreateReviewCmd) error {
	type fe struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}
	var errs []fe

	if cmd.MovieID == "" {
		errs = append(errs, fe{"movie_id", "movie_id is required"})
	}
	if cmd.UserID == "" {
		errs = append(errs, fe{"user_id", "user_id is required"})
	}
	if cmd.Type == domain.ReviewTypeVideo && cmd.VideoURL == "" {
		errs = append(errs, fe{"video_url", "video_url is required for video reviews"})
	}
	if cmd.Type != domain.ReviewTypeVideo && strings.TrimSpace(cmd.Body) == "" {
		errs = append(errs, fe{"body", "body is required for non-video reviews"})
	}
	const maxShort = 280
	if cmd.Type == domain.ReviewTypeShort && len(cmd.Body) > maxShort {
		errs = append(errs, fe{"body", "short reviews must be 280 characters or less"})
	}

	if len(errs) > 0 {
		return apierror.Validation("review input is invalid", errs)
	}
	return nil
}
