package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/platform/database/dbtest"
	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

var origin = time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)

func TestPostgresTelemetryRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool, "latest_measurements", "measurements", "device_channels", "facilities")
	channelID := dbtest.SeedChannel(t, pool, "air_temperature")
	store := telemetry.NewPostgresStore(pool)
	ctx := context.Background()

	batch := make([]telemetry.Measurement, 0, 60)
	for index := 0; index < 60; index++ {
		batch = append(batch, telemetry.Measurement{
			ChannelID: channelID, ObservedAt: origin.Add(time.Duration(index) * time.Minute),
			Value: 20 + float64(index%5), Unit: "degC", Quality: telemetry.QualityGood,
		})
	}
	written, err := store.Append(ctx, batch)
	if err != nil {
		t.Fatal(err)
	}
	if written != 60 {
		t.Fatalf("wrote %d of 60 samples", written)
	}

	// A device retry must not create duplicate history.
	repeated, err := store.Append(ctx, batch)
	if err != nil {
		t.Fatal(err)
	}
	if repeated != 0 {
		t.Fatalf("re-sending the same batch stored %d duplicate samples", repeated)
	}

	history, err := store.History(ctx, telemetry.Query{
		ChannelID: channelID, From: origin, To: origin.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 10 {
		t.Fatalf("range query returned %d samples, want 10", len(history))
	}
	// NUMERIC must round-trip a decimal value exactly.
	if history[0].Value != batch[9].Value {
		t.Fatalf("value = %v, want %v", history[0].Value, batch[9].Value)
	}

	latest, err := store.Latest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 1 || !latest[0].ObservedAt.Equal(batch[59].ObservedAt) {
		t.Fatalf("latest projection = %+v", latest)
	}

	buckets, err := store.Downsampled(ctx, telemetry.Query{
		ChannelID: channelID, From: origin, To: origin.Add(time.Hour), BucketSeconds: 600,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 6 {
		t.Fatalf("600s buckets over an hour = %d, want 6", len(buckets))
	}
	for _, bucket := range buckets {
		if bucket.Samples != 10 || bucket.Minimum > bucket.Average || bucket.Average > bucket.Maximum {
			t.Fatalf("bucket is inconsistent: %+v", bucket)
		}
	}

	removed, err := store.Prune(ctx, origin.Add(30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if removed != 30 {
		t.Fatalf("pruned %d rows, want 30", removed)
	}
}

// TestLatestProjectionDoesNotGoBackwards guards against an out-of-order arrival
// overwriting a newer reading with an older one.
func TestLatestProjectionDoesNotGoBackwards(t *testing.T) {
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool, "latest_measurements", "measurements", "device_channels", "facilities")
	channelID := dbtest.SeedChannel(t, pool, "water_temperature")
	store := telemetry.NewPostgresStore(pool)
	ctx := context.Background()

	newest := telemetry.Measurement{ChannelID: channelID, ObservedAt: origin.Add(time.Hour), Value: 21, Unit: "degC", Quality: telemetry.QualityGood}
	older := telemetry.Measurement{ChannelID: channelID, ObservedAt: origin, Value: 99, Unit: "degC", Quality: telemetry.QualityGood}
	if _, err := store.Append(ctx, []telemetry.Measurement{newest}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(ctx, []telemetry.Measurement{older}); err != nil {
		t.Fatal(err)
	}

	latest, err := store.Latest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 1 || latest[0].Value != 21 {
		t.Fatalf("a late-arriving older sample replaced the newest reading: %+v", latest)
	}
}

func TestUnknownChannelIsRejectedRatherThanStored(t *testing.T) {
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool, "latest_measurements", "measurements", "device_channels", "facilities")
	store := telemetry.NewPostgresStore(pool)

	_, err := store.Append(context.Background(), []telemetry.Measurement{{
		ChannelID: "00000000-0000-0000-0000-0000000000ff", ObservedAt: origin,
		Value: 20, Unit: "degC", Quality: telemetry.QualityGood,
	}})
	if err == nil {
		t.Fatal("a measurement for an unknown channel was accepted")
	}
}
