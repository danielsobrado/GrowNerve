package integration

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/farm"
	"github.com/jdanielsobrado/grownerve/internal/platform/auth"
	"github.com/jdanielsobrado/grownerve/internal/platform/outbox"
	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

const (
	facilityID   = "01990a20-6a00-7000-8000-000000000001"
	zoneID       = "01990a20-6a00-7000-8000-000000000003"
	esp32ID      = "01990a20-6a00-7000-8000-000000000020"
	plugIEEE     = "0x00158d0001a2b3c4"
	sensorIEEE   = "0x00158d0001ffffff"
	esp32Channel = "01990a20-6a00-7000-8000-000000000036"
)

func float(value float64) *float64 { return &value }

// fakeAdapter stands in for a provider. With reflect set, Execute reports the
// requested state back through the sink the way a real device's state update
// arrives from Zigbee2MQTT.
type fakeAdapter struct {
	mu        sync.Mutex
	sink      Sink
	available map[string]bool
	reflect   bool
	fail      error
	outcome   Outcome
	executed  []Command
}

func newFakeAdapter() *fakeAdapter {
	return &fakeAdapter{available: map[string]bool{plugIEEE: true, sensorIEEE: true}, outcome: Outcome{Result: OutcomePublished}}
}

func (adapter *fakeAdapter) Provider() Provider { return Zigbee2MQTT }
func (adapter *fakeAdapter) Start(_ context.Context, sink Sink) {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.sink = sink
}
func (adapter *fakeAdapter) Status() Status {
	return Status{State: StateConnected, Since: time.Now().UTC()}
}
func (adapter *fakeAdapter) Devices() []DiscoveredDevice {
	return []DiscoveredDevice{
		{Provider: Zigbee2MQTT, ExternalID: plugIEEE, Name: "Tent plug", SuggestedType: "fan", PowerSource: "mains", Firmware: "1.0",
			Capabilities: []Capability{
				{Key: "state", Label: "Power", Kind: KindCommand, ValueType: "boolean", Unit: UnitBoolean, Minimum: float(0), Maximum: float(1), SuggestedChannelKey: "tent_plug.state"},
				{Key: "power", Label: "Power draw", Kind: KindMeasurement, ValueType: "number", Unit: "W", SuggestedChannelKey: "tent_plug.power"},
			}},
		{Provider: Zigbee2MQTT, ExternalID: sensorIEEE, Name: "Tent climate", SuggestedType: "sensor", PowerSource: "battery",
			Capabilities: []Capability{
				{Key: "temperature", Label: "Temperature", Kind: KindMeasurement, ValueType: "number", Unit: "degC", Dimension: "temperature", SuggestedChannelKey: "tent_climate.temperature"},
			}},
	}
}
func (adapter *fakeAdapter) Available(externalID string) bool {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	return adapter.available[externalID]
}
func (adapter *fakeAdapter) setAvailable(externalID string, available bool) {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.available[externalID] = available
}
func (adapter *fakeAdapter) Execute(_ context.Context, command Command) (Outcome, error) {
	adapter.mu.Lock()
	adapter.executed = append(adapter.executed, command)
	sink, reflect, fail, outcome := adapter.sink, adapter.reflect, adapter.fail, adapter.outcome
	adapter.mu.Unlock()
	if fail != nil {
		return Outcome{}, fail
	}
	if reflect && command.Boolean != nil {
		value := 0.0
		if *command.Boolean {
			value = 1
		}
		sink.Readings(Zigbee2MQTT, []Reading{{ExternalID: command.ExternalID, Key: command.Key, Value: value, Unit: UnitBoolean}})
	}
	return outcome, nil
}
func (adapter *fakeAdapter) executions() int {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	return len(adapter.executed)
}

// recordingTransport is the MQTT bridge stand-in.
type recordingTransport struct {
	commands []string
	raw      []string
}

func (transport *recordingTransport) PublishCommand(_ context.Context, deviceID string, _ deviceprotocol.Command) error {
	transport.commands = append(transport.commands, deviceID)
	return nil
}
func (transport *recordingTransport) PublishRaw(_ context.Context, topic string, _ []byte) error {
	transport.raw = append(transport.raw, topic)
	return nil
}

type fixture struct {
	store     *farm.MemoryStore
	samples   *telemetry.MemoryStore
	adapter   *fakeAdapter
	manager   *Manager
	transport *recordingTransport
	router    *Router
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	store := farm.NewMemoryStore()
	state := `{"facilities":[{"id":"` + facilityID + `"}],"zones":[{"id":"` + zoneID + `"}],
		"devices":[{"id":"` + esp32ID + `","zone_id":"` + zoneID + `","online":true}],
		"channels":[{"id":"` + esp32Channel + `","device_id":"` + esp32ID + `","key":"fan.speed.command","kind":"command","value_type":"number","unit":"%"}],
		"channel_bindings":[],"commands":[]}`
	if _, err := store.Save(context.Background(), json.RawMessage(state), farm.AnyVersion); err != nil {
		t.Fatal(err)
	}
	samples := telemetry.NewMemoryStore(0)
	adapter := newFakeAdapter()
	manager := NewManager(Dependencies{
		Store: store, Telemetry: samples, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), LivenessInterval: time.Hour,
	}, adapter)
	adapter.Start(context.Background(), manager)
	transport := &recordingTransport{}
	return &fixture{store: store, samples: samples, adapter: adapter, manager: manager, transport: transport, router: NewRouter(transport, manager)}
}

func (fixture *fixture) adoptPlug(t *testing.T) AdoptResult {
	t.Helper()
	result, err := fixture.manager.Adopt(context.Background(), Zigbee2MQTT, plugIEEE, "manager-user", AdoptRequest{
		ZoneID: zoneID, AcknowledgeNoEdgeFailsafe: true,
		Channels: []AdoptChannel{{CapabilityKey: "state"}, {CapabilityKey: "power"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func (fixture *fixture) state(t *testing.T) map[string][]map[string]any {
	t.Helper()
	raw, _, err := fixture.store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var state map[string][]map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func refusalCode(err error) string {
	var refusal *RequestError
	if errors.As(err, &refusal) {
		return refusal.Code
	}
	return ""
}

func TestAdoptCreatesBoundDeviceChannelsAndBindings(t *testing.T) {
	fixture := newFixture(t)
	result := fixture.adoptPlug(t)
	deviceID := result.Device["id"].(string)

	state := fixture.state(t)
	if len(state["devices"]) != 2 || len(state["channels"]) != 3 || len(state["channel_bindings"]) != 2 {
		t.Fatalf("state = %v", state)
	}
	plug := state["devices"][1]
	binding := plug["integration"].(map[string]any)
	if plug["id"] != deviceID || plug["type"] != "fan" || plug["name"] != "Tent plug" || binding["provider"] != "zigbee2mqtt" || binding["external_id"] != plugIEEE {
		t.Fatalf("device = %v", plug)
	}
	power := state["channels"][1]
	if power["kind"] != "command" || power["value_type"] != "boolean" || power["unit"] != "bool" || power["integration_key"] != "state" ||
		power["key"] != "tent_plug.state" || power["safe_minimum"] != 0.0 || power["safe_maximum"] != 1.0 {
		t.Fatalf("command channel = %v", power)
	}

	if _, err := fixture.manager.Adopt(context.Background(), Zigbee2MQTT, plugIEEE, "m", AdoptRequest{
		ZoneID: zoneID, Channels: []AdoptChannel{{CapabilityKey: "power", Key: "other.key"}},
	}); refusalCode(err) != "ALREADY_ADOPTED" {
		t.Fatalf("second adoption err = %v", err)
	}
}

func TestAdoptRefusals(t *testing.T) {
	fixture := newFixture(t)
	cases := map[string]AdoptRequest{
		"UNKNOWN_ZONE":         {ZoneID: "missing", Channels: []AdoptChannel{{CapabilityKey: "temperature"}}},
		"UNKNOWN_CAPABILITY":   {ZoneID: zoneID, Channels: []AdoptChannel{{CapabilityKey: "pressure"}}},
		"NO_CHANNELS":          {ZoneID: zoneID},
		"CHANNEL_KEY_CONFLICT": {ZoneID: zoneID, Channels: []AdoptChannel{{CapabilityKey: "temperature", Key: "fan.speed.command"}}},
		"INVALID_DEVICE_TYPE":  {ZoneID: zoneID, Type: "toaster", Channels: []AdoptChannel{{CapabilityKey: "temperature"}}},
	}
	for code, request := range cases {
		if _, err := fixture.manager.Adopt(context.Background(), Zigbee2MQTT, sensorIEEE, "m", request); refusalCode(err) != code {
			t.Fatalf("%s: err = %v", code, err)
		}
	}
	if _, err := fixture.manager.Adopt(context.Background(), Zigbee2MQTT, plugIEEE, "m", AdoptRequest{
		ZoneID: zoneID, Channels: []AdoptChannel{{CapabilityKey: "state"}},
	}); refusalCode(err) != "EDGE_FAILSAFE_NOT_ACKNOWLEDGED" {
		t.Fatalf("unacknowledged command adoption err = %v", err)
	}
	if _, err := fixture.manager.Adopt(context.Background(), Zigbee2MQTT, plugIEEE, "m", AdoptRequest{
		ZoneID: zoneID, AcknowledgeNoEdgeFailsafe: true, Channels: []AdoptChannel{{CapabilityKey: "state", SafeMaximum: float(5)}},
	}); refusalCode(err) != "INVALID_SAFE_RANGE" {
		t.Fatalf("out-of-range safe limit err = %v", err)
	}
	if len(fixture.state(t)["devices"]) != 1 {
		t.Fatal("a refused adoption changed the farm")
	}
}

func TestReadingsAreRecordedOnlyForAdoptedCapabilities(t *testing.T) {
	fixture := newFixture(t)
	result := fixture.adoptPlug(t)
	channels := result.Channels
	now := time.Now().UTC()
	fixture.manager.Readings(Zigbee2MQTT, []Reading{
		{ExternalID: plugIEEE, Key: "state", Value: 1, Unit: UnitBoolean, ObservedAt: now},
		{ExternalID: plugIEEE, Key: "power", Value: 42.5, Unit: "W", ObservedAt: now},
		{ExternalID: plugIEEE, Key: "power", Value: 9, Unit: "kWh", ObservedAt: now},      // unit mismatch
		{ExternalID: plugIEEE, Key: "linkquality", Value: 90, Unit: "1", ObservedAt: now}, // not adopted
		{ExternalID: sensorIEEE, Key: "temperature", Value: 23, Unit: "degC", ObservedAt: now},
	})
	latest, err := fixture.samples.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]float64{}
	for _, measurement := range latest {
		values[measurement.ChannelID] = measurement.Value
	}
	if len(values) != 2 || values[channels[0]["id"].(string)] != 1 || values[channels[1]["id"].(string)] != 42.5 {
		t.Fatalf("latest = %+v", latest)
	}
	if plug := fixture.state(t)["devices"][1]; plug["state"] != true {
		t.Fatalf("reflected state was not projected onto the device: %v", plug)
	}
}

func TestRouterSendsOnlyIntegrationDevicesToAdapters(t *testing.T) {
	fixture := newFixture(t)
	plug := fixture.adoptPlug(t)
	now := time.Now().UTC()
	on := true
	command := deviceprotocol.Command{
		ProtocolVersion: deviceprotocol.Version, CommandID: "01990a20-6a00-7000-8000-000000000901",
		TargetChannelID: plug.Channels[0]["id"].(string), Type: "set_boolean", Value: json.RawMessage(`true`),
		IssuedAt: now, ExpiresAt: now.Add(30 * time.Second),
	}
	deviceID := plug.Device["id"].(string)
	if err := fixture.router.PublishCommand(context.Background(), deviceID, command); err != nil {
		t.Fatal(err)
	}
	if fixture.adapter.executions() != 1 || *fixture.adapter.executed[0].Boolean != on || fixture.adapter.executed[0].Key != "state" || fixture.adapter.executed[0].ExternalID != plugIEEE {
		t.Fatalf("adapter executions = %+v", fixture.adapter.executed)
	}
	// An outbox redelivery of the same command is not executed twice.
	payload, _ := json.Marshal(command)
	if err := fixture.router.PublishRaw(context.Background(), farm.CommandTopic(deviceID), payload); err != nil || fixture.adapter.executions() != 1 {
		t.Fatalf("redelivery err = %v, executions = %d", err, fixture.adapter.executions())
	}
	if err := fixture.router.PublishCommand(context.Background(), esp32ID, command); err != nil || len(fixture.transport.commands) != 1 {
		t.Fatalf("ESP32 command err = %v, bridge saw %v", err, fixture.transport.commands)
	}
	if err := fixture.router.PublishRaw(context.Background(), "grownerve/v1/devices/"+esp32ID+"/config", nil); err != nil || len(fixture.transport.raw) != 1 {
		t.Fatalf("non-command raw publish err = %v, bridge saw %v", err, fixture.transport.raw)
	}
}

func TestUnavailableIntegrationDeviceIsQueuedByTheOutbox(t *testing.T) {
	fixture := newFixture(t)
	plug := fixture.adoptPlug(t)
	fixture.adapter.setAvailable(plugIEEE, false)
	queue := outbox.NewMemoryStore()
	publisher := farm.NewDurablePublisher(fixture.router, queue, slog.New(slog.NewTextHandler(io.Discard, nil)))
	now := time.Now().UTC()
	command := deviceprotocol.Command{
		ProtocolVersion: deviceprotocol.Version, CommandID: "01990a20-6a00-7000-8000-000000000902",
		TargetChannelID: plug.Channels[0]["id"].(string), Type: "set_boolean", Value: json.RawMessage(`true`),
		IssuedAt: now, ExpiresAt: now.Add(30 * time.Second),
	}
	if err := publisher.PublishCommand(context.Background(), plug.Device["id"].(string), command); err == nil {
		t.Fatal("an unavailable device reported success")
	}
	pending, _ := queue.Pending(context.Background(), 10)
	if len(pending) != 1 || fixture.adapter.executions() != 0 {
		t.Fatalf("pending = %+v, executions = %d", pending, fixture.adapter.executions())
	}
	fixture.adapter.setAvailable(plugIEEE, true)
	if err := farm.NewOutboxWorker(queue, fixture.router, slog.New(slog.NewTextHandler(io.Discard, nil))).Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fixture.adapter.executions() != 1 {
		t.Fatal("the queued command was not delivered once the device came back")
	}
}

func TestCommandIsAppliedOnlyWhenTheDeviceReflectsIt(t *testing.T) {
	for name, reflect := range map[string]bool{"reflected": true, "silent": false} {
		t.Run(name, func(t *testing.T) {
			fixture := newFixture(t)
			plug := fixture.adoptPlug(t)
			fixture.adapter.reflect = reflect
			fixture.manager.livenessPass(context.Background())
			handler := farm.NewHandler(fixture.store, farm.WithCommandPublisher(fixture.router))
			request := httptest.NewRequest(http.MethodPost, "/api/v1/commands",
				strings.NewReader(`{"targetChannelId":"`+plug.Channels[0]["id"].(string)+`","value":true,"reason":"lights on"}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusAccepted {
				t.Fatalf("status = %d: %s", response.Code, response.Body.String())
			}
			command := fixture.state(t)["commands"][0]
			want := "published"
			if reflect {
				want = "applied"
			}
			if command["status"] != want {
				t.Fatalf("command = %v, want status %s", command, want)
			}
		})
	}
}

func TestLivenessFollowsProviderAvailability(t *testing.T) {
	fixture := newFixture(t)
	plug := fixture.adoptPlug(t)
	fixture.manager.livenessPass(context.Background())
	device := fixture.state(t)["devices"][1]
	if device["id"] != plug.Device["id"] || device["online"] != true || device["last_heartbeat"] == "" {
		t.Fatalf("available device = %v", device)
	}
	fixture.adapter.setAvailable(plugIEEE, false)
	fixture.manager.livenessPass(context.Background())
	if device := fixture.state(t)["devices"][1]; device["online"] != false {
		t.Fatalf("unavailable device stayed online: %v", device)
	}
	if esp := fixture.state(t)["devices"][0]; esp["online"] != true {
		t.Fatalf("liveness touched a non-integration device: %v", esp)
	}
}

func TestHTTPAuthorizationAndInventory(t *testing.T) {
	fixture := newFixture(t)
	fixture.adoptPlug(t)
	handler := NewHandler(fixture.manager, farm.RoleAuthorizer{}, nil)
	call := func(role auth.Role, method, path, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request = request.WithContext(auth.WithPrincipal(request.Context(), auth.Principal{Subject: "user", Role: role}))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	if response := call(auth.RoleViewer, http.MethodGet, "/api/v1/integrations", ""); response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), `"provider":"zigbee2mqtt","enabled":true,"state":"connected"`) ||
		!strings.Contains(response.Body.String(), `"provider":"matter","enabled":false,"state":"disabled"`) {
		t.Fatalf("statuses = %d %s", response.Code, response.Body.String())
	}
	if response := call(auth.RoleOperator, http.MethodGet, "/api/v1/integrations/zigbee2mqtt/devices", ""); response.Code != http.StatusForbidden {
		t.Fatalf("operator inventory status = %d", response.Code)
	}
	response := call(auth.RoleManager, http.MethodGet, "/api/v1/integrations/zigbee2mqtt/devices", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"adopted_device_id"`) {
		t.Fatalf("inventory = %d %s", response.Code, response.Body.String())
	}
	if response := call(auth.RoleManager, http.MethodGet, "/api/v1/integrations/matter/devices", ""); response.Code != http.StatusConflict {
		t.Fatalf("disabled provider status = %d", response.Code)
	}
	if response := call(auth.RoleManager, http.MethodGet, "/api/v1/integrations/zwave/devices", ""); response.Code != http.StatusNotFound {
		t.Fatalf("unknown provider status = %d", response.Code)
	}
	if response := call(auth.RoleManager, http.MethodPost, "/api/v1/integrations/zigbee2mqtt/devices/"+sensorIEEE+"/adopt",
		`{"zone_id":"`+zoneID+`","channels":[{"capability_key":"temperature"}]}`); response.Code != http.StatusCreated {
		t.Fatalf("adopt = %d %s", response.Code, response.Body.String())
	}
	if response := call(auth.RoleManager, http.MethodPost, "/api/v1/integrations/zigbee2mqtt/permit-join", `{"seconds":60}`); response.Code != http.StatusForbidden {
		t.Fatalf("manager permit-join status = %d", response.Code)
	}
	if response := call(auth.RoleAdministrator, http.MethodPost, "/api/v1/integrations/zigbee2mqtt/permit-join", `{"seconds":60}`); response.Code != http.StatusNotImplemented {
		t.Fatalf("permit-join on an adapter without support = %d %s", response.Code, response.Body.String())
	}
}
