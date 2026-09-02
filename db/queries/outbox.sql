-- name: EnqueueOutboxMessage :one
INSERT INTO outbox_messages (topic, message_key, payload)
VALUES (sqlc.arg(topic), sqlc.arg(message_key), sqlc.arg(payload))
ON CONFLICT (topic, message_key) DO UPDATE
SET payload = outbox_messages.payload
RETURNING id;

-- name: ListPendingOutboxMessages :many
SELECT id, topic, message_key, payload, attempts
FROM outbox_messages
WHERE published_at IS NULL
ORDER BY created_at
LIMIT sqlc.arg(batch_size);

-- name: MarkOutboxMessagePublished :exec
UPDATE outbox_messages
SET published_at = clock_timestamp(), attempts = attempts + 1, last_error = NULL
WHERE id = sqlc.arg(id);

-- name: MarkOutboxMessageFailed :exec
UPDATE outbox_messages
SET attempts = attempts + 1, last_error = sqlc.arg(last_error)
WHERE id = sqlc.arg(id);

-- name: DeletePublishedOutboxMessagesBefore :exec
DELETE FROM outbox_messages
WHERE published_at IS NOT NULL AND published_at < sqlc.arg(cutoff);
