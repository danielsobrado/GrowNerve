package farm

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

var ErrNotFound = errors.New("farm state not found")

type Store interface {
	Load(context.Context) (json.RawMessage, error)
	Save(context.Context, json.RawMessage) error
}

type MemoryStore struct {
	mu    sync.RWMutex
	state json.RawMessage
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

func (store *MemoryStore) Load(_ context.Context) (json.RawMessage, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	if len(store.state) == 0 {
		return nil, ErrNotFound
	}
	return append(json.RawMessage(nil), store.state...), nil
}

func (store *MemoryStore) Save(_ context.Context, state json.RawMessage) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.state = append(json.RawMessage(nil), state...)
	return nil
}
