package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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
