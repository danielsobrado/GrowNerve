package farm

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jdanielsobrado/grownerve/internal/platform/database/gen"
)

type PostgresStore struct{ queries *gen.Queries }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{queries: gen.New(pool)}
}

func (store *PostgresStore) Load(ctx context.Context) (json.RawMessage, int64, error) {
	row, err := store.queries.GetBrowserCompatibleState(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, NoVersion, ErrNotFound
	}
	if err != nil {
		return nil, NoVersion, err
	}
	return json.RawMessage(row.State), row.Version, nil
}

// Save applies the write as a compare-and-swap on the stored version so two
// concurrent writers cannot silently overwrite one another.
func (store *PostgresStore) Save(ctx context.Context, state json.RawMessage, expected int64) (int64, error) {
	if expected == NoVersion || expected == AnyVersion {
		version, err := store.queries.InsertBrowserCompatibleState(ctx, []byte(state))
		if err == nil {
			return version, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return NoVersion, err
		}
		if expected == NoVersion {
			// The row appeared between load and save: the caller's view is stale.
			_, current, loadErr := store.Load(ctx)
			if loadErr != nil {
				return NoVersion, loadErr
			}
			return current, ErrVersionConflict
		}
		return store.overwrite(ctx, state)
	}
	version, err := store.queries.UpdateBrowserCompatibleStateIfVersion(ctx, gen.UpdateBrowserCompatibleStateIfVersionParams{State: []byte(state), Version: expected})
	if errors.Is(err, pgx.ErrNoRows) {
		_, current, loadErr := store.Load(ctx)
		if loadErr != nil && !errors.Is(loadErr, ErrNotFound) {
			return NoVersion, loadErr
		}
		return current, ErrVersionConflict
	}
	return version, err
}

// overwrite implements AnyVersion for an existing row. It is used only where the
// caller owns the whole document, never for read-modify-write.
func (store *PostgresStore) overwrite(ctx context.Context, state json.RawMessage) (int64, error) {
	return store.queries.OverwriteBrowserCompatibleState(ctx, []byte(state))
}
