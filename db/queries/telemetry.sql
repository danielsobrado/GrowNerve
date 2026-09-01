-- name: InsertMeasurement :one
INSERT INTO measurements (channel_id, observed_at, sequence, value, unit, quality, source_device_id)
VALUES (
  sqlc.arg(channel_id), sqlc.arg(observed_at), sqlc.narg(sequence),
  sqlc.arg(value), sqlc.arg(unit), sqlc.arg(quality), sqlc.narg(source_device_id)
)
ON CONFLICT (channel_id, observed_at, sequence) DO NOTHING
RETURNING id;

-- name: UpsertLatestMeasurement :exec
INSERT INTO latest_measurements (channel_id, measurement_id, observed_at, value, unit, quality)
VALUES (sqlc.arg(channel_id), sqlc.arg(measurement_id), sqlc.arg(observed_at), sqlc.arg(value), sqlc.arg(unit), sqlc.arg(quality))
ON CONFLICT (channel_id) DO UPDATE
SET measurement_id = EXCLUDED.measurement_id,
    observed_at = EXCLUDED.observed_at,
    value = EXCLUDED.value,
    unit = EXCLUDED.unit,
    quality = EXCLUDED.quality
WHERE latest_measurements.observed_at <= EXCLUDED.observed_at;

-- name: ListMeasurements :many
SELECT id, channel_id, observed_at, received_at, sequence, value, unit, quality, source_device_id
FROM measurements
WHERE channel_id = sqlc.arg(channel_id)
  AND observed_at >= sqlc.arg(from_time)
  AND observed_at < sqlc.arg(to_time)
ORDER BY observed_at DESC
LIMIT sqlc.arg(row_limit);

-- name: ListDownsampledMeasurements :many
SELECT
  (to_timestamp(floor(extract(epoch FROM observed_at) / sqlc.arg(bucket_seconds)::double precision) * sqlc.arg(bucket_seconds)::double precision))::timestamptz AS bucket,
  avg(value)::double precision AS average_value,
  min(value)::double precision AS minimum_value,
  max(value)::double precision AS maximum_value,
  count(*)::bigint AS sample_count
FROM measurements
WHERE channel_id = sqlc.arg(channel_id)
  AND observed_at >= sqlc.arg(from_time)
  AND observed_at < sqlc.arg(to_time)
GROUP BY bucket
ORDER BY bucket DESC
LIMIT sqlc.arg(row_limit);

-- name: ListLatestMeasurements :many
SELECT channel_id, measurement_id, observed_at, value, unit, quality
FROM latest_measurements;

-- name: DeleteMeasurementsBefore :execrows
DELETE FROM measurements WHERE observed_at < sqlc.arg(cutoff);

-- name: ListRecentMeasurements :many
SELECT id, channel_id, observed_at, received_at, sequence, value, unit, quality, source_device_id
FROM measurements
ORDER BY observed_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: CountMeasurements :one
SELECT count(*)::bigint FROM measurements;
