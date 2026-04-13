package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/social/domain"
)

// FollowRepository manages the directed follow graph.
type FollowRepository interface {
	// Follow creates a follower → followee edge. Idempotent.
	Follow(ctx context.Context, followerID, followeeID string) error
	// Unfollow removes the edge. Idempotent.
	Unfollow(ctx context.Context, followerID, followeeID string) error
	// IsFollowing returns true if followerID follows followeeID.
	IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error)
	// ListFollowers returns users who follow userID, paginated.
	ListFollowers(ctx context.Context, userID string, limit, offset int) ([]string, int, error)
	// ListFollowing returns users that userID follows, paginated.
	ListFollowing(ctx context.Context, userID string, limit, offset int) ([]string, int, error)
	// GetStats returns follower/following counts for a user.
	GetStats(ctx context.Context, userID string) (*domain.FollowStats, error)
}

// ActivityRepository manages the activity feed.
type ActivityRepository interface {
	// Record persists a new activity event.
	Record(ctx context.Context, activity *domain.Activity) error
	// GetFeed returns a paginated feed of activities from users the viewer follows,
	// plus the viewer's own activities, ordered newest-first.
	GetFeed(ctx context.Context, filter domain.FeedFilter) ([]*domain.Activity, int, error)
	// GetByActor returns a single user's public activity stream.
	GetByActor(ctx context.Context, filter domain.FeedFilter) ([]*domain.Activity, int, error)
	// DeleteBySubject removes all activities related to a subject (e.g. when a
	// review is deleted, its "reviewed" activity should also disappear).
	DeleteBySubject(ctx context.Context, subjectType, subjectID string) error
}

// ThreadRepository manages discussion threads.
type ThreadRepository interface {
	Create(ctx context.Context, thread *domain.Thread) (*domain.Thread, error)
	GetByID(ctx context.Context, id domain.ThreadID) (*domain.Thread, error)
	List(ctx context.Context, filter domain.ThreadFilter) ([]*domain.Thread, int, error)
	Delete(ctx context.Context, id domain.ThreadID, callerID string) error
	// IncrementPostCount updates the denormalized post_count by delta.
	IncrementPostCount(ctx context.Context, id domain.ThreadID, delta int) error
}

// PostRepository manages posts inside threads.
type PostRepository interface {
	Create(ctx context.Context, post *domain.Post) (*domain.Post, error)
	GetByID(ctx context.Context, id domain.PostID) (*domain.Post, error)
	// List returns top-level posts in a thread (ParentID IS NULL), paginated.
	List(ctx context.Context, filter domain.PostFilter) ([]*domain.Post, int, error)
	// ListReplies returns direct replies to a parent post.
	ListReplies(ctx context.Context, parentID domain.PostID) ([]*domain.Post, error)
	// SoftDelete marks the post deleted without removing the row so reply trees
	// remain intact.
	SoftDelete(ctx context.Context, id domain.PostID, callerID string) error
	// LikePost records a user liking a post. Idempotent.
	LikePost(ctx context.Context, postID domain.PostID, userID string) error
	// UnlikePost removes the like. Idempotent.
	UnlikePost(ctx context.Context, postID domain.PostID, userID string) error
	// HasLiked returns true if userID has liked postID.
	HasLiked(ctx context.Context, postID domain.PostID, userID string) (bool, error)
}

// ChallengeRepository manages challenges and participant progress.
type ChallengeRepository interface {
	Create(ctx context.Context, challenge *domain.Challenge) (*domain.Challenge, error)
	GetByID(ctx context.Context, id domain.ChallengeID) (*domain.Challenge, error)
	List(ctx context.Context, filter domain.ChallengeFilter) ([]*domain.Challenge, int, error)
	Delete(ctx context.Context, id domain.ChallengeID, ownerID string) error

	// Join adds the caller as a participant.
	Join(ctx context.Context, participant *domain.ChallengeParticipant) error
	// Leave removes the caller from the challenge.
	Leave(ctx context.Context, challengeID domain.ChallengeID, userID string) error
	// GetParticipant fetches the caller's progress record.
	GetParticipant(ctx context.Context, challengeID domain.ChallengeID, userID string) (*domain.ChallengeParticipant, error)
	// MarkMovieWatched adds a movie to the participant's completed set and sets
	// IsCompleted = true if all required movies are done.
	MarkMovieWatched(ctx context.Context, challengeID domain.ChallengeID, userID, movieID string) error
	// ListParticipants returns all participants for a challenge (leaderboard).
	ListParticipants(ctx context.Context, challengeID domain.ChallengeID, limit, offset int) ([]*domain.ChallengeParticipant, int, error)
	// IncrementParticipantCount updates the denormalized count by delta.
	IncrementParticipantCount(ctx context.Context, challengeID domain.ChallengeID, delta int) error
}
