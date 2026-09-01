package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func digestOf(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func TestRoleRanking(t *testing.T) {
	if !RoleManager.AtLeast(RoleOperator) {
		t.Fatal("manager does not satisfy an operator requirement")
	}
	if RoleOperator.AtLeast(RoleManager) {
		t.Fatal("operator satisfied a manager requirement")
	}
	if _, err := ParseRole("superuser"); err == nil {
		t.Fatal("an unrecognised role was accepted")
	}
	// An unknown role must never fall through to a permissive default.
	if Role("").Valid() {
		t.Fatal("the empty role is treated as valid")
	}
}

func TestLocalAuthenticatorAcceptsOnlyConfiguredTokens(t *testing.T) {
	authenticator, err := NewLocalAuthenticator("alice:manager:" + digestOf("alice-token") + ",bob:viewer:" + digestOf("bob-token"))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/state", nil)
	request.Header.Set("Authorization", "Bearer alice-token")
	principal, err := authenticator.Authenticate(request)
	if err != nil {
		t.Fatal(err)
	}
	if principal.Subject != "alice" || principal.Role != RoleManager {
		t.Fatalf("principal = %+v", principal)
	}

	for name, header := range map[string]string{
		"no header":     "",
		"wrong token":   "Bearer not-a-token",
		"empty bearer":  "Bearer ",
		"basic auth":    "Basic YWxpY2U6c2VjcmV0",
		"raw token":     "alice-token",
		"token as user": "Bearer alice",
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/state", nil)
			if header != "" {
				request.Header.Set("Authorization", header)
			}
			if _, err := authenticator.Authenticate(request); err == nil {
				t.Fatal("authentication succeeded without a valid token")
			}
		})
	}
}

func TestLocalAuthenticatorRefusesMalformedConfiguration(t *testing.T) {
	for name, specification := range map[string]string{
		"empty":         "",
		"missing parts": "alice:manager",
		"unknown role":  "alice:wizard:" + digestOf("t"),
		"short digest":  "alice:manager:abcd",
		"not hex":       "alice:manager:zzzz",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewLocalAuthenticator(specification); err == nil {
				t.Fatal("malformed account configuration was accepted")
			}
		})
	}
}

func TestMiddlewareBlocksUnauthenticatedRequestsButNotProbes(t *testing.T) {
	authenticator, err := NewLocalAuthenticator("alice:operator:" + digestOf("alice-token"))
	if err != nil {
		t.Fatal(err)
	}
	reached := false
	var seen Principal
	var sawPrincipal bool
	guarded := Middleware(authenticator, []string{"/health", "/version"}, slog.New(slog.NewTextHandler(io.Discard, nil)))(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			reached = true
			seen, sawPrincipal = PrincipalFrom(request.Context())
			writer.WriteHeader(http.StatusOK)
		}))

	// A probe passes without a credential, because an orchestrator has none.
	response := httptest.NewRecorder()
	guarded.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("liveness probe status = %d", response.Code)
	}
	// A probe carries no identity, so nothing downstream may mistake it for one.
	if sawPrincipal {
		t.Fatalf("an unauthenticated probe was given a principal: %+v", seen)
	}

	// An API path without a credential never reaches the handler.
	reached = false
	response = httptest.NewRecorder()
	guarded.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/commands", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated command status = %d, want 401", response.Code)
	}
	if reached {
		t.Fatal("an unauthenticated request reached the handler")
	}
	if response.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("401 did not tell the client how to authenticate")
	}

	// With a credential it passes through and carries the principal.
	request := httptest.NewRequest(http.MethodPost, "/api/v1/commands", nil)
	request.Header.Set("Authorization", "Bearer alice-token")
	response = httptest.NewRecorder()
	guarded.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !reached {
		t.Fatalf("authenticated request status = %d, reached = %v", response.Code, reached)
	}
	if !sawPrincipal || seen.Subject != "alice" || seen.Role != RoleOperator {
		t.Fatalf("handler received principal %+v", seen)
	}
}

func TestPublicPrefixesDoNotLeakToSiblingPaths(t *testing.T) {
	// "/health" must not exempt "/healthcheck-admin" or "/api/v1/health-secrets".
	for path, exempt := range map[string]bool{
		"/health":        true,
		"/health/live":   true,
		"/healthz":       false,
		"/health-admin":  false,
		"/api/v1/health": false,
		"/version":       true,
		"/versions/leak": false,
	} {
		if got := isPublic(path, []string{"/health", "/version"}); got != exempt {
			t.Fatalf("isPublic(%q) = %v, want %v", path, got, exempt)
		}
	}
}
