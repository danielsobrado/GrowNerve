package main

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jdanielsobrado/grownerve/db/migrations"
)

func TestValidateAppliedVersions(t *testing.T) {
	if err := validateAppliedVersions(map[int]bool{1: true, 2: true}); err != nil {
		t.Fatalf("valid history rejected: %v", err)
	}
	for name, applied := range map[string]map[int]bool{
		"gap":              {2: true},
		"non-contiguous":   {1: true, 3: true},
		"newer than binary": {1: true, 2: true, migrations.LatestVersion + 1: true},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateAppliedVersions(applied); err == nil {
				t.Fatal("invalid migration history was accepted")
			}
		})
	}
}

func TestMigrateUpIsRepeatSafe(t *testing.T) {
	url := os.Getenv("GROWNERVE_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("GROWNERVE_TEST_DATABASE_URL is not configured")
	}
	config, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	ctx := context.Background()
	connection, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.Close(context.Background()) }()

	if err := migrateUp(ctx, connection); err != nil {
		t.Fatalf("first migrate up: %v", err)
	}
	if err := migrateUp(ctx, connection); err != nil {
		t.Fatalf("second migrate up: %v", err)
	}
	var version int
	if err := connection.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != migrations.LatestVersion {
		t.Fatalf("schema version = %d, want %d", version, migrations.LatestVersion)
	}
}
