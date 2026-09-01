package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoints(t *testing.T) {
	handler := NewHealthHandler(func() error { return nil })
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()
	handler.Live(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("live status = %d", response.Code)
	}

	handler = NewHealthHandler(func() error { return errors.New("database unavailable") })
	response = httptest.NewRecorder()
	handler.Ready(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d", response.Code)
	}
}
