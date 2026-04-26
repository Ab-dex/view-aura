package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/Ab-dex/view-aura/internal/platform/config"
)

type Pool struct {
	*pgxpool.Pool
}

func New(ctx context.Context, cfg config.DBConfig) (*Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("db: parse DSN: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.MaxIdleConns)
	poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	poolCfg.MaxConnIdleTime = cfg.ConnMaxIdleTime
	poolCfg.HealthCheckPeriod = 30 * time.Second
	poolCfg.ConnConfig.ConnectTimeout = 10 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: initial ping failed: %w", err)
	}

	stat := pool.Stat()
	log.Info().
		Int32("total_conns", stat.TotalConns()).
		Int32("idle_conns", stat.IdleConns()).
		Str("dsn_host", poolCfg.ConnConfig.Host).
		Msg("db: pool established")

	return &Pool{Pool: pool}, nil
}

func (p *Pool) Close() {
	p.Close()
	log.Info().Msg("db: pool closed")
}

func (p *Pool) WithTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := p.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	})
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}

	var committed bool
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if err := fn(tx); err != nil {
		return fmt.Errorf("db: exec callback: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit tx: %w", err)
	}

	committed = true
	return nil
}

// Executor is the minimal interface satisfied by both *pgxpool.Pool and pgx.Tx.
// Repositories accept Executor (or call db.Conn(ctx, pool) to get one) so they
// automatically participate in a transaction when one is active in the context.
type Executor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ─── Context key ──────────────────────────────────────────────────────────────

type txKey struct{}

// Inject stores the transaction in the context.
// Called inside TxManager.WithTx before invoking the callback.
func Inject(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// FromContext retrieves the transaction stored by Inject.
// Returns nil when no transaction is active.
func FromContext(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(txKey{}).(pgx.Tx)
	return tx
}

// Conn returns the active transaction when one exists in ctx, otherwise falls
// back to the pool.  Repositories call this instead of using the pool directly
// so they participate in transactions transparently.
//
// Usage in a repository:
//
//	conn := db.Conn(ctx, r.pool)
//	row := conn.QueryRow(ctx, q, args...)
func Conn(ctx context.Context, pool Executor) Executor {
	if tx := FromContext(ctx); tx != nil {
		return tx
	}
	return pool
}

// ─── TxManager ────────────────────────────────────────────────────────────────

// TxManager is the interface for running a function inside a database
// transaction.  The transaction is injected into the context so all repository
// calls made inside fn automatically use the same connection.
//
// If fn returns an error the transaction is rolled back.
// If fn returns nil the transaction is committed.
//
// Side effects that must run AFTER commit (event publishing, sending emails)
// must not be placed inside fn.  Run them after WithTx returns nil.
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// pgTxManager implements TxManager using *Pool.
type pgTxManager struct {
	pool *Pool
}

// NewTxManager constructs a TxManager from the platform pool.
func NewTxManager(pool *Pool) TxManager {
	return &pgTxManager{pool: pool}
}

func (m *pgTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted, // ReadCommitted for most OLTP operations
	})
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}

	// Inject the transaction into context so all repository calls inside fn
	// automatically use tx instead of the pool.
	txCtx := Inject(ctx, tx)

	var committed bool
	defer func() {
		if !committed {
			// Rollback is best-effort on error or panic.
			_ = tx.Rollback(ctx)
		}
	}()

	if err := fn(txCtx); err != nil {
		return err // defer rolls back
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit tx: %w", err)
	}
	committed = true
	return nil
}
