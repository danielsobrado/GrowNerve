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

func TestTrustedProxyUsesRightmostUntrustedForwardedAddress(t *testing.T) {
	trusted := parseTrustedProxyCIDRs([]string{"10.0.0.0/8"})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.9:443"
	request.Header.Set("X-Forwarded-For", "1.1.1.1, 203.0.113.50")

	if got := clientKeyWithProxies(request, trusted); got != "203.0.113.50" {
		t.Fatalf("client key = %q, want rightmost untrusted client", got)
	}
}

func TestTrustedProxySkipsTrustedIntermediateHops(t *testing.T) {
	trusted := parseTrustedProxyCIDRs([]string{"10.0.0.0/8"})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.9:443"
	request.Header.Set("X-Forwarded-For", "198.51.100.8, 10.0.0.8")

	if got := clientKeyWithProxies(request, trusted); got != "198.51.100.8" {
		t.Fatalf("client key = %q, want original untrusted client", got)
	}
}

func TestUntrustedPeerCannotUseForwardedAddress(t *testing.T) {
	trusted := parseTrustedProxyCIDRs([]string{"10.0.0.0/8"})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "203.0.113.99:443"
	request.Header.Set("X-Forwarded-For", "1.1.1.1")

	if got := clientKeyWithProxies(request, trusted); got != "203.0.113.99" {
		t.Fatalf("untrusted peer changed client identity to %q", got)
	}
}
