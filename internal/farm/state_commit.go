package farm

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jdanielsobrado/grownerve/internal/platform/database/gen"
	"github.com/jdanielsobrado/grownerve/internal/registry"
	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

// StateCommitter atomically commits one whole-state replacement and its
// relational projections. Production uses the PostgreSQL implementation; the
// interface keeps HTTP tests independent from a database.
type StateCommitter interface {
	CommitState(ctx context.Context, state json.RawMessage, expectedVersion int64) (int64, error)
}

type PostgresStateCommitter struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

func NewPostgresStateCommitter(pool *pgxpool.Pool) *PostgresStateCommitter {
	return &PostgresStateCommitter{pool: pool, queries: gen.New(pool)}
}

func (committer *PostgresStateCommitter) CommitState(ctx context.Context, state json.RawMessage, expectedVersion int64) (int64, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(state, &object); err != nil || object == nil {
		return NoVersion, errors.New("invalid farm state")
	}

	document, err := registry.Parse(state)
	if err != nil {
		return NoVersion, err
	}
	if err := document.Validate(); err != nil {
		return NoVersion, err
	}

	var measurements []telemetry.Measurement
	if raw, present := object["measurements"]; present {
		if err := json.Unmarshal(raw, &measurements); err != nil {
			return NoVersion, telemetry.ErrInvalidMeasurement
		}
		stripped, err := ReplaceKeys(state, map[string]any{"measurements": []telemetry.Measurement{}})
		if err != nil {
			return NoVersion, err
		}
		state = stripped
	}

	transaction, err := committer.pool.Begin(ctx)
	if err != nil {
		return NoVersion, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	queries := committer.queries.WithTx(transaction)

	version, err := saveStateWithQueries(ctx, queries, state, expectedVersion)
	if err != nil {
		return version, err
	}
	if err := registry.ProjectWithQueries(ctx, queries, document); err != nil {
		return NoVersion, err
	}
	if _, err := telemetry.AppendWithQueries(ctx, queries, measurements); err != nil {
		return NoVersion, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return NoVersion, err
	}
	return version, nil
}

func saveStateWithQueries(ctx context.Context, queries *gen.Queries, state json.RawMessage, expected int64) (int64, error) {
	if expected == NoVersion {
		version, err := queries.InsertBrowserCompatibleState(ctx, []byte(state))
		if errors.Is(err, pgx.ErrNoRows) {
			return currentVersion(ctx, queries)
		}
		return version, err
	}
	if expected == AnyVersion {
		return queries.OverwriteBrowserCompatibleState(ctx, []byte(state))
	}
	version, err := queries.UpdateBrowserCompatibleStateIfVersion(ctx, gen.UpdateBrowserCompatibleStateIfVersionParams{
		State: []byte(state), Version: expected,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return currentVersion(ctx, queries)
	}
	return version, err
}

func currentVersion(ctx context.Context, queries *gen.Queries) (int64, error) {
	row, err := queries.GetBrowserCompatibleState(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return NoVersion, ErrVersionConflict
	}
	if err != nil {
		return NoVersion, err
	}
	return row.Version, ErrVersionConflict
}
