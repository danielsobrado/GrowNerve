package farm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStateHandlerRoundTrip(t *testing.T) {
	store := NewMemoryStore()
	handler := NewHandler(store)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/state", strings.NewReader(`{"facilities":[{"id":"01990a20-6a00-7000-8000-000000000001"}],"zones":[]}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("PUT status = %d, body = %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/state", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET status = %d", response.Code)
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if _, found := state["facilities"]; !found {
		t.Fatal("GET state has no facilities")
	}
}

func TestStateHandlerRejectsInvalidBodyWithoutChangingState(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Save(context.Background(), json.RawMessage(`{"facilities":[]}`)); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/state", strings.NewReader(`[]`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	state, _ := store.Load(context.Background())
	if string(state) != `{"facilities":[]}` {
		t.Fatalf("state changed to %s", state)
	}
}
