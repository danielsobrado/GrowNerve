package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestCorruptMetadataCannotRedirectObjectPath(t *testing.T) {
	store, err := NewFilesystemStore(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	object, err := store.Put(context.Background(), "roots.png", bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatal(err)
	}
	corrupt, err := json.Marshal(storedMetadata{Object: object, Extension: "../../outside"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.metadataPath(object.ID), corrupt, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Get(context.Background(), object.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("corrupt metadata error = %v, want ErrNotFound", err)
	}
}

func TestStoredSizeMismatchFailsClosed(t *testing.T) {
	store, err := NewFilesystemStore(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	object, err := store.Put(context.Background(), "roots.png", bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.pathFor(object.ID, ".png"), pngBytes[:8], 0o640); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Get(context.Background(), object.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("truncated object error = %v, want ErrNotFound", err)
	}
}
