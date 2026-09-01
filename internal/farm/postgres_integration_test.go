package farm_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jdanielsobrado/grownerve/internal/farm"
	"github.com/jdanielsobrado/grownerve/internal/platform/database/dbtest"
)

// TestPostgresStoreCompareAndSwap proves the real database enforces the same
// contract the memory store does. The unit tests would still pass if PostgreSQL
// silently applied a stale write, so this is the one that matters.
func TestPostgresStoreCompareAndSwap(t *testing.T) {
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool, "browser_compatible_states")
	store := farm.NewPostgresStore(pool)
	ctx := context.Background()

	if _, _, err := store.Load(ctx); !errors.Is(err, farm.ErrNotFound) {
		t.Fatalf("Load on an empty store = %v, want ErrNotFound", err)
	}

	first, err := store.Save(ctx, json.RawMessage(`{"facilities":[]}`), farm.NoVersion)
	if err != nil {
		t.Fatal(err)
	}
	state, version, err := store.Load(ctx)
	if err != nil || version != first {
		t.Fatalf("Load = %s, %d, %v", state, version, err)
	}

	// A write with the version we read succeeds and advances the counter.
	next, err := store.Save(ctx, json.RawMessage(`{"facilities":[{"id":"one"}]}`), version)
	if err != nil || next <= version {
		t.Fatalf("Save with the current version = %d, %v", next, err)
	}

	// A write with the stale version is refused, and the stored document is
	// unchanged.
	if _, err := store.Save(ctx, json.RawMessage(`{"facilities":[{"id":"loser"}]}`), version); !errors.Is(err, farm.ErrVersionConflict) {
		t.Fatalf("stale Save = %v, want ErrVersionConflict", err)
	}
	current, _, _ := store.Load(ctx)
	if !strings.Contains(string(current), `"one"`) {
		t.Fatalf("a stale write overwrote the newer document: %s", current)
	}
}

// TestPostgresConcurrentMutateKeepsEveryIncrement is the database-level version
// of the lost-update regression.
func TestPostgresConcurrentMutateKeepsEveryIncrement(t *testing.T) {
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool, "browser_compatible_states")
	store := farm.NewPostgresStore(pool)
	ctx := context.Background()
	if _, err := store.Save(ctx, json.RawMessage(`{"n":0}`), farm.NoVersion); err != nil {
		t.Fatal(err)
	}

	const writers = 20
	var group sync.WaitGroup
	for writer := 0; writer < writers; writer++ {
		group.Add(1)
		go func() {
			defer group.Done()
			err := farm.Mutate(ctx, store, func(state json.RawMessage) (json.RawMessage, error) {
				var document struct {
					N int `json:"n"`
				}
				if err := json.Unmarshal(state, &document); err != nil {
					return nil, err
				}
				document.N++
				return json.Marshal(document)
			})
			if err != nil {
				t.Errorf("Mutate() = %v", err)
			}
		}()
	}
	group.Wait()

	final, _, _ := store.Load(ctx)
	var document struct {
		N int `json:"n"`
	}
	if err := json.Unmarshal(final, &document); err != nil {
		t.Fatal(err)
	}
	if document.N != writers {
		t.Fatalf("counter = %d, want %d: %d increments were lost against PostgreSQL", document.N, writers, writers-document.N)
	}
}
