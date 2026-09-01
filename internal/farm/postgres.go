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

func (store *PostgresStore) Load(ctx context.Context) (json.RawMessage, error) {
	state, err := store.queries.GetBrowserCompatibleState(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return json.RawMessage(state), err
}

func (store *PostgresStore) Save(ctx context.Context, state json.RawMessage) error {
	_, err := store.queries.SaveBrowserCompatibleState(ctx, []byte(state))
	return err
}
