package farm

import (
	"context"
	"encoding/json"
	"errors"
)

// CommitMutation is Mutate for changes that add or rename devices and
// channels. Mutate only rewrites the JSON document; the relational registry
// (which measurements reference by foreign key) is refreshed only when the new
// document goes through the StateCommitter. Without a committer (tests,
// memory deployments) it falls back to Mutate.
func CommitMutation(ctx context.Context, store Store, committer StateCommitter, mutator func(json.RawMessage) (json.RawMessage, error)) error {
	if committer == nil {
		return Mutate(ctx, store, mutator)
	}
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
		if _, err = committer.CommitState(ctx, next, version); err == nil {
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
