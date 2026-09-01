// Package telemetry owns measurement history. Measurements are the one data
// class that grows without bound, so they are stored append-only and separately
// from the farm configuration document rather than rewritten with it.
package telemetry

import (
	"context"
	"errors"
	"time"
)

// Quality mirrors the measurement quality vocabulary in the schema.
type Quality string

const (
	QualityGood        Quality = "good"
	QualitySuspect     Quality = "suspect"
	QualityStale       Quality = "stale"
	QualityCalibrating Quality = "calibrating"
	QualityFault       Quality = "fault"
	QualityUnknown     Quality = "unknown"
)

var qualities = map[Quality]bool{
	QualityGood: true, QualitySuspect: true, QualityStale: true,
	QualityCalibrating: true, QualityFault: true, QualityUnknown: true,
}

// Valid reports whether the quality is one the schema accepts.
func (quality Quality) Valid() bool { return qualities[quality] }

// ErrInvalidMeasurement reports a sample that must not be persisted.
var ErrInvalidMeasurement = errors.New("invalid measurement")

// Measurement is one sample on one logical channel.
type Measurement struct {
	ID             string    `json:"id,omitempty"`
	ChannelID      string    `json:"channel_id"`
	ObservedAt     time.Time `json:"observed_at"`
	ReceivedAt     time.Time `json:"received_at,omitempty"`
	Sequence       *int64    `json:"sequence,omitempty"`
	Value          float64   `json:"value"`
	Unit           string    `json:"unit"`
	Quality        Quality   `json:"quality"`
	SourceDeviceID string    `json:"source_device_id,omitempty"`
}

// Validate rejects a sample that could not round-trip through the schema.
func (measurement Measurement) Validate() error {
	switch {
	case measurement.ChannelID == "":
		return errors.New("measurement needs a channel")
	case measurement.ObservedAt.IsZero():
		return errors.New("measurement needs an observation time")
	case measurement.Unit == "":
		return errors.New("measurement needs a unit")
	case !measurement.Quality.Valid():
		return errors.New("measurement quality is not recognised")
	}
	return nil
}

// Bucket is one downsampled interval of a channel's history.
type Bucket struct {
	StartedAt time.Time `json:"started_at"`
	Average   float64   `json:"average"`
	Minimum   float64   `json:"minimum"`
	Maximum   float64   `json:"maximum"`
	Samples   int64     `json:"samples"`
}

// Query bounds a history read. Every read is bounded so a client cannot ask for
// an unlimited scan.
type Query struct {
	ChannelID string
	From      time.Time
	To        time.Time
	Limit     int
	// BucketSeconds requests server-side downsampling when greater than zero.
	BucketSeconds int
}

// MaximumLimit caps how many rows any single history read may return.
const MaximumLimit = 5000

// DefaultLimit applies when a caller does not ask for a specific size.
const DefaultLimit = 500

// Normalise clamps a query into the supported range and fills sensible defaults.
func (query Query) Normalise(now time.Time) Query {
	if query.To.IsZero() {
		query.To = now
	}
	if query.From.IsZero() {
		query.From = query.To.Add(-24 * time.Hour)
	}
	if query.From.After(query.To) {
		query.From, query.To = query.To, query.From
	}
	if query.Limit <= 0 {
		query.Limit = DefaultLimit
	}
	if query.Limit > MaximumLimit {
		query.Limit = MaximumLimit
	}
	if query.BucketSeconds < 0 {
		query.BucketSeconds = 0
	}
	return query
}

// Store is the persistence boundary for measurement history.
type Store interface {
	// Append persists samples, ignoring duplicates of an already-stored
	// (channel, observation time, sequence) triple so a device retry is safe.
	Append(ctx context.Context, measurements []Measurement) (int, error)
	History(ctx context.Context, query Query) ([]Measurement, error)
	Downsampled(ctx context.Context, query Query) ([]Bucket, error)
	Latest(ctx context.Context) ([]Measurement, error)
	// Recent returns the newest samples across every channel, bounded by limit.
	Recent(ctx context.Context, limit int) ([]Measurement, error)
	// Prune deletes samples observed before the cutoff and reports how many rows
	// were removed.
	Prune(ctx context.Context, cutoff time.Time) (int64, error)
}
