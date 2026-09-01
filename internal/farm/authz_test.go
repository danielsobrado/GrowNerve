package farm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/platform/auth"
)

// safeCommandState wires one commandable channel on one online device with a
// 25-100 safety range.
const safeCommandState = `{
  "channels":[{"id":"channel-1","device_id":"device-1","kind":"command","value_type":"number","safe_minimum":25,"safe_maximum":100},
              {"id":"sensor-1","device_id":"device-1","kind":"measurement","value_type":"number"}],
  "devices":[{"id":"device-1","online":true}],
  "commands":[],"events":[]
}`

func authorizedHandler(t *testing.T, state string) http.Handler {
	t.Helper()
	store := NewMemoryStore()
	if _, err := store.Save(context.Background(), json.RawMessage(state), AnyVersion); err != nil {
		t.Fatal(err)
	}
	return NewHandler(store, WithAuthorizer(RoleAuthorizer{}))
}

func as(role auth.Role, request *http.Request) *http.Request {
	if role == "" {
		return request
	}
	return request.WithContext(auth.WithPrincipal(request.Context(),
		auth.Principal{Subject: string(role) + "-user", DisplayName: "Test", Role: role}))
}

func TestRoleRequirementsPerAction(t *testing.T) {
	handler := authorizedHandler(t, safeCommandState)
	command := `{"targetChannelId":"channel-1","value":50,"reason":"manual check"}`

	tests := []struct {
		name   string
		role   auth.Role
		method string
		path   string
		body   string
		want   int
	}{
		{"anonymous read", "", http.MethodGet, "/api/v1/state", "", http.StatusUnauthorized},
		{"anonymous command", "", http.MethodPost, "/api/v1/commands", command, http.StatusUnauthorized},
		{"viewer reads", auth.RoleViewer, http.MethodGet, "/api/v1/state", "", http.StatusOK},
		{"viewer cannot command", auth.RoleViewer, http.MethodPost, "/api/v1/commands", command, http.StatusForbidden},
		{"operator commands", auth.RoleOperator, http.MethodPost, "/api/v1/commands", command, http.StatusAccepted},
		{"operator cannot rewrite configuration", auth.RoleOperator, http.MethodPut, "/api/v1/state", `{"facilities":[]}`, http.StatusForbidden},
		{"manager rewrites configuration", auth.RoleManager, http.MethodPut, "/api/v1/state", `{"facilities":[]}`, http.StatusNoContent},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
			if testCase.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, as(testCase.role, request))
			if response.Code != testCase.want {
				t.Fatalf("status = %d, want %d: %s", response.Code, testCase.want, response.Body.String())
			}
		})
	}
}

// TestAuthorizationIsSeparateFromSafety proves the two checks are independent:
// a permitted caller is still refused by an interlock.
func TestAuthorizationIsSeparateFromSafety(t *testing.T) {
	handler := authorizedHandler(t, safeCommandState)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/commands",
		strings.NewReader(`{"targetChannelId":"channel-1","value":5,"reason":"below the safe minimum"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, as(auth.RoleAdministrator, request))

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: an administrator bypassed a safety limit", response.Code)
	}
	if !strings.Contains(response.Body.String(), "COMMAND_VALUE_OUT_OF_RANGE") {
		t.Fatalf("body = %s", response.Body.String())
	}
}

// TestUnsafeCommandPathsAreRefused is the negative matrix the release definition
// of done requires: one case per rejection reason, asserted at the HTTP boundary
// rather than only in the domain unit.
func TestUnsafeCommandPathsAreRefused(t *testing.T) {
	expired := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	offlineState := strings.Replace(safeCommandState, `"online":true`, `"online":false`, 1)

	tests := []struct {
		name       string
		state      string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name: "value below the safe minimum", state: safeCommandState,
			body:       `{"targetChannelId":"channel-1","value":5,"reason":"too low"}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: "COMMAND_VALUE_OUT_OF_RANGE",
		},
		{
			name: "value above the safe maximum", state: safeCommandState,
			body:       `{"targetChannelId":"channel-1","value":400,"reason":"too high"}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: "COMMAND_VALUE_OUT_OF_RANGE",
		},
		{
			name: "device offline", state: offlineState,
			body:       `{"targetChannelId":"channel-1","value":50,"reason":"device is not there"}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: "DEVICE_OFFLINE",
		},
		{
			name: "channel is not controllable", state: safeCommandState,
			body:       `{"targetChannelId":"sensor-1","value":50,"reason":"sensor is not an actuator"}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: "CHANNEL_NOT_CONTROLLABLE",
		},
		{
			name: "already expired", state: safeCommandState,
			body:       `{"targetChannelId":"channel-1","value":50,"reason":"stale request","expiresAt":"` + expired + `"}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: "COMMAND_EXPIRED",
		},
		{
			name: "unknown channel", state: safeCommandState,
			body:       `{"targetChannelId":"nope","value":50,"reason":"typo"}`,
			wantStatus: http.StatusNotFound, wantCode: "UNKNOWN_CHANNEL",
		},
		{
			name: "no reason given", state: safeCommandState,
			body:       `{"targetChannelId":"channel-1","value":50,"reason":"   "}`,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_COMMAND",
		},
		{
			name: "unknown field", state: safeCommandState,
			body:       `{"targetChannelId":"channel-1","value":50,"reason":"x","force":true}`,
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_COMMAND",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			handler := authorizedHandler(t, testCase.state)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/commands", strings.NewReader(testCase.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, as(auth.RoleOperator, request))

			if response.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, testCase.wantStatus, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), testCase.wantCode) {
				t.Fatalf("response does not name %s: %s", testCase.wantCode, response.Body.String())
			}
		})
	}
}

// TestRejectedCommandIsRecordedNotDiscarded proves a refusal leaves an audit
// trail an operator can find later.
func TestRejectedCommandIsRecordedNotDiscarded(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.Save(context.Background(), json.RawMessage(safeCommandState), AnyVersion); err != nil {
		t.Fatal(err)
	}
	recorder := &collectingRecorder{}
	handler := NewHandler(store, WithAuthorizer(RoleAuthorizer{}), WithAuditRecorder(recorder))

	request := httptest.NewRequest(http.MethodPost, "/api/v1/commands",
		strings.NewReader(`{"targetChannelId":"channel-1","value":5,"reason":"unsafe"}`))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(httptest.NewRecorder(), as(auth.RoleOperator, request))

	if len(recorder.entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(recorder.entries))
	}
	entry := recorder.entries[0]
	if entry.Action != "command.requested" || entry.Actor != "operator-user" {
		t.Fatalf("entry = %+v", entry)
	}
	if entry.Detail["rejection"] != "COMMAND_VALUE_OUT_OF_RANGE" {
		t.Fatalf("audit entry does not record the rejection: %+v", entry.Detail)
	}

	// The rejected command is persisted too, so the history shows the attempt.
	stored, _, _ := store.Load(context.Background())
	if !strings.Contains(string(stored), `"status":"rejected"`) {
		t.Fatalf("rejected command was not persisted: %s", stored)
	}
}

type collectingRecorder struct{ entries []AuditEntry }

func (recorder *collectingRecorder) Record(_ context.Context, entry AuditEntry) {
	recorder.entries = append(recorder.entries, entry)
}
