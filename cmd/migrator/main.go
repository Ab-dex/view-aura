// Package main is the ViewAura database migrator.
//
// Usage:
//
//	# Apply all pending migrations
//	./viewaura-migrator up
//
//	# Roll back the last migration
//	./viewaura-migrator down
//
//	# Roll back N migrations
//	./viewaura-migrator down -n 3
//
//	# Show current version
//	./viewaura-migrator version
//
//	# Force a specific version (dangerous — use only to fix a dirty state)
//	./viewaura-migrator force -v 2
//
// The migrator reads the database DSN from:
//  1. --dsn CLI flag
//  2. VIEWAURA_DB_DSN environment variable
//  3. config/config.yaml → db.dsn
//
// Migration files live in infrastructure/postgres/migrations/ and follow
// the naming convention: NNN_description.sql (e.g. 001_initial_schema.sql).
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/Ab-dex/view-aura/internal/platform/config"
)

const migrationsPath = "infrastructure/postgres/migrations"

func main() {
	// ── Logging ───────────────────────────────────────────────────────────────
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Str("component", "migrator").Logger()

	// ── Flags ─────────────────────────────────────────────────────────────────
	cfgPath := flag.String("config", "config/config.yaml", "path to config file")
	dsnFlag := flag.String("dsn", "", "postgres DSN (overrides config and env)")
	stepsFlag := flag.Int("n", 1, "number of migrations to roll back (used with 'down')")
	vFlag := flag.Int("v", 0, "target version for 'force' command")
	flag.Parse()

	command := flag.Arg(0)
	if command == "" {
		command = "up"
	}

	// ── Resolve DSN ───────────────────────────────────────────────────────────
	dsn := resolveDSN(*dsnFlag, *cfgPath)

	// ── Create migrate instance ───────────────────────────────────────────────
	sourceURL := "file://" + migrationsPath
	m, err := migrate.New(sourceURL, dsn)
	if err != nil {
		log.Fatal().Err(err).
			Str("source", sourceURL).
			Str("dsn", maskDSN(dsn)).
			Msg("failed to create migrate instance")
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Error().Err(srcErr).Msg("error closing migration source")
		}
		if dbErr != nil {
			log.Error().Err(dbErr).Msg("error closing migration database connection")
		}
	}()

	// Log each migration step at INFO level.
	m.Log = &migrateLogger{}

	// ── Execute command ───────────────────────────────────────────────────────
	switch command {
	case "up":
		runUp(m)

	case "down":
		runDown(m, *stepsFlag)

	case "version":
		runVersion(m)

	case "force":
		if *vFlag == 0 {
			log.Fatal().Msg("force requires -v <version> flag")
		}
		runForce(m, *vFlag)

	default:
		log.Fatal().Str("command", command).
			Msg("unknown command — valid commands: up | down | version | force")
	}
}

// ─── Command implementations ──────────────────────────────────────────────────

func runUp(m *migrate.Migrate) {
	log.Info().Str("path", migrationsPath).Msg("applying pending migrations...")

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info().Msg("no pending migrations — schema is up to date")
			return
		}
		log.Fatal().Err(err).Msg("migration up failed")
	}

	v, dirty, _ := m.Version()
	log.Info().
		Uint("version", v).
		Bool("dirty", dirty).
		Msg("migrations applied successfully")
}

func runDown(m *migrate.Migrate, steps int) {
	if steps <= 0 {
		steps = 1
	}
	log.Warn().Int("steps", steps).Msg("rolling back migrations...")

	if err := m.Steps(-steps); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info().Msg("nothing to roll back")
			return
		}
		log.Fatal().Err(err).Msg("migration down failed")
	}

	v, dirty, _ := m.Version()
	log.Info().
		Uint("version", v).
		Bool("dirty", dirty).
		Msgf("rolled back %d migration(s) — now at version %d", steps, v)
}

func runVersion(m *migrate.Migrate) {
	v, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			log.Info().Msg("database has no migrations applied (version: nil)")
			return
		}
		log.Fatal().Err(err).Msg("failed to read migration version")
	}
	log.Info().
		Str("version", strconv.Itoa(int(v))).
		Bool("dirty", dirty).
		Msg("current migration version")
	if dirty {
		log.Warn().Msg("database is in a dirty state — the last migration did not complete cleanly")
		log.Warn().Msg("fix manually then run: ./viewaura-migrator force -v " + strconv.Itoa(int(v)))
	}
}

func runForce(m *migrate.Migrate, version int) {
	log.Warn().Int("version", version).
		Msg("forcing migration version — use only to recover a dirty state")
	if err := m.Force(version); err != nil {
		log.Fatal().Err(err).Msg("force failed")
	}
	log.Info().Int("version", version).Msg("version forced successfully")
}

// ─── DSN resolution ───────────────────────────────────────────────────────────

func resolveDSN(flag, cfgPath string) string {
	// Priority: --dsn flag > VIEWAURA_DB_DSN env > config file.
	if flag != "" {
		log.Info().Msg("using DSN from --dsn flag")
		return flag
	}
	if env := os.Getenv("VIEWAURA_DB_DSN"); env != "" {
		log.Info().Msg("using DSN from VIEWAURA_DB_DSN environment variable")
		return env
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatal().Err(err).Str("config", cfgPath).
			Msg("failed to load config — provide --dsn or set VIEWAURA_DB_DSN")
	}
	if cfg.DB.DSN == "" {
		log.Fatal().Msg("db.dsn is empty in config — provide --dsn or set VIEWAURA_DB_DSN")
	}
	log.Info().Str("config", cfgPath).Msg("using DSN from config file")
	return cfg.DB.DSN
}

// maskDSN replaces the password in a DSN with *** for safe logging.
func maskDSN(dsn string) string {
	// Simple heuristic: hide everything between :// and @ .
	// Real passwords may contain @ so this is best-effort.
	start := 0
	for i := 0; i < len(dsn)-2; i++ {
		if dsn[i] == '/' && dsn[i+1] == '/' {
			start = i + 2
			break
		}
	}
	end := len(dsn)
	for i := start; i < len(dsn); i++ {
		if dsn[i] == '@' {
			end = i
			break
		}
	}
	if start == 0 || end == len(dsn) {
		return "***"
	}
	// Replace the user:password section with user:*** .
	userPass := dsn[start:end]
	for i := 0; i < len(userPass); i++ {
		if userPass[i] == ':' {
			return dsn[:start] + userPass[:i+1] + "***" + dsn[end:]
		}
	}
	return dsn
}

// ─── Logger adapter ───────────────────────────────────────────────────────────

// migrateLogger bridges golang-migrate's Logger interface to zerolog.
type migrateLogger struct{}

func (l *migrateLogger) Printf(format string, v ...interface{}) {
	log.Info().Msg(fmt.Sprintf(format, v...))
}

func (l *migrateLogger) Verbose() bool { return false }
