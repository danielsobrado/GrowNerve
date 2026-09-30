package zigbee2mqtt_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/jdanielsobrado/grownerve/internal/farm"
	"github.com/jdanielsobrado/grownerve/internal/integration"
	"github.com/jdanielsobrado/grownerve/internal/platform/integrations/zigbee2mqtt"
	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

const brokerEnv = "GROWNERVE_TEST_MQTT_BROKER"

const (
	zoneID   = "01990a20-6a00-7000-8000-000000000003"
	plugIEEE = "0x0017880104e45517"
)

const devicesPayload = `[{"ieee_address":"` + plugIEEE + `","type":"Router","friendly_name":"Fan plug","power_source":"Mains (single phase)",
 "definition":{"model":"ZNCZ02LM","vendor":"Xiaomi","exposes":[
  {"type":"switch","features":[{"type":"binary","name":"state","property":"state","access":7,"value_on":"ON","value_off":"OFF"}]},
  {"type":"numeric","name":"power","property":"power","access":1,"unit":"W"}]}}]`

func eventually(t *testing.T, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal(message)
}

// TestZigbee2MQTTAdapterRealBroker runs discovery, adoption, telemetry and a
// command round trip against a real broker, with the test playing the part
// of Zigbee2MQTT and the plug.
func TestZigbee2MQTTAdapterRealBroker(t *testing.T) {
	broker := os.Getenv(brokerEnv)
	if broker == "" {
		t.Skipf("set %s to run integration tests against a broker", brokerEnv)
	}
	base := "gn-test-" + uuid.NewString()[:8]

	// The stand-in Zigbee2MQTT bridge: announces a plug and reflects /set.
	bridge := paho.NewClient(paho.NewClientOptions().AddBroker(broker).SetClientID("z2m-" + base))
	if token := bridge.Connect(); !token.WaitTimeout(10*time.Second) || token.Error() != nil {
		t.Fatalf("connect bridge: %v", token.Error())
	}
	t.Cleanup(func() {
		for _, topic := range []string{"bridge/state", "bridge/devices", "Fan plug/availability"} {
			bridge.Publish(base+"/"+topic, 1, true, []byte{}).Wait()
		}
		bridge.Disconnect(100)
	})
	publish := func(topic, payload string, retained bool) {
		if token := bridge.Publish(base+"/"+topic, 1, retained, payload); !token.WaitTimeout(5*time.Second) || token.Error() != nil {
			t.Fatalf("publish %s: %v", topic, token.Error())
		}
	}
	bridge.Subscribe(base+"/Fan plug/set", 1, func(_ paho.Client, message paho.Message) {
		var request map[string]string
		if json.Unmarshal(message.Payload(), &request) == nil && request["state"] != "" {
			power := "0"
			if request["state"] == "ON" {
				power = "37.5"
			}
			bridge.Publish(base+"/Fan plug", 1, false, `{"state":"`+request["state"]+`","power":`+power+`}`)
		}
	}).Wait()
	publish("bridge/state", `{"state":"online"}`, true)
	publish("bridge/devices", devicesPayload, true)
	publish("Fan plug/availability", `{"state":"online"}`, true)

	store := farm.NewMemoryStore()
	state := `{"facilities":[{"id":"01990a20-6a00-7000-8000-000000000001"}],"zones":[{"id":"` + zoneID + `"}],"devices":[],"channels":[],"channel_bindings":[],"commands":[]}`
	if _, err := store.Save(context.Background(), json.RawMessage(state), farm.AnyVersion); err != nil {
		t.Fatal(err)
	}
	samples := telemetry.NewMemoryStore(0)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	adapter := zigbee2mqtt.New(zigbee2mqtt.Config{
		Broker: broker, ClientID: "grownerve-" + base, BaseTopic: base, AvailabilityFallback: time.Hour, Logger: logger,
	})
	manager := integration.NewManager(integration.Dependencies{Store: store, Telemetry: samples, Logger: logger, LivenessInterval: 200 * time.Millisecond}, adapter)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	manager.Start(ctx)

	eventually(t, func() bool { return adapter.Available(plugIEEE) }, "the plug never became available through the broker")
	adopted, err := manager.Adopt(ctx, integration.Zigbee2MQTT, plugIEEE, "test", integration.AdoptRequest{
		ZoneID: zoneID, AcknowledgeNoEdgeFailsafe: true,
		Channels: []integration.AdoptChannel{{CapabilityKey: "state"}, {CapabilityKey: "power"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	stateChannel := adopted.Channels[0]["id"].(string)
	eventually(t, func() bool {
		raw, _, _ := store.Load(ctx)
		return strings.Contains(string(raw), `"online":true`)
	}, "liveness never marked the adopted plug online")

	publish("Fan plug", `{"state":"OFF","power":0}`, false)
	eventually(t, func() bool {
		latest, _ := samples.Latest(ctx)
		return len(latest) == 2
	}, "plug telemetry never reached the measurement store")

	handler := farm.NewHandler(store, farm.WithCommandPublisher(integration.NewRouter(nil, manager)))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/commands",
		strings.NewReader(`{"targetChannelId":"`+stateChannel+`","value":true,"reason":"ventilate"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("command status = %d: %s", response.Code, response.Body.String())
	}
	eventually(t, func() bool {
		raw, _, _ := store.Load(ctx)
		var document struct {
			Commands []map[string]any `json:"commands"`
			Devices  []map[string]any `json:"devices"`
		}
		_ = json.Unmarshal(raw, &document)
		return len(document.Commands) == 1 && document.Commands[0]["status"] == "applied" && document.Devices[0]["state"] == true
	}, "the command was never confirmed applied by the plug's reflected state")
}
