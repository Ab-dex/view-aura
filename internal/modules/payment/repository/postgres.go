package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ab-dex/view-aura/internal/modules/payment/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

// ─── Subscription ─────────────────────────────────────────────────────────────

type pgSubscriptionRepository struct{ pool *pgxpool.Pool }

func NewSubscriptionRepository(pool *pgxpool.Pool) SubscriptionRepository {
	return &pgSubscriptionRepository{pool: pool}
}

const subCols = `id, user_id, plan, status, stripe_sub_id, stripe_customer_id,
	current_period_end, cancel_at_end, created_at, updated_at`

func (r *pgSubscriptionRepository) Create(ctx context.Context, s *domain.Subscription) (*domain.Subscription, error) {
	const q = `
		INSERT INTO subscriptions
			(id, user_id, plan, status, stripe_sub_id, stripe_customer_id,
			 current_period_end, cancel_at_end, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())
		RETURNING ` + subCols
	return scanSub(r.pool.QueryRow(ctx, q,
		s.ID, s.UserID, s.Plan, s.Status,
		s.StripeSubID, s.StripeCustomerID,
		s.CurrentPeriodEnd, s.CancelAtEnd,
	))
}

func (r *pgSubscriptionRepository) GetByUserID(ctx context.Context, userID string) (*domain.Subscription, error) {
	const q = `SELECT ` + subCols + `
		FROM subscriptions
		WHERE user_id=$1 AND status != 'cancelled'
		ORDER BY created_at DESC LIMIT 1`
	s, err := scanSub(r.pool.QueryRow(ctx, q, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("payment: get subscription by user: %w", err)
	}
	return s, nil
}

func (r *pgSubscriptionRepository) GetByStripeSubID(ctx context.Context, stripeSubID string) (*domain.Subscription, error) {
	const q = `SELECT ` + subCols + ` FROM subscriptions WHERE stripe_sub_id=$1`
	s, err := scanSub(r.pool.QueryRow(ctx, q, stripeSubID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("payment: get subscription by stripe id: %w", err)
	}
	return s, nil
}

func (r *pgSubscriptionRepository) Update(ctx context.Context, s *domain.Subscription) (*domain.Subscription, error) {
	const q = `
		UPDATE subscriptions SET
			plan=$2, status=$3, current_period_end=$4, cancel_at_end=$5, updated_at=NOW()
		WHERE id=$1
		RETURNING ` + subCols
	result, err := scanSub(r.pool.QueryRow(ctx, q, s.ID, s.Plan, s.Status, s.CurrentPeriodEnd, s.CancelAtEnd))
	if err != nil {
		return nil, fmt.Errorf("payment: update subscription: %w", err)
	}
	return result, nil
}

func scanSub(row interface{ Scan(...any) error }) (*domain.Subscription, error) {
	var s domain.Subscription
	err := row.Scan(
		&s.ID, &s.UserID, &s.Plan, &s.Status,
		&s.StripeSubID, &s.StripeCustomerID,
		&s.CurrentPeriodEnd, &s.CancelAtEnd,
		&s.CreatedAt, &s.UpdatedAt,
	)
	return &s, err
}

// ─── Invoice ──────────────────────────────────────────────────────────────────

type pgInvoiceRepository struct{ pool *pgxpool.Pool }

func NewInvoiceRepository(pool *pgxpool.Pool) InvoiceRepository {
	return &pgInvoiceRepository{pool: pool}
}

const invCols = `id, user_id, subscription_id, stripe_invoice_id,
	amount_cents, currency, status, paid_at, created_at`

func (r *pgInvoiceRepository) Create(ctx context.Context, inv *domain.Invoice) (*domain.Invoice, error) {
	const q = `
		INSERT INTO invoices
			(id, user_id, subscription_id, stripe_invoice_id,
			 amount_cents, currency, status, paid_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW())
		RETURNING ` + invCols
	return scanInvoice(r.pool.QueryRow(ctx, q,
		inv.ID, inv.UserID, inv.SubscriptionID, inv.StripeInvoiceID,
		inv.AmountCents, inv.Currency, inv.Status, inv.PaidAt,
	))
}

func (r *pgInvoiceRepository) GetByStripeInvoiceID(ctx context.Context, id string) (*domain.Invoice, error) {
	inv, err := scanInvoice(r.pool.QueryRow(ctx,
		`SELECT `+invCols+` FROM invoices WHERE stripe_invoice_id=$1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("payment: get invoice by stripe id: %w", err)
	}
	return inv, nil
}

func (r *pgInvoiceRepository) Update(ctx context.Context, inv *domain.Invoice) (*domain.Invoice, error) {
	const q = `
		UPDATE invoices SET status=$2, paid_at=$3
		WHERE id=$1
		RETURNING ` + invCols
	result, err := scanInvoice(r.pool.QueryRow(ctx, q, inv.ID, inv.Status, inv.PaidAt))
	if err != nil {
		return nil, fmt.Errorf("payment: update invoice: %w", err)
	}
	return result, nil
}

func (r *pgInvoiceRepository) ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.Invoice, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM invoices WHERE user_id=$1`, userID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("payment: count invoices: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT `+invCols+` FROM invoices WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("payment: list invoices: %w", err)
	}
	defer rows.Close()

	var out []*domain.Invoice
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("payment: scan invoice: %w", err)
		}
		out = append(out, inv)
	}
	return out, total, rows.Err()
}

func scanInvoice(row interface{ Scan(...any) error }) (*domain.Invoice, error) {
	var inv domain.Invoice
	err := row.Scan(
		&inv.ID, &inv.UserID, &inv.SubscriptionID, &inv.StripeInvoiceID,
		&inv.AmountCents, &inv.Currency, &inv.Status, &inv.PaidAt, &inv.CreatedAt,
	)
	return &inv, err
}

// ─── Screener License ─────────────────────────────────────────────────────────

type pgLicenseRepository struct{ pool *pgxpool.Pool }

func NewLicenseRepository(pool *pgxpool.Pool) LicenseRepository {
	return &pgLicenseRepository{pool: pool}
}

const licCols = `id, user_id, movie_id, amount_cents, currency, expires_at, created_at`

func (r *pgLicenseRepository) Create(ctx context.Context, l *domain.ScreenerLicense) (*domain.ScreenerLicense, error) {
	const q = `
		INSERT INTO screener_licenses (id, user_id, movie_id, amount_cents, currency, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,NOW())
		RETURNING ` + licCols
	var out domain.ScreenerLicense
	err := r.pool.QueryRow(ctx, q,
		l.ID, l.UserID, l.MovieID, l.AmountCents, l.Currency, l.ExpiresAt,
	).Scan(&out.ID, &out.UserID, &out.MovieID, &out.AmountCents, &out.Currency, &out.ExpiresAt, &out.CreatedAt)
	return &out, err
}

func (r *pgLicenseRepository) GetByUserAndMovie(ctx context.Context, userID, movieID string) (*domain.ScreenerLicense, error) {
	const q = `SELECT ` + licCols + `
		FROM screener_licenses
		WHERE user_id=$1 AND movie_id=$2 AND expires_at > NOW()
		ORDER BY created_at DESC LIMIT 1`
	var out domain.ScreenerLicense
	err := r.pool.QueryRow(ctx, q, userID, movieID).
		Scan(&out.ID, &out.UserID, &out.MovieID, &out.AmountCents, &out.Currency, &out.ExpiresAt, &out.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("payment: get screener license: %w", err)
	}
	return &out, nil
}

func (r *pgLicenseRepository) ListByUser(ctx context.Context, userID string) ([]*domain.ScreenerLicense, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+licCols+` FROM screener_licenses WHERE user_id=$1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("payment: list screener licenses: %w", err)
	}
	defer rows.Close()

	var out []*domain.ScreenerLicense
	for rows.Next() {
		var l domain.ScreenerLicense
		if err := rows.Scan(&l.ID, &l.UserID, &l.MovieID, &l.AmountCents, &l.Currency, &l.ExpiresAt, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("payment: scan screener license: %w", err)
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}
