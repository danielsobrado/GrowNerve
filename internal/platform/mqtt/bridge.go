package mqtt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/farm"
	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

// Bridge moves messages between the broker and the server's stores. Telemetry
// is appended to the measurement store; only the small, low-frequency facts
// (device liveness, command results) touch the farm document.
type Bridge struct {
	client    paho.Client
	store     farm.Store
	telemetry telemetry.Store
	notifier  farm.Notifier
	logger    *slog.Logger
	username  string
	password  string
}

// Option configures optional bridge collaborators.
type Option func(*Bridge)

// WithTelemetryStore directs measurements to relational storage.
func WithTelemetryStore(store telemetry.Store) Option {
	return func(bridge *Bridge) { bridge.telemetry = store }
}

// WithCredentials supplies broker credentials. Empty values leave the client
// anonymous, which is only appropriate on a development broker.
func WithCredentials(username, password string) Option {
	return func(bridge *Bridge) { bridge.username, bridge.password = username, password }
}

// WithNotifier publishes change hints so live-update clients can refresh.
func WithNotifier(notifier farm.Notifier) Option {
	return func(bridge *Bridge) { bridge.notifier = notifier }
}

func NewBridge(broker, clientID string, store farm.Store, logger *slog.Logger, options ...Option) *Bridge {
	bridge := &Bridge{store: store, logger: logger}
	for _, option := range options {
		option(bridge)
	}
	if bridge.telemetry == nil {
		bridge.telemetry = telemetry.NewMemoryStore(0)
	}
	paho.ERROR = slog.NewLogLogger(logger.Handler(), slog.LevelError)
	clientOptions := paho.NewClientOptions().AddBroker(broker).SetClientID(clientID).
		SetAutoReconnect(true).SetConnectRetry(true).SetConnectRetryInterval(5 * time.Second).SetOrderMatters(false)
	clientOptions.SetOnConnectHandler(func(client paho.Client) {
		for topic, handler := range map[string]paho.MessageHandler{
			"grownerve/v1/devices/+/telemetry":  bridge.handleTelemetry,
			"grownerve/v1/devices/+/acks":       bridge.handleAcknowledgement,
			"grownerve/v1/devices/+/health":     bridge.handleHealth,
			"grownerve/v1/devices/+/config/ack": bridge.handleConfigAcknowledgement,
		} {
			if token := client.Subscribe(topic, 1, handler); token.Wait() && token.Error() != nil {
				logger.Error("mqtt_subscribe_failed", "topic", topic, "error", token.Error())
			}
		}
		logger.Info("mqtt_connected", "broker", broker)
	})
	if bridge.username != "" {
		clientOptions.SetUsername(bridge.username).SetPassword(bridge.password)
	}
	clientOptions.SetConnectionLostHandler(func(_ paho.Client, err error) { logger.Warn("mqtt_connection_lost", "error", err) })
	bridge.client = paho.NewClient(clientOptions)
	return bridge
}

func (bridge *Bridge) Start(ctx context.Context) {
	go func() {
		token := bridge.client.Connect()
		for !token.WaitTimeout(time.Second) {
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
		if err := token.Error(); err != nil {
			bridge.logger.Warn("mqtt_initial_connect_failed", "error", err)
		}
		<-ctx.Done()
		bridge.client.Disconnect(250)
	}()
}

// Connected reports broker connectivity for readiness reporting.
func (bridge *Bridge) Connected() bool { return bridge.client.IsConnected() }

func (bridge *Bridge) PublishCommand(ctx context.Context, deviceID string, command deviceprotocol.Command) error {
	if err := command.Validate(time.Now().UTC()); err != nil {
		return err
	}
	payload, err := json.Marshal(command)
	if err != nil {
		return err
	}
	return bridge.publish(ctx, fmt.Sprintf("grownerve/v1/devices/%s/commands", deviceID), payload, false)
}

// PublishConfig delivers a retained edge configuration so a controller that
// reconnects after a server outage recovers its schedules from the broker
// without waiting for the server to come back.
func (bridge *Bridge) PublishConfig(ctx context.Context, deviceID string, payload []byte) error {
	return bridge.publish(ctx, fmt.Sprintf("grownerve/v1/devices/%s/config", deviceID), payload, true)
}

// PublishRaw publishes an already-encoded payload, which is what the outbox
// worker replays after a broker outage.
func (bridge *Bridge) PublishRaw(ctx context.Context, topic string, payload []byte) error {
	return bridge.publish(ctx, topic, payload, false)
}

func (bridge *Bridge) publish(ctx context.Context, topic string, payload []byte, retained bool) error {
	if !bridge.client.IsConnected() {
		return errors.New("MQTT broker is unavailable")
	}
	token := bridge.client.Publish(topic, 1, retained, payload)
	for !token.WaitTimeout(100 * time.Millisecond) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return token.Error()
}

func (bridge *Bridge) notify(topic string) {
	if bridge.notifier != nil {
		bridge.notifier.Notify(topic)
	}
}

// stateDocument is the narrow view of the farm document the bridge maintains.
type stateDocument struct {
	Devices  []map[string]any `json:"devices"`
	Channels []struct {
		ID       string `json:"id"`
		DeviceID string `json:"device_id"`
		Unit     string `json:"unit"`
	} `json:"channels"`
	Commands []map[string]any `json:"commands"`
}

// mutate applies a compare-and-swap read-modify-write against the farm document.
// The mutator can run more than once, so it must not carry state between calls.
func (bridge *Bridge) mutate(mutator func(*stateDocument) error) {
	err := farm.Mutate(context.Background(), bridge.store, func(state json.RawMessage) (json.RawMessage, error) {
		var document stateDocument
		if err := json.Unmarshal(state, &document); err != nil {
			return nil, errInvalidState
		}
		if err := mutator(&document); err != nil {
			return nil, err
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(state, &object); err != nil {
			return nil, errInvalidState
		}
		object["devices"], _ = json.Marshal(document.Devices)
		object["commands"], _ = json.Marshal(document.Commands)
		return json.Marshal(object)
	})
	switch {
	case err == nil:
	case errors.Is(err, errInvalidState):
		bridge.logger.Warn("mqtt_state_invalid")
	case errors.Is(err, farm.ErrNotFound):
		bridge.logger.Warn("mqtt_state_load_failed", "error", err)
	default:
		bridge.logger.Warn("mqtt_message_rejected", "reason", err)
	}
}

var errInvalidState = errors.New("stored farm state is invalid")

func (bridge *Bridge) handleTelemetry(_ paho.Client, message paho.Message) {
	envelope, err := deviceprotocol.ParseTelemetry(message.Payload())
	if err != nil {
		bridge.logger.Warn("mqtt_telemetry_invalid", "error", err)
		return
	}
	// Resolve the device and channels against the configuration document first:
	// an unknown or decommissioned device must not be able to write history.
	//
	// The envelope is all-or-nothing. Accepting the samples that resolved while
	// rejecting the envelope would let a device widen its own reach by mixing one
	// unknown channel into an otherwise valid batch, and would record history the
	// operator was never told had been accepted.
	var accepted []telemetry.Measurement
	var resolved []telemetry.Measurement
	bridge.mutate(func(document *stateDocument) error {
		knownDevice := false
		for _, device := range document.Devices {
			if device["id"] == envelope.DeviceID {
				knownDevice = true
				device["online"] = true
				device["last_heartbeat"] = time.Now().UTC()
				break
			}
		}
		if !knownDevice {
			return errors.New("unknown_device")
		}
		resolved = resolved[:0]
		for _, sample := range envelope.Samples {
			knownChannel := false
			for _, channel := range document.Channels {
				if channel.ID == sample.ChannelID && channel.DeviceID == envelope.DeviceID {
					if channel.Unit != sample.Unit {
						return errors.New("unit_mismatch")
					}
					knownChannel = true
					break
				}
			}
			if !knownChannel {
				return errors.New("unknown_channel")
			}
			sequence := int64(envelope.Sequence)
			resolved = append(resolved, telemetry.Measurement{
				ChannelID: sample.ChannelID, ObservedAt: envelope.ObservedAt, ReceivedAt: time.Now().UTC(),
				Sequence: &sequence, Value: sample.Value, Unit: sample.Unit,
				Quality: telemetry.Quality(sample.Quality), SourceDeviceID: envelope.DeviceID,
			})
		}
		// Only a mutation that returns nil reaches this point, so the samples are
		// promoted out of the retryable scratch slice exactly once the whole
		// envelope has been accepted.
		accepted = append(accepted[:0], resolved...)
		return nil
	})
	if len(accepted) == 0 {
		return
	}
	if _, err := bridge.telemetry.Append(context.Background(), accepted); err != nil {
		bridge.logger.Error("telemetry_append_failed", "error", err, "device", envelope.DeviceID)
		return
	}
	bridge.notify("measurements")
}

func (bridge *Bridge) handleAcknowledgement(_ paho.Client, message paho.Message) {
	var ack deviceprotocol.Acknowledgement
	if json.Unmarshal(message.Payload(), &ack) != nil || ack.Validate() != nil {
		bridge.logger.Warn("mqtt_ack_invalid")
		return
	}
	bridge.mutate(func(document *stateDocument) error {
		for _, command := range document.Commands {
			if command["id"] != ack.CommandID {
				continue
			}
			// A terminal command is not reopened by a late or replayed
			// acknowledgement.
			if status, _ := command["status"].(string); status == "applied" || status == "rejected" || status == "timed_out" || status == "cancelled" {
				return errors.New("command_already_final")
			}
			switch ack.Result {
			case "applied":
				command["status"] = "applied"
			case "accepted":
				command["status"] = "acknowledged"
			default:
				command["status"] = "rejected"
			}
			command["reason_code"] = ack.ReasonCode
			command["updated_at"] = ack.AcknowledgedAt
			return nil
		}
		return errors.New("unknown_command")
	})
	bridge.notify("commands")
}

func (bridge *Bridge) handleHealth(_ paho.Client, message paho.Message) {
	var health deviceprotocol.Health
	if json.Unmarshal(message.Payload(), &health) != nil || health.ProtocolVersion != deviceprotocol.Version {
		bridge.logger.Warn("mqtt_health_invalid")
		return
	}
	deviceID := strings.TrimSpace(health.DeviceID)
	bridge.mutate(func(document *stateDocument) error {
		for _, device := range document.Devices {
			if device["id"] == deviceID {
				device["online"] = true
				device["last_heartbeat"] = health.ObservedAt
				device["firmware_version"] = health.FirmwareVersion
				device["active_config_version"] = health.ActiveConfigVersion
				return nil
			}
		}
		return errors.New("unknown_device")
	})
	bridge.notify("devices")
}

// handleConfigAcknowledgement records which edge configuration a controller has
// actually adopted, which is the only trustworthy signal that a schedule change
// reached the hardware.
func (bridge *Bridge) handleConfigAcknowledgement(_ paho.Client, message paho.Message) {
	var ack struct {
		ProtocolVersion int    `json:"protocolVersion"`
		DeviceID        string `json:"deviceId"`
		ConfigVersion   string `json:"configVersion"`
		Accepted        bool   `json:"accepted"`
		Detail          string `json:"detail"`
	}
	if json.Unmarshal(message.Payload(), &ack) != nil || ack.ProtocolVersion != deviceprotocol.Version || ack.DeviceID == "" {
		bridge.logger.Warn("mqtt_config_ack_invalid")
		return
	}
	bridge.mutate(func(document *stateDocument) error {
		for _, device := range document.Devices {
			if device["id"] == ack.DeviceID {
				if ack.Accepted {
					device["active_config_version"] = ack.ConfigVersion
				}
				device["last_config_result"] = map[string]any{"version": ack.ConfigVersion, "accepted": ack.Accepted, "detail": ack.Detail}
				return nil
			}
		}
		return errors.New("unknown_device")
	})
	bridge.notify("devices")
}
