package domain

import "time"

// ─── IDs ──────────────────────────────────────────────────────────────────────

type ActivityID string
type ThreadID string
type PostID string
type ChallengeID string
type ParticipantID string

func (id ActivityID) String() string  { return string(id) }
func (id ThreadID) String() string    { return string(id) }
func (id PostID) String() string      { return string(id) }
func (id ChallengeID) String() string { return string(id) }

// ─── Follow graph ─────────────────────────────────────────────────────────────

// Follow records a directed follow relationship: Follower follows Followee.
type Follow struct {
	FollowerID string
	FolloweeID string
	CreatedAt  time.Time
}

// FollowStats holds the counts shown on a user profile.
type FollowStats struct {
	UserID         string
	FollowerCount  int
	FollowingCount int
}

// ─── Activity feed ────────────────────────────────────────────────────────────

// ActivityType classifies what happened so the feed renderer can pick the
// right template ("Sarah rated…", "Alex added to list…").
type ActivityType string

const (
	ActivityRated              ActivityType = "rated"         // user rated a movie
	ActivityReviewed           ActivityType = "reviewed"      // user wrote a review
	ActivityWatched            ActivityType = "watched"       // status → watched
	ActivityAddedToList        ActivityType = "added_to_list" // movie added to custom list
	ActivityFollowed           ActivityType = "followed"      // user followed another user
	ActivityJoinedChallenge    ActivityType = "joined_challenge"
	ActivityCompletedChallenge ActivityType = "completed_challenge"
)

// Activity is a single event in a user's public activity stream.
// SubjectID + SubjectType point at the entity that was acted upon
// (movie, list, user, challenge).
type Activity struct {
	ID          ActivityID
	ActorID     string // the user who performed the action
	Type        ActivityType
	SubjectType string // "movie", "list", "user", "challenge"
	SubjectID   string
	// Payload holds a small denormalized JSON blob rendered in the feed
	// ("Dune: Part Two", "★★★★★", list title, etc.) — avoids join queries
	// on the hot feed read path.
	Payload   map[string]any
	CreatedAt time.Time
}

// FeedPage is the paginated result returned to the client.
type FeedPage struct {
	Activities []*Activity
	Total      int
	Limit      int
	Offset     int
}

// ─── Discussion threads ───────────────────────────────────────────────────────

// ThreadScope classifies what a thread is attached to.
type ThreadScope string

const (
	ThreadScopeMovie ThreadScope = "movie" // per-movie discussion
	ThreadScopeScene ThreadScope = "scene" // per-scene (timestamped, spoiler)
	ThreadScopeGenre ThreadScope = "genre" // genre chat room
)

// Thread is the top-level discussion container.
type Thread struct {
	ID      ThreadID
	Scope   ThreadScope
	ScopeID string // movie ID, genre slug, etc.
	Title   string
	// SceneTimestampSecs is set only for ThreadScopeScene threads.
	SceneTimestampSecs *int
	IsSpoiler          bool
	PostCount          int // denormalized
	CreatedBy          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Post is a single message inside a Thread.
type Post struct {
	ID       PostID
	ThreadID ThreadID
	AuthorID string
	Body     string
	// ParentID is set when this post is a reply to another post.
	ParentID  *PostID
	LikeCount int
	IsDeleted bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PostLike records a user liking a post (one per user/post pair).
type PostLike struct {
	PostID    PostID
	UserID    string
	CreatedAt time.Time
}

// ─── Challenges ───────────────────────────────────────────────────────────────

// ChallengeType classifies the challenge goal.
type ChallengeType string

const (
	ChallengeTypeWatchList  ChallengeType = "watch_list"  // watch a specific set of movies
	ChallengeTypeGenreBlind ChallengeType = "genre_blind" // watch N movies from a genre user hasn't explored
	ChallengeTypeCustom     ChallengeType = "custom"      // any user-defined criteria
)

// Challenge is a timed, goal-based viewing challenge.
type Challenge struct {
	ID          ChallengeID
	CreatedBy   string
	Type        ChallengeType
	Title       string
	Description string
	// MovieIDs is the list of required films for watch_list challenges.
	MovieIDs []string
	// TargetGenre and TargetCount are used for genre_blind challenges.
	TargetGenre      string
	TargetCount      int
	DeadlineAt       *time.Time
	IsPublic         bool
	ParticipantCount int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ChallengeParticipant tracks a user's progress in a challenge.
type ChallengeParticipant struct {
	ID          ParticipantID
	ChallengeID ChallengeID
	UserID      string
	// CompletedMovieIDs is the subset of required movies the user has watched.
	CompletedMovieIDs []string
	IsCompleted       bool
	JoinedAt          time.Time
	CompletedAt       *time.Time
}

// ─── Commands ─────────────────────────────────────────────────────────────────

type CreateThreadCmd struct {
	Scope              ThreadScope
	ScopeID            string
	Title              string
	SceneTimestampSecs *int
	IsSpoiler          bool
	CreatedBy          string
}

type CreatePostCmd struct {
	ThreadID ThreadID
	AuthorID string
	Body     string
	ParentID *PostID
}

type CreateChallengeCmd struct {
	CreatedBy   string
	Type        ChallengeType
	Title       string
	Description string
	MovieIDs    []string
	TargetGenre string
	TargetCount int
	DeadlineAt  *time.Time
	IsPublic    bool
}

// ─── Filter types ─────────────────────────────────────────────────────────────

type FeedFilter struct {
	// UserID is whose feed to load (their own activity + followees').
	UserID string
	// ActorID filters to a single user's activity stream (profile page).
	ActorID string
	Limit   int
	Offset  int
}

type ThreadFilter struct {
	Scope           ThreadScope
	ScopeID         string
	IncludeSpoilers bool
	Limit           int
	Offset          int
}

type PostFilter struct {
	ThreadID ThreadID
	Limit    int
	Offset   int
}

type ChallengeFilter struct {
	CreatedBy string
	IsPublic  *bool
	Limit     int
	Offset    int
}
