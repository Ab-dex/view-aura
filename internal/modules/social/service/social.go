package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Ab-dex/view-aura/internal/modules/social/domain"
	"github.com/Ab-dex/view-aura/internal/modules/social/repository"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// SocialService aggregates all social operations into one interface.
// The four sub-domains (follows, feed, discussions, challenges) are related
// enough to share a single service and module — they all depend on the follow
// graph and emit/consume activities.
type SocialService interface {
	// ── Follow graph ──────────────────────────────────────────────────────────
	Follow(ctx context.Context, followerID, followeeID string) error
	Unfollow(ctx context.Context, followerID, followeeID string) error
	IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error)
	ListFollowers(ctx context.Context, userID string, limit, offset int) ([]string, int, error)
	ListFollowing(ctx context.Context, userID string, limit, offset int) ([]string, int, error)
	GetFollowStats(ctx context.Context, userID string) (*domain.FollowStats, error)

	// ── Activity feed ─────────────────────────────────────────────────────────
	// RecordActivity persists an event — called by other modules (rating, review,
	// watchlist) after a state-changing operation.
	RecordActivity(ctx context.Context, a *domain.Activity) error
	GetFeed(ctx context.Context, filter domain.FeedFilter) (*domain.FeedPage, error)
	GetUserActivity(ctx context.Context, filter domain.FeedFilter) (*domain.FeedPage, error)

	// ── Discussions ───────────────────────────────────────────────────────────
	CreateThread(ctx context.Context, cmd domain.CreateThreadCmd) (*domain.Thread, error)
	GetThread(ctx context.Context, id domain.ThreadID) (*domain.Thread, error)
	ListThreads(ctx context.Context, filter domain.ThreadFilter) ([]*domain.Thread, int, error)
	DeleteThread(ctx context.Context, id domain.ThreadID, callerID string) error

	CreatePost(ctx context.Context, cmd domain.CreatePostCmd) (*domain.Post, error)
	GetPost(ctx context.Context, id domain.PostID) (*domain.Post, error)
	ListPosts(ctx context.Context, filter domain.PostFilter) ([]*domain.Post, int, error)
	ListReplies(ctx context.Context, parentID domain.PostID) ([]*domain.Post, error)
	DeletePost(ctx context.Context, id domain.PostID, callerID string) error
	LikePost(ctx context.Context, postID domain.PostID, userID string) error
	UnlikePost(ctx context.Context, postID domain.PostID, userID string) error

	// ── Challenges ────────────────────────────────────────────────────────────
	CreateChallenge(ctx context.Context, cmd domain.CreateChallengeCmd) (*domain.Challenge, error)
	GetChallenge(ctx context.Context, id domain.ChallengeID) (*ChallengeDetail, error)
	ListChallenges(ctx context.Context, filter domain.ChallengeFilter) ([]*domain.Challenge, int, error)
	DeleteChallenge(ctx context.Context, id domain.ChallengeID, callerID string) error
	JoinChallenge(ctx context.Context, challengeID domain.ChallengeID, userID string) error
	LeaveChallenge(ctx context.Context, challengeID domain.ChallengeID, userID string) error
	MarkMovieWatched(ctx context.Context, challengeID domain.ChallengeID, userID, movieID string) error
	GetMyProgress(ctx context.Context, challengeID domain.ChallengeID, userID string) (*domain.ChallengeParticipant, error)
	ListLeaderboard(ctx context.Context, challengeID domain.ChallengeID, limit, offset int) ([]*domain.ChallengeParticipant, int, error)
}

// ChallengeDetail enriches a challenge with the caller's participant record.
type ChallengeDetail struct {
	Challenge   *domain.Challenge
	Participant *domain.ChallengeParticipant // nil if not joined
}

// ─── Implementation ───────────────────────────────────────────────────────────

type socialService struct {
	follows    repository.FollowRepository
	activities repository.ActivityRepository
	threads    repository.ThreadRepository
	posts      repository.PostRepository
	challenges repository.ChallengeRepository
}

func NewSocialService(
	follows repository.FollowRepository,
	activities repository.ActivityRepository,
	threads repository.ThreadRepository,
	posts repository.PostRepository,
	challenges repository.ChallengeRepository,
) SocialService {
	return &socialService{
		follows:    follows,
		activities: activities,
		threads:    threads,
		posts:      posts,
		challenges: challenges,
	}
}

// ─── Follow graph ─────────────────────────────────────────────────────────────

func (s *socialService) Follow(ctx context.Context, followerID, followeeID string) error {
	if err := s.follows.Follow(ctx, followerID, followeeID); err != nil {
		return err
	}
	// Emit a "followed" activity so it appears in the followee's fans' feeds.
	_ = s.recordActivity(ctx, followerID, domain.ActivityFollowed, "user", followeeID, map[string]any{
		"followee_id": followeeID,
	})
	logger.FromContext(ctx).Info().
		Str("follower", followerID).Str("followee", followeeID).
		Msg("user followed")
	return nil
}

func (s *socialService) Unfollow(ctx context.Context, followerID, followeeID string) error {
	if err := s.follows.Unfollow(ctx, followerID, followeeID); err != nil {
		return err
	}
	_ = s.activities.DeleteBySubject(ctx, "user", followeeID)
	return nil
}

func (s *socialService) IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error) {
	return s.follows.IsFollowing(ctx, followerID, followeeID)
}

func (s *socialService) ListFollowers(ctx context.Context, userID string, limit, offset int) ([]string, int, error) {
	return s.follows.ListFollowers(ctx, userID, limit, offset)
}

func (s *socialService) ListFollowing(ctx context.Context, userID string, limit, offset int) ([]string, int, error) {
	return s.follows.ListFollowing(ctx, userID, limit, offset)
}

func (s *socialService) GetFollowStats(ctx context.Context, userID string) (*domain.FollowStats, error) {
	return s.follows.GetStats(ctx, userID)
}

// ─── Activity feed ────────────────────────────────────────────────────────────

func (s *socialService) RecordActivity(ctx context.Context, a *domain.Activity) error {
	if a.ID == "" {
		a.ID = domain.ActivityID(uuid.New().String())
	}
	return s.activities.Record(ctx, a)
}

func (s *socialService) GetFeed(ctx context.Context, filter domain.FeedFilter) (*domain.FeedPage, error) {
	acts, total, err := s.activities.GetFeed(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &domain.FeedPage{
		Activities: acts,
		Total:      total,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
	}, nil
}

func (s *socialService) GetUserActivity(ctx context.Context, filter domain.FeedFilter) (*domain.FeedPage, error) {
	acts, total, err := s.activities.GetByActor(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &domain.FeedPage{
		Activities: acts,
		Total:      total,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
	}, nil
}

// ─── Discussions ──────────────────────────────────────────────────────────────

func (s *socialService) CreateThread(ctx context.Context, cmd domain.CreateThreadCmd) (*domain.Thread, error) {
	if err := validateThread(cmd); err != nil {
		return nil, err
	}
	thread := &domain.Thread{
		ID:                 domain.ThreadID(uuid.New().String()),
		Scope:              cmd.Scope,
		ScopeID:            cmd.ScopeID,
		Title:              strings.TrimSpace(cmd.Title),
		SceneTimestampSecs: cmd.SceneTimestampSecs,
		IsSpoiler:          cmd.IsSpoiler,
		CreatedBy:          cmd.CreatedBy,
	}
	return s.threads.Create(ctx, thread)
}

func (s *socialService) GetThread(ctx context.Context, id domain.ThreadID) (*domain.Thread, error) {
	return s.threads.GetByID(ctx, id)
}

func (s *socialService) ListThreads(ctx context.Context, filter domain.ThreadFilter) ([]*domain.Thread, int, error) {
	return s.threads.List(ctx, filter)
}

func (s *socialService) DeleteThread(ctx context.Context, id domain.ThreadID, callerID string) error {
	return s.threads.Delete(ctx, id, callerID)
}

func (s *socialService) CreatePost(ctx context.Context, cmd domain.CreatePostCmd) (*domain.Post, error) {
	if strings.TrimSpace(cmd.Body) == "" {
		return nil, apierror.Validation("post body is required", nil)
	}
	// Verify the thread exists before inserting.
	if _, err := s.threads.GetByID(ctx, cmd.ThreadID); err != nil {
		return nil, err
	}
	// Verify parent post exists if this is a reply.
	if cmd.ParentID != nil {
		if _, err := s.posts.GetByID(ctx, *cmd.ParentID); err != nil {
			return nil, err
		}
	}

	post := &domain.Post{
		ID:       domain.PostID(uuid.New().String()),
		ThreadID: cmd.ThreadID,
		AuthorID: cmd.AuthorID,
		Body:     strings.TrimSpace(cmd.Body),
		ParentID: cmd.ParentID,
	}
	result, err := s.posts.Create(ctx, post)
	if err != nil {
		return nil, err
	}
	// Update the denormalized post count only for top-level posts.
	if cmd.ParentID == nil {
		_ = s.threads.IncrementPostCount(ctx, cmd.ThreadID, 1)
	}
	return result, nil
}

func (s *socialService) GetPost(ctx context.Context, id domain.PostID) (*domain.Post, error) {
	return s.posts.GetByID(ctx, id)
}

func (s *socialService) ListPosts(ctx context.Context, filter domain.PostFilter) ([]*domain.Post, int, error) {
	return s.posts.List(ctx, filter)
}

func (s *socialService) ListReplies(ctx context.Context, parentID domain.PostID) ([]*domain.Post, error) {
	return s.posts.ListReplies(ctx, parentID)
}

func (s *socialService) DeletePost(ctx context.Context, id domain.PostID, callerID string) error {
	post, err := s.posts.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.posts.SoftDelete(ctx, id, callerID); err != nil {
		return err
	}
	// Decrement count only for top-level posts.
	if post.ParentID == nil {
		_ = s.threads.IncrementPostCount(ctx, post.ThreadID, -1)
	}
	return nil
}

func (s *socialService) LikePost(ctx context.Context, postID domain.PostID, userID string) error {
	return s.posts.LikePost(ctx, postID, userID)
}

func (s *socialService) UnlikePost(ctx context.Context, postID domain.PostID, userID string) error {
	return s.posts.UnlikePost(ctx, postID, userID)
}

// ─── Challenges ───────────────────────────────────────────────────────────────

func (s *socialService) CreateChallenge(ctx context.Context, cmd domain.CreateChallengeCmd) (*domain.Challenge, error) {
	if err := validateChallenge(cmd); err != nil {
		return nil, err
	}
	challenge := &domain.Challenge{
		ID:          domain.ChallengeID(uuid.New().String()),
		CreatedBy:   cmd.CreatedBy,
		Type:        cmd.Type,
		Title:       strings.TrimSpace(cmd.Title),
		Description: strings.TrimSpace(cmd.Description),
		MovieIDs:    cmd.MovieIDs,
		TargetGenre: cmd.TargetGenre,
		TargetCount: cmd.TargetCount,
		DeadlineAt:  cmd.DeadlineAt,
		IsPublic:    cmd.IsPublic,
	}
	return s.challenges.Create(ctx, challenge)
}

func (s *socialService) GetChallenge(ctx context.Context, id domain.ChallengeID) (*ChallengeDetail, error) {
	challenge, err := s.challenges.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &ChallengeDetail{Challenge: challenge}, nil
}

func (s *socialService) ListChallenges(ctx context.Context, filter domain.ChallengeFilter) ([]*domain.Challenge, int, error) {
	return s.challenges.List(ctx, filter)
}

func (s *socialService) DeleteChallenge(ctx context.Context, id domain.ChallengeID, callerID string) error {
	return s.challenges.Delete(ctx, id, callerID)
}

func (s *socialService) JoinChallenge(ctx context.Context, challengeID domain.ChallengeID, userID string) error {
	challenge, err := s.challenges.GetByID(ctx, challengeID)
	if err != nil {
		return err
	}
	// Guard: can't join an expired challenge.
	if challenge.DeadlineAt != nil && time.Now().After(*challenge.DeadlineAt) {
		return apierror.Validation("this challenge has already ended", nil)
	}

	participant := &domain.ChallengeParticipant{
		ID:          domain.ParticipantID(uuid.New().String()),
		ChallengeID: challengeID,
		UserID:      userID,
	}
	if err := s.challenges.Join(ctx, participant); err != nil {
		return err
	}
	_ = s.challenges.IncrementParticipantCount(ctx, challengeID, 1)

	_ = s.recordActivity(ctx, userID, domain.ActivityJoinedChallenge, "challenge", string(challengeID), map[string]any{
		"challenge_title": challenge.Title,
	})
	return nil
}

func (s *socialService) LeaveChallenge(ctx context.Context, challengeID domain.ChallengeID, userID string) error {
	if err := s.challenges.Leave(ctx, challengeID, userID); err != nil {
		return err
	}
	_ = s.challenges.IncrementParticipantCount(ctx, challengeID, -1)
	return nil
}

func (s *socialService) MarkMovieWatched(ctx context.Context, challengeID domain.ChallengeID, userID, movieID string) error {
	participant, err := s.challenges.GetParticipant(ctx, challengeID, userID)
	if err != nil {
		return err
	}

	wasCompleted := participant.IsCompleted
	if err := s.challenges.MarkMovieWatched(ctx, challengeID, userID, movieID); err != nil {
		return err
	}

	// Reload to check if the challenge just became complete.
	if !wasCompleted {
		updated, err := s.challenges.GetParticipant(ctx, challengeID, userID)
		if err == nil && updated.IsCompleted {
			challenge, _ := s.challenges.GetByID(ctx, challengeID)
			title := ""
			if challenge != nil {
				title = challenge.Title
			}
			_ = s.recordActivity(ctx, userID, domain.ActivityCompletedChallenge, "challenge", string(challengeID), map[string]any{
				"challenge_title": title,
			})
		}
	}
	return nil
}

func (s *socialService) GetMyProgress(ctx context.Context, challengeID domain.ChallengeID, userID string) (*domain.ChallengeParticipant, error) {
	return s.challenges.GetParticipant(ctx, challengeID, userID)
}

func (s *socialService) ListLeaderboard(ctx context.Context, challengeID domain.ChallengeID, limit, offset int) ([]*domain.ChallengeParticipant, int, error) {
	return s.challenges.ListParticipants(ctx, challengeID, limit, offset)
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (s *socialService) recordActivity(ctx context.Context, actorID string, t domain.ActivityType, subjectType, subjectID string, payload map[string]any) error {
	return s.activities.Record(ctx, &domain.Activity{
		ID:          domain.ActivityID(uuid.New().String()),
		ActorID:     actorID,
		Type:        t,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		Payload:     payload,
	})
}

// ─── Validation ───────────────────────────────────────────────────────────────

func validateThread(cmd domain.CreateThreadCmd) error {
	type fe struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}
	var errs []fe
	if strings.TrimSpace(cmd.Title) == "" {
		errs = append(errs, fe{"title", "thread title is required"})
	}
	if cmd.ScopeID == "" {
		errs = append(errs, fe{"scope_id", "scope_id is required"})
	}
	if len(errs) > 0 {
		return apierror.Validation("thread input is invalid", errs)
	}
	return nil
}

func validateChallenge(cmd domain.CreateChallengeCmd) error {
	type fe struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}
	var errs []fe
	if strings.TrimSpace(cmd.Title) == "" {
		errs = append(errs, fe{"title", "challenge title is required"})
	}
	if cmd.Type == domain.ChallengeTypeWatchList && len(cmd.MovieIDs) == 0 {
		errs = append(errs, fe{"movie_ids", "watch_list challenges require at least one movie"})
	}
	if cmd.Type == domain.ChallengeTypeGenreBlind && cmd.TargetGenre == "" {
		errs = append(errs, fe{"target_genre", "genre_blind challenges require a target genre"})
	}
	if len(errs) > 0 {
		return apierror.Validation("challenge input is invalid", errs)
	}
	return nil
}
