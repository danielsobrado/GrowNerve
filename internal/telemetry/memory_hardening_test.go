package telemetry

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestMeasurementRejectsNegativeSequenceAndNonFiniteValue(t *testing.T) {
	observedAt := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
	negative := int64(-1)
	for name, measurement := range map[string]Measurement{
		"negative sequence": {
			ChannelID: "channel", ObservedAt: observedAt, Sequence: &negative,
			Value: 1, Unit: "unit", Quality: QualityGood,
		},
		"NaN value": {
			ChannelID: "channel", ObservedAt: observedAt,
			Value: math.NaN(), Unit: "unit", Quality: QualityGood,
		},
		"infinite value": {
			ChannelID: "channel", ObservedAt: observedAt,
			Value: math.Inf(1), Unit: "unit", Quality: QualityGood,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := measurement.Validate(); err == nil {
				t.Fatal("invalid measurement was accepted")
			}
		})
	}
}

func TestMemoryRecentUsesObservationOrderNotArrivalOrder(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(10)
	base := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
	measurements := []Measurement{
		{ChannelID: "channel", ObservedAt: base.Add(2 * time.Minute), Value: 2, Unit: "u", Quality: QualityGood},
		{ChannelID: "channel", ObservedAt: base, Value: 0, Unit: "u", Quality: QualityGood},
		{ChannelID: "channel", ObservedAt: base.Add(time.Minute), Value: 1, Unit: "u", Quality: QualityGood},
	}
	if _, err := store.Append(ctx, measurements); err != nil {
		t.Fatal(err)
	}

	recent, err := store.Recent(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 || recent[0].Value != 1 || recent[1].Value != 2 {
		t.Fatalf("recent = %+v, want the two newest observations in chronological order", recent)
	}
}

func TestMemoryCapacityEvictsOldestObservation(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(2)
	base := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
	if _, err := store.Append(ctx, []Measurement{
		{ChannelID: "channel", ObservedAt: base.Add(2 * time.Minute), Value: 2, Unit: "u", Quality: QualityGood},
		{ChannelID: "channel", ObservedAt: base.Add(time.Minute), Value: 1, Unit: "u", Quality: QualityGood},
		{ChannelID: "channel", ObservedAt: base, Value: 0, Unit: "u", Quality: QualityGood},
	}); err != nil {
		t.Fatal(err)
	}

	recent, err := store.Recent(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 || recent[0].Value != 1 || recent[1].Value != 2 {
		t.Fatalf("capacity kept the wrong observations: %+v", recent)
	}
}
