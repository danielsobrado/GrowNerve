package mqtt

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/farm"
	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

const (
	testDevice     = "01990a20-6a00-7000-8000-000000000001"
	testChannel    = "01990a20-6a00-7000-8000-000000000002"
	testCommand    = "01990a20-6a00-7000-8000-000000000003"
	foreignDevice  = "01990a20-6a00-7000-8000-000000000011"
	foreignChannel = "01990a20-6a00-7000-8000-000000000012"
)

type message struct {
	topic   string
	payload []byte
}

func (message) Duplicate() bool   { return false }
func (message) Qos() byte         { return 1 }
func (message) Retained() bool    { return false }
func (m message) Topic() string   { return m.topic }
func (message) MessageID() uint16 { return 1 }
func (m message) Payload() []byte { return m.payload }
func (message) Ack()              {}

func testBridge(state string) (*Bridge, *farm.MemoryStore, *telemetry.MemoryStore) {
	store := farm.NewMemoryStore()
	_, _ = store.Save(context.Background(), json.RawMessage(state), farm.AnyVersion)
	samples := telemetry.NewMemoryStore(0)
	bridge := NewBridge("tcp://127.0.0.1:65534", "test", store, slog.New(slog.NewTextHandler(io.Discard, nil)), WithTelemetryStore(samples))
	return bridge, store, samples
}

func deviceTopic(deviceID, suffix string) string {
	return "grownerve/v1/devices/" + deviceID + "/" + suffix
}

func TestBridgeIngestsTelemetryHealthAndAcknowledgements(t *testing.T) {
	now := time.Now().UTC()
	state := `{"devices":[{"id":"` + testDevice + `","online":false}],"channels":[{"id":"` + testChannel + `","device_id":"` + testDevice + `","unit":"degC"}],"measurements":[],"commands":[{"id":"` + testCommand + `","target_channel_id":"` + testChannel + `","status":"published","expires_at":"` + now.Add(time.Minute).Format(time.RFC3339Nano) + `"}]}`
	bridge, store, samples := testBridge(state)

	telemetryPayload, _ := json.Marshal(deviceprotocol.TelemetryEnvelope{
		ProtocolVersion: 1,
		DeviceID:        testDevice,
		BootID:          "boot",
		Sequence:        1,
		ObservedAt:      now,
		Samples: []deviceprotocol.Sample{{
			ChannelID: testChannel,
			Value:     22,
			Unit:      "degC",
			Quality:   deviceprotocol.QualityGood,
		}},
	})
	bridge.handleTelemetry(nil, message{topic: deviceTopic(testDevice, "telemetry"), payload: telemetryPayload})

	health, _ := json.Marshal(deviceprotocol.Health{
		ProtocolVersion:     1,
		DeviceID:            testDevice,
		FirmwareVersion:     "v1",
		ActiveConfigVersion: "c1",
		ObservedAt:          now,
	})
	bridge.handleHealth(nil, message{topic: deviceTopic(testDevice, "health"), payload: health})

	ack, _ := json.Marshal(deviceprotocol.Acknowledgement{
		ProtocolVersion: 1,
		DeviceID:        testDevice,
		CommandID:       testCommand,
		Result:          "applied",
		AcknowledgedAt:  now,
	})
	bridge.handleAcknowledgement(nil, message{topic: deviceTopic(testDevice, "acks"), payload: ack})

	result, _, _ := store.Load(context.Background())
	if !containsAll(string(result), `"firmware_version":"v1"`, `"status":"applied"`, `"online":true`) {
		t.Fatalf("state = %s", result)
	}
	if containsAll(string(result), `"value":22`) {
		t.Fatalf("telemetry was written into the farm document: %s", result)
	}
	stored, err := samples.History(context.Background(), telemetry.Query{ChannelID: testChannel, From: now.Add(-time.Hour), To: now.Add(time.Hour)})
	if err != nil || len(stored) != 1 || stored[0].Value != 22 || stored[0].Unit != "degC" {
		t.Fatalf("history = %+v, err = %v", stored, err)
	}
}

func TestBridgeRejectsInvalidMessagesAndDisconnectedPublish(t *testing.T) {
	bridge, store, samples := testBridge(`{"devices":[],"channels":[],"measurements":[],"commands":[]}`)
	before, _, _ := store.Load(context.Background())
	bridge.handleTelemetry(nil, message{topic: deviceTopic(testDevice, "telemetry"), payload: []byte("bad")})
	bridge.handleHealth(nil, message{topic: deviceTopic(testDevice, "health"), payload: []byte("bad")})
	bridge.handleAcknowledgement(nil, message{topic: deviceTopic(testDevice, "acks"), payload: []byte("bad")})
	after, _, _ := store.Load(context.Background())
	if string(before) != string(after) {
		t.Fatal("invalid input changed state")
	}
	if err := bridge.PublishCommand(context.Background(), testDevice, deviceprotocol.Command{}); err == nil {
		t.Fatal("disconnected publish succeeded")
	}
	if count, _ := samples.Recent(context.Background(), 10); len(count) != 0 {
		t.Fatalf("invalid telemetry was stored: %+v", count)
	}
}

func TestTopicIdentityMismatchCannotSpoofTelemetryOrHealth(t *testing.T) {
	state := `{"devices":[{"id":"` + testDevice + `","online":false},{"id":"` + foreignDevice + `","online":false}],` +
		`"channels":[{"id":"` + testChannel + `","device_id":"` + testDevice + `","unit":"degC"},{"id":"` + foreignChannel + `","device_id":"` + foreignDevice + `","unit":"degC"}],` +
		`"measurements":[],"commands":[]}`
	bridge, store, samples := testBridge(state)
	now := time.Now().UTC()
	before, _, _ := store.Load(context.Background())

	payload, _ := json.Marshal(deviceprotocol.TelemetryEnvelope{
		ProtocolVersion: 1,
		DeviceID:        foreignDevice,
		BootID:          "foreign",
		Sequence:        1,
		ObservedAt:      now,
		Samples: []deviceprotocol.Sample{{
			ChannelID: foreignChannel,
			Value:     99,
			Unit:      "degC",
			Quality:   deviceprotocol.QualityGood,
		}},
	})
	bridge.handleTelemetry(nil, message{topic: deviceTopic(testDevice, "telemetry"), payload: payload})

	health, _ := json.Marshal(deviceprotocol.Health{
		ProtocolVersion: 1,
		DeviceID:        foreignDevice,
		FirmwareVersion: "forged",
		ObservedAt:      now,
	})
	bridge.handleHealth(nil, message{topic: deviceTopic(testDevice, "health"), payload: health})

	after, _, _ := store.Load(context.Background())
	if string(before) != string(after) {
		t.Fatalf("spoofed topic identity changed state: before=%s after=%s", before, after)
	}
	if stored, _ := samples.Recent(context.Background(), 10); len(stored) != 0 {
		t.Fatalf("spoofed telemetry was stored: %+v", stored)
	}
}

func TestAcknowledgementCannotCrossDeviceBoundary(t *testing.T) {
	state := `{"devices":[{"id":"` + testDevice + `","online":true},{"id":"` + foreignDevice + `","online":true}],` +
		`"channels":[{"id":"` + testChannel + `","device_id":"` + testDevice + `","unit":"pct"},{"id":"` + foreignChannel + `","device_id":"` + foreignDevice + `","unit":"pct"}],` +
		`"measurements":[],"commands":[{"id":"` + testCommand + `","target_channel_id":"` + foreignChannel + `","status":"published"}]}`
	bridge, store, _ := testBridge(state)
	now := time.Now().UTC()
	ack, _ := json.Marshal(deviceprotocol.Acknowledgement{
		ProtocolVersion: 1,
		DeviceID:        testDevice,
		CommandID:       testCommand,
		Result:          "applied",
		AcknowledgedAt:  now,
	})
	bridge.handleAcknowledgement(nil, message{topic: deviceTopic(testDevice, "acks"), payload: ack})

	result, _, _ := store.Load(context.Background())
	if !strings.Contains(string(result), `"status":"published"`) {
		t.Fatalf("cross-device acknowledgement changed command state: %s", result)
	}
}

func TestConfigAcknowledgementCannotSpoofAnotherDevice(t *testing.T) {
	state := `{"devices":[{"id":"` + testDevice + `","online":true},{"id":"` + foreignDevice + `","online":true}],"channels":[],"measurements":[],"commands":[]}`
	bridge, store, _ := testBridge(state)
	before, _, _ := store.Load(context.Background())
	ack, _ := json.Marshal(deviceprotocol.ConfigAcknowledgement{
		ProtocolVersion: 1,
		DeviceID:        foreignDevice,
		ConfigVersion:   "forged-v2",
		Accepted:        true,
		AcknowledgedAt:  time.Now().UTC(),
	})
	bridge.handleConfigAcknowledgement(nil, message{topic: deviceTopic(testDevice, "config/ack"), payload: ack})
	after, _, _ := store.Load(context.Background())
	if string(before) != string(after) {
		t.Fatalf("spoofed config acknowledgement changed state: before=%s after=%s", before, after)
	}
}

func TestPartiallyResolvableEnvelopeIsRejectedEntirely(t *testing.T) {
	state := `{"devices":[{"id":"` + testDevice + `","online":false}],` +
		`"channels":[{"id":"` + testChannel + `","device_id":"` + testDevice + `","unit":"degC"}],` +
		`"measurements":[],"commands":[]}`
	bridge, _, samples := testBridge(state)
	now := time.Now().UTC()

	payload, _ := json.Marshal(deviceprotocol.TelemetryEnvelope{
		ProtocolVersion: 1,
		DeviceID:        testDevice,
		BootID:          "boot",
		Sequence:        1,
		ObservedAt:      now,
		Samples: []deviceprotocol.Sample{
			{ChannelID: testChannel, Value: 22, Unit: "degC", Quality: deviceprotocol.QualityGood},
			{ChannelID: "01990a20-6a00-7000-8000-0000000000fe", Value: 99, Unit: "degC", Quality: deviceprotocol.QualityGood},
		},
	})
	bridge.handleTelemetry(nil, message{topic: deviceTopic(testDevice, "telemetry"), payload: payload})

	stored, err := samples.Recent(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 0 {
		t.Fatalf("a partially resolvable envelope stored %d samples: %+v", len(stored), stored)
	}
}

func TestUnitMismatchRejectsTheWholeEnvelope(t *testing.T) {
	state := `{"devices":[{"id":"` + testDevice + `","online":false}],` +
		`"channels":[{"id":"` + testChannel + `","device_id":"` + testDevice + `","unit":"degC"}],` +
		`"measurements":[],"commands":[]}`
	bridge, _, samples := testBridge(state)

	payload, _ := json.Marshal(deviceprotocol.TelemetryEnvelope{
		ProtocolVersion: 1,
		DeviceID:        testDevice,
		BootID:          "boot",
		Sequence:        1,
		ObservedAt:      time.Now().UTC(),
		Samples: []deviceprotocol.Sample{{
			ChannelID: testChannel,
			Value:     71,
			Unit:      "degF",
			Quality:   deviceprotocol.QualityGood,
		}},
	})
	bridge.handleTelemetry(nil, message{topic: deviceTopic(testDevice, "telemetry"), payload: payload})

	if stored, _ := samples.Recent(context.Background(), 10); len(stored) != 0 {
		t.Fatalf("a unit mismatch was stored: %+v", stored)
	}
}

func TestTopicDeviceIDRequiresExactDeviceScopedTopic(t *testing.T) {
	valid := map[string]string{
		deviceTopic(testDevice, "telemetry"):  "telemetry",
		deviceTopic(testDevice, "acks"):       "acks",
		deviceTopic(testDevice, "health"):     "health",
		deviceTopic(testDevice, "config/ack"): "config/ack",
	}
	for topic, suffix := range valid {
		deviceID, ok := topicDeviceID(topic, suffix)
		if !ok || deviceID != testDevice {
			t.Fatalf("topicDeviceID(%q, %q) = %q, %v", topic, suffix, deviceID, ok)
		}
	}

	for _, topic := range []string{
		"grownerve/v1/devices//telemetry",
		"grownerve/v1/devices/" + testDevice + "/nested/telemetry",
		"grownerve/v1/device/" + testDevice + "/telemetry",
	} {
		if deviceID, ok := topicDeviceID(topic, "telemetry"); ok {
			t.Fatalf("invalid topic %q resolved to %q", topic, deviceID)
		}
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
