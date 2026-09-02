package runtime

import (
	"context"
	"testing"
)

func TestSweepCommandsTerminalizesInvalidExpiry(t *testing.T) {
	state := `{"commands":[{"id":"bad-expiry","status":"published"}],"devices":[],"channels":[],"alerts":[]}`
	supervisor, store, _ := testSupervisor(t, state)

	if err := supervisor.SweepCommands(context.Background()); err != nil {
		t.Fatal(err)
	}
	commands := loadDocument(t, store)["commands"].([]any)
	command := commands[0].(map[string]any)
	if command["status"] != "timed_out" || command["reason_code"] != "COMMAND_EXPIRY_INVALID" {
		t.Fatalf("invalid expiry remained active: %+v", command)
	}
}

func TestDeviceWithoutTrustworthyHeartbeatIsMarkedOffline(t *testing.T) {
	state := `{"devices":[
		{"id":"missing","online":true},
		{"id":"invalid","online":true,"last_heartbeat":"not-a-time"},
		{"id":"future","online":true,"last_heartbeat":"2099-01-01T00:00:00Z"}
	],"channels":[],"alerts":[],"commands":[]}`
	supervisor, store, _ := testSupervisor(t, state)

	if err := supervisor.EvaluateAlerts(context.Background()); err != nil {
		t.Fatal(err)
	}
	devices := loadDocument(t, store)["devices"].([]any)
	for _, entry := range devices {
		device := entry.(map[string]any)
		if online, _ := device["online"].(bool); online {
			t.Fatalf("device with untrustworthy heartbeat remained online: %+v", device)
		}
	}
}
