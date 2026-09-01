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

// TestAcceptedCommandSurvivesABrokerOutage is the point of the outbox: an
// operator was told the command was accepted, so it must still reach the device
// once the broker returns rather than being silently dropped.
func TestAcceptedCommandSurvivesABrokerOutage(t *testing.T) {
	broker := &flakyBroker{available: false}
	queue := outbox.NewMemoryStore()
	publisher := NewDurablePublisher(broker, queue, quietLogger())
	ctx := context.Background()

	command := deviceprotocol.Command{
		ProtocolVersion: deviceprotocol.Version, CommandID: "command-1", TargetChannelID: "channel-1",
		Type: "set_percent", Value: json.RawMessage(`60`), IssuedAt: time.Now().UTC(), ExpiresAt: time.Now().Add(time.Minute).UTC(),
	}
	// The publish fails, and the caller is told so, but the message is retained.
	if err := publisher.PublishCommand(ctx, "device-1", command); err == nil {
		t.Fatal("publish to an unavailable broker reported success")
	}
	pending, err := queue.Pending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("queued %d messages, want 1", len(pending))
	}

	// The broker returns; the worker delivers what was queued.
	broker.available = true
	worker := NewOutboxWorker(queue, broker, quietLogger())
	if err := worker.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if len(broker.published) != 1 || broker.published[0] != "command-1" {
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
	command := deviceprotocol.Command{CommandID: "command-2", Value: json.RawMessage(`true`)}

	if err := publisher.PublishCommand(context.Background(), "device-1", command); err != nil {
		t.Fatal(err)
	}
	pending, _ := queue.Pending(context.Background(), 10)
	if len(pending) != 0 {
		t.Fatalf("a successful publish was also queued: %d messages", len(pending))
	}
}

// TestRepeatedFailureParksTheMessage proves a permanently bad message cannot
// block the queue forever.
func TestRepeatedFailureParksTheMessage(t *testing.T) {
	broker := &flakyBroker{available: false}
	queue := outbox.NewMemoryStore()
	ctx := context.Background()
	if _, err := queue.Enqueue(ctx, "grownerve/v1/devices/device-1/commands", "command-3", json.RawMessage(`{"commandId":"command-3"}`)); err != nil {
		t.Fatal(err)
	}
	worker := NewOutboxWorker(queue, broker, quietLogger())
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
	// Only the first message should have consumed an attempt; a broker outage
	// must not exhaust every queued message at once.
	if pending[0].Attempts != 1 || pending[1].Attempts != 0 {
		t.Fatalf("attempts = %d, %d; the outage burned attempts on the whole batch", pending[0].Attempts, pending[1].Attempts)
	}
}
