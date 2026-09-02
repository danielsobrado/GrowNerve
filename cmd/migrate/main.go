package main

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jdanielsobrado/grownerve/db/migrations"
)

const migrationLockID int64 = 0x47524f57

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}
	url := os.Getenv("MIGRATION_DATABASE_URL")
	if url == "" {
		url = os.Getenv("GROWNERVE_DATABASE_URL")
	}
	if url == "" {
		return fmt.Errorf("MIGRATION_DATABASE_URL or GROWNERVE_DATABASE_URL is required")
	}
	config, err := pgx.ParseConfig(url)
	if err != nil {
		return fmt.Errorf("parse database URL: %w", err)
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	ctx := context.Background()
	connection, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = connection.Close(context.Background()) }()

	if _, err := connection.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() { _, _ = connection.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID) }()

	switch direction {
	case "up":
		return migrateUp(ctx, connection)
	case "down":
		return migrateDown(ctx, connection)
	default:
		return fmt.Errorf("direction must be up or down")
	}
}

func migrateUp(ctx context.Context, connection *pgx.Conn) error {
	applied, err := appliedVersions(ctx, connection)
	if err != nil {
		return err
	}
	if err := validateAppliedVersions(applied); err != nil {
		return err
	}
	for _, migration := range migrations.All {
		if applied[migration.Version] {
			continue
		}
		if err := applyMigration(ctx, connection, migration); err != nil {
			return err
		}
		applied[migration.Version] = true
	}
	return nil
}

func migrateDown(ctx context.Context, connection *pgx.Conn) error {
	applied, err := appliedVersions(ctx, connection)
	if err != nil {
		return err
	}
	if err := validateAppliedVersions(applied); err != nil {
		return err
	}
	for index := len(migrations.All) - 1; index >= 0; index-- {
		migration := migrations.All[index]
		if !applied[migration.Version] {
			continue
		}
		if err := rollbackMigration(ctx, connection, migration); err != nil {
			return err
		}
	}
	return nil
}

func appliedVersions(ctx context.Context, connection *pgx.Conn) (map[int]bool, error) {
	var exists bool
	if err := connection.QueryRow(ctx, "SELECT to_regclass('public.schema_migrations') IS NOT NULL").Scan(&exists); err != nil {
		return nil, fmt.Errorf("inspect migration table: %w", err)
	}
	applied := map[int]bool{}
	if !exists {
		return applied, nil
	}
	rows, err := connection.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("read migration versions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("read migration version: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read migration versions: %w", err)
	}
	return applied, nil
}

func validateAppliedVersions(applied map[int]bool) error {
	versions := make([]int, 0, len(applied))
	for version := range applied {
		versions = append(versions, version)
	}
	sort.Ints(versions)
	for index, version := range versions {
		expected := index + 1
		if version != expected {
			return fmt.Errorf("database migration history is not contiguous: expected version %d, found %d", expected, version)
		}
		if version > migrations.LatestVersion {
			return fmt.Errorf("database schema version %d is newer than supported version %d", version, migrations.LatestVersion)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, connection *pgx.Conn, migration migrations.Migration) error {
	transaction, err := connection.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", migration.Version, err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err := transaction.Exec(ctx, migration.Up); err != nil {
		return fmt.Errorf("apply migration %d: %w", migration.Version, err)
	}
	if _, err := transaction.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES ($1) ON CONFLICT DO NOTHING", migration.Version); err != nil {
		return fmt.Errorf("record migration %d: %w", migration.Version, err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %d: %w", migration.Version, err)
	}
	return nil
}

func rollbackMigration(ctx context.Context, connection *pgx.Conn, migration migrations.Migration) error {
	transaction, err := connection.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin rollback %d: %w", migration.Version, err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err := transaction.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", migration.Version); err != nil {
		return fmt.Errorf("unrecord migration %d: %w", migration.Version, err)
	}
	if _, err := transaction.Exec(ctx, migration.Down); err != nil {
		return fmt.Errorf("rollback migration %d: %w", migration.Version, err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit rollback %d: %w", migration.Version, err)
	}
	return nil
}
