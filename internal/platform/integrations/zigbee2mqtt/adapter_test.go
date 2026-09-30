package zigbee2mqtt

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/integration"
)

const (
	sensorIEEE = "0x00158d0001a2b3c4"
	plugIEEE   = "0x0017880104e45517"
	lightIEEE  = "0x000b57fffec6a5b2"
)

// bridgeDevices mirrors a Zigbee2MQTT 2.x bridge/devices payload: a climate
// sensor, a smart plug, a dimmable bulb (whose friendly name contains a
// slash), the coordinator and an unsupported device.
const bridgeDevices = `[
 {"ieee_address":"0x00124b0000000000","type":"Coordinator","friendly_name":"Coordinator"},
 {"ieee_address":"` + sensorIEEE + `","type":"EndDevice","friendly_name":"Tent climate","power_source":"Battery",
  "definition":{"model":"WSDCGQ11LM","vendor":"Aqara","exposes":[
   {"type":"numeric","name":"temperature","property":"temperature","access":1,"unit":"°C"},
   {"type":"numeric","name":"humidity","property":"humidity","access":1,"unit":"%"},
   {"type":"numeric","name":"pressure","property":"pressure","access":1,"unit":"hPa"},
   {"type":"numeric","name":"battery","property":"battery","access":1,"unit":"%","category":"diagnostic"},
   {"type":"numeric","name":"linkquality","property":"linkquality","access":1,"unit":"lqi"}]}},
 {"ieee_address":"` + plugIEEE + `","type":"Router","friendly_name":"Fan plug","power_source":"Mains (single phase)","software_build_id":"1.0.6",
  "definition":{"model":"ZNCZ02LM","vendor":"Xiaomi","exposes":[
   {"type":"switch","features":[{"type":"binary","name":"state","property":"state","access":7,"value_on":"ON","value_off":"OFF"}]},
   {"type":"numeric","name":"power","property":"power","access":1,"unit":"W"},
   {"type":"numeric","name":"energy","property":"energy","access":1,"unit":"kWh"},
   {"type":"binary","name":"child_lock","property":"child_lock","access":7,"value_on":"LOCK","value_off":"UNLOCK","category":"config"},
   {"type":"enum","name":"power_outage_memory","property":"power_outage_memory","access":7,"values":["on","off"]}]}},
 {"ieee_address":"` + lightIEEE + `","type":"Router","friendly_name":"Room/Grow light","power_source":"Mains (single phase)",
  "definition":{"model":"LED1545G12","vendor":"IKEA","exposes":[
   {"type":"light","features":[
    {"type":"binary","name":"state","property":"state","access":7,"value_on":"ON","value_off":"OFF"},
    {"type":"numeric","name":"brightness","property":"brightness","access":7,"value_min":0,"value_max":254},
    {"type":"composite","name":"color_xy","property":"color","access":7,"features":[]}]}]}},
 {"ieee_address":"0xdeadbeef00000000","type":"EndDevice","friendly_name":"Mystery","definition":null}
]`

type fakeTransport struct {
	mu        sync.Mutex
	connected bool
	published map[string]string
}

func (transport *fakeTransport) Connected() bool {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	return transport.connected
}

func (transport *fakeTransport) Publish(_ context.Context, topic string, payload []byte) error {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.published[topic] = string(payload)
	return nil
}

type recordingSink struct {
	mu       sync.Mutex
	readings []integration.Reading
	status   int
}

func (sink *recordingSink) Readings(_ integration.Provider, readings []integration.Reading) {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	sink.readings = append(sink.readings, readings...)
}
func (sink *recordingSink) AvailabilityChanged(integration.Provider) {}
func (sink *recordingSink) DevicesChanged(integration.Provider)      {}
func (sink *recordingSink) StatusChanged(integration.Provider) {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	sink.status++
}

func (sink *recordingSink) values() map[string]float64 {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	values := map[string]float64{}
	for _, reading := range sink.readings {
		values[reading.ExternalID+"/"+reading.Key+" "+reading.Unit] = reading.Value
	}
	return values
}

type clock struct{ now time.Time }

func (clock *clock) Now() time.Time { return clock.now }

func connectedAdapter(t *testing.T) (*Adapter, *fakeTransport, *recordingSink, *clock) {
	t.Helper()
	moment := &clock{now: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)}
	adapter := New(Config{BaseTopic: "zigbee2mqtt", AvailabilityFallback: time.Hour, Now: moment.Now})
	transport := &fakeTransport{connected: true, published: map[string]string{}}
	adapter.transport = transport
	sink := &recordingSink{}
	adapter.Start(context.Background(), sink)
	adapter.setConnected(true, "")
	adapter.handle("zigbee2mqtt/bridge/state", []byte(`{"state":"online"}`))
	return adapter, transport, sink, moment
}

func capabilityKeys(device integration.DiscoveredDevice) map[string]integration.Capability {
	keys := map[string]integration.Capability{}
	for _, capability := range device.Capabilities {
		keys[capability.Key] = capability
	}
	return keys
}

func TestDiscoveryTurnsExposesIntoCapabilities(t *testing.T) {
	adapter, _, _, _ := connectedAdapter(t)
	adapter.handle("zigbee2mqtt/bridge/devices", []byte(bridgeDevices))
	devices := map[string]integration.DiscoveredDevice{}
	for _, device := range adapter.Devices() {
		devices[device.ExternalID] = device
	}
	if len(devices) != 3 {
		t.Fatalf("devices = %v; want the sensor, plug and light only", devices)
	}

	sensor := devices[sensorIEEE]
	sensorCaps := capabilityKeys(sensor)
	if sensor.SuggestedType != "sensor" || sensor.PowerSource != "battery" || sensor.Manufacturer != "Aqara" {
		t.Fatalf("sensor = %+v", sensor)
	}
	if temperature := sensorCaps["temperature"]; temperature.Unit != "degC" || temperature.Kind != integration.KindMeasurement || temperature.SuggestedChannelKey != "tent_climate.temperature" {
		t.Fatalf("temperature = %+v", temperature)
	}
	if humidity := sensorCaps["humidity"]; humidity.Unit != "%RH" {
		t.Fatalf("humidity = %+v", humidity)
	}
	if !sensorCaps["battery"].Diagnostic || !sensorCaps["linkquality"].Diagnostic || sensorCaps["temperature"].Diagnostic {
		t.Fatal("diagnostic flags are wrong")
	}

	plugCaps := capabilityKeys(devices[plugIEEE])
	if state := plugCaps["state"]; state.Kind != integration.KindCommand || state.ValueType != "boolean" || state.Unit != integration.UnitBoolean {
		t.Fatalf("plug state = %+v", state)
	}
	if _, found := plugCaps["child_lock"]; found {
		t.Fatal("a configuration switch was offered as a capability")
	}
	if _, found := plugCaps["power_outage_memory"]; found {
		t.Fatal("an enum was offered as a capability")
	}
	if devices[plugIEEE].SuggestedType != "controller" || devices[plugIEEE].Firmware != "1.0.6" {
		t.Fatalf("plug = %+v", devices[plugIEEE])
	}

	light := devices[lightIEEE]
	if brightness := capabilityKeys(light)["brightness"]; light.SuggestedType != "light" || brightness.Kind != integration.KindCommand || brightness.Unit != "%" || *brightness.Maximum != 100 {
		t.Fatalf("light = %+v", light)
	}
}

func TestStateMessagesBecomeCanonicalReadings(t *testing.T) {
	adapter, _, sink, _ := connectedAdapter(t)
	// Retained state can arrive before bridge/devices; it must not be lost.
	adapter.handle("zigbee2mqtt/Tent climate", []byte(`{"temperature":23.4,"humidity":61,"battery":87,"linkquality":120}`))
	adapter.handle("zigbee2mqtt/bridge/devices", []byte(bridgeDevices))
	adapter.handle("zigbee2mqtt/Fan plug", []byte(`{"state":"ON","power":41.2,"child_lock":"LOCK"}`))
	adapter.handle("zigbee2mqtt/Room/Grow light", []byte(`{"state":"OFF","brightness":127}`))
	adapter.handle("zigbee2mqtt/Fan plug/set", []byte(`{"state":"OFF"}`)) // our own echo

	values := sink.values()
	expected := map[string]float64{
		sensorIEEE + "/temperature degC": 23.4, sensorIEEE + "/humidity %RH": 61, sensorIEEE + "/battery %": 87,
		plugIEEE + "/state bool": 1, plugIEEE + "/power W": 41.2,
		lightIEEE + "/state bool": 0, lightIEEE + "/brightness %": 50,
	}
	for key, value := range expected {
		if got, found := values[key]; !found || got != value {
			t.Fatalf("%s = %v (found %v); all readings: %v", key, got, found, values)
		}
	}
	if _, found := values[plugIEEE+"/child_lock bool"]; found {
		t.Fatal("an unexposed property produced a reading")
	}
}

func TestAvailabilityPrefersReportsThenFallsBackToLastSeen(t *testing.T) {
	adapter, transport, _, moment := connectedAdapter(t)
	// Retained availability may arrive before the device list describes it.
	adapter.handle("zigbee2mqtt/Room/Grow light/availability", []byte(`{"state":"online"}`))
	adapter.handle("zigbee2mqtt/bridge/devices", []byte(bridgeDevices))
	if !adapter.Available(lightIEEE) {
		t.Fatal("availability that arrived before bridge/devices was lost")
	}
	if adapter.Available(plugIEEE) {
		t.Fatal("a device never heard from is available")
	}
	adapter.handle("zigbee2mqtt/Fan plug/availability", []byte(`{"state":"online"}`))
	if !adapter.Available(plugIEEE) {
		t.Fatal("2.x availability report ignored")
	}
	adapter.handle("zigbee2mqtt/Fan plug/availability", []byte(`offline`))
	if adapter.Available(plugIEEE) {
		t.Fatal("1.x availability report ignored")
	}

	adapter.handle("zigbee2mqtt/Tent climate", []byte(`{"temperature":20}`))
	if !adapter.Available(sensorIEEE) {
		t.Fatal("a device heard from recently is unavailable")
	}
	moment.now = moment.now.Add(2 * time.Hour)
	if adapter.Available(sensorIEEE) {
		t.Fatal("a device silent for longer than the fallback is still available")
	}

	adapter.handle("zigbee2mqtt/Fan plug/availability", []byte(`online`))
	adapter.handle("zigbee2mqtt/bridge/state", []byte(`offline`))
	if adapter.Available(plugIEEE) || adapter.Status().State != integration.StateDegraded {
		t.Fatal("devices stayed available behind an offline bridge")
	}
	adapter.handle("zigbee2mqtt/bridge/state", []byte(`{"state":"online"}`))
	transport.connected = false
	adapter.setConnected(false, "lost")
	if adapter.Available(plugIEEE) || adapter.Status().State != integration.StateConnecting {
		t.Fatal("devices stayed available after the broker connection dropped")
	}
}

func TestExecutePublishesAbsoluteTargets(t *testing.T) {
	adapter, transport, _, _ := connectedAdapter(t)
	adapter.handle("zigbee2mqtt/bridge/devices", []byte(bridgeDevices))
	on, half := true, 50.0
	outcome, err := adapter.Execute(context.Background(), integration.Command{ExternalID: plugIEEE, Key: "state", Boolean: &on})
	if err != nil || outcome.Result != integration.OutcomePublished {
		t.Fatalf("outcome = %+v, err = %v", outcome, err)
	}
	if outcome, err := adapter.Execute(context.Background(), integration.Command{ExternalID: lightIEEE, Key: "brightness", Percent: &half}); err != nil || outcome.Result != integration.OutcomePublished {
		t.Fatalf("brightness outcome = %+v, err = %v", outcome, err)
	}
	if transport.published["zigbee2mqtt/Fan plug/set"] != `{"state":"ON"}` || transport.published["zigbee2mqtt/Room/Grow light/set"] != `{"brightness":127}` {
		t.Fatalf("published = %v", transport.published)
	}
	if outcome, _ := adapter.Execute(context.Background(), integration.Command{ExternalID: plugIEEE, Key: "power", Percent: &half}); outcome.Result != integration.OutcomeRejected {
		t.Fatalf("a read-only capability was commanded: %+v", outcome)
	}
	transport.connected = false
	if _, err := adapter.Execute(context.Background(), integration.Command{ExternalID: plugIEEE, Key: "state", Boolean: &on}); err == nil {
		t.Fatal("a disconnected adapter reported a published command")
	}
}

func TestPermitJoinUsesTheBridgeRequestTopic(t *testing.T) {
	adapter, transport, _, moment := connectedAdapter(t)
	until, err := adapter.PermitJoin(context.Background(), 120)
	if err != nil || !until.Equal(moment.now.Add(120*time.Second)) {
		t.Fatalf("until = %v, err = %v", until, err)
	}
	var body map[string]int
	if json.Unmarshal([]byte(transport.published["zigbee2mqtt/bridge/request/permit_join"]), &body) != nil || body["time"] != 120 {
		t.Fatalf("permit join request = %v", transport.published)
	}
	if adapter.Status().PermitJoinUntil == nil {
		t.Fatal("status does not show the open join window")
	}
	adapter.handle("zigbee2mqtt/bridge/info", []byte(`{"permit_join":false,"permit_join_end":null}`))
	if adapter.Status().PermitJoinUntil != nil {
		t.Fatal("bridge/info closing the window was ignored")
	}
}

func TestParseOnlineAcceptsBothGenerations(t *testing.T) {
	for payload, want := range map[string]bool{`online`: true, `"online"`: true, `{"state":"online"}`: true, `offline`: false, `{"state":"offline"}`: false} {
		if got, valid := parseOnline([]byte(payload)); !valid || got != want {
			t.Fatalf("%s = %v, %v", payload, got, valid)
		}
	}
	if _, valid := parseOnline([]byte(`{"state":"sleepy"}`)); valid {
		t.Fatal("an unknown availability value was accepted")
	}
	if !strings.Contains(bridgeDevices, "Room/Grow light") {
		t.Fatal("fixture lost its slash-named device")
	}
}
