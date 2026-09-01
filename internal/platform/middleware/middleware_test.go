package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChainAddsCorrelationAndSecurityHeaders(t *testing.T) {
	handler := Chain(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if CorrelationID(request.Context()) == "" {
			t.Fatal("correlation ID missing from context")
		}
		writer.WriteHeader(http.StatusNoContent)
	}), Options{AllowedOrigins: []string{"http://127.0.0.1:5173"}})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/state", nil)
	request.Header.Set("Origin", "http://127.0.0.1:5173")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Header().Get("X-Correlation-ID") == "" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers = %#v", response.Header())
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:5173" {
		t.Fatalf("CORS origin = %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestChainHandlesPreflightAndPanic(t *testing.T) {
	options := Options{AllowedOrigins: []string{"http://allowed"}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	handler := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), options)
	request := httptest.NewRequest(http.MethodOptions, "/", nil)
	request.Header.Set("Origin", "http://allowed")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("preflight = %d", response.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Correlation-ID", "not-a-uuid")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError || CorrelationID(context.Background()) != "" {
		t.Fatalf("panic response = %d", response.Code)
	}
}
