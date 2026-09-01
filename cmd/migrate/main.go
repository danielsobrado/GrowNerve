package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jdanielsobrado/grownerve/db/migrations"
)

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
	connection, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = connection.Close(context.Background()) }()
	var sql string
	switch direction {
	case "up":
		sql = migrations.Up
	case "down":
		sql = migrations.Down
	default:
		return fmt.Errorf("direction must be up or down")
	}
	if _, err := connection.Exec(context.Background(), sql); err != nil {
		return fmt.Errorf("migrate %s: %w", direction, err)
	}
	return nil
}
