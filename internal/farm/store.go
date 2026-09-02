package farm

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"sync"
	"time"
)

var (
	// ErrNotFound reports that no farm state has been stored yet.
	ErrNotFound = errors.New("farm state not found")
	// ErrVersionConflict reports that the state changed between load and save.
	ErrVersionConflict = errors.New("farm state version conflict")
	// ErrInvalidState reports content that cannot be stored as a JSON document.
	ErrInvalidState = errors.New("farm state is not valid JSON")
)

// AnyVersion saves unconditionally. Use it only where the caller genuinely owns
// the whole document; every read-modify-write must pass the version it loaded.
const AnyVersion int64 = -1

// NoVersion is the version reported for state that does not exist yet.
const NoVersion int64 = 0

// Store persists the farm state document with an optimistic version counter.
// Save is a compare-and-swap: it must reject a write whose expected version no
// longer matches, so concurrent writers conflict instead of losing updates.
type Store interface {
	Load(context.Context) (json.RawMessage, int64, error)
	Save(ctx context.Context, state json.RawMessage, expected int64) (int64, error)
}

// mutateAttempts bounds the optimistic retry loop so a hot writer cannot spin
// forever. It is generous because losing a write is far worse than retrying one:
// under real contention several writers routinely lose a round before winning.
const mutateAttempts = 64

// mutateBackoffCeiling caps the wait between attempts.
const mutateBackoffCeiling = 50 * time.Millisecond

// Mutate applies a read-modify-write against the store, retrying while another
// writer wins the race. The mutator must be free of side effects because it can
// run more than once.
func Mutate(ctx context.Context, store Store, mutator func(json.RawMessage) (json.RawMessage, error)) error {
	var lastErr error
	for attempt := 0; attempt < mutateAttempts; attempt++ {
		current, version, err := store.Load(ctx)
		if err != nil {
			return err
		}
		next, err := mutator(current)
		if err != nil {
			return err
		}
		if _, err = store.Save(ctx, next, version); err == nil {
			return nil
		}
		if !errors.Is(err, ErrVersionConflict) {
			return err
		}
		lastErr = err
		if err := waitBeforeRetry(ctx, attempt); err != nil {
			return err
		}
	}
	return lastErr
}

// waitBeforeRetry backs off with jitter. Without the jitter, writers that
// collide once tend to collide again on the next attempt because they retry in
// lockstep; spreading them out is what makes contention resolve.
func waitBeforeRetry(ctx context.Context, attempt int) error {
	backoff := time.Duration(1<<min(attempt, 6)) * time.Millisecond
	backoff = min(backoff, mutateBackoffCeiling)
	jitter := time.Duration(rand.Int64N(int64(backoff) + 1))
	timer := time.NewTimer(jitter)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// MemoryStore is the in-process Store used by tests and the browser-parity
// harness. It applies the same compare-and-swap contract as PostgreSQL.
type MemoryStore struct {
	mu      sync.RWMutex
	state   json.RawMessage
	version int64
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

func (store *MemoryStore) Load(ctx context.Context) (json.RawMessage, int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, NoVersion, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	if len(store.state) == 0 {
		return nil, NoVersion, ErrNotFound
	}
	return append(json.RawMessage(nil), store.state...), store.version, nil
}

func (store *MemoryStore) Save(ctx context.Context, state json.RawMessage, expected int64) (int64, error) {
	if err := ctx.Err(); err != nil {
		return store.version, err
	}
	if !json.Valid(state) {
		return store.version, ErrInvalidState
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return store.version, err
	}
	if expected != AnyVersion && expected != store.version {
		return store.version, ErrVersionConflict
	}
	store.state = append(json.RawMessage(nil), state...)
	store.version++
	return store.version, nil
}
