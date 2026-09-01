// Package dbtest connects integration tests to a real PostgreSQL instance.
//
// Tests that use it skip when GROWNERVE_TEST_DATABASE_URL is unset, so a
// developer without a database still gets a green unit run, while CI — which
// does set it — exercises the real schema.
package dbtest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// URLEnv names the variable holding the integration database URL.
const URLEnv = "GROWNERVE_TEST_DATABASE_URL"

// Pool returns a connection pool, skipping the test when no database is
// configured. The pool is closed when the test finishes.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv(URLEnv)
	if url == "" {
		t.Skipf("set %s to run integration tests against PostgreSQL", URLEnv)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect to %s: %v", URLEnv, err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping integration database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Reset truncates the tables an integration test touches so each test starts
// from a known state without needing a fresh database.
func Reset(t *testing.T, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	ctx := context.Background()
	for _, table := range tables {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE "+table+" CASCADE"); err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}
}

// SeedChannel inserts the minimum rows a measurement needs: a facility and a
// channel that references it. It returns the channel identifier.
func SeedChannel(t *testing.T, pool *pgxpool.Pool, key string) string {
	t.Helper()
	ctx := context.Background()
	var facilityID string
	err := pool.QueryRow(ctx,
		`INSERT INTO facilities (name, timezone) VALUES ($1, 'UTC') RETURNING id`,
		"integration-"+key).Scan(&facilityID)
	if err != nil {
		t.Fatalf("seed facility: %v", err)
	}
	var channelID string
	err = pool.QueryRow(ctx,
		`INSERT INTO device_channels (facility_id, entity_type, entity_id, key, name, kind, value_type, unit, stale_after)
		 VALUES ($1, 'zone', $1, $2, $2, 'measurement', 'number', 'degC', INTERVAL '10 minutes')
		 RETURNING id`, facilityID, key).Scan(&channelID)
	if err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	return channelID
}
