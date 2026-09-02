// Package outbox gives command delivery a durable retry path. A command is
// persisted before it is published; if the broker is unreachable at that moment
// the message is queued here instead of being lost, and a background worker
// keeps trying.
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// Message is one pending publication.
type Message struct {
	ID       string
	Topic    string
	Key      string
	Payload  json.RawMessage
	Attempts int
}

// Store persists pending messages.
type Store interface {
	Enqueue(ctx context.Context, topic, key string, payload json.RawMessage) (string, error)
	Pending(ctx context.Context, batchSize int) ([]Message, error)
	MarkPublished(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id, reason string) error
	// PruneBefore removes published messages older than the cutoff so the table
	// does not grow without bound.
	PruneBefore(ctx context.Context, cutoff time.Time) error
}

// MaximumAttempts bounds retries. A message that keeps failing is parked rather
// than retried forever, so a permanently bad payload cannot block the queue.
const MaximumAttempts = 12

// ErrExhausted reports a message that has used all its attempts.
var ErrExhausted = errors.New("outbox message exhausted its attempts")

// MemoryStore is the in-process implementation used by tests and by deployments
// without PostgreSQL.
type MemoryStore struct {
	mu       sync.Mutex
	messages []memoryMessage
	nextID   int
}

type memoryMessage struct {
	Message
	published   bool
	publishedAt time.Time
	lastError   string
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

func (store *MemoryStore) Enqueue(_ context.Context, topic, key string, payload json.RawMessage) (string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, message := range store.messages {
		if message.Topic == topic && message.Key == key {
			return message.ID, nil
		}
	}
	store.nextID++
	id := time.Now().UTC().Format("20060102150405.000000") + "-" + itoa(store.nextID)
	store.messages = append(store.messages, memoryMessage{Message: Message{
		ID: id, Topic: topic, Key: key, Payload: append(json.RawMessage(nil), payload...),
	}})
	return id, nil
}

func (store *MemoryStore) Pending(_ context.Context, batchSize int) ([]Message, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var pending []Message
	for _, message := range store.messages {
		if message.published || message.Attempts >= MaximumAttempts {
			continue
		}
		pending = append(pending, message.Message)
		if len(pending) >= batchSize {
			break
		}
	}
	return pending, nil
}

func (store *MemoryStore) MarkPublished(_ context.Context, id string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for index := range store.messages {
		if store.messages[index].ID == id {
			store.messages[index].published = true
			store.messages[index].publishedAt = time.Now().UTC()
			store.messages[index].Attempts++
			return nil
		}
	}
	return nil
}

func (store *MemoryStore) MarkFailed(_ context.Context, id, reason string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for index := range store.messages {
		if store.messages[index].ID == id {
			store.messages[index].Attempts++
			store.messages[index].lastError = reason
			return nil
		}
	}
	return nil
}

func (store *MemoryStore) PruneBefore(_ context.Context, cutoff time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	kept := store.messages[:0]
	for _, message := range store.messages {
		if message.published && message.publishedAt.Before(cutoff) {
			continue
		}
		kept = append(kept, message)
	}
	store.messages = append([]memoryMessage(nil), kept...)
	return nil
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
