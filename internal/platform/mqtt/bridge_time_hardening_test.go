package mqtt

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

func TestLateAcknowledgementCannotApplyExpiredCommand(t *testing.T) {
	now := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
	state := `{"devices":[{"id":"` + testDevice + `","online":true}],` +
		`"channels":[{"id":"` + testChannel + `","device_id":"` + testDevice + `","unit":"pct"}],` +
		`"commands":[{"id":"` + testCommand + `","target_channel_id":"` + testChannel + `","status":"published","expires_at":"` + now.Add(-time.Second).Format(time.RFC3339Nano) + `"}]}`
	bridge, store, _ := testBridge(state)
	bridge.now = func() time.Time { return now }
	deviceAckAt := now.Add(-2 * time.Second)
	payload, _ := json.Marshal(deviceprotocol.Acknowledgement{
		ProtocolVersion: deviceprotocol.Version,
		DeviceID:        testDevice,
		CommandID:       testCommand,
		Result:          "applied",
		AcknowledgedAt:  deviceAckAt,
	})

	bridge.handleAcknowledgement(nil, message{topic: deviceTopic(testDevice, "acks"), payload: payload})

	stateAfter, _, _ := store.Load(context.Background())
	if !containsAll(string(stateAfter), `"status":"timed_out"`, `"reason_code":"COMMAND_EXPIRED"`) {
		t.Fatalf("late acknowledgement resurrected an expired command: %s", stateAfter)
	}
	if !strings.Contains(string(stateAfter), `"updated_at":"`+now.Format(time.RFC3339Nano)+`"`) {
		t.Fatalf("server receipt time was not authoritative: %s", stateAfter)
	}
}

func TestHealthUsesServerReceiptTimeForLiveness(t *testing.T) {
	now := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
	state := `{"devices":[{"id":"` + testDevice + `","online":false}],"channels":[],"commands":[]}`
	bridge, store, _ := testBridge(state)
	bridge.now = func() time.Time { return now }
	observedAt := now.Add(-30 * time.Second)
	payload, _ := json.Marshal(deviceprotocol.Health{
		ProtocolVersion: deviceprotocol.Version,
		DeviceID:        testDevice,
		FirmwareVersion: "v1",
		ObservedAt:      observedAt,
	})

	bridge.handleHealth(nil, message{topic: deviceTopic(testDevice, "health"), payload: payload})

	stateAfter, _, _ := store.Load(context.Background())
	if !strings.Contains(string(stateAfter), `"last_heartbeat":"`+now.Format(time.RFC3339Nano)+`"`) {
		t.Fatalf("device clock controlled liveness: %s", stateAfter)
	}
	if !strings.Contains(string(stateAfter), `"last_device_observed_at":"`+observedAt.Format(time.RFC3339Nano)+`"`) {
		t.Fatalf("device timestamp was not retained as metadata: %s", stateAfter)
	}
}

func TestFutureHealthAndTelemetryAreRejected(t *testing.T) {
	now := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
	state := `{"devices":[{"id":"` + testDevice + `","online":false}],` +
		`"channels":[{"id":"` + testChannel + `","device_id":"` + testDevice + `","unit":"degC"}],"commands":[]}`
	bridge, store, samples := testBridge(state)
	bridge.now = func() time.Time { return now }
	before, _, _ := store.Load(context.Background())
	future := now.Add(deviceprotocol.MaximumFutureClockSkew + time.Second)

	health, _ := json.Marshal(deviceprotocol.Health{
		ProtocolVersion: deviceprotocol.Version,
		DeviceID:        testDevice,
		ObservedAt:      future,
	})
	bridge.handleHealth(nil, message{topic: deviceTopic(testDevice, "health"), payload: health})

	measurement, _ := json.Marshal(deviceprotocol.TelemetryEnvelope{
		ProtocolVersion: deviceprotocol.Version,
		DeviceID:        testDevice,
		BootID:          "boot",
		Sequence:        1,
		ObservedAt:      future,
		Samples: []deviceprotocol.Sample{{
			ChannelID: testChannel,
			Value:     22,
			Unit:      "degC",
			Quality:   deviceprotocol.QualityGood,
		}},
	})
	bridge.handleTelemetry(nil, message{topic: deviceTopic(testDevice, "telemetry"), payload: measurement})

	after, _, _ := store.Load(context.Background())
	if string(after) != string(before) {
		t.Fatalf("future device timestamp changed state: before=%s after=%s", before, after)
	}
	stored, _ := samples.Recent(context.Background(), 10)
	if len(stored) != 0 {
		t.Fatalf("future telemetry was stored: %+v", stored)
	}
}

func TestStaleConfigAcknowledgementCannotReplaceDesiredVersion(t *testing.T) {
	now := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
	state := `{"devices":[{"id":"` + testDevice + `","online":true,"active_config_version":"v1","desired_config_version":"v3"}],"channels":[],"commands":[]}`
	bridge, store, _ := testBridge(state)
	bridge.now = func() time.Time { return now }
	payload, _ := json.Marshal(deviceprotocol.ConfigAcknowledgement{
		ProtocolVersion: deviceprotocol.Version,
		DeviceID:        testDevice,
		ConfigVersion:   "v2",
		Accepted:        true,
		AcknowledgedAt:  now,
	})

	bridge.handleConfigAcknowledgement(nil, message{topic: deviceTopic(testDevice, "config/ack"), payload: payload})

	stateAfter, _, _ := store.Load(context.Background())
	if !strings.Contains(string(stateAfter), `"active_config_version":"v1"`) {
		t.Fatalf("stale config acknowledgement replaced active version: %s", stateAfter)
	}
}

func TestTelemetryReceiptTimeIsStoredSeparatelyFromObservedTime(t *testing.T) {
	now := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
	observedAt := now.Add(-10 * time.Second)
	state := `{"devices":[{"id":"` + testDevice + `","online":false}],` +
		`"channels":[{"id":"` + testChannel + `","device_id":"` + testDevice + `","unit":"degC"}],"commands":[]}`
	bridge, _, samples := testBridge(state)
	bridge.now = func() time.Time { return now }
	payload, _ := json.Marshal(deviceprotocol.TelemetryEnvelope{
		ProtocolVersion: deviceprotocol.Version,
		DeviceID:        testDevice,
		BootID:          "boot",
		Sequence:        1,
		ObservedAt:      observedAt,
		Samples: []deviceprotocol.Sample{{
			ChannelID: testChannel,
			Value:     22,
			Unit:      "degC",
			Quality:   deviceprotocol.QualityGood,
		}},
	})

	bridge.handleTelemetry(nil, message{topic: deviceTopic(testDevice, "telemetry"), payload: payload})

	stored, err := samples.History(context.Background(), telemetry.Query{ChannelID: testChannel, From: observedAt.Add(-time.Second), To: now.Add(time.Second)})
	if err != nil || len(stored) != 1 {
		t.Fatalf("stored telemetry = %+v, err=%v", stored, err)
	}
	if !stored[0].ObservedAt.Equal(observedAt) || !stored[0].ReceivedAt.Equal(now) {
		t.Fatalf("timestamps = observed %s received %s", stored[0].ObservedAt, stored[0].ReceivedAt)
	}
}
