package mqtt_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/edge"
	"github.com/jdanielsobrado/grownerve/internal/farm"
	mqttbridge "github.com/jdanielsobrado/grownerve/internal/platform/mqtt"
	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

// brokerEnv names the variable holding a reachable MQTT broker. Tests skip when
// it is unset so a developer without a broker still gets a green unit run.
const brokerEnv = "GROWNERVE_TEST_MQTT_BROKER"

const (
	integrationDevice  = "01990a20-6a00-7000-8000-0000000000a1"
	integrationChannel = "01990a20-6a00-7000-8000-0000000000a2"
	lightChannel       = "01990a20-6a00-7000-8000-0000000000a3"
)

func brokerURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv(brokerEnv)
	if url == "" {
		t.Skipf("set %s to run integration tests against a broker", brokerEnv)
	}
	return url
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// eventually polls until the condition holds, so the test asserts on the outcome
// rather than on a fixed sleep.
func eventually(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal(message)
}

func deviceClient(t *testing.T, url, clientID string) paho.Client {
	t.Helper()
	client := paho.NewClient(paho.NewClientOptions().AddBroker(url).SetClientID(clientID))
	token := client.Connect()
	if !token.WaitTimeout(10*time.Second) || token.Error() != nil {
		t.Fatalf("connect device client: %v", token.Error())
	}
	t.Cleanup(func() { client.Disconnect(100) })
	return client
}

func farmState(t *testing.T) *farm.MemoryStore {
	t.Helper()
	store := farm.NewMemoryStore()
	state := fmt.Sprintf(`{
		"devices":[{"id":%q,"name":"Tent controller","online":false,"active_config_version":"","desired_config_version":"pilot-v1",
			"desired_config":{"photoperiod":{"onHour":6,"offHour":22,"channelId":%q},"safeOutputs":{%q:0}}}],
		"channels":[{"id":%q,"device_id":%q,"unit":"degC","kind":"measurement"}],
		"commands":[],"alerts":[]
	}`, integrationDevice, lightChannel, lightChannel, integrationChannel, integrationDevice)
	if _, err := store.Save(context.Background(), json.RawMessage(state), farm.AnyVersion); err != nil {
		t.Fatal(err)
	}
	return store
}

// TestTelemetryReachesTheStoreThroughARealBroker exercises the path the unit
// tests fake: publish, subscribe, decode, validate, persist.
func TestTelemetryReachesTheStoreThroughARealBroker(t *testing.T) {
	url := brokerURL(t)
	store := farmState(t)
	samples := telemetry.NewMemoryStore(0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bridge := mqttbridge.NewBridge(url, "grownerve-test-"+t.Name(), store, quiet(), mqttbridge.WithTelemetryStore(samples))
	bridge.Start(ctx)
	eventually(t, 15*time.Second, bridge.Connected, "the bridge never connected to the broker")

	device := deviceClient(t, url, "device-"+t.Name())
	envelope := deviceprotocol.TelemetryEnvelope{
		ProtocolVersion: deviceprotocol.Version, DeviceID: integrationDevice, BootID: "boot-1",
		Sequence: 1, ObservedAt: time.Now().UTC(),
		Samples: []deviceprotocol.Sample{{ChannelID: integrationChannel, Value: 21.5, Unit: "degC", Quality: deviceprotocol.QualityGood}},
	}
	payload, _ := json.Marshal(envelope)
	device.Publish(fmt.Sprintf("grownerve/v1/devices/%s/telemetry", integrationDevice), 1, false, payload).WaitTimeout(5 * time.Second)

	eventually(t, 15*time.Second, func() bool {
		stored, err := samples.Recent(context.Background(), 10)
		return err == nil && len(stored) == 1 && stored[0].Value == 21.5
	}, "telemetry published to the broker never reached the measurement store")

	// The device is marked online by its own traffic, not by an announcement.
	state, _, _ := store.Load(context.Background())
	var document struct {
		Devices []map[string]any `json:"devices"`
	}
	_ = json.Unmarshal(state, &document)
	if document.Devices[0]["online"] != true {
		t.Fatalf("telemetry did not mark the device online: %v", document.Devices[0])
	}
}

// TestUnknownDeviceCannotWriteHistory is the compromised-device case from the
// threat assumptions: anything that can reach the broker must still be refused.
func TestUnknownDeviceCannotWriteHistory(t *testing.T) {
	url := brokerURL(t)
	store := farmState(t)
	samples := telemetry.NewMemoryStore(0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bridge := mqttbridge.NewBridge(url, "grownerve-test-"+t.Name(), store, quiet(), mqttbridge.WithTelemetryStore(samples))
	bridge.Start(ctx)
	eventually(t, 15*time.Second, bridge.Connected, "the bridge never connected")

	stranger := "01990a20-6a00-7000-8000-0000000000ff"
	device := deviceClient(t, url, "stranger-"+t.Name())
	envelope := deviceprotocol.TelemetryEnvelope{
		ProtocolVersion: deviceprotocol.Version, DeviceID: stranger, BootID: "boot-x", Sequence: 1,
		ObservedAt: time.Now().UTC(),
		Samples:    []deviceprotocol.Sample{{ChannelID: integrationChannel, Value: 999, Unit: "degC", Quality: deviceprotocol.QualityGood}},
	}
	payload, _ := json.Marshal(envelope)
	device.Publish(fmt.Sprintf("grownerve/v1/devices/%s/telemetry", stranger), 1, false, payload).WaitTimeout(5 * time.Second)

	// Malformed payloads must be refused too, without disturbing the bridge.
	device.Publish(fmt.Sprintf("grownerve/v1/devices/%s/telemetry", integrationDevice), 1, false, []byte("not json")).WaitTimeout(5 * time.Second)

	time.Sleep(time.Second)
	stored, err := samples.Recent(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 0 {
		t.Fatalf("an unregistered device wrote %d measurements: %+v", len(stored), stored)
	}
}

// TestRetainedConfigurationSurvivesServerLoss is the software half of the phase
// 8 exit criterion. The server publishes configuration retained, then goes away
// entirely; a controller that connects afterwards must still receive it and keep
// running its schedules alone.
func TestRetainedConfigurationSurvivesServerLoss(t *testing.T) {
	url := brokerURL(t)
	topic := fmt.Sprintf("grownerve/v1/devices/%s/config", integrationDevice)

	// Clear any retained message a previous run left behind.
	cleaner := deviceClient(t, url, "cleaner-"+t.Name())
	cleaner.Publish(topic, 1, true, []byte{}).WaitTimeout(5 * time.Second)
	time.Sleep(200 * time.Millisecond)

	serverContext, stopServer := context.WithCancel(context.Background())
	bridge := mqttbridge.NewBridge(url, "grownerve-test-"+t.Name(), farmState(t), quiet())
	bridge.Start(serverContext)
	eventually(t, 15*time.Second, bridge.Connected, "the bridge never connected")

	config := deviceprotocol.EdgeConfig{
		ProtocolVersion: deviceprotocol.Version, DeviceID: integrationDevice,
		ConfigVersion: "pilot-v1", IssuedAt: time.Now().UTC(),
		Config: deviceprotocol.EdgeSettings{
			Photoperiod: &deviceprotocol.Photoperiod{OnHour: 6, OffHour: 22, ChannelID: lightChannel},
			SafeOutputs: map[string]float64{lightChannel: 0},
		},
	}
	payload, _ := json.Marshal(config)
	if err := bridge.PublishConfig(context.Background(), integrationDevice, payload); err != nil {
		t.Fatal(err)
	}

	// The server disappears completely.
	stopServer()
	time.Sleep(500 * time.Millisecond)

	// A controller boots afterwards with nothing but the broker.
	controller := edge.NewController(integrationDevice)
	received := make(chan deviceprotocol.EdgeConfig, 1)
	device := deviceClient(t, url, "late-device-"+t.Name())
	token := device.Subscribe(topic, 1, func(_ paho.Client, message paho.Message) {
		var delivered deviceprotocol.EdgeConfig
		if json.Unmarshal(message.Payload(), &delivered) == nil && delivered.ConfigVersion != "" {
			select {
			case received <- delivered:
			default:
			}
		}
	})
	if !token.WaitTimeout(10*time.Second) || token.Error() != nil {
		t.Fatalf("subscribe: %v", token.Error())
	}

	select {
	case delivered := <-received:
		if err := controller.ApplyConfig(delivered); err != nil {
			t.Fatalf("controller rejected the retained configuration: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a controller connecting after the server stopped received no retained configuration")
	}

	// With the server gone, the photoperiod still runs from the controller's own
	// clock and persisted configuration.
	daytime := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	if resolution := controller.Resolve(lightChannel, daytime); resolution.Value != 100 {
		t.Fatalf("light did not run its schedule without a server: %+v", resolution)
	}
	night := time.Date(2026, 6, 15, 23, 0, 0, 0, time.UTC)
	if resolution := controller.Resolve(lightChannel, night); resolution.Value != 0 {
		t.Fatalf("light did not switch off on schedule without a server: %+v", resolution)
	}

	cleaner.Publish(topic, 1, true, []byte{}).WaitTimeout(5 * time.Second)
}

// TestCommandAcknowledgementUpdatesTheDurableRecord closes the control loop over
// a real broker.
func TestCommandAcknowledgementUpdatesTheDurableRecord(t *testing.T) {
	url := brokerURL(t)
	store := farm.NewMemoryStore()
	commandID := "01990a20-6a00-7000-8000-0000000000b1"
	state := fmt.Sprintf(`{"devices":[{"id":%q,"online":true}],"channels":[],"commands":[{"id":%q,"status":"published"}]}`,
		integrationDevice, commandID)
	if _, err := store.Save(context.Background(), json.RawMessage(state), farm.AnyVersion); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bridge := mqttbridge.NewBridge(url, "grownerve-test-"+t.Name(), store, quiet())
	bridge.Start(ctx)
	eventually(t, 15*time.Second, bridge.Connected, "the bridge never connected")

	device := deviceClient(t, url, "acking-device-"+t.Name())
	ack := deviceprotocol.Acknowledgement{
		ProtocolVersion: deviceprotocol.Version, CommandID: commandID, DeviceID: integrationDevice,
		Result: "applied", AcknowledgedAt: time.Now().UTC(),
	}
	payload, _ := json.Marshal(ack)
	device.Publish(fmt.Sprintf("grownerve/v1/devices/%s/acks", integrationDevice), 1, false, payload).WaitTimeout(5 * time.Second)

	eventually(t, 15*time.Second, func() bool {
		current, _, err := store.Load(context.Background())
		if err != nil {
			return false
		}
		var document struct {
			Commands []map[string]any `json:"commands"`
		}
		_ = json.Unmarshal(current, &document)
		return len(document.Commands) == 1 && document.Commands[0]["status"] == "applied"
	}, "an acknowledgement over the broker never reached the durable command record")
}
