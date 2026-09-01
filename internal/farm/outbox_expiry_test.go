package farm

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/platform/outbox"
)

func TestOutboxDiscardsExpiredCommandWithoutPublishing(t *testing.T) {
	const (
		commandID = "01990a20-6a00-7000-8000-000000000101"
		channelID = "01990a20-6a00-7000-8000-000000000102"
	)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	command := deviceprotocol.Command{
		ProtocolVersion: deviceprotocol.Version,
		CommandID:       commandID,
		TargetChannelID: channelID,
		Type:            "set_percent",
		Value:           json.RawMessage(`70`),
		IssuedAt:        now.Add(-2 * time.Minute),
		ExpiresAt:       now.Add(-time.Second),
	}
	payload, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}

	queue := outbox.NewMemoryStore()
	if _, err := queue.Enqueue(context.Background(), "grownerve/v1/devices/device-1/commands", commandID, payload); err != nil {
		t.Fatal(err)
	}
	broker := &flakyBroker{available: true}
	worker := NewOutboxWorker(queue, broker, quietLogger())
	worker.now = func() time.Time { return now }

	if err := worker.Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(broker.published) != 0 {
		t.Fatalf("expired command reached the transport: %v", broker.published)
	}
	pending, err := queue.Pending(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("expired command remained retryable: %+v", pending)
	}
}

func TestOutboxPublishesCommandBeforeAbsoluteExpiry(t *testing.T) {
	const (
		commandID = "01990a20-6a00-7000-8000-000000000111"
		channelID = "01990a20-6a00-7000-8000-000000000112"
	)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	command := deviceprotocol.Command{
		ProtocolVersion: deviceprotocol.Version,
		CommandID:       commandID,
		TargetChannelID: channelID,
		Type:            "set_percent",
		Value:           json.RawMessage(`70`),
		IssuedAt:        now.Add(-time.Second),
		ExpiresAt:       now.Add(time.Second),
	}
	payload, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}

	queue := outbox.NewMemoryStore()
	if _, err := queue.Enqueue(context.Background(), "grownerve/v1/devices/device-1/commands", commandID, payload); err != nil {
		t.Fatal(err)
	}
	broker := &flakyBroker{available: true}
	worker := NewOutboxWorker(queue, broker, quietLogger())
	worker.now = func() time.Time { return now }

	if err := worker.Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(broker.published) != 1 || broker.published[0] != commandID {
		t.Fatalf("unexpired command was not published: %v", broker.published)
	}
}
