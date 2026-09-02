WITH ranked AS (
  SELECT
    id,
    row_number() OVER (
      PARTITION BY topic, message_key
      ORDER BY (published_at IS NULL), COALESCE(published_at, created_at), created_at, id
    ) AS duplicate_rank
  FROM outbox_messages
)
DELETE FROM outbox_messages
WHERE id IN (
  SELECT id
  FROM ranked
  WHERE duplicate_rank > 1
);

CREATE UNIQUE INDEX ux_outbox_message_key
  ON outbox_messages(topic, message_key);
