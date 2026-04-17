// Package main is the ViewAura database migrator.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/Ab-dex/view-aura/internal/platform/config"
)

const migrationsPath = "/Users/mac/Desktop/jobs/viewaura/infrastructure/postgres/migrations"

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
		log.Fatal().
			Err(err).
			Str("source", sourceURL).
			Str("dsn", maskDSN(dsn)).
			Msg("failed to create migrate instance")
	}
	defer m.Close()

	m.Log = &migrateLogger{}

	// ── Execute command ───────────────────────────────────────────────────────
	switch command {
	case "up":
		runUp(m, dsn)

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
		log.Fatal().
			Str("command", command).
			Msg("unknown command — valid: up | down | version | force")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// CORE FIX: smart error handling for dirty DB state
// ──────────────────────────────────────────────────────────────────────────────

func runUp(m *migrate.Migrate, dsn string) {
	log.Info().Str("path", migrationsPath).Msg("applying pending migrations...")

	var migrated bool

	err := m.Up()
	if err != nil {

		if strings.Contains(err.Error(), "Dirty database version") {
			v, dirty, vErr := m.Version()
			if vErr != nil {
				log.Fatal().Err(err).Msg("dirty migration + cannot read version")
			}

			log.Warn().
				Uint("version", v).
				Bool("dirty", dirty).
				Msg("dirty database detected — forcing recovery")

			if forceErr := m.Force(int(v)); forceErr != nil {
				log.Fatal().Err(forceErr).Msg("failed to force migration version")
			}

			if retryErr := m.Up(); retryErr != nil &&
				!errors.Is(retryErr, migrate.ErrNoChange) {
				log.Fatal().Err(retryErr).Msg("migration failed after recovery")
			}

			migrated = true
		} else if errors.Is(err, migrate.ErrNoChange) {
			log.Info().Msg("no migrations to apply")
			migrated = false
		} else {
			log.Fatal().Err(err).Msg("migration failed")
		}
	} else {
		migrated = true
	}

	v, dirty, _ := m.Version()
	log.Info().
		Uint("version", v).
		Bool("dirty", dirty).
		Msg("migration complete")

	// ✅ ALWAYS RUN DUMP (your requirement)
	// even if no changes
	if err := tryDump(dsn); err != nil {
		log.Error().Err(err).Msg("schema dump failed (non-fatal)")
	}

	_ = migrated // optional for future logic
}

func runDown(m *migrate.Migrate, steps int) {
	if steps <= 0 {
		steps = 1
	}

	log.Warn().Int("steps", steps).Msg("rolling back migrations...")

	err := m.Steps(-steps)
	if err != nil {
		handleMigrationError(err, m)
		return
	}

	v, dirty, _ := m.Version()
	log.Info().
		Uint("version", v).
		Bool("dirty", dirty).
		Msg("rollback successful")
}

func runVersion(m *migrate.Migrate) {
	v, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			log.Info().Msg("no migrations applied yet")
			return
		}
		handleMigrationError(err, m)
		return
	}

	log.Info().
		Uint("version", v).
		Bool("dirty", dirty).
		Msg("current migration version")

	if dirty {
		log.Warn().Msg("DB is DIRTY — run:")
		log.Warn().Msg("  ./viewaura-migrator force -v " + strconv.Itoa(int(v)))
	}
}

func runForce(m *migrate.Migrate, version int) {
	log.Warn().Int("version", version).
		Msg("forcing migration version (use only for recovery)")

	if err := m.Force(version); err != nil {
		log.Fatal().Err(err).Msg("force failed")
	}

	log.Info().Int("version", version).Msg("version forced successfully")
}

// ──────────────────────────────────────────────────────────────────────────────
// 🔥 NEW: Central error handler (this is the fix)
// ──────────────────────────────────────────────────────────────────────────────

func handleMigrationError(err error, m *migrate.Migrate) {
	v, dirty, vErr := m.Version()

	// Case 1: dirty database state
	if dirty || strings.Contains(err.Error(), "Dirty database version") {
		log.Fatal().
			Err(err).
			Msgf(
				"DIRTY MIGRATION DETECTED (version %d). Fix with:\n  ./viewaura-migrator force -v %d",
				v, v,
			)
	}

	// Case 2: no version yet
	if errors.Is(err, migrate.ErrNilVersion) {
		log.Fatal().
			Err(err).
			Msg("no migrations applied yet")
	}

	// Case 3: generic failure
	if vErr == nil {
		log.Fatal().
			Err(err).
			Uint("version", v).
			Msg("migration failed")
	}

	log.Fatal().Err(err).Msg("migration failed")
}

// ──────────────────────────────────────────────────────────────────────────────
// DSN resolution
// ──────────────────────────────────────────────────────────────────────────────

func resolveDSN(flagVal, cfgPath string) string {
	if flagVal != "" {
		log.Info().Msg("using DSN from flag")
		return flagVal
	}

	if env := os.Getenv("VIEWAURA_DB_DSN"); env != "" {
		log.Info().Msg("using DSN from env")
		return env
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	if cfg.DB.DSN == "" {
		log.Fatal().Msg("db.dsn is empty")
	}

	log.Info().Msg("using DSN from config file")
	return cfg.DB.DSN
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func maskDSN(dsn string) string {
	start := strings.Index(dsn, "://")
	if start == -1 {
		return "***"
	}
	start += 3

	at := strings.Index(dsn[start:], "@")
	if at == -1 {
		return "***"
	}
	at += start

	return dsn[:start] + "***@" + dsn[at+1:]
}

// ──────────────────────────────────────────────────────────────────────────────
// Logger adapter
// ──────────────────────────────────────────────────────────────────────────────

type migrateLogger struct{}

func (l *migrateLogger) Printf(format string, v ...interface{}) {
	log.Info().Msg(fmt.Sprintf(format, v...))
}

func (l *migrateLogger) Verbose() bool { return false }

func dumpSchema(dsn string) error {
	log.Info().Msg("dumping schema with pg_dump...")

	outFile := "/Users/mac/Desktop/jobs/viewaura/infrastructure/postgres/schema.sql"

	cmd := exec.Command(
		"pg_dump",
		"--schema-only",
		"--no-owner",
		"--no-privileges",
		dsn,
	)

	file, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer file.Close()

	cmd.Stdout = file
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func tryDump(dsn string) error {

	if err := dumpSchema(dsn); err != nil {
		log.Error().Err(err).Msg("schema dump failed (non-fatal)")
		return err
	}

	log.Info().Msg("schema.sql updated successfully")
	return nil
}
