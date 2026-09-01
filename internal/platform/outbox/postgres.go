package outbox

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jdanielsobrado/grownerve/internal/platform/database/gen"
)

// PostgresStore persists pending publications alongside the rest of the farm
// data, so a server restart does not lose an accepted command that had not yet
// reached the broker.
type PostgresStore struct{ queries *gen.Queries }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{queries: gen.New(pool)}
}

func (store *PostgresStore) Enqueue(ctx context.Context, topic, key string, payload json.RawMessage) (string, error) {
	id, err := store.queries.EnqueueOutboxMessage(ctx, gen.EnqueueOutboxMessageParams{
		Topic: topic, MessageKey: key, Payload: []byte(payload),
	})
	if err != nil {
		return "", err
	}
	return uuidText(id), nil
}

func (store *PostgresStore) Pending(ctx context.Context, batchSize int) ([]Message, error) {
	rows, err := store.queries.ListPendingOutboxMessages(ctx, int32(batchSize))
	if err != nil {
		return nil, err
	}
	messages := make([]Message, 0, len(rows))
	for _, row := range rows {
		if int(row.Attempts) >= MaximumAttempts {
			continue
		}
		messages = append(messages, Message{
			ID: uuidText(row.ID), Topic: row.Topic, Key: row.MessageKey,
			Payload: json.RawMessage(row.Payload), Attempts: int(row.Attempts),
		})
	}
	return messages, nil
}

func (store *PostgresStore) MarkPublished(ctx context.Context, id string) error {
	identifier, err := parseUUID(id)
	if err != nil {
		return err
	}
	return store.queries.MarkOutboxMessagePublished(ctx, identifier)
}

func (store *PostgresStore) MarkFailed(ctx context.Context, id, reason string) error {
	identifier, err := parseUUID(id)
	if err != nil {
		return err
	}
	// The reason is truncated so a verbose driver error cannot bloat the row.
	if len(reason) > 500 {
		reason = reason[:500]
	}
	return store.queries.MarkOutboxMessageFailed(ctx, gen.MarkOutboxMessageFailedParams{
		ID: identifier, LastError: pgtype.Text{String: reason, Valid: true},
	})
}

func (store *PostgresStore) PruneBefore(ctx context.Context, cutoff time.Time) error {
	return store.queries.DeletePublishedOutboxMessagesBefore(ctx, pgtype.Timestamptz{Time: cutoff.UTC(), Valid: true})
}

func parseUUID(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	err := id.Scan(value)
	return id, err
}

func uuidText(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	value, err := id.Value()
	if err != nil {
		return ""
	}
	text, _ := value.(string)
	return text
}
