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

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
)

const (
	ackDeviceID  = "01990a20-6a00-7000-8000-000000000601"
	ackChannelID = "01990a20-6a00-7000-8000-000000000602"
)

func commandReadyStore(t *testing.T) *MemoryStore {
	t.Helper()
	store := NewMemoryStore()
	state := `{"devices":[{"id":"` + ackDeviceID + `","online":true}],
		"channels":[{"id":"` + ackChannelID + `","device_id":"` + ackDeviceID + `","kind":"command","value_type":"boolean"}],
		"commands":[]}`
	if _, err := store.Save(context.Background(), json.RawMessage(state), AnyVersion); err != nil {
		t.Fatal(err)
	}
	return store
}

// instantAcknowledger answers inside PublishCommand, the way an in-process
// integration adapter can confirm a device state before the handler records
// that the command was published.
type instantAcknowledger struct{ store Store }

func (publisher instantAcknowledger) PublishCommand(ctx context.Context, deviceID string, command deviceprotocol.Command) error {
	return ApplyCommandAcknowledgement(ctx, publisher.store, deviceID, deviceprotocol.Acknowledgement{
		ProtocolVersion: deviceprotocol.Version, CommandID: command.CommandID, DeviceID: deviceID,
		Result: "applied", AcknowledgedAt: time.Now().UTC(),
	}, time.Now().UTC())
}

func storedCommands(t *testing.T, store Store) []map[string]any {
	t.Helper()
	state, _, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Commands []map[string]any `json:"commands"`
	}
	if err := json.Unmarshal(state, &document); err != nil {
		t.Fatal(err)
	}
	return document.Commands
}

func TestPublishAcceptedDoesNotRegressAnAppliedCommand(t *testing.T) {
	store := commandReadyStore(t)
	handler := NewHandler(store, WithCommandPublisher(instantAcknowledger{store: store}))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/commands",
		strings.NewReader(`{"targetChannelId":"`+ackChannelID+`","value":true,"reason":"test"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	commands := storedCommands(t, store)
	if len(commands) != 1 || commands[0]["status"] != "applied" {
		t.Fatalf("stored commands = %v; the published write overwrote the applied result", commands)
	}
	if !strings.Contains(response.Body.String(), `"status":"applied"`) {
		t.Fatalf("response = %s; want the applied record", response.Body.String())
	}
}

func TestApplyCommandAcknowledgementGuards(t *testing.T) {
	now := time.Now().UTC()
	seed := func(status string, expires time.Time) *MemoryStore {
		store := commandReadyStore(t)
		command, _ := json.Marshal(map[string]any{
			"id": "01990a20-6a00-7000-8000-000000000603", "target_channel_id": ackChannelID,
			"status": status, "expires_at": expires,
		})
		next, _ := ReplaceKeys(mustLoad(t, store), map[string]any{"commands": []json.RawMessage{command}})
		if _, err := store.Save(context.Background(), next, AnyVersion); err != nil {
			t.Fatal(err)
		}
		return store
	}
	ack := func(deviceID, result string) deviceprotocol.Acknowledgement {
		return deviceprotocol.Acknowledgement{
			ProtocolVersion: deviceprotocol.Version, CommandID: "01990a20-6a00-7000-8000-000000000603",
			DeviceID: deviceID, Result: result, AcknowledgedAt: now, AppliedValue: true,
		}
	}
	ctx := context.Background()

	if err := ApplyCommandAcknowledgement(ctx, seed("published", now.Add(time.Minute)), "other-device", ack("other-device", "applied"), now); !errors.Is(err, ErrCommandDeviceMismatch) {
		t.Fatalf("cross-device ack err = %v", err)
	}
	if err := ApplyCommandAcknowledgement(ctx, seed("rejected", now.Add(time.Minute)), ackDeviceID, ack(ackDeviceID, "applied"), now); !errors.Is(err, ErrCommandAlreadyFinal) {
		t.Fatalf("final ack err = %v", err)
	}
	store := seed("published", now.Add(-time.Second))
	if err := ApplyCommandAcknowledgement(ctx, store, ackDeviceID, ack(ackDeviceID, "applied"), now); err != nil {
		t.Fatal(err)
	}
	if status := storedCommands(t, store)[0]["status"]; status != "timed_out" {
		t.Fatalf("late ack status = %v, want timed_out", status)
	}
	store = seed("published", now.Add(time.Minute))
	if err := ApplyCommandAcknowledgement(ctx, store, ackDeviceID, ack(ackDeviceID, "applied"), now); err != nil {
		t.Fatal(err)
	}
	if command := storedCommands(t, store)[0]; command["status"] != "applied" || command["applied_value"] != true {
		t.Fatalf("applied command = %v", command)
	}
}

func mustLoad(t *testing.T, store Store) json.RawMessage {
	t.Helper()
	state, _, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestCommandTopicRoundTrip(t *testing.T) {
	if id, ok := ParseCommandTopic(CommandTopic(ackDeviceID)); !ok || id != ackDeviceID {
		t.Fatalf("round trip = %q, %v", id, ok)
	}
	for _, topic := range []string{"grownerve/v1/devices//commands", "grownerve/v1/devices/a/b/commands", "zigbee2mqtt/plug/set"} {
		if _, ok := ParseCommandTopic(topic); ok {
			t.Fatalf("%q parsed as a command topic", topic)
		}
	}
}
