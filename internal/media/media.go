// Package media stores operator photographs. Uploads are validated by sniffed
// content rather than by a client-supplied type or filename, and every object is
// written under a generated key so a hostile filename cannot escape the store.
package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Object is a stored file's metadata.
type Object struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	MimeType  string    `json:"mime_type"`
	SizeBytes int64     `json:"size_bytes"`
	SHA256    string    `json:"sha256"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	// ErrTooLarge reports an upload beyond the configured limit.
	ErrTooLarge = errors.New("media object exceeds the configured size limit")
	// ErrUnsupportedType reports content that is not an accepted image.
	ErrUnsupportedType = errors.New("media type is not supported")
	// ErrNotFound reports a missing object.
	ErrNotFound = errors.New("media object not found")
)

// acceptedTypes is an allow-list. Anything not sniffed as one of these is
// refused, which keeps HTML and SVG — both of which can carry script — out of a
// store whose contents are served back to browsers.
var acceptedTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// Store persists media objects.
type Store interface {
	Put(ctx context.Context, filename string, content io.Reader) (Object, error)
	Get(ctx context.Context, id string) (Object, io.ReadCloser, error)
	Delete(ctx context.Context, id string) error
}

// FilesystemStore writes objects beneath a single directory.
type FilesystemStore struct {
	root         string
	maximumBytes int64
}

func NewFilesystemStore(root string, maximumBytes int64) (*FilesystemStore, error) {
	if maximumBytes <= 0 {
		maximumBytes = 16 << 20
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		return nil, fmt.Errorf("create media directory: %w", err)
	}
	return &FilesystemStore{root: absolute, maximumBytes: maximumBytes}, nil
}

// Put validates and stores content. The upload is read through a limit so a
// client cannot exhaust disk by lying about its size, and the type is decided by
// sniffing the leading bytes rather than by trusting the request.
func (store *FilesystemStore) Put(_ context.Context, filename string, content io.Reader) (Object, error) {
	// One byte beyond the limit is read so an oversized upload is detected
	// rather than silently truncated.
	buffered, err := io.ReadAll(io.LimitReader(content, store.maximumBytes+1))
	if err != nil {
		return Object{}, err
	}
	if int64(len(buffered)) > store.maximumBytes {
		return Object{}, ErrTooLarge
	}
	if len(buffered) == 0 {
		return Object{}, ErrUnsupportedType
	}
	mimeType := http.DetectContentType(buffered)
	extension, accepted := acceptedTypes[mimeType]
	if !accepted {
		return Object{}, fmt.Errorf("%w: %s", ErrUnsupportedType, mimeType)
	}

	digest := sha256.Sum256(buffered)
	object := Object{
		ID: uuid.NewString(), Filename: safeFilename(filename), MimeType: mimeType,
		SizeBytes: int64(len(buffered)), SHA256: hex.EncodeToString(digest[:]), CreatedAt: time.Now().UTC(),
	}
	path := store.pathFor(object.ID, extension)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return Object{}, err
	}
	if err := os.WriteFile(path, buffered, 0o640); err != nil {
		return Object{}, err
	}
	if err := store.writeMetadata(object, extension); err != nil {
		_ = os.Remove(path)
		return Object{}, err
	}
	return object, nil
}

func (store *FilesystemStore) Get(_ context.Context, id string) (Object, io.ReadCloser, error) {
	object, extension, err := store.readMetadata(id)
	if err != nil {
		return Object{}, nil, err
	}
	file, err := os.Open(store.pathFor(id, extension))
	if err != nil {
		return Object{}, nil, ErrNotFound
	}
	return object, file, nil
}

func (store *FilesystemStore) Delete(_ context.Context, id string) error {
	_, extension, err := store.readMetadata(id)
	if err != nil {
		return err
	}
	if err := os.Remove(store.pathFor(id, extension)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(store.metadataPath(id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// pathFor builds the storage path from the generated identifier only. The
// client's filename never reaches the filesystem, so path traversal is not
// something this store has to defend against after the fact.
func (store *FilesystemStore) pathFor(id, extension string) string {
	return filepath.Join(store.root, "objects", id[:2], id+extension)
}

func (store *FilesystemStore) metadataPath(id string) string {
	return filepath.Join(store.root, "objects", id[:2], id+".meta.json")
}
