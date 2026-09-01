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

func testBridge(state string) (*Bridge, *farm.MemoryStore) {
	store := farm.NewMemoryStore()
	_ = store.Save(context.Background(), json.RawMessage(state))
	return NewBridge("tcp://127.0.0.1:65534", "test", store, slog.New(slog.NewTextHandler(io.Discard, nil))), store
}

func TestBridgeIngestsTelemetryHealthAndAcknowledgements(t *testing.T) {
	state := `{"devices":[{"id":"` + testDevice + `","online":false}],"channels":[{"id":"` + testChannel + `","device_id":"` + testDevice + `","unit":"degC"}],"measurements":[],"commands":[{"id":"` + testCommand + `","status":"published"}]}`
	bridge, store := testBridge(state)
	now := time.Now().UTC()
	telemetry, _ := json.Marshal(deviceprotocol.TelemetryEnvelope{ProtocolVersion: 1, DeviceID: testDevice, BootID: "boot", Sequence: 1, ObservedAt: now, Samples: []deviceprotocol.Sample{{ChannelID: testChannel, Value: 22, Unit: "degC", Quality: deviceprotocol.QualityGood}}})
	bridge.handleTelemetry(nil, message{payload: telemetry})
	health, _ := json.Marshal(deviceprotocol.Health{ProtocolVersion: 1, DeviceID: testDevice, FirmwareVersion: "v1", ActiveConfigVersion: "c1", ObservedAt: now})
	bridge.handleHealth(nil, message{payload: health})
	ack, _ := json.Marshal(deviceprotocol.Acknowledgement{ProtocolVersion: 1, DeviceID: testDevice, CommandID: testCommand, Result: "applied", AcknowledgedAt: now})
	bridge.handleAcknowledgement(nil, message{payload: ack})
	result, _ := store.Load(context.Background())
	if !containsAll(string(result), `"value":22`, `"firmware_version":"v1"`, `"status":"applied"`) {
		t.Fatalf("state = %s", result)
	}
}

func TestBridgeRejectsInvalidMessagesAndDisconnectedPublish(t *testing.T) {
	bridge, store := testBridge(`{"devices":[],"channels":[],"measurements":[],"commands":[]}`)
	before, _ := store.Load(context.Background())
	bridge.handleTelemetry(nil, message{payload: []byte("bad")})
	bridge.handleHealth(nil, message{payload: []byte("bad")})
	bridge.handleAcknowledgement(nil, message{payload: []byte("bad")})
	after, _ := store.Load(context.Background())
	if string(before) != string(after) {
		t.Fatalf("invalid input changed state")
	}
	if err := bridge.PublishCommand(context.Background(), testDevice, deviceprotocol.Command{}); err == nil {
		t.Fatal("disconnected publish succeeded")
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
