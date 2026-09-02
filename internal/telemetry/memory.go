package telemetry

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"time"
)

// MemoryStore keeps measurements in process. It backs tests and the
// document-only deployment path, and applies the same duplicate rejection and
// bounding rules as PostgreSQL so behaviour does not diverge between them.
type MemoryStore struct {
	mu       sync.RWMutex
	byKey    map[string]struct{}
	ordered  []Measurement
	capacity int
}

// NewMemoryStore bounds retention by sample count, because an in-process store
// has no other backstop against unbounded growth.
func NewMemoryStore(capacity int) *MemoryStore {
	if capacity <= 0 {
		capacity = 50_000
	}
	return &MemoryStore{byKey: map[string]struct{}{}, capacity: capacity}
}

func duplicateKey(measurement Measurement) string {
	sequence := "-"
	if measurement.Sequence != nil {
		sequence = strconv.FormatInt(*measurement.Sequence, 10)
	}
	return measurement.ChannelID + "|" + measurement.ObservedAt.UTC().Format(time.RFC3339Nano) + "|" + sequence
}

func newerMeasurement(left, right Measurement) bool {
	if !left.ObservedAt.Equal(right.ObservedAt) {
		return left.ObservedAt.After(right.ObservedAt)
	}
	leftSequence, rightSequence := int64(-1), int64(-1)
	if left.Sequence != nil {
		leftSequence = *left.Sequence
	}
	if right.Sequence != nil {
		rightSequence = *right.Sequence
	}
	return leftSequence > rightSequence
}

func (store *MemoryStore) Append(_ context.Context, measurements []Measurement) (int, error) {
	for _, measurement := range measurements {
		if err := measurement.Validate(); err != nil {
			return 0, err
		}
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	written := 0
	for _, measurement := range measurements {
		key := duplicateKey(measurement)
		if _, seen := store.byKey[key]; seen {
			continue
		}
		if measurement.ReceivedAt.IsZero() {
			measurement.ReceivedAt = time.Now().UTC()
		}
		store.byKey[key] = struct{}{}
		store.ordered = append(store.ordered, measurement)
		written++
	}
	if overflow := len(store.ordered) - store.capacity; overflow > 0 {
		sort.SliceStable(store.ordered, func(i, j int) bool { return newerMeasurement(store.ordered[i], store.ordered[j]) })
		for _, evicted := range store.ordered[len(store.ordered)-overflow:] {
			delete(store.byKey, duplicateKey(evicted))
		}
		store.ordered = append([]Measurement(nil), store.ordered[:len(store.ordered)-overflow]...)
	}
	return written, nil
}

func (store *MemoryStore) History(_ context.Context, query Query) ([]Measurement, error) {
	query = query.Normalise(time.Now().UTC())
	store.mu.RLock()
	defer store.mu.RUnlock()
	var matched []Measurement
	for _, measurement := range store.ordered {
		if measurement.ChannelID != query.ChannelID {
			continue
		}
		if measurement.ObservedAt.Before(query.From) || !measurement.ObservedAt.Before(query.To) {
			continue
		}
		matched = append(matched, measurement)
	}
	sort.SliceStable(matched, func(i, j int) bool { return newerMeasurement(matched[i], matched[j]) })
	if len(matched) > query.Limit {
		matched = matched[:query.Limit]
	}
	return matched, nil
}

func (store *MemoryStore) Downsampled(ctx context.Context, query Query) ([]Bucket, error) {
	query = query.Normalise(time.Now().UTC())
	if query.BucketSeconds <= 0 {
		query.BucketSeconds = 60
	}
	samples, err := store.History(ctx, Query{ChannelID: query.ChannelID, From: query.From, To: query.To, Limit: MaximumLimit})
	if err != nil {
		return nil, err
	}
	width := int64(query.BucketSeconds)
	aggregated := map[int64]*Bucket{}
	for _, sample := range samples {
		start := sample.ObservedAt.Unix() / width * width
		bucket, found := aggregated[start]
		if !found {
			bucket = &Bucket{StartedAt: time.Unix(start, 0).UTC(), Minimum: sample.Value, Maximum: sample.Value}
			aggregated[start] = bucket
		}
		bucket.Average = (bucket.Average*float64(bucket.Samples) + sample.Value) / float64(bucket.Samples+1)
		bucket.Samples++
		bucket.Minimum = min(bucket.Minimum, sample.Value)
		bucket.Maximum = max(bucket.Maximum, sample.Value)
	}
	buckets := make([]Bucket, 0, len(aggregated))
	for _, bucket := range aggregated {
		buckets = append(buckets, *bucket)
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].StartedAt.After(buckets[j].StartedAt) })
	if len(buckets) > query.Limit {
		buckets = buckets[:query.Limit]
	}
	return buckets, nil
}

func (store *MemoryStore) Latest(_ context.Context) ([]Measurement, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	newest := map[string]Measurement{}
	for _, measurement := range store.ordered {
		if existing, found := newest[measurement.ChannelID]; found && !newerMeasurement(measurement, existing) {
			continue
		}
		newest[measurement.ChannelID] = measurement
	}
	latest := make([]Measurement, 0, len(newest))
	for _, measurement := range newest {
		latest = append(latest, measurement)
	}
	sort.Slice(latest, func(i, j int) bool { return latest[i].ChannelID < latest[j].ChannelID })
	return latest, nil
}

func (store *MemoryStore) Prune(_ context.Context, cutoff time.Time) (int64, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	kept := store.ordered[:0]
	var removed int64
	for _, measurement := range store.ordered {
		if measurement.ObservedAt.Before(cutoff) {
			delete(store.byKey, duplicateKey(measurement))
			removed++
			continue
		}
		kept = append(kept, measurement)
	}
	store.ordered = append([]Measurement(nil), kept...)
	return removed, nil
}

// Recent returns the newest samples across all channels, bounded by limit. It
// backs the compatibility projection that the browser adapter still reads.
func (store *MemoryStore) Recent(_ context.Context, limit int) ([]Measurement, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	if limit <= 0 || limit > len(store.ordered) {
		limit = len(store.ordered)
	}
	ordered := append([]Measurement(nil), store.ordered...)
	sort.SliceStable(ordered, func(i, j int) bool { return newerMeasurement(ordered[i], ordered[j]) })
	ordered = ordered[:limit]
	for left, right := 0, len(ordered)-1; left < right; left, right = left+1, right-1 {
		ordered[left], ordered[right] = ordered[right], ordered[left]
	}
	return ordered, nil
}
