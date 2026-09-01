package farm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/platform/outbox"
)

const (
	queuedCommandID = "01990a20-6a00-7000-8000-000000000401"
	queuedChannelID = "01990a20-6a00-7000-8000-000000000402"
	queuedDeviceID  = "01990a20-6a00-7000-8000-000000000403"
)

type flakyBroker struct {
	available bool
	published []string
}

func (broker *flakyBroker) PublishCommand(_ context.Context, _ string, command deviceprotocol.Command) error {
	if !broker.available {
		return errors.New("MQTT broker is unavailable")
	}
	broker.published = append(broker.published, command.CommandID)
	return nil
}

func (broker *flakyBroker) PublishRaw(_ context.Context, _ string, payload []byte) error {
	if !broker.available {
		return errors.New("MQTT broker is unavailable")
	}
	var command deviceprotocol.Command
	if err := json.Unmarshal(payload, &command); err != nil {
		return err
	}
	broker.published = append(broker.published, command.CommandID)
	return nil
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func validQueuedCommand(now time.Time) deviceprotocol.Command {
	return deviceprotocol.Command{
		ProtocolVersion: deviceprotocol.Version,
		CommandID:       queuedCommandID,
		TargetChannelID: queuedChannelID,
		Type:            "set_percent",
		Value:           json.RawMessage(`60`),
		IssuedAt:        now,
		ExpiresAt:       now.Add(time.Minute),
	}
}

func TestAcceptedCommandSurvivesABrokerOutage(t *testing.T) {
	broker := &flakyBroker{available: false}
	queue := outbox.NewMemoryStore()
	publisher := NewDurablePublisher(broker, queue, quietLogger())
	ctx := context.Background()
	now := time.Now().UTC()
	command := validQueuedCommand(now)

	if err := publisher.PublishCommand(ctx, queuedDeviceID, command); err == nil {
		t.Fatal("publish to an unavailable broker reported success")
	}
	pending, err := queue.Pending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("queued %d messages, want 1", len(pending))
	}

	broker.available = true
	worker := NewOutboxWorker(queue, broker, quietLogger())
	worker.now = func() time.Time { return now.Add(time.Second) }
	if err := worker.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if len(broker.published) != 1 || broker.published[0] != queuedCommandID {
		t.Fatalf("published = %v, want the queued command", broker.published)
	}
	remaining, _ := queue.Pending(ctx, 10)
	if len(remaining) != 0 {
		t.Fatalf("%d messages remain queued after a successful drain", len(remaining))
	}
}

func TestSuccessfulPublishIsNotQueued(t *testing.T) {
	broker := &flakyBroker{available: true}
	queue := outbox.NewMemoryStore()
	publisher := NewDurablePublisher(broker, queue, quietLogger())
	command := validQueuedCommand(time.Now().UTC())

	if err := publisher.PublishCommand(context.Background(), queuedDeviceID, command); err != nil {
		t.Fatal(err)
	}
	pending, _ := queue.Pending(context.Background(), 10)
	if len(pending) != 0 {
		t.Fatalf("a successful publish was also queued: %d messages", len(pending))
	}
}

func TestRepeatedFailureParksTheMessage(t *testing.T) {
	broker := &flakyBroker{available: false}
	queue := outbox.NewMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()
	payload, err := json.Marshal(validQueuedCommand(now))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queue.Enqueue(ctx, "grownerve/v1/devices/"+queuedDeviceID+"/commands", queuedCommandID, payload); err != nil {
		t.Fatal(err)
	}
	worker := NewOutboxWorker(queue, broker, quietLogger())
	worker.now = func() time.Time { return now.Add(time.Second) }
	for attempt := 0; attempt < outbox.MaximumAttempts+2; attempt++ {
		if err := worker.Drain(ctx); err != nil {
			t.Fatal(err)
		}
	}
	pending, _ := queue.Pending(ctx, 10)
	if len(pending) != 0 {
		t.Fatalf("a message that always fails is still being retried: %+v", pending)
	}
}

func TestInvalidQueuedCommandIsDiscardedWithoutTransport(t *testing.T) {
	broker := &flakyBroker{available: true}
	queue := outbox.NewMemoryStore()
	ctx := context.Background()
	if _, err := queue.Enqueue(ctx, "grownerve/v1/devices/"+queuedDeviceID+"/commands", queuedCommandID, json.RawMessage(`{"commandId":"wrong"}`)); err != nil {
		t.Fatal(err)
	}
	worker := NewOutboxWorker(queue, broker, quietLogger())
	if err := worker.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if len(broker.published) != 0 {
		t.Fatalf("invalid command reached transport: %v", broker.published)
	}
	pending, _ := queue.Pending(ctx, 10)
	if len(pending) != 0 {
		t.Fatalf("invalid command remained retryable: %+v", pending)
	}
}

func TestDrainStopsAtTheFirstFailureRatherThanBurningEveryAttempt(t *testing.T) {
	broker := &flakyBroker{available: false}
	queue := outbox.NewMemoryStore()
	ctx := context.Background()
	for index := 0; index < 3; index++ {
		if _, err := queue.Enqueue(ctx, "topic", "key", json.RawMessage(`{}`)); err != nil {
			t.Fatal(err)
		}
	}
	if err := NewOutboxWorker(queue, broker, quietLogger()).Drain(ctx); err != nil {
		t.Fatal(err)
	}
	pending, _ := queue.Pending(ctx, 10)
	if len(pending) != 3 {
		t.Fatalf("pending = %d, want all three still queued", len(pending))
	}
	if pending[0].Attempts != 1 || pending[1].Attempts != 0 {
		t.Fatalf("attempts = %d, %d; the outage burned attempts on the whole batch", pending[0].Attempts, pending[1].Attempts)
	}
}
