package farm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// TestConcurrentStateWritesNeverLoseUpdates is the regression guard for the
// lost-update defect: fifty clients each read the state, append one facility and
// write it back with the ETag they read. Before the store became a
// compare-and-swap, six of the fifty writes survived and the other forty-four
// were reported to their clients as successful.
//
// Every writer must now either land or be told it conflicted. Silent loss is the
// failure this test exists to catch.
func TestConcurrentStateWritesNeverLoseUpdates(t *testing.T) {
	const writers = 50

	store := NewMemoryStore()
	if _, err := store.Save(context.Background(), json.RawMessage(`{"facilities":[]}`), AnyVersion); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store)

	var mu sync.Mutex
	accepted, conflicted := 0, 0

	var group sync.WaitGroup
	for writer := 0; writer < writers; writer++ {
		group.Add(1)
		go func(id int) {
			defer group.Done()
			// Retry on conflict exactly as a well-behaved client would, so the
			// test measures durability rather than first-attempt luck.
			for attempt := 0; attempt < 200; attempt++ {
				read := httptest.NewRecorder()
				handler.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/api/v1/state", nil))
				etag := read.Header().Get("ETag")

				var object map[string]json.RawMessage
				if err := json.Unmarshal(read.Body.Bytes(), &object); err != nil {
					t.Errorf("writer %d: unreadable state: %v", id, err)
					return
				}
				var facilities []json.RawMessage
				if err := json.Unmarshal(object["facilities"], &facilities); err != nil {
					t.Errorf("writer %d: unreadable facilities: %v", id, err)
					return
				}
				facilities = append(facilities, json.RawMessage(`{"id":"appended"}`))
				object["facilities"], _ = json.Marshal(facilities)
				body, _ := json.Marshal(object)

				request := httptest.NewRequest(http.MethodPut, "/api/v1/state", strings.NewReader(string(body)))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("If-Match", etag)
				write := httptest.NewRecorder()
				handler.ServeHTTP(write, request)

				mu.Lock()
				switch write.Code {
				case http.StatusNoContent:
					accepted++
					mu.Unlock()
					return
				case http.StatusConflict:
					conflicted++
					mu.Unlock()
				default:
					mu.Unlock()
					t.Errorf("writer %d: unexpected status %d: %s", id, write.Code, write.Body.String())
					return
				}
			}
			t.Errorf("writer %d: never won a write", id)
		}(writer)
	}
	group.Wait()

	final, _, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(final, &object); err != nil {
		t.Fatal(err)
	}
	var facilities []json.RawMessage
	if err := json.Unmarshal(object["facilities"], &facilities); err != nil {
		t.Fatal(err)
	}
	if len(facilities) != writers {
		t.Fatalf("stored %d facilities from %d accepted writes; %d writes were lost (conflicts reported: %d)",
			len(facilities), accepted, writers-len(facilities), conflicted)
	}
	if accepted != writers {
		t.Fatalf("accepted %d writes, want %d", accepted, writers)
	}
}

// TestStaleWriteIsRefusedNotApplied proves the conflict is reported rather than
// silently overwriting a newer document.
func TestStaleWriteIsRefusedNotApplied(t *testing.T) {
	store := NewMemoryStore()
	handler := NewHandler(store)
	if _, err := store.Save(context.Background(), json.RawMessage(`{"facilities":[]}`), AnyVersion); err != nil {
		t.Fatal(err)
	}

	read := httptest.NewRecorder()
	handler.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/api/v1/state", nil))
	staleETag := read.Header().Get("ETag")

	// Another writer moves the state on.
	if _, err := store.Save(context.Background(), json.RawMessage(`{"facilities":[{"id":"winner"}]}`), 1); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPut, "/api/v1/state", strings.NewReader(`{"facilities":[{"id":"loser"}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("If-Match", staleETag)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("stale write status = %d, want 409", response.Code)
	}
	current, _, _ := store.Load(context.Background())
	if !strings.Contains(string(current), "winner") {
		t.Fatalf("stale write overwrote the newer document: %s", current)
	}
}

// TestMutateRetriesUntilItWins covers the helper every read-modify-write path
// uses, including the MQTT bridge.
func TestMutateRetriesUntilItWins(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.Save(context.Background(), json.RawMessage(`{"n":0}`), AnyVersion); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for writer := 0; writer < 25; writer++ {
		group.Add(1)
		go func() {
			defer group.Done()
			err := Mutate(context.Background(), store, func(state json.RawMessage) (json.RawMessage, error) {
				var document struct {
					N int `json:"n"`
				}
				if err := json.Unmarshal(state, &document); err != nil {
					return nil, err
				}
				document.N++
				return json.Marshal(document)
			})
			if err != nil {
				t.Errorf("Mutate() = %v", err)
			}
		}()
	}
	group.Wait()

	final, _, _ := store.Load(context.Background())
	var document struct {
		N int `json:"n"`
	}
	if err := json.Unmarshal(final, &document); err != nil {
		t.Fatal(err)
	}
	if document.N != 25 {
		t.Fatalf("counter = %d, want 25: %d increments were lost", document.N, 25-document.N)
	}
}
