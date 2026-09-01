-- name: GetBrowserCompatibleState :one
SELECT state, version FROM browser_compatible_states WHERE singleton = TRUE;

-- name: InsertBrowserCompatibleState :one
INSERT INTO browser_compatible_states (singleton, state, version)
VALUES (TRUE, sqlc.arg(state), 1)
ON CONFLICT (singleton) DO NOTHING
RETURNING version;

-- name: UpdateBrowserCompatibleStateIfVersion :one
UPDATE browser_compatible_states
SET state = sqlc.arg(state),
    version = version + 1,
    updated_at = clock_timestamp()
WHERE singleton = TRUE AND version = sqlc.arg(version)
RETURNING version;

-- name: OverwriteBrowserCompatibleState :one
INSERT INTO browser_compatible_states (singleton, state, version)
VALUES (TRUE, sqlc.arg(state), 1)
ON CONFLICT (singleton) DO UPDATE
SET state = EXCLUDED.state,
    version = browser_compatible_states.version + 1,
    updated_at = clock_timestamp()
RETURNING version;
