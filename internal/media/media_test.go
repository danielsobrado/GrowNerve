package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pngBytes is a minimal valid PNG: enough for content sniffing to identify it.
var pngBytes = append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, bytes.Repeat([]byte{0}, 64)...)

func TestStoresAndReadsBackAnImage(t *testing.T) {
	store, err := NewFilesystemStore(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	object, err := store.Put(context.Background(), "roots.png", bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatal(err)
	}
	if object.MimeType != "image/png" || object.SizeBytes != int64(len(pngBytes)) {
		t.Fatalf("object = %+v", object)
	}
	if object.SHA256 == "" || object.ID == "" {
		t.Fatalf("object is missing its digest or identifier: %+v", object)
	}

	stored, content, err := store.Get(context.Background(), object.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = content.Close() }()
	roundTripped, _ := io.ReadAll(content)
	if !bytes.Equal(roundTripped, pngBytes) {
		t.Fatal("stored content does not match what was written")
	}
	if stored.SHA256 != object.SHA256 {
		t.Fatal("metadata did not survive the round trip")
	}
}

func TestRejectsOversizedUploads(t *testing.T) {
	store, err := NewFilesystemStore(t.TempDir(), 32)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Put(context.Background(), "big.png", bytes.NewReader(pngBytes))
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

// TestRejectsContentThatCanExecuteInABrowser covers the reason the allow-list is
// based on sniffed bytes: a hostile file renamed to .png must still be refused.
func TestRejectsContentThatCanExecuteInABrowser(t *testing.T) {
	store, err := NewFilesystemStore(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"html disguised as png": `<html><script>alert(1)</script></html>`,
		"svg with script":       `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"empty":                 "",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := store.Put(context.Background(), "photo.png", strings.NewReader(content)); !errors.Is(err, ErrUnsupportedType) {
				t.Fatalf("err = %v, want ErrUnsupportedType", err)
			}
		})
	}
}

// TestHostileFilenamesCannotEscapeTheStore proves the client's filename never
// reaches the filesystem path.
func TestHostileFilenamesCannotEscapeTheStore(t *testing.T) {
	root := t.TempDir()
	store, err := NewFilesystemStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{
		"../../../../etc/passwd", `..\..\windows\system32\config`, "/absolute/path.png", "with\x00null.png",
	} {
		object, err := store.Put(context.Background(), filename, bytes.NewReader(pngBytes))
		if err != nil {
			t.Fatalf("%q: %v", filename, err)
		}
		if strings.ContainsAny(object.Filename, `/\`) {
			t.Fatalf("separator survived sanitisation: %q", object.Filename)
		}
	}
	// Everything written must live beneath the configured root.
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasPrefix(path, root) {
			t.Fatalf("object written outside the store: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnknownAndMalformedIdentifiersAreNotFound(t *testing.T) {
	store, err := NewFilesystemStore(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "../../etc/passwd", "not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		if _, _, err := store.Get(context.Background(), id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get(%q) = %v, want ErrNotFound", id, err)
		}
	}
}

func TestDeleteRemovesContentAndMetadata(t *testing.T) {
	store, err := NewFilesystemStore(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	object, err := store.Put(context.Background(), "roots.png", bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), object.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Get(context.Background(), object.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted object still readable: %v", err)
	}
}
