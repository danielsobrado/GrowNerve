package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiterRefillsOverTime(t *testing.T) {
	limiter := NewLimiter(RateLimit{Rate: 2, Burst: 2})
	clock := time.Now()
	limiter.now = func() time.Time { return clock }

	if !limiter.Allow("client") || !limiter.Allow("client") {
		t.Fatal("burst allowance was not honoured")
	}
	if limiter.Allow("client") {
		t.Fatal("a third immediate request was allowed past a burst of two")
	}
	// Half a second at two per second restores exactly one token.
	clock = clock.Add(500 * time.Millisecond)
	if !limiter.Allow("client") {
		t.Fatal("the bucket did not refill")
	}
	if limiter.Allow("client") {
		t.Fatal("the bucket refilled more than it should have")
	}
}

func TestLimiterIsPerClient(t *testing.T) {
	limiter := NewLimiter(RateLimit{Rate: 1, Burst: 1})
	if !limiter.Allow("first") {
		t.Fatal("first client refused")
	}
	if !limiter.Allow("second") {
		t.Fatal("one client's usage throttled another")
	}
	if limiter.Allow("first") {
		t.Fatal("first client exceeded its own allowance")
	}
}

// TestWritesAreThrottledSeparatelyFromReads matters because a flood of actuator
// commands is a different risk from a flood of reads.
func TestWritesAreThrottledSeparatelyFromReads(t *testing.T) {
	handler := Chain(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}), Options{
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		ReadLimit:  RateLimit{Rate: 1000, Burst: 1000},
		WriteLimit: RateLimit{Rate: 1, Burst: 2},
	})

	send := func(method string) int {
		request := httptest.NewRequest(method, "/api/v1/commands", nil)
		request.RemoteAddr = "10.0.0.5:5555"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response.Code
	}

	if send(http.MethodPost) != http.StatusOK || send(http.MethodPost) != http.StatusOK {
		t.Fatal("write burst was not honoured")
	}
	if code := send(http.MethodPost); code != http.StatusTooManyRequests {
		t.Fatalf("third write status = %d, want 429", code)
	}
	// Reads have their own generous allowance and must not be affected.
	for attempt := 0; attempt < 20; attempt++ {
		if code := send(http.MethodGet); code != http.StatusOK {
			t.Fatalf("read %d was throttled by the write limiter: %d", attempt, code)
		}
	}
}

func TestThrottledResponseTellsTheClientWhenToRetry(t *testing.T) {
	handler := Chain(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}), Options{WriteLimit: RateLimit{Rate: 1, Burst: 1}})

	send := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/state", nil)
		request.RemoteAddr = "10.0.0.6:5555"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	send()
	throttled := send()
	if throttled.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", throttled.Code)
	}
	if throttled.Header().Get("Retry-After") == "" {
		t.Fatal("throttled response did not say when to retry")
	}
}

// TestForwardedHeadersCannotSplitTheAllowance guards against a client
// masquerading as many by setting its own forwarding headers.
func TestForwardedHeadersCannotSplitTheAllowance(t *testing.T) {
	handler := Chain(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}), Options{WriteLimit: RateLimit{Rate: 1, Burst: 1}})

	send := func(forwarded string) int {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/commands", nil)
		request.RemoteAddr = "10.0.0.7:5555"
		request.Header.Set("X-Forwarded-For", forwarded)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response.Code
	}
	if send("1.1.1.1") != http.StatusOK {
		t.Fatal("first request refused")
	}
	if code := send("2.2.2.2"); code != http.StatusTooManyRequests {
		t.Fatalf("status = %d: a forged forwarding header bought a fresh allowance", code)
	}
}
