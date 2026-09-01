package farm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/platform/outbox"
)

// DurablePublisher publishes commands to the broker and, when the broker is
// unreachable, queues them for retry instead of dropping them. The command is
// already persisted by the time this runs, so the queue only ever holds work
// that an operator was told had been accepted.
type DurablePublisher struct {
	direct CommandPublisher
	queue  outbox.Store
	logger *slog.Logger
}

func NewDurablePublisher(direct CommandPublisher, queue outbox.Store, logger *slog.Logger) *DurablePublisher {
	return &DurablePublisher{direct: direct, queue: queue, logger: logger}
}

func (publisher *DurablePublisher) PublishCommand(ctx context.Context, deviceID string, command deviceprotocol.Command) error {
	err := publisher.direct.PublishCommand(ctx, deviceID, command)
	if err == nil {
		return nil
	}
	// The broker refused or was unavailable. An accepted command must not
	// disappear because of that, so it is queued for the outbox worker.
	payload, encodeErr := json.Marshal(command)
	if encodeErr != nil {
		return err
	}
	topic := fmt.Sprintf("grownerve/v1/devices/%s/commands", deviceID)
	if _, queueErr := publisher.queue.Enqueue(ctx, topic, command.CommandID, payload); queueErr != nil {
		publisher.logger.Error("command_enqueue_failed", "error", queueErr, "command", command.CommandID)
		return err
	}
	publisher.logger.Warn("command_queued_for_retry", "command", command.CommandID, "device", deviceID, "error", err)
	return err
}

// OutboxWorker drains queued publications.
type OutboxWorker struct {
	queue     outbox.Store
	transport RawPublisher
	logger    *slog.Logger
	batchSize int
	// retention is how long published rows are kept before pruning.
	retention time.Duration
	pruned    time.Time
}

// RawPublisher publishes an already-encoded payload to a topic.
type RawPublisher interface {
	PublishRaw(ctx context.Context, topic string, payload []byte) error
}

func NewOutboxWorker(queue outbox.Store, transport RawPublisher, logger *slog.Logger) *OutboxWorker {
	return &OutboxWorker{queue: queue, transport: transport, logger: logger, batchSize: 64, retention: 48 * time.Hour}
}

// Drain publishes one batch. It is safe to call repeatedly; a message that keeps
// failing burns its attempts and is eventually parked rather than retried
// forever.
func (worker *OutboxWorker) Drain(ctx context.Context) error {
	pending, err := worker.queue.Pending(ctx, worker.batchSize)
	if err != nil {
		return err
	}
	for _, message := range pending {
		if err := worker.transport.PublishRaw(ctx, message.Topic, message.Payload); err != nil {
			if markErr := worker.queue.MarkFailed(ctx, message.ID, err.Error()); markErr != nil {
				return markErr
			}
			if message.Attempts+1 >= outbox.MaximumAttempts {
				worker.logger.Error("outbox_message_parked", "message", message.ID, "topic", message.Topic, "attempts", message.Attempts+1)
			}
			// The broker is down for everything, not just this message, so the
			// rest of the batch waits for the next tick.
			return nil
		}
		if err := worker.queue.MarkPublished(ctx, message.ID); err != nil {
			return err
		}
		worker.logger.Info("outbox_message_published", "message", message.ID, "topic", message.Topic)
	}
	return worker.prune(ctx)
}

func (worker *OutboxWorker) prune(ctx context.Context) error {
	now := time.Now().UTC()
	if now.Sub(worker.pruned) < time.Hour {
		return nil
	}
	worker.pruned = now
	return worker.queue.PruneBefore(ctx, now.Add(-worker.retention))
}
