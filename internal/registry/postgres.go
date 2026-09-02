package registry

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jdanielsobrado/grownerve/internal/platform/database/gen"
)

type PostgresProjector struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

func NewPostgresProjector(pool *pgxpool.Pool) *PostgresProjector {
	return &PostgresProjector{pool: pool, queries: gen.New(pool)}
}

func (projector *PostgresProjector) Project(ctx context.Context, document Document) error {
	transaction, err := projector.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if err := ProjectWithQueries(ctx, projector.queries.WithTx(transaction), document); err != nil {
		return err
	}
	return transaction.Commit(ctx)
}

// ProjectWithQueries projects a registry through the supplied query set. When
// the queries are transaction-bound, callers can commit the registry atomically
// with the configuration document and imported measurements.
func ProjectWithQueries(ctx context.Context, queries *gen.Queries, document Document) error {
	if err := document.Validate(); err != nil {
		return err
	}
	if len(document.Facilities) == 0 {
		return nil
	}

	owner, err := uuidOf(document.Facilities[0].ID)
	if err != nil {
		return &InvalidError{"facility id must be a UUID"}
	}
	for _, facility := range document.Facilities {
		id, err := uuidOf(facility.ID)
		if err != nil {
			return &InvalidError{"facility id must be a UUID"}
		}
		if err := queries.UpsertFacility(ctx, gen.UpsertFacilityParams{
			ID: id, Name: nonEmpty(facility.Name, "Facility"), Timezone: nonEmpty(facility.Timezone, "UTC"),
		}); err != nil {
			return fmt.Errorf("project facility %s: %w", facility.ID, err)
		}
	}
	for _, device := range document.Devices {
		id, err := uuidOf(device.ID)
		if err != nil {
			return &InvalidError{"device id must be a UUID"}
		}
		if err := queries.UpsertDevice(ctx, gen.UpsertDeviceParams{
			ID: id, FacilityID: owner, Name: nonEmpty(device.Name, "Device"),
			Type: device.DeviceType(), Status: device.DeviceStatus(),
		}); err != nil {
			return fmt.Errorf("project device %s: %w", device.ID, err)
		}
	}
	for _, channel := range document.Channels {
		id, err := uuidOf(channel.ID)
		if err != nil {
			return &InvalidError{"channel id must be a UUID"}
		}
		entityID, err := uuidOf(channel.EntityID)
		if err != nil {
			entityID = owner
		}
		if err := queries.RetireConflictingDeviceChannelKey(ctx, gen.RetireConflictingDeviceChannelKeyParams{
			FacilityID: owner, Key: channel.Key, ID: id,
		}); err != nil {
			return fmt.Errorf("retire historical channel key %s: %w", channel.Key, err)
		}
		params := gen.UpsertDeviceChannelParams{
			ID: id, FacilityID: owner,
			EntityType: nonEmpty(channel.EntityType, "facility"), EntityID: entityID,
			Key: channel.Key, Name: nonEmpty(channel.Name, channel.Key),
			Kind: channel.Kind, ValueType: channel.ValueType,
			StaleAfter: intervalOf(channel.StaleAfter().Microseconds()),
		}
		if channel.Unit != "" {
			params.Unit = pgtype.Text{String: channel.Unit, Valid: true}
		}
		if channel.Dimension != "" {
			params.Dimension = pgtype.Text{String: channel.Dimension, Valid: true}
		}
		params.PhysicalMinimum = numericOf(channel.Minimum)
		params.PhysicalMaximum = numericOf(channel.Maximum)
		params.SafeMinimum = numericOf(channel.SafeMinimum)
		params.SafeMaximum = numericOf(channel.SafeMaximum)
		if err := queries.UpsertDeviceChannel(ctx, params); err != nil {
			return fmt.Errorf("project channel %s: %w", channel.Key, err)
		}
	}
	return nil
}

func uuidOf(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	err := id.Scan(value)
	return id, err
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func intervalOf(microseconds int64) pgtype.Interval {
	return pgtype.Interval{Microseconds: microseconds, Valid: true}
}

func numericOf(value *float64) pgtype.Numeric {
	var numeric pgtype.Numeric
	if value == nil {
		return numeric
	}
	if err := numeric.Scan(fmt.Sprintf("%v", *value)); err != nil {
		return pgtype.Numeric{}
	}
	return numeric
}
