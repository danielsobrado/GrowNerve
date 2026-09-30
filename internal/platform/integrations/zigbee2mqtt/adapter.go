package zigbee2mqtt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/jdanielsobrado/grownerve/internal/integration"
)

// maximumBufferedStates bounds state payloads kept for friendly names that
// bridge/devices has not described yet (retained messages arrive unordered).
const maximumBufferedStates = 1024

type Config struct {
	Broker    string
	ClientID  string
	Username  string
	Password  string
	BaseTopic string
	// AvailabilityFallback applies to devices without availability reporting.
	AvailabilityFallback time.Duration
	Logger               *slog.Logger
	Now                  func() time.Time
}

// transport is the MQTT client surface the adapter needs; tests replace it.
type transport interface {
	Publish(ctx context.Context, topic string, payload []byte) error
	Connected() bool
}

type Adapter struct {
	config    Config
	transport transport
	client    paho.Client

	mu      sync.RWMutex
	sink    integration.Sink
	devices map[string]*device
	byName  map[string]string
	// availability is keyed by friendly name: retained availability messages
	// can arrive before bridge/devices has described the device.
	availability    map[string]bool
	lastSeen        map[string]time.Time
	buffered        map[string][]byte
	bridgeKnown     bool
	bridgeOnline    bool
	permitJoinUntil *time.Time
	connected       bool
	since           time.Time
	detail          string
}

func New(config Config) *Adapter {
	if config.Logger == nil {
		config.Logger = slog.New(slog.DiscardHandler)
	}
	if config.Now == nil {
		config.Now = func() time.Time { return time.Now().UTC() }
	}
	if config.BaseTopic == "" {
		config.BaseTopic = "zigbee2mqtt"
	}
	adapter := &Adapter{
		config: config, devices: map[string]*device{}, byName: map[string]string{},
		availability: map[string]bool{}, lastSeen: map[string]time.Time{}, buffered: map[string][]byte{},
		since: config.Now(), detail: "connecting to the MQTT broker",
	}
	return adapter
}

func (adapter *Adapter) Provider() integration.Provider { return integration.Zigbee2MQTT }

func (adapter *Adapter) Start(ctx context.Context, sink integration.Sink) {
	adapter.mu.Lock()
	adapter.sink = sink
	adapter.mu.Unlock()
	if adapter.transport != nil {
		return
	}
	options := paho.NewClientOptions().AddBroker(adapter.config.Broker).SetClientID(adapter.config.ClientID).
		SetAutoReconnect(true).SetConnectRetry(true).SetConnectRetryInterval(5 * time.Second).SetOrderMatters(false)
	if adapter.config.Username != "" {
		options.SetUsername(adapter.config.Username).SetPassword(adapter.config.Password)
	}
	options.SetOnConnectHandler(func(client paho.Client) {
		if token := client.Subscribe(adapter.config.BaseTopic+"/#", 1, func(_ paho.Client, message paho.Message) {
			adapter.handle(message.Topic(), message.Payload())
		}); token.Wait() && token.Error() != nil {
			adapter.config.Logger.Error("zigbee2mqtt_subscribe_failed", "error", token.Error())
			return
		}
		adapter.setConnected(true, "waiting for the Zigbee2MQTT bridge")
		adapter.config.Logger.Info("zigbee2mqtt_connected", "broker", adapter.config.Broker, "base_topic", adapter.config.BaseTopic)
	})
	options.SetConnectionLostHandler(func(_ paho.Client, err error) {
		adapter.config.Logger.Warn("zigbee2mqtt_connection_lost", "error", err)
		adapter.setConnected(false, "MQTT connection lost: "+err.Error())
	})
	adapter.client = paho.NewClient(options)
	adapter.transport = pahoTransport{adapter.client}
	go func() {
		token := adapter.client.Connect()
		for !token.WaitTimeout(time.Second) {
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
		if err := token.Error(); err != nil {
			adapter.config.Logger.Warn("zigbee2mqtt_initial_connect_failed", "error", err)
		}
		<-ctx.Done()
		adapter.client.Disconnect(250)
	}()
}

func (adapter *Adapter) setConnected(connected bool, detail string) {
	adapter.mu.Lock()
	changed := adapter.connected != connected
	adapter.connected, adapter.detail = connected, detail
	if changed {
		adapter.since = adapter.config.Now()
	}
	if !connected {
		adapter.bridgeKnown = false
	}
	sink := adapter.sink
	adapter.mu.Unlock()
	if changed && sink != nil {
		sink.StatusChanged(integration.Zigbee2MQTT)
	}
}

// handle dispatches one message under the base topic.
func (adapter *Adapter) handle(topic string, payload []byte) {
	rest, found := strings.CutPrefix(topic, adapter.config.BaseTopic+"/")
	if !found {
		return
	}
	switch rest {
	case "bridge/devices":
		adapter.handleDevices(payload)
		return
	case "bridge/state":
		adapter.handleBridgeState(payload)
		return
	case "bridge/info":
		adapter.handleBridgeInfo(payload)
		return
	}
	if strings.HasPrefix(rest, "bridge/") {
		return
	}
	adapter.mu.RLock()
	ieee, isDevice := adapter.byName[rest]
	adapter.mu.RUnlock()
	if isDevice {
		adapter.handleState(ieee, payload)
		return
	}
	if name, isAvailability := strings.CutSuffix(rest, "/availability"); isAvailability {
		adapter.handleAvailability(name, payload)
		return
	}
	if strings.HasSuffix(rest, "/set") || strings.HasSuffix(rest, "/get") || strings.Contains(rest, "/set/") {
		return
	}
	// A state for a name bridge/devices has not described yet.
	adapter.mu.Lock()
	if _, known := adapter.buffered[rest]; known || len(adapter.buffered) < maximumBufferedStates {
		adapter.buffered[rest] = append([]byte(nil), payload...)
	}
	adapter.mu.Unlock()
}

func (adapter *Adapter) handleDevices(payload []byte) {
	parsed, err := parseDevices(payload)
	if err != nil {
		adapter.config.Logger.Warn("zigbee2mqtt_devices_invalid", "error", err)
		return
	}
	adapter.mu.Lock()
	adapter.devices, adapter.byName = map[string]*device{}, map[string]string{}
	var replay []struct {
		ieee    string
		payload []byte
	}
	for _, entry := range parsed {
		adapter.devices[entry.ieee] = entry
		adapter.byName[entry.name] = entry.ieee
		if buffered, waiting := adapter.buffered[entry.name]; waiting {
			replay = append(replay, struct {
				ieee    string
				payload []byte
			}{entry.ieee, buffered})
			delete(adapter.buffered, entry.name)
		}
	}
	sink := adapter.sink
	adapter.mu.Unlock()
	adapter.config.Logger.Info("zigbee2mqtt_devices", "count", len(parsed))
	if sink != nil {
		sink.DevicesChanged(integration.Zigbee2MQTT)
		sink.AvailabilityChanged(integration.Zigbee2MQTT)
	}
	for _, pending := range replay {
		adapter.handleState(pending.ieee, pending.payload)
	}
}

func (adapter *Adapter) handleBridgeState(payload []byte) {
	online, valid := parseOnline(payload)
	if !valid {
		return
	}
	adapter.mu.Lock()
	changed := !adapter.bridgeKnown || adapter.bridgeOnline != online
	adapter.bridgeKnown, adapter.bridgeOnline = true, online
	if changed {
		adapter.since = adapter.config.Now()
	}
	sink := adapter.sink
	adapter.mu.Unlock()
	if changed && sink != nil {
		sink.StatusChanged(integration.Zigbee2MQTT)
	}
}

func (adapter *Adapter) handleBridgeInfo(payload []byte) {
	var info struct {
		PermitJoin    bool   `json:"permit_join"`
		PermitJoinEnd *int64 `json:"permit_join_end"`
	}
	if json.Unmarshal(payload, &info) != nil {
		return
	}
	adapter.mu.Lock()
	adapter.permitJoinUntil = nil
	if info.PermitJoin && info.PermitJoinEnd != nil {
		until := time.UnixMilli(*info.PermitJoinEnd).UTC()
		adapter.permitJoinUntil = &until
	}
	sink := adapter.sink
	adapter.mu.Unlock()
	if sink != nil {
		sink.StatusChanged(integration.Zigbee2MQTT)
	}
}

func (adapter *Adapter) handleAvailability(name string, payload []byte) {
	online, valid := parseOnline(payload)
	if !valid {
		return
	}
	adapter.mu.Lock()
	_, known := adapter.byName[name]
	previous, reported := adapter.availability[name]
	if known || reported || len(adapter.availability) < maximumBufferedStates {
		adapter.availability[name] = online
	}
	sink := adapter.sink
	adapter.mu.Unlock()
	if known && (!reported || previous != online) && sink != nil {
		sink.AvailabilityChanged(integration.Zigbee2MQTT)
	}
}

func (adapter *Adapter) handleState(ieee string, payload []byte) {
	now := adapter.config.Now()
	adapter.mu.Lock()
	entry := adapter.devices[ieee]
	adapter.lastSeen[ieee] = now
	sink := adapter.sink
	adapter.mu.Unlock()
	if entry == nil {
		return
	}
	readings, err := entry.readings(payload)
	if err != nil {
		adapter.config.Logger.Warn("zigbee2mqtt_state_invalid", "device", entry.name, "error", err)
		return
	}
	for index := range readings {
		readings[index].ObservedAt = now
	}
	if len(readings) > 0 && sink != nil {
		sink.Readings(integration.Zigbee2MQTT, readings)
	}
}

func (adapter *Adapter) Status() integration.Status {
	adapter.mu.RLock()
	defer adapter.mu.RUnlock()
	status := integration.Status{Provider: integration.Zigbee2MQTT, Since: adapter.since, PermitJoinUntil: adapter.permitJoinUntil}
	switch {
	case !adapter.connected:
		status.State, status.Detail = integration.StateConnecting, adapter.detail
	case !adapter.bridgeKnown:
		status.State, status.Detail = integration.StateConnecting, "waiting for the Zigbee2MQTT bridge to report its state"
	case !adapter.bridgeOnline:
		status.State, status.Detail = integration.StateDegraded, "the Zigbee2MQTT bridge is offline"
	default:
		status.State = integration.StateConnected
	}
	if status.PermitJoinUntil != nil && !status.PermitJoinUntil.After(adapter.config.Now()) {
		status.PermitJoinUntil = nil
	}
	return status
}

func (adapter *Adapter) Devices() []integration.DiscoveredDevice {
	adapter.mu.RLock()
	defer adapter.mu.RUnlock()
	devices := make([]integration.DiscoveredDevice, 0, len(adapter.devices))
	for ieee, entry := range adapter.devices {
		discovered := entry.discovered
		discovered.Capabilities = append([]integration.Capability(nil), entry.discovered.Capabilities...)
		discovered.Available = adapter.availableLocked(ieee)
		devices = append(devices, discovered)
	}
	return devices
}

func (adapter *Adapter) Available(externalID string) bool {
	adapter.mu.RLock()
	defer adapter.mu.RUnlock()
	return adapter.availableLocked(externalID)
}

func (adapter *Adapter) availableLocked(ieee string) bool {
	if !adapter.connected || (adapter.bridgeKnown && !adapter.bridgeOnline) {
		return false
	}
	entry, known := adapter.devices[ieee]
	if !known {
		return false
	}
	if online, reported := adapter.availability[entry.name]; reported {
		return online
	}
	seen, ever := adapter.lastSeen[ieee]
	return ever && adapter.config.Now().Sub(seen) <= adapter.config.AvailabilityFallback
}

func (adapter *Adapter) Execute(ctx context.Context, command integration.Command) (integration.Outcome, error) {
	adapter.mu.RLock()
	entry := adapter.devices[command.ExternalID]
	adapter.mu.RUnlock()
	if entry == nil {
		return integration.Outcome{Result: integration.OutcomeRejected, ReasonCode: "UNKNOWN_EXTERNAL_DEVICE"}, nil
	}
	target, known := entry.properties[command.Key]
	if !known {
		return integration.Outcome{Result: integration.OutcomeRejected, ReasonCode: "UNKNOWN_CAPABILITY"}, nil
	}
	body, err := target.setPayload(command.Key, command)
	if err != nil {
		return integration.Outcome{Result: integration.OutcomeRejected, ReasonCode: "NOT_CONTROLLABLE"}, nil
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return integration.Outcome{}, err
	}
	if adapter.transport == nil || !adapter.transport.Connected() {
		return integration.Outcome{}, errors.New("zigbee2mqtt broker connection is down")
	}
	// Zigbee2MQTT has no per-request reply for device writes. The command
	// stays published until the device's next state report reflects it.
	if err := adapter.transport.Publish(ctx, adapter.config.BaseTopic+"/"+entry.name+"/set", payload); err != nil {
		return integration.Outcome{}, err
	}
	return integration.Outcome{Result: integration.OutcomePublished}, nil
}

// PermitJoin opens the Zigbee network (Zigbee2MQTT 2.x request format);
// seconds of zero closes it.
func (adapter *Adapter) PermitJoin(ctx context.Context, seconds int) (time.Time, error) {
	if adapter.transport == nil || !adapter.transport.Connected() {
		return time.Time{}, errors.New("zigbee2mqtt broker connection is down")
	}
	payload, _ := json.Marshal(map[string]int{"time": seconds})
	if err := adapter.transport.Publish(ctx, adapter.config.BaseTopic+"/bridge/request/permit_join", payload); err != nil {
		return time.Time{}, fmt.Errorf("request permit join: %w", err)
	}
	until := adapter.config.Now().Add(time.Duration(seconds) * time.Second)
	adapter.mu.Lock()
	adapter.permitJoinUntil = nil
	if seconds > 0 {
		adapter.permitJoinUntil = &until
	}
	adapter.mu.Unlock()
	return until, nil
}

type pahoTransport struct{ client paho.Client }

func (transport pahoTransport) Connected() bool { return transport.client.IsConnected() }

func (transport pahoTransport) Publish(ctx context.Context, topic string, payload []byte) error {
	token := transport.client.Publish(topic, 1, false, payload)
	for !token.WaitTimeout(100 * time.Millisecond) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return token.Error()
}
