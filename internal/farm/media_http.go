package farm

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jdanielsobrado/grownerve/internal/media"
)

// WithMediaStore enables photograph upload and retrieval.
func WithMediaStore(store media.Store) HandlerOption {
	return func(handler *Handler) { handler.media = store }
}

// maximumUploadBytes bounds the whole multipart request. The store applies its
// own per-object limit; this one stops a request from being buffered at all.
const maximumUploadBytes = 32 << 20

func (handler *Handler) uploadMedia(writer http.ResponseWriter, request *http.Request) {
	if !handler.permit(writer, request, ActionObserve) {
		return
	}
	if handler.media == nil {
		writeProblem(writer, request, http.StatusServiceUnavailable, "MEDIA_UNAVAILABLE", "Media storage is not configured")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maximumUploadBytes)
	file, header, err := request.FormFile("file")
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "MEDIA_FILE_REQUIRED", "Attach the image as a form field named file")
		return
	}
	defer func() { _ = file.Close() }()

	object, err := handler.media.Put(request.Context(), header.Filename, file)
	switch {
	case errors.Is(err, media.ErrTooLarge):
		writeProblem(writer, request, http.StatusRequestEntityTooLarge, "MEDIA_TOO_LARGE", "The image is larger than this deployment allows")
		return
	case errors.Is(err, media.ErrUnsupportedType):
		writeProblem(writer, request, http.StatusUnsupportedMediaType, "MEDIA_TYPE_UNSUPPORTED", "Upload a JPEG, PNG, WebP, or GIF image")
		return
	case err != nil:
		writeProblem(writer, request, http.StatusInternalServerError, "MEDIA_WRITE_FAILED", "The image could not be stored")
		return
	}
	handler.record(request.Context(), AuditEntry{
		Actor: ActorOf(request), Action: "media.uploaded", TargetType: "media", TargetID: object.ID,
		CorrelationID: request.Header.Get("X-Correlation-ID"),
		Detail:        map[string]any{"mime_type": object.MimeType, "bytes": object.SizeBytes},
	})
	handler.notify("media")
	writeJSON(writer, http.StatusCreated, object)
}

func (handler *Handler) downloadMedia(writer http.ResponseWriter, request *http.Request) {
	if !handler.permit(writer, request, ActionRead) {
		return
	}
	if handler.media == nil {
		writeProblem(writer, request, http.StatusServiceUnavailable, "MEDIA_UNAVAILABLE", "Media storage is not configured")
		return
	}
	object, content, err := handler.media.Get(request.Context(), request.PathValue("id"))
	if err != nil {
		writeProblem(writer, request, http.StatusNotFound, "MEDIA_NOT_FOUND", "That image does not exist")
		return
	}
	defer func() { _ = content.Close() }()

	writer.Header().Set("Content-Type", object.MimeType)
	writer.Header().Set("Content-Length", fmt.Sprintf("%d", object.SizeBytes))
	writer.Header().Set("ETag", `"`+object.SHA256+`"`)
	// Stored content is served as an attachment and with sniffing disabled, so a
	// file that slipped past validation still cannot execute in the page's origin.
	writer.Header().Set("Content-Disposition", "attachment; filename=\""+strings.ReplaceAll(object.Filename, `"`, "")+`"`)
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(http.StatusOK)
	_, _ = io.Copy(writer, content)
}
