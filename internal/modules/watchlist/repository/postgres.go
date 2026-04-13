package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ab-dex/view-aura/internal/modules/watchlist/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

type rowScanner interface{ Scan(dest ...any) error }

// ─── Watchlist entry repository ───────────────────────────────────────────────

type pgWatchlistRepository struct{ pool *pgxpool.Pool }

func NewWatchlistRepository(pool *pgxpool.Pool) WatchlistRepository {
	return &pgWatchlistRepository{pool: pool}
}

var _ WatchlistRepository = (*pgWatchlistRepository)(nil)

const entryCols = `id, user_id, movie_id, status, priority, created_at, updated_at`

func (r *pgWatchlistRepository) Upsert(ctx context.Context, e *domain.WatchlistEntry) (*domain.WatchlistEntry, error) {
	const q = `
		INSERT INTO watchlist_entries (id, user_id, movie_id, status, priority, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,NOW(),NOW())
		ON CONFLICT (user_id, movie_id) DO UPDATE SET
			status     = EXCLUDED.status,
			priority   = EXCLUDED.priority,
			updated_at = NOW()
		RETURNING ` + entryCols

	row := r.pool.QueryRow(ctx, q, e.ID, e.UserID, e.MovieID, e.Status, e.Priority)
	return scanEntry(row)
}

func (r *pgWatchlistRepository) GetByUserAndMovie(ctx context.Context, userID, movieID string) (*domain.WatchlistEntry, error) {
	const q = `SELECT ` + entryCols + ` FROM watchlist_entries WHERE user_id=$1 AND movie_id=$2`
	entry, err := scanEntry(r.pool.QueryRow(ctx, q, userID, movieID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapErr(err, "get watchlist entry")
	}
	return entry, nil
}

func (r *pgWatchlistRepository) Delete(ctx context.Context, userID, movieID string) error {
	const q = `DELETE FROM watchlist_entries WHERE user_id=$1 AND movie_id=$2`
	ct, err := r.pool.Exec(ctx, q, userID, movieID)
	if err != nil {
		return mapErr(err, "delete watchlist entry")
	}
	if ct.RowsAffected() == 0 {
		return apierror.New(404, apierror.CodeNotFound, "watchlist entry not found")
	}
	return nil
}

func (r *pgWatchlistRepository) List(ctx context.Context, f domain.EntryFilter) ([]*domain.WatchlistEntry, int, error) {
	var conds []string
	var args []any
	i := 1
	arg := func(v any) string {
		args = append(args, v)
		s := fmt.Sprintf("$%d", i)
		i++
		return s
	}

	conds = append(conds, fmt.Sprintf("user_id = %s", arg(f.UserID)))
	if f.Status != nil {
		conds = append(conds, fmt.Sprintf("status = %s", arg(*f.Status)))
	}

	where := "WHERE " + strings.Join(conds, " AND ")
	limit := clamp(f.Limit, 1, 100, 20)
	offset := max0(f.Offset)

	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM watchlist_entries %s", where), args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err, "count watchlist entries")
	}

	rows, err := r.pool.Query(ctx,
		fmt.Sprintf("SELECT "+entryCols+" FROM watchlist_entries %s ORDER BY updated_at DESC LIMIT %s OFFSET %s",
			where, arg(limit), arg(offset)),
		args...,
	)
	if err != nil {
		return nil, 0, mapErr(err, "list watchlist entries")
	}
	defer rows.Close()

	var out []*domain.WatchlistEntry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, 0, mapErr(err, "scan watchlist entry")
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func (r *pgWatchlistRepository) CountByStatus(ctx context.Context, userID string) (map[domain.WatchStatus]int, error) {
	const q = `SELECT status, COUNT(*) FROM watchlist_entries WHERE user_id=$1 GROUP BY status`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, mapErr(err, "count by status")
	}
	defer rows.Close()

	out := make(map[domain.WatchStatus]int)
	for rows.Next() {
		var status domain.WatchStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, mapErr(err, "scan status count")
		}
		out[status] = count
	}
	return out, rows.Err()
}

func scanEntry(row rowScanner) (*domain.WatchlistEntry, error) {
	var e domain.WatchlistEntry
	if err := row.Scan(&e.ID, &e.UserID, &e.MovieID, &e.Status, &e.Priority, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return nil, err
	}
	return &e, nil
}

// ─── List repository ──────────────────────────────────────────────────────────

type pgListRepository struct{ pool *pgxpool.Pool }

func NewListRepository(pool *pgxpool.Pool) ListRepository {
	return &pgListRepository{pool: pool}
}

var _ ListRepository = (*pgListRepository)(nil)

const listCols = `id, owner_id, title, description, visibility, cover_url, share_slug, item_count, created_at, updated_at`

func (r *pgListRepository) Create(ctx context.Context, l *domain.List) (*domain.List, error) {
	const q = `
		INSERT INTO lists (id, owner_id, title, description, visibility, cover_url, share_slug, item_count, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,0,NOW(),NOW())
		RETURNING ` + listCols

	row := r.pool.QueryRow(ctx, q, l.ID, l.OwnerID, l.Title, l.Description, l.Visibility, l.CoverURL, l.ShareSlug)
	return scanList(row)
}

func (r *pgListRepository) GetByID(ctx context.Context, id domain.ListID) (*domain.List, error) {
	const q = `SELECT ` + listCols + ` FROM lists WHERE id=$1`
	l, err := scanList(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "list not found")
		}
		return nil, mapErr(err, "get list by id")
	}
	return l, nil
}

func (r *pgListRepository) GetByShareSlug(ctx context.Context, slug string) (*domain.List, error) {
	const q = `SELECT ` + listCols + ` FROM lists WHERE share_slug=$1`
	l, err := scanList(r.pool.QueryRow(ctx, q, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "list not found")
		}
		return nil, mapErr(err, "get list by slug")
	}
	return l, nil
}

func (r *pgListRepository) Update(ctx context.Context, l *domain.List) (*domain.List, error) {
	const q = `
		UPDATE lists SET title=$2, description=$3, visibility=$4, cover_url=$5, updated_at=NOW()
		WHERE id=$1 AND owner_id=$6
		RETURNING ` + listCols

	result, err := scanList(r.pool.QueryRow(ctx, q, l.ID, l.Title, l.Description, l.Visibility, l.CoverURL, l.OwnerID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "list not found")
		}
		return nil, mapErr(err, "update list")
	}
	return result, nil
}

func (r *pgListRepository) Delete(ctx context.Context, id domain.ListID, ownerID string) error {
	const q = `DELETE FROM lists WHERE id=$1 AND owner_id=$2`
	ct, err := r.pool.Exec(ctx, q, id, ownerID)
	if err != nil {
		return mapErr(err, "delete list")
	}
	if ct.RowsAffected() == 0 {
		return apierror.New(404, apierror.CodeNotFound, "list not found")
	}
	return nil
}

func (r *pgListRepository) ListByOwner(ctx context.Context, f domain.ListFilter) ([]*domain.List, int, error) {
	var conds []string
	var args []any
	i := 1
	arg := func(v any) string {
		args = append(args, v)
		s := fmt.Sprintf("$%d", i)
		i++
		return s
	}

	conds = append(conds, fmt.Sprintf("owner_id = %s", arg(f.OwnerID)))
	if f.Visibility != nil {
		conds = append(conds, fmt.Sprintf("visibility = %s", arg(*f.Visibility)))
	}

	where := "WHERE " + strings.Join(conds, " AND ")
	limit := clamp(f.Limit, 1, 100, 20)
	offset := max0(f.Offset)

	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM lists %s", where), args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err, "count lists")
	}

	rows, err := r.pool.Query(ctx,
		fmt.Sprintf("SELECT "+listCols+" FROM lists %s ORDER BY updated_at DESC LIMIT %s OFFSET %s",
			where, arg(limit), arg(offset)),
		args...,
	)
	if err != nil {
		return nil, 0, mapErr(err, "list by owner")
	}
	defer rows.Close()

	var out []*domain.List
	for rows.Next() {
		l, err := scanList(rows)
		if err != nil {
			return nil, 0, mapErr(err, "scan list")
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}

// ─── List items ───────────────────────────────────────────────────────────────

const itemCols = `id, list_id, movie_id, note, sort_order, added_at`

func (r *pgListRepository) AddItem(ctx context.Context, item *domain.ListItem) (*domain.ListItem, error) {
	const q = `
		INSERT INTO list_items (id, list_id, movie_id, note, sort_order, added_at)
		VALUES ($1,$2,$3,$4,$5,NOW())
		ON CONFLICT (list_id, movie_id) DO NOTHING
		RETURNING ` + itemCols

	result, err := scanItem(r.pool.QueryRow(ctx, q, item.ID, item.ListID, item.MovieID, item.Note, item.SortOrder))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// ON CONFLICT DO NOTHING fired — movie already in list.
			return r.GetItem(ctx, item.ListID, item.MovieID)
		}
		return nil, mapErr(err, "add list item")
	}
	return result, nil
}

func (r *pgListRepository) GetItem(ctx context.Context, listID domain.ListID, movieID string) (*domain.ListItem, error) {
	const q = `SELECT ` + itemCols + ` FROM list_items WHERE list_id=$1 AND movie_id=$2`
	item, err := scanItem(r.pool.QueryRow(ctx, q, listID, movieID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "item not found in list")
		}
		return nil, mapErr(err, "get list item")
	}
	return item, nil
}

func (r *pgListRepository) UpdateItem(ctx context.Context, item *domain.ListItem) (*domain.ListItem, error) {
	const q = `
		UPDATE list_items SET note=$3, sort_order=$4
		WHERE list_id=$1 AND movie_id=$2
		RETURNING ` + itemCols

	result, err := scanItem(r.pool.QueryRow(ctx, q, item.ListID, item.MovieID, item.Note, item.SortOrder))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "item not found in list")
		}
		return nil, mapErr(err, "update list item")
	}
	return result, nil
}

func (r *pgListRepository) RemoveItem(ctx context.Context, listID domain.ListID, movieID string) error {
	const q = `DELETE FROM list_items WHERE list_id=$1 AND movie_id=$2`
	ct, err := r.pool.Exec(ctx, q, listID, movieID)
	if err != nil {
		return mapErr(err, "remove list item")
	}
	if ct.RowsAffected() == 0 {
		return apierror.New(404, apierror.CodeNotFound, "item not found in list")
	}
	return nil
}

func (r *pgListRepository) ListItems(ctx context.Context, listID domain.ListID) ([]*domain.ListItem, error) {
	const q = `SELECT ` + itemCols + ` FROM list_items WHERE list_id=$1 ORDER BY sort_order ASC, added_at ASC`
	rows, err := r.pool.Query(ctx, q, listID)
	if err != nil {
		return nil, mapErr(err, "list items")
	}
	defer rows.Close()

	var out []*domain.ListItem
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, mapErr(err, "scan list item")
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *pgListRepository) ReorderItems(ctx context.Context, listID domain.ListID, orderedMovieIDs []string) error {
	// Execute one UPDATE per position inside a transaction.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return mapErr(err, "begin reorder tx")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for i, movieID := range orderedMovieIDs {
		if _, err := tx.Exec(ctx,
			`UPDATE list_items SET sort_order=$3 WHERE list_id=$1 AND movie_id=$2`,
			listID, movieID, i,
		); err != nil {
			return mapErr(err, "reorder item")
		}
	}
	return mapErr(tx.Commit(ctx), "commit reorder tx")
}

func (r *pgListRepository) IncrementItemCount(ctx context.Context, listID domain.ListID, delta int) error {
	const q = `UPDATE lists SET item_count = GREATEST(0, item_count + $2), updated_at=NOW() WHERE id=$1`
	_, err := r.pool.Exec(ctx, q, listID, delta)
	return mapErr(err, "increment item count")
}

// ─── Collaborators ────────────────────────────────────────────────────────────

func (r *pgListRepository) AddCollaborator(ctx context.Context, c *domain.ListCollaborator) error {
	const q = `
		INSERT INTO list_collaborators (list_id, user_id, added_at)
		VALUES ($1,$2,NOW())
		ON CONFLICT (list_id, user_id) DO NOTHING`
	_, err := r.pool.Exec(ctx, q, c.ListID, c.UserID)
	return mapErr(err, "add collaborator")
}

func (r *pgListRepository) RemoveCollaborator(ctx context.Context, listID domain.ListID, userID string) error {
	const q = `DELETE FROM list_collaborators WHERE list_id=$1 AND user_id=$2`
	_, err := r.pool.Exec(ctx, q, listID, userID)
	return mapErr(err, "remove collaborator")
}

func (r *pgListRepository) IsCollaborator(ctx context.Context, listID domain.ListID, userID string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM list_collaborators WHERE list_id=$1 AND user_id=$2)`
	var ok bool
	if err := r.pool.QueryRow(ctx, q, listID, userID).Scan(&ok); err != nil {
		return false, mapErr(err, "is collaborator")
	}
	return ok, nil
}

// ─── Scanners ─────────────────────────────────────────────────────────────────

func scanList(row rowScanner) (*domain.List, error) {
	var l domain.List
	err := row.Scan(&l.ID, &l.OwnerID, &l.Title, &l.Description, &l.Visibility,
		&l.CoverURL, &l.ShareSlug, &l.ItemCount, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func scanItem(row rowScanner) (*domain.ListItem, error) {
	var item domain.ListItem
	if err := row.Scan(&item.ID, &item.ListID, &item.MovieID, &item.Note, &item.SortOrder, &item.AddedAt); err != nil {
		return nil, err
	}
	return &item, nil
}

// ─── Error mapper ─────────────────────────────────────────────────────────────

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

// ─── Small helpers ────────────────────────────────────────────────────────────

func clamp(v, min, max, def int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	if v < min {
		return min
	}
	return v
}

func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}
