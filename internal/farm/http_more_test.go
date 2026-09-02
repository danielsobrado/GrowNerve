package farm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	commanddomain "github.com/jdanielsobrado/grownerve/internal/command"
	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
)

type failingStore struct {
	load             json.RawMessage
	loadErr, saveErr error
}

func (store failingStore) Load(context.Context) (json.RawMessage, int64, error) {
	return store.load, 1, store.loadErr
}

func (store failingStore) Save(context.Context, json.RawMessage, int64) (int64, error) {
	return 0, store.saveErr
}

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
	_, _ = store.Save(context.Background(), json.RawMessage(`{"facilities":[{"id":"one"}]}`), AnyVersion)
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
	if versionETag(7) != `"v7"` {
		t.Fatalf("ETag = %s", versionETag(7))
	}
	if version, valid := parseETag(`W/"v7"`); !valid || version != 7 {
		t.Fatalf("parseETag = %d %v", version, valid)
	}
	if _, valid := parseETag(`"deadbeef"`); valid {
		t.Fatal("hash-style ETag accepted as a version")
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
	_, _ = store.Save(context.Background(), json.RawMessage(commandStateJSON(true, 25, 100)), AnyVersion)
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
	_, _ = store.Save(context.Background(), json.RawMessage(commandStateJSON(true, 25, 100)), AnyVersion)
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

func TestIdempotencyKeyCannotBeReusedForDifferentCommand(t *testing.T) {
	store := NewMemoryStore()
	_, _ = store.Save(context.Background(), json.RawMessage(commandStateJSON(true, 0, 100)), AnyVersion)
	handler := NewHandler(store)

	send := func(value int) *httptest.ResponseRecorder {
		body := `{"targetChannelId":"01990a20-6a00-7000-8000-000000000001","value":` + strconv.Itoa(value) + `,"reason":"test"}`
		request := httptest.NewRequest(http.MethodPost, "/api/v1/commands", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "stable-key")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	first := send(5)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first command = %d: %s", first.Code, first.Body.String())
	}
	second := send(6)
	if second.Code != http.StatusConflict || !strings.Contains(second.Body.String(), "IDEMPOTENCY_KEY_REUSED") {
		t.Fatalf("different command reused key = %d: %s", second.Code, second.Body.String())
	}
}

func TestCommandParsingAcceptsJSONParametersAndRejectsTrailingData(t *testing.T) {
	store := NewMemoryStore()
	_, _ = store.Save(context.Background(), json.RawMessage(commandStateJSON(true, 0, 100)), AnyVersion)
	handler := NewHandler(store)
	body := `{"targetChannelId":"01990a20-6a00-7000-8000-000000000001","value":50,"reason":"test"}`

	if response := makeRequest(handler, http.MethodPost, "/api/v1/commands", body, "application/json; charset=utf-8"); response.Code != http.StatusAccepted {
		t.Fatalf("JSON with charset = %d: %s", response.Code, response.Body.String())
	}
	if response := makeRequest(handler, http.MethodPost, "/api/v1/commands", body+` {}`, "application/json"); response.Code != http.StatusBadRequest {
		t.Fatalf("trailing JSON = %d: %s", response.Code, response.Body.String())
	}
}

func TestCommandBodyLimitReturns413(t *testing.T) {
	store := NewMemoryStore()
	_, _ = store.Save(context.Background(), json.RawMessage(commandStateJSON(true, 0, 100)), AnyVersion)
	handler := NewHandler(store)
	body := `{"targetChannelId":"01990a20-6a00-7000-8000-000000000001","value":50,"reason":"` + strings.Repeat("x", 70<<10) + `"}`
	response := makeRequest(handler, http.MethodPost, "/api/v1/commands", body, "application/json")
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized command = %d: %s", response.Code, response.Body.String())
	}
}

func TestCommandRejectsExcessiveTTLAndUnsupportedValueType(t *testing.T) {
	now := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	_, _ = store.Save(context.Background(), json.RawMessage(commandStateJSON(true, 0, 100)), AnyVersion)
	handler := NewHandler(store, WithClock(func() time.Time { return now }))
	future := now.Add(commanddomain.MaximumTTL + time.Second).Format(time.RFC3339Nano)
	body := `{"targetChannelId":"01990a20-6a00-7000-8000-000000000001","value":50,"reason":"test","expiresAt":"` + future + `"}`
	response := makeRequest(handler, http.MethodPost, "/api/v1/commands", body, "application/json")
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "COMMAND_TTL_TOO_LONG") {
		t.Fatalf("excessive TTL = %d: %s", response.Code, response.Body.String())
	}

	state := strings.Replace(commandStateJSON(true, 0, 100), `"value_type":"number"`, `"value_type":"enum"`, 1)
	_, _ = store.Save(context.Background(), json.RawMessage(state), AnyVersion)
	response = makeRequest(NewHandler(store), http.MethodPost, "/api/v1/commands", `{"targetChannelId":"01990a20-6a00-7000-8000-000000000001","value":50,"reason":"test"}`, "application/json")
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "UNSUPPORTED_COMMAND_CHANNEL") {
		t.Fatalf("unsupported value type = %d: %s", response.Code, response.Body.String())
	}
}

func TestMemoryStoreCopiesState(t *testing.T) {
	store := NewMemoryStore()
	if _, _, err := store.Load(context.Background()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() = %v", err)
	}
	input := json.RawMessage(`{"a":1}`)
	_, _ = store.Save(context.Background(), input, AnyVersion)
	input[2] = 'x'
	loaded, _, _ := store.Load(context.Background())
	loaded[2] = 'y'
	again, _, _ := store.Load(context.Background())
	if string(again) != `{"a":1}` {
		t.Fatalf("state aliased: %s", again)
	}
}

type recordingPublisher struct{ called bool }

func (publisher *recordingPublisher) PublishCommand(context.Context, string, deviceprotocol.Command) error {
	publisher.called = true
	return nil
}

func TestAcceptedCommandPublishesAfterPersistence(t *testing.T) {
	store := NewMemoryStore()
	_, _ = store.Save(context.Background(), json.RawMessage(commandStateJSON(true, 25, 100)), AnyVersion)
	publisher := &recordingPublisher{}
	handler := NewHandler(store, WithCommandPublisher(publisher))
	body := `{"targetChannelId":"01990a20-6a00-7000-8000-000000000001","value":50,"reason":"test","expiresAt":"` + time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano) + `"}`
	response := makeRequest(handler, http.MethodPost, "/api/v1/commands", body, "application/json")
	if response.Code != http.StatusAccepted || !publisher.called || !strings.Contains(response.Body.String(), `"status":"published"`) {
		t.Fatalf("response = %d %s, called=%v", response.Code, response.Body.String(), publisher.called)
	}
}
