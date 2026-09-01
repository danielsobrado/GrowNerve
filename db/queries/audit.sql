-- name: InsertAuditEntry :exec
INSERT INTO audit_log (actor_id, action, target_type, target_id, occurred_at, correlation_id, detail)
VALUES (
  sqlc.narg(actor_id), sqlc.arg(action), sqlc.arg(target_type), sqlc.narg(target_id),
  sqlc.arg(occurred_at), sqlc.narg(correlation_id), sqlc.arg(detail)
);

-- name: ListAuditEntries :many
SELECT id, actor_id, action, target_type, target_id, occurred_at, correlation_id, detail
FROM audit_log
ORDER BY occurred_at DESC, id DESC
LIMIT sqlc.arg(row_limit);
