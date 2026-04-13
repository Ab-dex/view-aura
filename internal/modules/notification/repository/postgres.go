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

	"github.com/Ab-dex/view-aura/internal/modules/notification/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

type rowScanner interface{ Scan(dest ...any) error }

// ─── Notification repository ──────────────────────────────────────────────────

type pgNotificationRepository struct{ pool *pgxpool.Pool }

func NewNotificationRepository(pool *pgxpool.Pool) NotificationRepository {
	return &pgNotificationRepository{pool: pool}
}

var _ NotificationRepository = (*pgNotificationRepository)(nil)

const notifCols = `id, user_id, type, title, body, payload,
	reference_type, reference_id, is_read, created_at, read_at`

func (r *pgNotificationRepository) Create(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
	payload, err := json.Marshal(n.Payload)
	if err != nil {
		return nil, apierror.Internal("marshal notification payload", err)
	}
	const q = `
		INSERT INTO notifications (id, user_id, type, title, body, payload,
			reference_type, reference_id, is_read, created_at, read_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,false,NOW(),NULL)
		RETURNING ` + notifCols

	return scanNotif(r.pool.QueryRow(ctx, q,
		n.ID, n.UserID, n.Type, n.Title, n.Body, payload,
		n.ReferenceType, n.ReferenceID,
	))
}

func (r *pgNotificationRepository) GetByID(ctx context.Context, id domain.NotificationID) (*domain.Notification, error) {
	const q = `SELECT ` + notifCols + ` FROM notifications WHERE id=$1`
	n, err := scanNotif(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "notification not found")
		}
		return nil, mapErr(err, "get notification")
	}
	return n, nil
}

func (r *pgNotificationRepository) List(ctx context.Context, f domain.NotificationFilter) ([]*domain.Notification, int, error) {
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
	if f.IsRead != nil {
		conds = append(conds, fmt.Sprintf("is_read = %s", arg(*f.IsRead)))
	}

	where := "WHERE " + strings.Join(conds, " AND ")
	limit := clamp(f.Limit, 100, 20)
	offset := max0(f.Offset)

	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM notifications %s", where), args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err, "count notifications")
	}

	rows, err := r.pool.Query(ctx,
		fmt.Sprintf("SELECT "+notifCols+" FROM notifications %s ORDER BY created_at DESC LIMIT %s OFFSET %s",
			where, arg(limit), arg(offset)),
		args...,
	)
	if err != nil {
		return nil, 0, mapErr(err, "list notifications")
	}
	defer rows.Close()

	var out []*domain.Notification
	for rows.Next() {
		n, err := scanNotif(rows)
		if err != nil {
			return nil, 0, mapErr(err, "scan notification")
		}
		out = append(out, n)
	}
	return out, total, rows.Err()
}

func (r *pgNotificationRepository) MarkRead(ctx context.Context, id domain.NotificationID, userID string) error {
	const q = `
		UPDATE notifications SET is_read=true, read_at=NOW()
		WHERE id=$1 AND user_id=$2 AND is_read=false`
	_, err := r.pool.Exec(ctx, q, id, userID)
	return mapErr(err, "mark read")
}

func (r *pgNotificationRepository) MarkAllRead(ctx context.Context, userID string) error {
	const q = `UPDATE notifications SET is_read=true, read_at=NOW() WHERE user_id=$1 AND is_read=false`
	_, err := r.pool.Exec(ctx, q, userID)
	return mapErr(err, "mark all read")
}

func (r *pgNotificationRepository) Delete(ctx context.Context, id domain.NotificationID, userID string) error {
	const q = `DELETE FROM notifications WHERE id=$1 AND user_id=$2`
	ct, err := r.pool.Exec(ctx, q, id, userID)
	if err != nil {
		return mapErr(err, "delete notification")
	}
	if ct.RowsAffected() == 0 {
		return apierror.New(404, apierror.CodeNotFound, "notification not found")
	}
	return nil
}

func (r *pgNotificationRepository) UnreadCount(ctx context.Context, userID string) (int, error) {
	const q = `SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND is_read=false`
	var count int
	if err := r.pool.QueryRow(ctx, q, userID).Scan(&count); err != nil {
		return 0, mapErr(err, "unread count")
	}
	return count, nil
}

func scanNotif(row rowScanner) (*domain.Notification, error) {
	var n domain.Notification
	var rawPayload []byte
	err := row.Scan(
		&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &rawPayload,
		&n.ReferenceType, &n.ReferenceID, &n.IsRead, &n.CreatedAt, &n.ReadAt,
	)
	if err != nil {
		return nil, err
	}
	if len(rawPayload) > 0 {
		_ = json.Unmarshal(rawPayload, &n.Payload)
	}
	return &n, nil
}

// ─── Preference repository ────────────────────────────────────────────────────

type pgPreferenceRepository struct{ pool *pgxpool.Pool }

func NewPreferenceRepository(pool *pgxpool.Pool) PreferenceRepository {
	return &pgPreferenceRepository{pool: pool}
}

var _ PreferenceRepository = (*pgPreferenceRepository)(nil)

func (r *pgPreferenceRepository) Upsert(ctx context.Context, p *domain.NotificationPreference) error {
	const q = `
		INSERT INTO notification_preferences (user_id, type, channel, enabled, updated_at)
		VALUES ($1,$2,$3,$4,NOW())
		ON CONFLICT (user_id, type, channel) DO UPDATE SET
			enabled    = EXCLUDED.enabled,
			updated_at = NOW()`
	_, err := r.pool.Exec(ctx, q, p.UserID, p.Type, p.Channel, p.Enabled)
	return mapErr(err, "upsert preference")
}

func (r *pgPreferenceRepository) GetAll(ctx context.Context, userID string) ([]*domain.NotificationPreference, error) {
	const q = `SELECT user_id, type, channel, enabled, updated_at
		FROM notification_preferences WHERE user_id=$1`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, mapErr(err, "get all preferences")
	}
	defer rows.Close()

	var out []*domain.NotificationPreference
	for rows.Next() {
		var p domain.NotificationPreference
		if err := rows.Scan(&p.UserID, &p.Type, &p.Channel, &p.Enabled, &p.UpdatedAt); err != nil {
			return nil, mapErr(err, "scan preference")
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

func (r *pgPreferenceRepository) IsEnabled(ctx context.Context, userID string, t domain.NotificationType, ch domain.Channel) (bool, error) {
	const q = `SELECT enabled FROM notification_preferences WHERE user_id=$1 AND type=$2 AND channel=$3`
	var enabled bool
	err := r.pool.QueryRow(ctx, q, userID, t, ch).Scan(&enabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil // opt-out model: missing row = enabled
		}
		return false, mapErr(err, "is enabled")
	}
	return enabled, nil
}

// ─── Push token repository ────────────────────────────────────────────────────

type pgPushTokenRepository struct{ pool *pgxpool.Pool }

func NewPushTokenRepository(pool *pgxpool.Pool) PushTokenRepository {
	return &pgPushTokenRepository{pool: pool}
}

var _ PushTokenRepository = (*pgPushTokenRepository)(nil)

func (r *pgPushTokenRepository) Upsert(ctx context.Context, t *domain.PushToken) error {
	const q = `
		INSERT INTO push_tokens (id, user_id, token, platform, created_at, updated_at)
		VALUES ($1,$2,$3,$4,NOW(),NOW())
		ON CONFLICT (user_id, token) DO UPDATE SET
			platform   = EXCLUDED.platform,
			updated_at = NOW()`
	_, err := r.pool.Exec(ctx, q, t.ID, t.UserID, t.Token, t.Platform)
	return mapErr(err, "upsert push token")
}

func (r *pgPushTokenRepository) Delete(ctx context.Context, userID, token string) error {
	const q = `DELETE FROM push_tokens WHERE user_id=$1 AND token=$2`
	_, err := r.pool.Exec(ctx, q, userID, token)
	return mapErr(err, "delete push token")
}

func (r *pgPushTokenRepository) ListByUser(ctx context.Context, userID string) ([]*domain.PushToken, error) {
	const q = `SELECT id, user_id, token, platform, created_at, updated_at FROM push_tokens WHERE user_id=$1`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, mapErr(err, "list push tokens")
	}
	defer rows.Close()

	var out []*domain.PushToken
	for rows.Next() {
		var t domain.PushToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Token, &t.Platform, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, mapErr(err, "scan push token")
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

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
