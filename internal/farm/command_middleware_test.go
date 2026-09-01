package farm

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCommandIdempotencyMiddlewareRejectsMissingOrInvalidKeys(t *testing.T) {
	next := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusAccepted)
	})
	handler := RequireCommandIdempotency(next)

	for name, key := range map[string]string{
		"missing": "",
		"too short": "short",
		"too long": strings.Repeat("x", maximumIdempotencyKeyLength+1),
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/commands", nil)
			if key != "" {
				request.Header.Set("Idempotency-Key", key)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", response.Code)
			}
		})
	}
}

func TestCommandIdempotencyMiddlewarePassesValidKeyAndOtherRoutes(t *testing.T) {
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/v1/commands" && request.Header.Get("Idempotency-Key") != "valid-key" {
			t.Fatalf("trimmed idempotency key = %q", request.Header.Get("Idempotency-Key"))
		}
		writer.WriteHeader(http.StatusAccepted)
	})
	handler := RequireCommandIdempotency(next)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/commands", nil)
	request.Header.Set("Idempotency-Key", " valid-key ")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("valid command status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPut, "/api/v1/state", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("unrelated route status = %d", response.Code)
	}
}
