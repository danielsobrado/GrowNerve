-- name: GetBrowserCompatibleState :one
SELECT state FROM browser_compatible_states WHERE singleton = TRUE;

-- name: SaveBrowserCompatibleState :one
INSERT INTO browser_compatible_states (singleton, state, version)
VALUES (TRUE, sqlc.arg(state), 1)
ON CONFLICT (singleton) DO UPDATE
SET state = EXCLUDED.state,
    version = browser_compatible_states.version + 1,
    updated_at = clock_timestamp()
RETURNING version;

-- name: ListPendingOutboxMessages :many
SELECT id, topic, message_key, payload, attempts
FROM outbox_messages
WHERE published_at IS NULL
ORDER BY created_at
LIMIT sqlc.arg(batch_size);
