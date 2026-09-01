package telemetry

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jdanielsobrado/grownerve/internal/platform/database/gen"
)

type PostgresStore struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool, queries: gen.New(pool)}
}

func (store *PostgresStore) Append(ctx context.Context, measurements []Measurement) (int, error) {
	if len(measurements) == 0 {
		return 0, nil
	}
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	written, err := AppendWithQueries(ctx, store.queries.WithTx(transaction), measurements)
	if err != nil {
		return written, err
	}
	return written, transaction.Commit(ctx)
}

// AppendWithQueries writes a measurement batch through the supplied query set.
// A transaction-bound query set lets state import commit history, registry, and
// configuration as one atomic operation.
func AppendWithQueries(ctx context.Context, queries *gen.Queries, measurements []Measurement) (int, error) {
	written := 0
	for _, measurement := range measurements {
		if err := measurement.Validate(); err != nil {
			return written, err
		}
		channelID, err := parseUUID(measurement.ChannelID)
		if err != nil {
			return written, err
		}
		value, err := numericOf(measurement.Value)
		if err != nil {
			return written, err
		}
		params := gen.InsertMeasurementParams{
			ChannelID:  channelID,
			ObservedAt: pgtype.Timestamptz{Time: measurement.ObservedAt.UTC(), Valid: true},
			Value:      value,
			Unit:       measurement.Unit,
			Quality:    string(measurement.Quality),
		}
		if measurement.Sequence != nil {
			params.Sequence = pgtype.Int8{Int64: *measurement.Sequence, Valid: true}
		}
		if measurement.SourceDeviceID != "" {
			deviceID, err := parseUUID(measurement.SourceDeviceID)
			if err != nil {
				return written, err
			}
			params.SourceDeviceID = deviceID
		}
		id, err := queries.InsertMeasurement(ctx, params)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return written, err
		}
		if err := queries.UpsertLatestMeasurement(ctx, gen.UpsertLatestMeasurementParams{
			ChannelID: channelID, MeasurementID: id, ObservedAt: params.ObservedAt,
			Value: value, Unit: measurement.Unit, Quality: string(measurement.Quality),
		}); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}

func (store *PostgresStore) History(ctx context.Context, query Query) ([]Measurement, error) {
	query = query.Normalise(time.Now().UTC())
	channelID, err := parseUUID(query.ChannelID)
	if err != nil {
		return nil, err
	}
	rows, err := store.queries.ListMeasurements(ctx, gen.ListMeasurementsParams{
		ChannelID: channelID,
		FromTime:  pgtype.Timestamptz{Time: query.From, Valid: true},
		ToTime:    pgtype.Timestamptz{Time: query.To, Valid: true},
		RowLimit:  int32(query.Limit),
	})
	if err != nil {
		return nil, err
	}
	measurements := make([]Measurement, 0, len(rows))
	for _, row := range rows {
		measurements = append(measurements, measurementOf(row))
	}
	return measurements, nil
}

func (store *PostgresStore) Downsampled(ctx context.Context, query Query) ([]Bucket, error) {
	query = query.Normalise(time.Now().UTC())
	if query.BucketSeconds <= 0 {
		query.BucketSeconds = 60
	}
	channelID, err := parseUUID(query.ChannelID)
	if err != nil {
		return nil, err
	}
	rows, err := store.queries.ListDownsampledMeasurements(ctx, gen.ListDownsampledMeasurementsParams{
		BucketSeconds: float64(query.BucketSeconds),
		ChannelID:     channelID,
		FromTime:      pgtype.Timestamptz{Time: query.From, Valid: true},
		ToTime:        pgtype.Timestamptz{Time: query.To, Valid: true},
		RowLimit:      int32(query.Limit),
	})
	if err != nil {
		return nil, err
	}
	buckets := make([]Bucket, 0, len(rows))
	for _, row := range rows {
		buckets = append(buckets, Bucket{
			StartedAt: row.Bucket.Time.UTC(), Average: row.AverageValue,
			Minimum: row.MinimumValue, Maximum: row.MaximumValue, Samples: row.SampleCount,
		})
	}
	return buckets, nil
}

func (store *PostgresStore) Latest(ctx context.Context) ([]Measurement, error) {
	rows, err := store.queries.ListLatestMeasurements(ctx)
	if err != nil {
		return nil, err
	}
	measurements := make([]Measurement, 0, len(rows))
	for _, row := range rows {
		measurements = append(measurements, Measurement{
			ID:         uuidText(row.MeasurementID),
			ChannelID:  uuidText(row.ChannelID),
			ObservedAt: row.ObservedAt.Time.UTC(),
			Value:      floatOf(row.Value),
			Unit:       row.Unit,
			Quality:    Quality(row.Quality),
		})
	}
	return measurements, nil
}

func (store *PostgresStore) Recent(ctx context.Context, limit int) ([]Measurement, error) {
	if limit <= 0 || limit > MaximumLimit {
		limit = MaximumLimit
	}
	rows, err := store.queries.ListRecentMeasurements(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	measurements := make([]Measurement, 0, len(rows))
	for index := len(rows) - 1; index >= 0; index-- {
		measurements = append(measurements, measurementOf(rows[index]))
	}
	return measurements, nil
}

func (store *PostgresStore) Prune(ctx context.Context, cutoff time.Time) (int64, error) {
	return store.queries.DeleteMeasurementsBefore(ctx, pgtype.Timestamptz{Time: cutoff.UTC(), Valid: true})
}

func measurementOf(row gen.Measurement) Measurement {
	measurement := Measurement{
		ID:         uuidText(row.ID),
		ChannelID:  uuidText(row.ChannelID),
		ObservedAt: row.ObservedAt.Time.UTC(),
		ReceivedAt: row.ReceivedAt.Time.UTC(),
		Value:      floatOf(row.Value),
		Unit:       row.Unit,
		Quality:    Quality(row.Quality),
	}
	if row.Sequence.Valid {
		sequence := row.Sequence.Int64
		measurement.Sequence = &sequence
	}
	if row.SourceDeviceID.Valid {
		measurement.SourceDeviceID = uuidText(row.SourceDeviceID)
	}
	return measurement
}

func parseUUID(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		return id, ErrInvalidMeasurement
	}
	return id, nil
}

func uuidText(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	text, err := id.Value()
	if err != nil {
		return ""
	}
	value, _ := text.(string)
	return value
}

func numericOf(value float64) (pgtype.Numeric, error) {
	var numeric pgtype.Numeric
	if err := numeric.Scan(strconv.FormatFloat(value, 'f', -1, 64)); err != nil {
		return numeric, ErrInvalidMeasurement
	}
	return numeric, nil
}

func floatOf(numeric pgtype.Numeric) float64 {
	if !numeric.Valid {
		return 0
	}
	value, err := numeric.Float64Value()
	if err != nil || !value.Valid {
		return 0
	}
	return value.Float64
}
