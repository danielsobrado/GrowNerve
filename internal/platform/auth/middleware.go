package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// Middleware establishes the principal for every request. Requests that fail
// authentication are rejected here rather than reaching a handler, so no
// endpoint can be reached without an identity having been decided.
//
// Paths in public are exempt: liveness and readiness must answer an orchestrator
// that holds no credential.
func Middleware(authenticator Authenticator, public []string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if isPublic(request.URL.Path, public) {
				next.ServeHTTP(writer, request)
				return
			}
			principal, err := authenticator.Authenticate(request)
			if err != nil {
				// The reason is logged but never returned: telling a caller why a
				// credential failed helps an attacker more than a legitimate user.
				logger.Warn("authentication_failed", "path", request.URL.Path, "mode", authenticator.Mode(), "error", err)
				writer.Header().Set("WWW-Authenticate", `Bearer realm="grownerve"`)
				writeUnauthorized(writer, request)
				return
			}
			next.ServeHTTP(writer, request.WithContext(WithPrincipal(request.Context(), principal)))
		})
	}
}

func isPublic(path string, public []string) bool {
	for _, prefix := range public {
		if path == prefix || strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")+"/") {
			return true
		}
	}
	return false
}

func writeUnauthorized(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/problem+json")
	writer.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"type": "https://grownerve.local/problems/unauthenticated", "title": "Unauthorized",
		"status": http.StatusUnauthorized, "detail": "Sign in to use this endpoint",
		"instance": request.URL.Path, "code": "UNAUTHENTICATED",
		"correlationId": request.Header.Get("X-Correlation-ID"),
	})
}
