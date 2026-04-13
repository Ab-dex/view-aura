package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ab-dex/view-aura/internal/modules/social/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

type rowScanner interface{ Scan(dest ...any) error }

// ─── Follow repository ────────────────────────────────────────────────────────

type pgFollowRepository struct{ pool *pgxpool.Pool }

func NewFollowRepository(pool *pgxpool.Pool) FollowRepository {
	return &pgFollowRepository{pool: pool}
}

var _ FollowRepository = (*pgFollowRepository)(nil)

func (r *pgFollowRepository) Follow(ctx context.Context, followerID, followeeID string) error {
	if followerID == followeeID {
		return apierror.Validation("users cannot follow themselves", nil)
	}
	const q = `
		INSERT INTO follows (follower_id, followee_id, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (follower_id, followee_id) DO NOTHING`
	_, err := r.pool.Exec(ctx, q, followerID, followeeID)
	return mapErr(err, "follow")
}

func (r *pgFollowRepository) Unfollow(ctx context.Context, followerID, followeeID string) error {
	const q = `DELETE FROM follows WHERE follower_id=$1 AND followee_id=$2`
	_, err := r.pool.Exec(ctx, q, followerID, followeeID)
	return mapErr(err, "unfollow")
}

func (r *pgFollowRepository) IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id=$1 AND followee_id=$2)`
	var ok bool
	if err := r.pool.QueryRow(ctx, q, followerID, followeeID).Scan(&ok); err != nil {
		return false, mapErr(err, "is following")
	}
	return ok, nil
}

func (r *pgFollowRepository) ListFollowers(ctx context.Context, userID string, limit, offset int) ([]string, int, error) {
	return r.listEdge(ctx, "followee_id", "follower_id", userID, limit, offset)
}

func (r *pgFollowRepository) ListFollowing(ctx context.Context, userID string, limit, offset int) ([]string, int, error) {
	return r.listEdge(ctx, "follower_id", "followee_id", userID, limit, offset)
}

func (r *pgFollowRepository) listEdge(ctx context.Context, filterCol, selectCol, userID string, limit, offset int) ([]string, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM follows WHERE %s=$1`, filterCol), userID,
	).Scan(&total); err != nil {
		return nil, 0, mapErr(err, "count follows")
	}

	rows, err := r.pool.Query(ctx,
		fmt.Sprintf(`SELECT %s FROM follows WHERE %s=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, selectCol, filterCol),
		userID, clamp(limit, 100, 20), max0(offset),
	)
	if err != nil {
		return nil, 0, mapErr(err, "list follows")
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, 0, mapErr(err, "scan follow")
		}
		out = append(out, id)
	}
	return out, total, rows.Err()
}

func (r *pgFollowRepository) GetStats(ctx context.Context, userID string) (*domain.FollowStats, error) {
	const q = `
		SELECT
			(SELECT COUNT(*) FROM follows WHERE followee_id=$1) AS followers,
			(SELECT COUNT(*) FROM follows WHERE follower_id=$1) AS following`
	stats := &domain.FollowStats{UserID: userID}
	if err := r.pool.QueryRow(ctx, q, userID).Scan(&stats.FollowerCount, &stats.FollowingCount); err != nil {
		return nil, mapErr(err, "get follow stats")
	}
	return stats, nil
}

// ─── Activity repository ──────────────────────────────────────────────────────

type pgActivityRepository struct{ pool *pgxpool.Pool }

func NewActivityRepository(pool *pgxpool.Pool) ActivityRepository {
	return &pgActivityRepository{pool: pool}
}

var _ ActivityRepository = (*pgActivityRepository)(nil)

func (r *pgActivityRepository) Record(ctx context.Context, a *domain.Activity) error {
	payload, err := json.Marshal(a.Payload)
	if err != nil {
		return apierror.Internal("marshal activity payload", err)
	}
	const q = `
		INSERT INTO activities (id, actor_id, type, subject_type, subject_id, payload, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,NOW())`
	_, err = r.pool.Exec(ctx, q, a.ID, a.ActorID, a.Type, a.SubjectType, a.SubjectID, payload)
	return mapErr(err, "record activity")
}

func (r *pgActivityRepository) GetFeed(ctx context.Context, f domain.FeedFilter) ([]*domain.Activity, int, error) {
	// Fetch activities from the viewer's followees + the viewer themselves.
	const countQ = `
		SELECT COUNT(*) FROM activities
		WHERE actor_id = $1
		   OR actor_id IN (SELECT followee_id FROM follows WHERE follower_id = $1)`
	var total int
	if err := r.pool.QueryRow(ctx, countQ, f.UserID).Scan(&total); err != nil {
		return nil, 0, mapErr(err, "count feed")
	}

	const listQ = `
		SELECT id, actor_id, type, subject_type, subject_id, payload, created_at
		FROM activities
		WHERE actor_id = $1
		   OR actor_id IN (SELECT followee_id FROM follows WHERE follower_id = $1)
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	return r.query(ctx, listQ, f.UserID, clamp(f.Limit, 100, 20), max0(f.Offset), total)
}

func (r *pgActivityRepository) GetByActor(ctx context.Context, f domain.FeedFilter) ([]*domain.Activity, int, error) {
	const countQ = `SELECT COUNT(*) FROM activities WHERE actor_id=$1`
	var total int
	if err := r.pool.QueryRow(ctx, countQ, f.ActorID).Scan(&total); err != nil {
		return nil, 0, mapErr(err, "count actor activities")
	}
	const listQ = `
		SELECT id, actor_id, type, subject_type, subject_id, payload, created_at
		FROM activities WHERE actor_id=$1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	return r.query(ctx, listQ, f.ActorID, clamp(f.Limit, 100, 20), max0(f.Offset), total)
}

func (r *pgActivityRepository) DeleteBySubject(ctx context.Context, subjectType, subjectID string) error {
	const q = `DELETE FROM activities WHERE subject_type=$1 AND subject_id=$2`
	_, err := r.pool.Exec(ctx, q, subjectType, subjectID)
	return mapErr(err, "delete activities by subject")
}

func (r *pgActivityRepository) query(ctx context.Context, q string, args ...any) ([]*domain.Activity, int, error) {
	// last arg is always the pre-computed total — extract it before querying.
	total := args[len(args)-1].(int)
	args = args[:len(args)-1]

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, mapErr(err, "query activities")
	}
	defer rows.Close()

	var out []*domain.Activity
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			return nil, 0, mapErr(err, "scan activity")
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func scanActivity(row rowScanner) (*domain.Activity, error) {
	var a domain.Activity
	var rawPayload []byte
	err := row.Scan(&a.ID, &a.ActorID, &a.Type, &a.SubjectType, &a.SubjectID, &rawPayload, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	if len(rawPayload) > 0 {
		_ = json.Unmarshal(rawPayload, &a.Payload)
	}
	return &a, nil
}

// ─── Thread repository ────────────────────────────────────────────────────────

type pgThreadRepository struct{ pool *pgxpool.Pool }

func NewThreadRepository(pool *pgxpool.Pool) ThreadRepository {
	return &pgThreadRepository{pool: pool}
}

var _ ThreadRepository = (*pgThreadRepository)(nil)

const threadCols = `id, scope, scope_id, title, scene_timestamp_secs, is_spoiler, post_count, created_by, created_at, updated_at`

func (r *pgThreadRepository) Create(ctx context.Context, t *domain.Thread) (*domain.Thread, error) {
	const q = `
		INSERT INTO threads (id, scope, scope_id, title, scene_timestamp_secs, is_spoiler, post_count, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,0,$7,NOW(),NOW())
		RETURNING ` + threadCols
	return scanThread(r.pool.QueryRow(ctx, q, t.ID, t.Scope, t.ScopeID, t.Title, t.SceneTimestampSecs, t.IsSpoiler, t.CreatedBy))
}

func (r *pgThreadRepository) GetByID(ctx context.Context, id domain.ThreadID) (*domain.Thread, error) {
	const q = `SELECT ` + threadCols + ` FROM threads WHERE id=$1`
	t, err := scanThread(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "thread not found")
		}
		return nil, mapErr(err, "get thread")
	}
	return t, nil
}

func (r *pgThreadRepository) List(ctx context.Context, f domain.ThreadFilter) ([]*domain.Thread, int, error) {
	var conds []string
	var args []any
	i := 1
	arg := func(v any) string {
		args = append(args, v)
		s := fmt.Sprintf("$%d", i)
		i++
		return s
	}

	if f.Scope != "" {
		conds = append(conds, fmt.Sprintf("scope = %s", arg(f.Scope)))
	}
	if f.ScopeID != "" {
		conds = append(conds, fmt.Sprintf("scope_id = %s", arg(f.ScopeID)))
	}
	if !f.IncludeSpoilers {
		conds = append(conds, "is_spoiler = false")
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM threads %s", where), args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err, "count threads")
	}

	rows, err := r.pool.Query(ctx,
		fmt.Sprintf("SELECT "+threadCols+" FROM threads %s ORDER BY updated_at DESC LIMIT %s OFFSET %s",
			where, arg(clamp(f.Limit, 100, 20)), arg(max0(f.Offset))),
		args...,
	)
	if err != nil {
		return nil, 0, mapErr(err, "list threads")
	}
	defer rows.Close()

	var out []*domain.Thread
	for rows.Next() {
		t, err := scanThread(rows)
		if err != nil {
			return nil, 0, mapErr(err, "scan thread")
		}
		out = append(out, t)
	}
	return out, total, rows.Err()
}

func (r *pgThreadRepository) Delete(ctx context.Context, id domain.ThreadID, callerID string) error {
	const q = `DELETE FROM threads WHERE id=$1 AND created_by=$2`
	ct, err := r.pool.Exec(ctx, q, id, callerID)
	if err != nil {
		return mapErr(err, "delete thread")
	}
	if ct.RowsAffected() == 0 {
		return apierror.New(404, apierror.CodeNotFound, "thread not found")
	}
	return nil
}

func (r *pgThreadRepository) IncrementPostCount(ctx context.Context, id domain.ThreadID, delta int) error {
	const q = `UPDATE threads SET post_count = GREATEST(0, post_count + $2), updated_at=NOW() WHERE id=$1`
	_, err := r.pool.Exec(ctx, q, id, delta)
	return mapErr(err, "increment post count")
}

func scanThread(row rowScanner) (*domain.Thread, error) {
	var t domain.Thread
	err := row.Scan(&t.ID, &t.Scope, &t.ScopeID, &t.Title,
		&t.SceneTimestampSecs, &t.IsSpoiler, &t.PostCount,
		&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ─── Post repository ──────────────────────────────────────────────────────────

type pgPostRepository struct{ pool *pgxpool.Pool }

func NewPostRepository(pool *pgxpool.Pool) PostRepository {
	return &pgPostRepository{pool: pool}
}

var _ PostRepository = (*pgPostRepository)(nil)

const postCols = `id, thread_id, author_id, body, parent_id, like_count, is_deleted, created_at, updated_at`

func (r *pgPostRepository) Create(ctx context.Context, p *domain.Post) (*domain.Post, error) {
	const q = `
		INSERT INTO posts (id, thread_id, author_id, body, parent_id, like_count, is_deleted, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,0,false,NOW(),NOW())
		RETURNING ` + postCols
	return scanPost(r.pool.QueryRow(ctx, q, p.ID, p.ThreadID, p.AuthorID, p.Body, p.ParentID))
}

func (r *pgPostRepository) GetByID(ctx context.Context, id domain.PostID) (*domain.Post, error) {
	const q = `SELECT ` + postCols + ` FROM posts WHERE id=$1`
	p, err := scanPost(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "post not found")
		}
		return nil, mapErr(err, "get post")
	}
	return p, nil
}

func (r *pgPostRepository) List(ctx context.Context, f domain.PostFilter) ([]*domain.Post, int, error) {
	const countQ = `SELECT COUNT(*) FROM posts WHERE thread_id=$1 AND parent_id IS NULL`
	var total int
	if err := r.pool.QueryRow(ctx, countQ, f.ThreadID).Scan(&total); err != nil {
		return nil, 0, mapErr(err, "count posts")
	}

	const listQ = `SELECT ` + postCols + `
		FROM posts WHERE thread_id=$1 AND parent_id IS NULL
		ORDER BY created_at ASC LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, listQ, f.ThreadID, clamp(f.Limit, 100, 20), max0(f.Offset))
	if err != nil {
		return nil, 0, mapErr(err, "list posts")
	}
	defer rows.Close()

	var out []*domain.Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, 0, mapErr(err, "scan post")
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func (r *pgPostRepository) ListReplies(ctx context.Context, parentID domain.PostID) ([]*domain.Post, error) {
	const q = `SELECT ` + postCols + ` FROM posts WHERE parent_id=$1 ORDER BY created_at ASC`
	rows, err := r.pool.Query(ctx, q, parentID)
	if err != nil {
		return nil, mapErr(err, "list replies")
	}
	defer rows.Close()

	var out []*domain.Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, mapErr(err, "scan reply")
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *pgPostRepository) SoftDelete(ctx context.Context, id domain.PostID, callerID string) error {
	// Preserve the row so reply threads remain intact; body is cleared.
	const q = `
		UPDATE posts SET is_deleted=true, body='[deleted]', updated_at=NOW()
		WHERE id=$1 AND author_id=$2`
	ct, err := r.pool.Exec(ctx, q, id, callerID)
	if err != nil {
		return mapErr(err, "soft delete post")
	}
	if ct.RowsAffected() == 0 {
		return apierror.New(404, apierror.CodeNotFound, "post not found")
	}
	return nil
}

func (r *pgPostRepository) LikePost(ctx context.Context, postID domain.PostID, userID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return mapErr(err, "begin like tx")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insertLike = `
		INSERT INTO post_likes (post_id, user_id, created_at)
		VALUES ($1,$2,NOW())
		ON CONFLICT (post_id, user_id) DO NOTHING`
	ct, err := tx.Exec(ctx, insertLike, postID, userID)
	if err != nil {
		return mapErr(err, "insert post like")
	}
	if ct.RowsAffected() == 0 {
		// Already liked — idempotent.
		return tx.Commit(ctx)
	}

	_, err = tx.Exec(ctx, `UPDATE posts SET like_count = like_count + 1 WHERE id=$1`, postID)
	if err != nil {
		return mapErr(err, "increment like count")
	}
	return mapErr(tx.Commit(ctx), "commit like tx")
}

func (r *pgPostRepository) UnlikePost(ctx context.Context, postID domain.PostID, userID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return mapErr(err, "begin unlike tx")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const deleteLike = `DELETE FROM post_likes WHERE post_id=$1 AND user_id=$2`
	ct, err := tx.Exec(ctx, deleteLike, postID, userID)
	if err != nil {
		return mapErr(err, "delete post like")
	}
	if ct.RowsAffected() == 0 {
		return tx.Commit(ctx) // not liked — idempotent
	}

	_, err = tx.Exec(ctx, `UPDATE posts SET like_count = GREATEST(0, like_count - 1) WHERE id=$1`, postID)
	if err != nil {
		return mapErr(err, "decrement like count")
	}
	return mapErr(tx.Commit(ctx), "commit unlike tx")
}

func (r *pgPostRepository) HasLiked(ctx context.Context, postID domain.PostID, userID string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM post_likes WHERE post_id=$1 AND user_id=$2)`
	var ok bool
	if err := r.pool.QueryRow(ctx, q, postID, userID).Scan(&ok); err != nil {
		return false, mapErr(err, "has liked")
	}
	return ok, nil
}

func scanPost(row rowScanner) (*domain.Post, error) {
	var p domain.Post
	err := row.Scan(&p.ID, &p.ThreadID, &p.AuthorID, &p.Body,
		&p.ParentID, &p.LikeCount, &p.IsDeleted, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ─── Challenge repository ─────────────────────────────────────────────────────

type pgChallengeRepository struct{ pool *pgxpool.Pool }

func NewChallengeRepository(pool *pgxpool.Pool) ChallengeRepository {
	return &pgChallengeRepository{pool: pool}
}

var _ ChallengeRepository = (*pgChallengeRepository)(nil)

const challengeCols = `id, created_by, type, title, description, movie_ids,
	target_genre, target_count, deadline_at, is_public, participant_count, created_at, updated_at`

func (r *pgChallengeRepository) Create(ctx context.Context, c *domain.Challenge) (*domain.Challenge, error) {
	const q = `
		INSERT INTO challenges (id, created_by, type, title, description, movie_ids,
			target_genre, target_count, deadline_at, is_public, participant_count, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,0,NOW(),NOW())
		RETURNING ` + challengeCols
	return scanChallenge(r.pool.QueryRow(ctx, q,
		c.ID, c.CreatedBy, c.Type, c.Title, c.Description, c.MovieIDs,
		c.TargetGenre, c.TargetCount, c.DeadlineAt, c.IsPublic,
	))
}

func (r *pgChallengeRepository) GetByID(ctx context.Context, id domain.ChallengeID) (*domain.Challenge, error) {
	const q = `SELECT ` + challengeCols + ` FROM challenges WHERE id=$1`
	c, err := scanChallenge(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "challenge not found")
		}
		return nil, mapErr(err, "get challenge")
	}
	return c, nil
}

func (r *pgChallengeRepository) List(ctx context.Context, f domain.ChallengeFilter) ([]*domain.Challenge, int, error) {
	var conds []string
	var args []any
	i := 1
	arg := func(v any) string {
		args = append(args, v)
		s := fmt.Sprintf("$%d", i)
		i++
		return s
	}

	if f.CreatedBy != "" {
		conds = append(conds, fmt.Sprintf("created_by = %s", arg(f.CreatedBy)))
	}
	if f.IsPublic != nil {
		conds = append(conds, fmt.Sprintf("is_public = %s", arg(*f.IsPublic)))
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM challenges %s", where), args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err, "count challenges")
	}

	rows, err := r.pool.Query(ctx,
		fmt.Sprintf("SELECT "+challengeCols+" FROM challenges %s ORDER BY created_at DESC LIMIT %s OFFSET %s",
			where, arg(clamp(f.Limit, 100, 20)), arg(max0(f.Offset))),
		args...,
	)
	if err != nil {
		return nil, 0, mapErr(err, "list challenges")
	}
	defer rows.Close()

	var out []*domain.Challenge
	for rows.Next() {
		c, err := scanChallenge(rows)
		if err != nil {
			return nil, 0, mapErr(err, "scan challenge")
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (r *pgChallengeRepository) Delete(ctx context.Context, id domain.ChallengeID, ownerID string) error {
	const q = `DELETE FROM challenges WHERE id=$1 AND created_by=$2`
	ct, err := r.pool.Exec(ctx, q, id, ownerID)
	if err != nil {
		return mapErr(err, "delete challenge")
	}
	if ct.RowsAffected() == 0 {
		return apierror.New(404, apierror.CodeNotFound, "challenge not found")
	}
	return nil
}

const participantCols = `id, challenge_id, user_id, completed_movie_ids, is_completed, joined_at, completed_at`

func (r *pgChallengeRepository) Join(ctx context.Context, p *domain.ChallengeParticipant) error {
	const q = `
		INSERT INTO challenge_participants (id, challenge_id, user_id, completed_movie_ids, is_completed, joined_at)
		VALUES ($1,$2,$3,'{}',false,NOW())
		ON CONFLICT (challenge_id, user_id) DO NOTHING`
	_, err := r.pool.Exec(ctx, q, p.ID, p.ChallengeID, p.UserID)
	return mapErr(err, "join challenge")
}

func (r *pgChallengeRepository) Leave(ctx context.Context, challengeID domain.ChallengeID, userID string) error {
	const q = `DELETE FROM challenge_participants WHERE challenge_id=$1 AND user_id=$2`
	_, err := r.pool.Exec(ctx, q, challengeID, userID)
	return mapErr(err, "leave challenge")
}

func (r *pgChallengeRepository) GetParticipant(ctx context.Context, challengeID domain.ChallengeID, userID string) (*domain.ChallengeParticipant, error) {
	const q = `SELECT ` + participantCols + ` FROM challenge_participants WHERE challenge_id=$1 AND user_id=$2`
	p, err := scanParticipant(r.pool.QueryRow(ctx, q, challengeID, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "not a participant")
		}
		return nil, mapErr(err, "get participant")
	}
	return p, nil
}

func (r *pgChallengeRepository) MarkMovieWatched(ctx context.Context, challengeID domain.ChallengeID, userID, movieID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return mapErr(err, "begin mark tx")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Append movieID to the completed set (array_append is idempotent with
	// the NOT EXISTS guard; duplicates prevented at application level).
	const q = `
		UPDATE challenge_participants
		SET completed_movie_ids = array_append(completed_movie_ids, $3)
		WHERE challenge_id=$1 AND user_id=$2
		  AND NOT ($3 = ANY(completed_movie_ids))`
	if _, err := tx.Exec(ctx, q, challengeID, userID, movieID); err != nil {
		return mapErr(err, "mark movie watched")
	}

	// Check if all required movies are now done and flip is_completed.
	const checkQ = `
		UPDATE challenge_participants cp
		SET is_completed=true, completed_at=NOW()
		FROM challenges ch
		WHERE cp.challenge_id = ch.id
		  AND cp.challenge_id=$1 AND cp.user_id=$2
		  AND ch.movie_ids <@ cp.completed_movie_ids
		  AND cp.is_completed = false`
	if _, err := tx.Exec(ctx, checkQ, challengeID, userID); err != nil {
		return mapErr(err, "check completion")
	}

	return mapErr(tx.Commit(ctx), "commit mark tx")
}

func (r *pgChallengeRepository) ListParticipants(ctx context.Context, challengeID domain.ChallengeID, limit, offset int) ([]*domain.ChallengeParticipant, int, error) {
	const countQ = `SELECT COUNT(*) FROM challenge_participants WHERE challenge_id=$1`
	var total int
	if err := r.pool.QueryRow(ctx, countQ, challengeID).Scan(&total); err != nil {
		return nil, 0, mapErr(err, "count participants")
	}

	const listQ = `SELECT ` + participantCols + `
		FROM challenge_participants WHERE challenge_id=$1
		ORDER BY is_completed DESC, joined_at ASC LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, listQ, challengeID, clamp(limit, 100, 20), max0(offset))
	if err != nil {
		return nil, 0, mapErr(err, "list participants")
	}
	defer rows.Close()

	var out []*domain.ChallengeParticipant
	for rows.Next() {
		p, err := scanParticipant(rows)
		if err != nil {
			return nil, 0, mapErr(err, "scan participant")
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func (r *pgChallengeRepository) IncrementParticipantCount(ctx context.Context, challengeID domain.ChallengeID, delta int) error {
	const q = `UPDATE challenges SET participant_count = GREATEST(0, participant_count + $2), updated_at=NOW() WHERE id=$1`
	_, err := r.pool.Exec(ctx, q, challengeID, delta)
	return mapErr(err, "increment participant count")
}

func scanChallenge(row rowScanner) (*domain.Challenge, error) {
	var c domain.Challenge
	err := row.Scan(&c.ID, &c.CreatedBy, &c.Type, &c.Title, &c.Description, &c.MovieIDs,
		&c.TargetGenre, &c.TargetCount, &c.DeadlineAt, &c.IsPublic, &c.ParticipantCount,
		&c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func scanParticipant(row rowScanner) (*domain.ChallengeParticipant, error) {
	var p domain.ChallengeParticipant
	err := row.Scan(&p.ID, &p.ChallengeID, &p.UserID, &p.CompletedMovieIDs,
		&p.IsCompleted, &p.JoinedAt, &p.CompletedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ─── Shared helpers ───────────────────

func mapErr(err error, op string) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return apierror.Conflict(fmt.Sprintf("%s: %s", op, pgErr.Detail)).WithCause(err)
	}
	return apierror.DatabaseError(fmt.Errorf("%s: %w", op, err))
}

func clamp(v, max, def int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}
