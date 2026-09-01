package farm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type failingStore struct {
	load             json.RawMessage
	loadErr, saveErr error
}

func (store failingStore) Load(context.Context) (json.RawMessage, error) {
	return store.load, store.loadErr
}
func (store failingStore) Save(context.Context, json.RawMessage) error { return store.saveErr }

func makeRequest(handler http.Handler, method, path, body, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestStateHandlerReadAndWriteFailures(t *testing.T) {
	if got := makeRequest(NewHandler(NewMemoryStore()), http.MethodGet, "/api/v1/state", "", "").Code; got != http.StatusNoContent {
		t.Fatalf("empty GET = %d", got)
	}
	if got := makeRequest(NewHandler(failingStore{loadErr: errors.New("read")}), http.MethodGet, "/api/v1/state", "", "").Code; got != http.StatusInternalServerError {
		t.Fatalf("failed GET = %d", got)
	}
	if got := makeRequest(NewHandler(NewMemoryStore()), http.MethodPut, "/api/v1/state", `{}`, "text/plain").Code; got != http.StatusUnsupportedMediaType {
		t.Fatalf("content type = %d", got)
	}
	if got := makeRequest(NewHandler(NewMemoryStore()), http.MethodPut, "/api/v1/state", ``, "application/json").Code; got != http.StatusBadRequest {
		t.Fatalf("empty body = %d", got)
	}
	if got := makeRequest(NewHandler(failingStore{saveErr: errors.New("write")}), http.MethodPut, "/api/v1/state", `{}`, "application/json").Code; got != http.StatusInternalServerError {
		t.Fatalf("failed PUT = %d", got)
	}
}

func TestCollectionsAndETagConflict(t *testing.T) {
	store := NewMemoryStore()
	handler := NewHandler(store)
	if got := makeRequest(handler, http.MethodGet, "/api/v1/facilities", "", "").Body.String(); got != "[]" {
		t.Fatalf("empty collection = %q", got)
	}
	_ = store.Save(context.Background(), json.RawMessage(`{"facilities":[{"id":"one"}]}`))
	if got := makeRequest(handler, http.MethodGet, "/api/v1/facilities", "", "").Body.String(); got != `[{"id":"one"}]` {
		t.Fatalf("collection = %q", got)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/state", strings.NewReader(`{"facilities":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", `"wrong"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusConflict {
		t.Fatalf("conflict = %d", response.Code)
	}
	if stateETag([]byte("same")) != stateETag([]byte("same")) {
		t.Fatal("ETag is not deterministic")
	}
}

func commandStateJSON(online bool, minimum, maximum float64) string {
	min, _ := json.Marshal(minimum)
	max, _ := json.Marshal(maximum)
	onlineJSON, _ := json.Marshal(online)
	return `{"channels":[{"id":"01990a20-6a00-7000-8000-000000000001","device_id":"01990a20-6a00-7000-8000-000000000002","kind":"command","value_type":"number","safe_minimum":` + string(min) + `,"safe_maximum":` + string(max) + `}],"devices":[{"id":"01990a20-6a00-7000-8000-000000000002","online":` + string(onlineJSON) + `}],"commands":[],"events":[]}`
}

func TestCommandValidationAndIdempotency(t *testing.T) {
	if got := makeRequest(NewHandler(NewMemoryStore()), http.MethodPost, "/api/v1/commands", `{}`, "text/plain").Code; got != http.StatusUnsupportedMediaType {
		t.Fatalf("type = %d", got)
	}
	if got := makeRequest(NewHandler(NewMemoryStore()), http.MethodPost, "/api/v1/commands", `{}`, "application/json").Code; got != http.StatusBadRequest {
		t.Fatalf("body = %d", got)
	}
	if got := makeRequest(NewHandler(NewMemoryStore()), http.MethodPost, "/api/v1/commands", `{"targetChannelId":"x","value":50,"reason":"test"}`, "application/json").Code; got != http.StatusConflict {
		t.Fatalf("no farm = %d", got)
	}
	store := NewMemoryStore()
	_ = store.Save(context.Background(), json.RawMessage(commandStateJSON(true, 25, 100)))
	handler := NewHandler(store)
	if got := makeRequest(handler, http.MethodPost, "/api/v1/commands", `{"targetChannelId":"unknown","value":50,"reason":"test"}`, "application/json").Code; got != http.StatusNotFound {
		t.Fatalf("unknown = %d", got)
	}
	if got := makeRequest(handler, http.MethodPost, "/api/v1/commands", `{"targetChannelId":"01990a20-6a00-7000-8000-000000000001","value":"bad","reason":"test"}`, "application/json").Code; got != http.StatusBadRequest {
		t.Fatalf("bad value = %d", got)
	}
	if got := makeRequest(handler, http.MethodPost, "/api/v1/commands", `{"targetChannelId":"01990a20-6a00-7000-8000-000000000001","value":10,"reason":"test"}`, "application/json").Code; got != http.StatusUnprocessableEntity {
		t.Fatalf("unsafe = %d", got)
	}
	_ = store.Save(context.Background(), json.RawMessage(commandStateJSON(true, 25, 100)))
	body := `{"targetChannelId":"01990a20-6a00-7000-8000-000000000001","value":50,"reason":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commands", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "same-key")
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, req)
	if first.Code != http.StatusAccepted {
		t.Fatalf("accepted = %d: %s", first.Code, first.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/commands", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "same-key")
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, req)
	if second.Code != http.StatusOK || second.Body.String() != first.Body.String() {
		t.Fatalf("replay = %d %s", second.Code, second.Body.String())
	}
}

func TestMemoryStoreCopiesState(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.Load(context.Background()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() = %v", err)
	}
	input := json.RawMessage(`{"a":1}`)
	_ = store.Save(context.Background(), input)
	input[2] = 'x'
	loaded, _ := store.Load(context.Background())
	loaded[2] = 'y'
	again, _ := store.Load(context.Background())
	if string(again) != `{"a":1}` {
		t.Fatalf("state aliased: %s", again)
	}
}
