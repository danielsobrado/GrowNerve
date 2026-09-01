package httpx

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// EventBroker fans small invalidation hints out to connected browsers over
// server-sent events. It deliberately carries no farm data: a client is told
// what changed and re-reads it through the normal authorized endpoints, so the
// stream cannot leak anything the client could not already fetch.
type EventBroker struct {
	mu          sync.RWMutex
	subscribers map[uint64]chan string
	nextID      atomic.Uint64
	// buffer bounds each subscriber's queue. A client that stops reading is
	// disconnected rather than allowed to grow memory without limit.
	buffer int
	// heartbeat keeps intermediaries from closing an idle stream.
	heartbeat time.Duration
}

func NewEventBroker() *EventBroker {
	return &EventBroker{subscribers: map[uint64]chan string{}, buffer: 16, heartbeat: 25 * time.Second}
}

// Notify records that a topic changed. It never blocks: a subscriber whose queue
// is full is skipped, because the client will still re-read on the next hint or
// on reconnect.
func (broker *EventBroker) Notify(topic string) {
	broker.mu.RLock()
	defer broker.mu.RUnlock()
	for _, subscriber := range broker.subscribers {
		select {
		case subscriber <- topic:
		default:
		}
	}
}

// Subscribers reports the current connection count for health reporting.
func (broker *EventBroker) Subscribers() int {
	broker.mu.RLock()
	defer broker.mu.RUnlock()
	return len(broker.subscribers)
}

func (broker *EventBroker) add() (uint64, chan string) {
	id := broker.nextID.Add(1)
	channel := make(chan string, broker.buffer)
	broker.mu.Lock()
	broker.subscribers[id] = channel
	broker.mu.Unlock()
	return id, channel
}

func (broker *EventBroker) remove(id uint64) {
	broker.mu.Lock()
	delete(broker.subscribers, id)
	broker.mu.Unlock()
}

// Stream serves one server-sent-event connection.
func (broker *EventBroker) Stream(writer http.ResponseWriter, request *http.Request) {
	flusher, streamable := writer.(http.Flusher)
	if !streamable {
		http.Error(writer, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Connection", "keep-alive")
	// Proxies that buffer would defeat the point of the stream.
	writer.Header().Set("X-Accel-Buffering", "no")
	writer.WriteHeader(http.StatusOK)

	id, updates := broker.add()
	defer broker.remove(id)

	// An immediate hello lets the client distinguish "connected and quiet" from
	// "still connecting".
	_, _ = fmt.Fprintf(writer, "event: ready\ndata: {\"retry\":3000}\n\nretry: 3000\n\n")
	flusher.Flush()

	ticker := time.NewTicker(broker.heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-request.Context().Done():
			return
		case topic := <-updates:
			if _, err := fmt.Fprintf(writer, "event: change\ndata: {\"topic\":%q}\n\n", topic); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := fmt.Fprint(writer, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
