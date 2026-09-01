package media

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

type storedMetadata struct {
	Object    Object `json:"object"`
	Extension string `json:"extension"`
}

func (store *FilesystemStore) writeMetadata(object Object, extension string) error {
	encoded, err := json.Marshal(storedMetadata{Object: object, Extension: extension})
	if err != nil {
		return err
	}
	path := store.metadataPath(object.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o640)
}

func (store *FilesystemStore) readMetadata(id string) (Object, string, error) {
	// The identifier is validated as a UUID before it is used in a path, so a
	// caller cannot address anything outside the store.
	if _, err := uuid.Parse(id); err != nil {
		return Object{}, "", ErrNotFound
	}
	contents, err := os.ReadFile(store.metadataPath(id))
	if err != nil {
		return Object{}, "", ErrNotFound
	}
	var stored storedMetadata
	if err := json.Unmarshal(contents, &stored); err != nil {
		return Object{}, "", ErrNotFound
	}
	return stored.Object, stored.Extension, nil
}

// safeFilename keeps a readable label for the operator without letting it
// influence storage. Separators and control characters are removed because the
// value is echoed back in a Content-Disposition header.
func safeFilename(filename string) string {
	base := filepath.Base(strings.ReplaceAll(filename, `\`, "/"))
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, base)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "upload"
	}
	if len(cleaned) > 120 {
		cleaned = cleaned[:120]
	}
	return cleaned
}
