package farm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/platform/auth"
	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

func TestObserveDevicesKeepsUnknownFieldsAndSkipsNoOpWrites(t *testing.T) {
	store := NewMemoryStore()
	state := `{"devices":[{"id":"plug","online":false,"integration":{"provider":"zigbee2mqtt","external_id":"0x1"}},{"id":"other","online":true}]}`
	if _, err := store.Save(context.Background(), json.RawMessage(state), AnyVersion); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	on := true
	changed, err := ObserveDevices(context.Background(), store, map[string]DeviceObservation{
		"plug": {Online: &on, Heartbeat: true, State: &on}, "missing": {Online: &on},
	}, now)
	if err != nil || !changed {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
	stored := string(mustLoad(t, store))
	for _, expected := range []string{`"integration":{"external_id":"0x1","provider":"zigbee2mqtt"}`, `"state":true`, `"last_heartbeat":"2026-09-30T12:00:00Z"`, `"id":"other","online":true`} {
		if !strings.Contains(stored, expected) {
			t.Fatalf("state lost %s: %s", expected, stored)
		}
	}
	_, versionBefore, _ := store.Load(context.Background())
	changed, err = ObserveDevices(context.Background(), store, map[string]DeviceObservation{"plug": {Online: &on, State: &on}}, now)
	_, versionAfter, _ := store.Load(context.Background())
	if err != nil || changed || versionAfter != versionBefore {
		t.Fatalf("no-op observation wrote the document: changed = %v, versions %d -> %d", changed, versionBefore, versionAfter)
	}
}

// recordingCommitter proves CommitMutation routes writes through the
// committer (and therefore the registry projection), retrying on conflict.
type recordingCommitter struct {
	store     *MemoryStore
	conflicts int
	commits   int
}

func (committer *recordingCommitter) CommitState(ctx context.Context, state json.RawMessage, expected int64) (int64, error) {
	if committer.conflicts > 0 {
		committer.conflicts--
		return expected, ErrVersionConflict
	}
	committer.commits++
	return committer.store.Save(ctx, state, expected)
}

func TestCommitMutationUsesTheCommitterAndRetriesConflicts(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.Save(context.Background(), json.RawMessage(`{"devices":[]}`), AnyVersion); err != nil {
		t.Fatal(err)
	}
	committer := &recordingCommitter{store: store, conflicts: 2}
	runs := 0
	err := CommitMutation(context.Background(), store, committer, func(state json.RawMessage) (json.RawMessage, error) {
		runs++
		return ReplaceKeys(state, map[string]any{"devices": []map[string]any{{"id": "new"}}})
	})
	if err != nil {
		t.Fatal(err)
	}
	if committer.commits != 1 || runs != 3 {
		t.Fatalf("commits = %d, runs = %d; want one commit after two conflicts", committer.commits, runs)
	}
	if !strings.Contains(string(mustLoad(t, store)), `"id":"new"`) {
		t.Fatal("committed mutation is missing from the store")
	}
	refusal := errors.New("refused")
	if err := CommitMutation(context.Background(), store, committer, func(json.RawMessage) (json.RawMessage, error) { return nil, refusal }); !errors.Is(err, refusal) {
		t.Fatalf("mutator error = %v", err)
	}
}

func TestIntegrationActionsRequireManagerAndAdministrator(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	cases := []struct {
		role    auth.Role
		action  string
		allowed bool
	}{
		{auth.RoleOperator, ActionManageIntegrations, false},
		{auth.RoleManager, ActionManageIntegrations, true},
		{auth.RoleManager, ActionPairDevices, false},
		{auth.RoleAdministrator, ActionPairDevices, true},
	}
	for _, test := range cases {
		err := RoleAuthorizer{}.Authorize(as(test.role, request), test.action)
		if (err == nil) != test.allowed {
			t.Fatalf("%s %s: err = %v, want allowed = %v", test.role, test.action, err, test.allowed)
		}
	}
}

func TestStateProjectionKeepsTheLatestValueOfSlowChannels(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.Save(context.Background(), json.RawMessage(`{"channels":[]}`), AnyVersion); err != nil {
		t.Fatal(err)
	}
	samples := telemetry.NewMemoryStore(0)
	now := time.Now().UTC()
	batch := []telemetry.Measurement{{ChannelID: "sleepy", ObservedAt: now.Add(-time.Hour), ReceivedAt: now.Add(-time.Hour), Value: 21, Unit: "degC", Quality: "good"}}
	for index := 0; index < projectionWindow+5; index++ {
		at := now.Add(time.Duration(index) * time.Millisecond)
		batch = append(batch, telemetry.Measurement{ChannelID: "chatty", ObservedAt: at, ReceivedAt: at, Value: 1, Unit: "bool", Quality: "good"})
	}
	if _, err := samples.Append(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store, WithTelemetry(samples))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/state", nil))
	if !strings.Contains(response.Body.String(), `"channel_id":"sleepy"`) {
		t.Fatal("the slow channel's latest reading was pushed out of the state projection")
	}
}
