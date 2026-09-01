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

const testDevice = "01990a20-6a00-7000-8000-000000000001"
const testChannel = "01990a20-6a00-7000-8000-000000000002"
const testCommand = "01990a20-6a00-7000-8000-000000000003"

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

func TestBridgeIngestsTelemetryHealthAndAcknowledgements(t *testing.T) {
	state := `{"devices":[{"id":"` + testDevice + `","online":false}],"channels":[{"id":"` + testChannel + `","device_id":"` + testDevice + `","unit":"degC"}],"measurements":[],"commands":[{"id":"` + testCommand + `","status":"published"}]}`
	bridge, store, samples := testBridge(state)
	now := time.Now().UTC()
	telemetryPayload, _ := json.Marshal(deviceprotocol.TelemetryEnvelope{ProtocolVersion: 1, DeviceID: testDevice, BootID: "boot", Sequence: 1, ObservedAt: now, Samples: []deviceprotocol.Sample{{ChannelID: testChannel, Value: 22, Unit: "degC", Quality: deviceprotocol.QualityGood}}})
	bridge.handleTelemetry(nil, message{payload: telemetryPayload})
	health, _ := json.Marshal(deviceprotocol.Health{ProtocolVersion: 1, DeviceID: testDevice, FirmwareVersion: "v1", ActiveConfigVersion: "c1", ObservedAt: now})
	bridge.handleHealth(nil, message{payload: health})
	ack, _ := json.Marshal(deviceprotocol.Acknowledgement{ProtocolVersion: 1, DeviceID: testDevice, CommandID: testCommand, Result: "applied", AcknowledgedAt: now})
	bridge.handleAcknowledgement(nil, message{payload: ack})
	result, _, _ := store.Load(context.Background())
	if !containsAll(string(result), `"firmware_version":"v1"`, `"status":"applied"`, `"online":true`) {
		t.Fatalf("state = %s", result)
	}
	// Telemetry is written to the measurement store, not back into the document.
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
	bridge.handleTelemetry(nil, message{payload: []byte("bad")})
	bridge.handleHealth(nil, message{payload: []byte("bad")})
	bridge.handleAcknowledgement(nil, message{payload: []byte("bad")})
	after, _, _ := store.Load(context.Background())
	if string(before) != string(after) {
		t.Fatalf("invalid input changed state")
	}
	if err := bridge.PublishCommand(context.Background(), testDevice, deviceprotocol.Command{}); err == nil {
		t.Fatal("disconnected publish succeeded")
	}
	if count, _ := samples.Recent(context.Background(), 10); len(count) != 0 {
		t.Fatalf("invalid telemetry was stored: %+v", count)
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

// TestPartiallyResolvableEnvelopeIsRejectedEntirely covers the case where a
// batch mixes known and unknown channels. Storing the resolvable half would let
// a device widen its own reach by attaching one unknown channel to an otherwise
// valid batch, and would record history nothing acknowledged.
func TestPartiallyResolvableEnvelopeIsRejectedEntirely(t *testing.T) {
	state := `{"devices":[{"id":"` + testDevice + `","online":false}],` +
		`"channels":[{"id":"` + testChannel + `","device_id":"` + testDevice + `","unit":"degC"}],` +
		`"measurements":[],"commands":[]}`
	bridge, _, samples := testBridge(state)
	now := time.Now().UTC()

	payload, _ := json.Marshal(deviceprotocol.TelemetryEnvelope{
		ProtocolVersion: 1, DeviceID: testDevice, BootID: "boot", Sequence: 1, ObservedAt: now,
		Samples: []deviceprotocol.Sample{
			{ChannelID: testChannel, Value: 22, Unit: "degC", Quality: deviceprotocol.QualityGood},
			{ChannelID: "01990a20-6a00-7000-8000-0000000000fe", Value: 99, Unit: "degC", Quality: deviceprotocol.QualityGood},
		},
	})
	bridge.handleTelemetry(nil, message{payload: payload})

	stored, err := samples.Recent(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 0 {
		t.Fatalf("a partially resolvable envelope stored %d samples: %+v", len(stored), stored)
	}
}

// TestUnitMismatchRejectsTheWholeEnvelope covers the same rule for a sample whose
// unit disagrees with its channel: comparing unrelated quantities is worse than
// having no reading.
func TestUnitMismatchRejectsTheWholeEnvelope(t *testing.T) {
	state := `{"devices":[{"id":"` + testDevice + `","online":false}],` +
		`"channels":[{"id":"` + testChannel + `","device_id":"` + testDevice + `","unit":"degC"}],` +
		`"measurements":[],"commands":[]}`
	bridge, _, samples := testBridge(state)

	payload, _ := json.Marshal(deviceprotocol.TelemetryEnvelope{
		ProtocolVersion: 1, DeviceID: testDevice, BootID: "boot", Sequence: 1, ObservedAt: time.Now().UTC(),
		Samples: []deviceprotocol.Sample{
			{ChannelID: testChannel, Value: 71, Unit: "degF", Quality: deviceprotocol.QualityGood},
		},
	})
	bridge.handleTelemetry(nil, message{payload: payload})

	if stored, _ := samples.Recent(context.Background(), 10); len(stored) != 0 {
		t.Fatalf("a unit mismatch was stored: %+v", stored)
	}
}
