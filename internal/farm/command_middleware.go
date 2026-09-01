package farm

import (
	"net/http"
	"strings"
)

const (
	minimumIdempotencyKeyLength = 8
	maximumIdempotencyKeyLength = 128
)

// RequireCommandIdempotency makes retry safety an HTTP invariant for the
// physical command endpoint rather than a convention individual clients may
// forget to follow.
func RequireCommandIdempotency(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost && request.URL.Path == "/api/v1/commands" {
			key := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
			if len(key) < minimumIdempotencyKeyLength || len(key) > maximumIdempotencyKeyLength {
				writeProblem(writer, request, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key must contain between 8 and 128 characters")
				return
			}
			request.Header.Set("Idempotency-Key", key)
		}
		next.ServeHTTP(writer, request)
	})
}
