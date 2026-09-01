package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncRecorder is a ResponseWriter the test can read while the handler is still
// writing. httptest.ResponseRecorder is not safe for that, and using it here
// would report a race in the test rather than in the code under test.
type syncRecorder struct {
	mu      sync.Mutex
	body    strings.Builder
	headers http.Header
	status  int
}

func newSyncRecorder() *syncRecorder {
	return &syncRecorder{headers: http.Header{}, status: http.StatusOK}
}

func (recorder *syncRecorder) Header() http.Header { return recorder.headers }

func (recorder *syncRecorder) Write(payload []byte) (int, error) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return recorder.body.Write(payload)
}

func (recorder *syncRecorder) WriteHeader(status int) { recorder.status = status }

func (recorder *syncRecorder) Flush() {}

func (recorder *syncRecorder) text() string {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return recorder.body.String()
}

// readUntil polls the recorder until it contains want or the deadline passes.
func readUntil(t *testing.T, response *syncRecorder, want string) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(response.text(), want) {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func TestStreamAnnouncesReadinessAndDeliversChanges(t *testing.T) {
	broker := NewEventBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request := httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil).WithContext(ctx)
	response := newSyncRecorder()
	finished := make(chan struct{})
	go func() { broker.Stream(response, request); close(finished) }()

	if !readUntil(t, response, "event: ready") {
		t.Fatalf("stream did not announce readiness: %q", response.text())
	}
	if got := response.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	// A buffering proxy would defeat the stream entirely.
	if response.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatal("stream did not ask intermediaries to stop buffering")
	}

	// Wait for the subscriber to register before notifying, otherwise the test
	// races the goroutine rather than the code under test.
	for attempt := 0; broker.Subscribers() == 0 && attempt < 200; attempt++ {
		time.Sleep(5 * time.Millisecond)
	}
	broker.Notify("measurements")
	if !readUntil(t, response, `"topic":"measurements"`) {
		t.Fatalf("change hint was not delivered: %q", response.text())
	}

	cancel()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not close when the client disconnected")
	}
	if broker.Subscribers() != 0 {
		t.Fatalf("subscriber was not released: %d remain", broker.Subscribers())
	}
}

// TestNotifyNeverBlocks is the property that keeps a stalled browser from
// stalling telemetry ingestion: publishing a hint must not wait for readers.
func TestNotifyNeverBlocks(t *testing.T) {
	broker := NewEventBroker()
	// A subscriber that never reads; its queue fills immediately.
	_, _ = broker.add()

	done := make(chan struct{})
	go func() {
		for index := 0; index < 10_000; index++ {
			broker.Notify("measurements")
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Notify blocked on a subscriber that stopped reading")
	}
}

func TestStreamCarriesNoFarmData(t *testing.T) {
	broker := NewEventBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil).WithContext(ctx)
	response := newSyncRecorder()
	go broker.Stream(response, request)

	for attempt := 0; broker.Subscribers() == 0 && attempt < 200; attempt++ {
		time.Sleep(5 * time.Millisecond)
	}
	broker.Notify("alerts")
	if !readUntil(t, response, `"topic":"alerts"`) {
		t.Fatal("hint not delivered")
	}
	// The stream is an invalidation channel. Anything more would bypass the
	// authorization applied to the real endpoints.
	body := response.text()
	for _, forbidden := range []string{"value", "measurement", "facility", "detail"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("stream carried farm data (%q): %s", forbidden, body)
		}
	}
}
