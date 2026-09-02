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

	measurements, err := importedMeasurements(object, document)
	if err != nil {
		return NoVersion, err
	}
	if _, present := object["measurements"]; present {
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

func importedMeasurements(object map[string]json.RawMessage, document registry.Document) ([]telemetry.Measurement, error) {
	raw, present := object["measurements"]
	if !present {
		return nil, nil
	}
	var measurements []telemetry.Measurement
	if err := json.Unmarshal(raw, &measurements); err != nil {
		return nil, telemetry.ErrInvalidMeasurement
	}

	channels := make(map[string]registry.Channel, len(document.Channels))
	for _, channel := range document.Channels {
		channels[channel.ID] = channel
	}
	devices := make(map[string]struct{}, len(document.Devices))
	for _, device := range document.Devices {
		devices[device.ID] = struct{}{}
	}

	for _, measurement := range measurements {
		if err := measurement.Validate(); err != nil {
			return nil, telemetry.ErrInvalidMeasurement
		}
		channel, found := channels[measurement.ChannelID]
		if !found {
			return nil, telemetry.ErrInvalidMeasurement
		}
		if channel.Unit != "" && measurement.Unit != channel.Unit {
			return nil, telemetry.ErrInvalidMeasurement
		}
		if measurement.SourceDeviceID != "" {
			if _, found := devices[measurement.SourceDeviceID]; !found {
				return nil, telemetry.ErrInvalidMeasurement
			}
			if channel.DeviceID != "" && measurement.SourceDeviceID != channel.DeviceID {
				return nil, telemetry.ErrInvalidMeasurement
			}
		}
	}
	return measurements, nil
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
