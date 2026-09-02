-- name: UpsertFacility :exec
INSERT INTO facilities (id, name, timezone)
VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(timezone))
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name, timezone = EXCLUDED.timezone;

-- name: UpsertDevice :exec
INSERT INTO devices (id, facility_id, name, type, status)
VALUES (sqlc.arg(id), sqlc.arg(facility_id), sqlc.arg(name), sqlc.arg(type), sqlc.arg(status))
ON CONFLICT (id) DO UPDATE
SET facility_id = EXCLUDED.facility_id, name = EXCLUDED.name,
    type = EXCLUDED.type, status = EXCLUDED.status;

-- name: RetireConflictingDeviceChannelKey :exec
UPDATE device_channels
SET key = 'retired:' || id::text || ':' || key
WHERE facility_id = sqlc.arg(facility_id)
  AND key = sqlc.arg(key)
  AND id <> sqlc.arg(id);

-- name: UpsertDeviceChannel :exec
INSERT INTO device_channels (
  id, facility_id, entity_type, entity_id, key, name, kind, value_type,
  unit, dimension, physical_minimum, physical_maximum, safe_minimum, safe_maximum, stale_after
) VALUES (
  sqlc.arg(id), sqlc.arg(facility_id), sqlc.arg(entity_type), sqlc.arg(entity_id),
  sqlc.arg(key), sqlc.arg(name), sqlc.arg(kind), sqlc.arg(value_type),
  sqlc.narg(unit), sqlc.narg(dimension), sqlc.narg(physical_minimum), sqlc.narg(physical_maximum),
  sqlc.narg(safe_minimum), sqlc.narg(safe_maximum), sqlc.arg(stale_after)
)
ON CONFLICT (id) DO UPDATE
SET facility_id = EXCLUDED.facility_id, entity_type = EXCLUDED.entity_type,
    entity_id = EXCLUDED.entity_id, key = EXCLUDED.key, name = EXCLUDED.name,
    kind = EXCLUDED.kind, value_type = EXCLUDED.value_type, unit = EXCLUDED.unit,
    dimension = EXCLUDED.dimension, physical_minimum = EXCLUDED.physical_minimum,
    physical_maximum = EXCLUDED.physical_maximum, safe_minimum = EXCLUDED.safe_minimum,
    safe_maximum = EXCLUDED.safe_maximum, stale_after = EXCLUDED.stale_after;

-- name: CountDeviceChannels :one
SELECT count(*)::bigint FROM device_channels;
