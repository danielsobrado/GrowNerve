package farm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/platform/outbox"
)

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

type OutboxWorker struct {
	queue     outbox.Store
	transport RawPublisher
	logger    *slog.Logger
	batchSize int
	retention time.Duration
	pruned    time.Time
	now       func() time.Time
}

type RawPublisher interface {
	PublishRaw(ctx context.Context, topic string, payload []byte) error
}

func NewOutboxWorker(queue outbox.Store, transport RawPublisher, logger *slog.Logger) *OutboxWorker {
	return &OutboxWorker{
		queue: queue, transport: transport, logger: logger, batchSize: 64,
		retention: 48 * time.Hour, now: func() time.Time { return time.Now().UTC() },
	}
}

func (worker *OutboxWorker) Drain(ctx context.Context) error {
	pending, err := worker.queue.Pending(ctx, worker.batchSize)
	if err != nil {
		return err
	}
	for _, message := range pending {
		if command, expired := expiredCommand(message, worker.now()); expired {
			// MarkPublished is the store's existing terminal-success state. Here it
			// means the queued work is complete without transport because publishing
			// it would violate the command's absolute expiry safety boundary.
			if err := worker.queue.MarkPublished(ctx, message.ID); err != nil {
				return err
			}
			worker.logger.Warn("outbox_expired_command_discarded", "message", message.ID, "command", command.CommandID, "topic", message.Topic)
			continue
		}
		if err := worker.transport.PublishRaw(ctx, message.Topic, message.Payload); err != nil {
			if markErr := worker.queue.MarkFailed(ctx, message.ID, err.Error()); markErr != nil {
				return markErr
			}
			if message.Attempts+1 >= outbox.MaximumAttempts {
				worker.logger.Error("outbox_message_parked", "message", message.ID, "topic", message.Topic, "attempts", message.Attempts+1)
			}
			return nil
		}
		if err := worker.queue.MarkPublished(ctx, message.ID); err != nil {
			return err
		}
		worker.logger.Info("outbox_message_published", "message", message.ID, "topic", message.Topic)
	}
	return worker.prune(ctx)
}

func expiredCommand(message outbox.Message, now time.Time) (deviceprotocol.Command, bool) {
	var command deviceprotocol.Command
	if !strings.HasSuffix(message.Topic, "/commands") || json.Unmarshal(message.Payload, &command) != nil {
		return command, false
	}
	return command, !command.ExpiresAt.IsZero() && !command.ExpiresAt.After(now)
}

func (worker *OutboxWorker) prune(ctx context.Context) error {
	now := worker.now()
	if now.Sub(worker.pruned) < time.Hour {
		return nil
	}
	worker.pruned = now
	return worker.queue.PruneBefore(ctx, now.Add(-worker.retention))
}
