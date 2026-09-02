package outbox

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMemoryEnqueueIsIdempotentByTopicAndKey(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	firstID, err := store.Enqueue(ctx, "grownerve/v1/devices/device/commands", "command-1", json.RawMessage(`{"value":1}`))
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := store.Enqueue(ctx, "grownerve/v1/devices/device/commands", "command-1", json.RawMessage(`{"value":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if secondID != firstID {
		t.Fatalf("duplicate enqueue created a new row: first=%q second=%q", firstID, secondID)
	}
	pending, err := store.Pending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending messages = %d, want 1", len(pending))
	}
	if string(pending[0].Payload) != `{"value":1}` {
		t.Fatalf("duplicate enqueue replaced the original payload: %s", pending[0].Payload)
	}
}
