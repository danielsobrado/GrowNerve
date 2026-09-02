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

const mqttStoreTimeout = 5 * time.Second

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
	now       func() time.Time
}

type Option func(*Bridge)

func WithTelemetryStore(store telemetry.Store) Option {
	return func(bridge *Bridge) { bridge.telemetry = store }
}

func WithCredentials(username, password string) Option {
	return func(bridge *Bridge) { bridge.username, bridge.password = username, password }
}

func WithNotifier(notifier farm.Notifier) Option {
	return func(bridge *Bridge) { bridge.notifier = notifier }
}

func WithClock(now func() time.Time) Option {
	return func(bridge *Bridge) {
		if now != nil {
			bridge.now = now
		}
	}
}

func NewBridge(broker, clientID string, store farm.Store, logger *slog.Logger, options ...Option) *Bridge {
	bridge := &Bridge{store: store, logger: logger, now: func() time.Time { return time.Now().UTC() }}
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

func (bridge *Bridge) Connected() bool { return bridge.client.IsConnected() }

func (bridge *Bridge) PublishCommand(ctx context.Context, deviceID string, command deviceprotocol.Command) error {
	if err := command.Validate(bridge.now()); err != nil {
		return err
	}
	payload, err := json.Marshal(command)
	if err != nil {
		return err
	}
	return bridge.publish(ctx, fmt.Sprintf("grownerve/v1/devices/%s/commands", deviceID), payload, false)
}

func (bridge *Bridge) PublishConfig(ctx context.Context, deviceID string, payload []byte) error {
	return bridge.publish(ctx, fmt.Sprintf("grownerve/v1/devices/%s/config", deviceID), payload, true)
}

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

type stateDocument struct {
	Devices  []map[string]any `json:"devices"`
	Channels []struct {
		ID       string `json:"id"`
		DeviceID string `json:"device_id"`
		Unit     string `json:"unit"`
	} `json:"channels"`
	Commands []map[string]any `json:"commands"`
}

func (bridge *Bridge) mutate(mutator func(*stateDocument) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), mqttStoreTimeout)
	defer cancel()
	err := farm.Mutate(ctx, bridge.store, func(state json.RawMessage) (json.RawMessage, error) {
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
	return err
}

var errInvalidState = errors.New("stored farm state is invalid")

func topicDeviceID(topic, suffix string) (string, bool) {
	const prefix = "grownerve/v1/devices/"
	ending := "/" + suffix
	if !strings.HasPrefix(topic, prefix) || !strings.HasSuffix(topic, ending) {
		return "", false
	}
	deviceID := strings.TrimSuffix(strings.TrimPrefix(topic, prefix), ending)
	if deviceID == "" || strings.Contains(deviceID, "/") {
		return "", false
	}
	return deviceID, true
}

func channelOwnedBy(document *stateDocument, deviceID, channelID string) bool {
	for _, channel := range document.Channels {
		if channel.ID == channelID && channel.DeviceID == deviceID {
			return true
		}
	}
	return false
}

func storedTime(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC(), !typed.IsZero()
	case string:
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
			if parsed, err := time.Parse(layout, typed); err == nil {
				return parsed.UTC(), true
			}
		}
	}
	return time.Time{}, false
}

func (bridge *Bridge) handleTelemetry(_ paho.Client, message paho.Message) {
	topicDevice, validTopic := topicDeviceID(message.Topic(), "telemetry")
	envelope, err := deviceprotocol.ParseTelemetry(message.Payload())
	if err != nil {
		bridge.logger.Warn("mqtt_telemetry_invalid", "error", err)
		return
	}
	if !validTopic || envelope.DeviceID != topicDevice {
		bridge.logger.Warn("mqtt_device_identity_mismatch", "topic", message.Topic(), "payload_device", envelope.DeviceID)
		return
	}
	receivedAt := bridge.now()
	if envelope.ObservedAt.After(receivedAt.Add(deviceprotocol.MaximumFutureClockSkew)) {
		bridge.logger.Warn("mqtt_telemetry_future_timestamp", "device", topicDevice, "observed_at", envelope.ObservedAt, "received_at", receivedAt)
		return
	}

	var accepted []telemetry.Measurement
	var resolved []telemetry.Measurement
	if err := bridge.mutate(func(document *stateDocument) error {
		knownDevice := false
		for _, device := range document.Devices {
			if device["id"] == topicDevice {
				knownDevice = true
				device["online"] = true
				device["last_heartbeat"] = receivedAt
				device["last_device_observed_at"] = envelope.ObservedAt
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
				if channel.ID == sample.ChannelID && channel.DeviceID == topicDevice {
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
				ChannelID: sample.ChannelID, ObservedAt: envelope.ObservedAt, ReceivedAt: receivedAt,
				Sequence: &sequence, Value: sample.Value, Unit: sample.Unit,
				Quality: telemetry.Quality(sample.Quality), SourceDeviceID: topicDevice,
			})
		}
		accepted = append(accepted[:0], resolved...)
		return nil
	}); err != nil || len(accepted) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), mqttStoreTimeout)
	defer cancel()
	if _, err := bridge.telemetry.Append(ctx, accepted); err != nil {
		bridge.logger.Error("telemetry_append_failed", "error", err, "device", topicDevice)
		return
	}
	bridge.notify("measurements")
}

func (bridge *Bridge) handleAcknowledgement(_ paho.Client, message paho.Message) {
	topicDevice, validTopic := topicDeviceID(message.Topic(), "acks")
	var ack deviceprotocol.Acknowledgement
	if json.Unmarshal(message.Payload(), &ack) != nil || ack.Validate() != nil {
		bridge.logger.Warn("mqtt_ack_invalid")
		return
	}
	if !validTopic || ack.DeviceID != topicDevice {
		bridge.logger.Warn("mqtt_device_identity_mismatch", "topic", message.Topic(), "payload_device", ack.DeviceID)
		return
	}
	receivedAt := bridge.now()
	if err := bridge.mutate(func(document *stateDocument) error {
		for _, command := range document.Commands {
			if command["id"] != ack.CommandID {
				continue
			}
			targetChannel, _ := command["target_channel_id"].(string)
			if !channelOwnedBy(document, topicDevice, targetChannel) {
				return errors.New("command_device_mismatch")
			}
			if status, _ := command["status"].(string); status == "applied" || status == "rejected" || status == "timed_out" || status == "cancelled" {
				return errors.New("command_already_final")
			}
			expiresAt, valid := storedTime(command["expires_at"])
			if !valid {
				return errors.New("command_expiry_invalid")
			}
			command["acknowledged_at"] = ack.AcknowledgedAt
			command["updated_at"] = receivedAt
			if !expiresAt.After(receivedAt) {
				command["status"] = "timed_out"
				command["reason_code"] = "COMMAND_EXPIRED"
				return nil
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
			return nil
		}
		return errors.New("unknown_command")
	}); err == nil {
		bridge.notify("commands")
	}
}

func (bridge *Bridge) handleHealth(_ paho.Client, message paho.Message) {
	topicDevice, validTopic := topicDeviceID(message.Topic(), "health")
	var health deviceprotocol.Health
	if json.Unmarshal(message.Payload(), &health) != nil || health.ProtocolVersion != deviceprotocol.Version || health.ObservedAt.IsZero() {
		bridge.logger.Warn("mqtt_health_invalid")
		return
	}
	deviceID := strings.TrimSpace(health.DeviceID)
	if !validTopic || deviceID != topicDevice {
		bridge.logger.Warn("mqtt_device_identity_mismatch", "topic", message.Topic(), "payload_device", deviceID)
		return
	}
	receivedAt := bridge.now()
	if health.ObservedAt.After(receivedAt.Add(deviceprotocol.MaximumFutureClockSkew)) {
		bridge.logger.Warn("mqtt_health_future_timestamp", "device", topicDevice, "observed_at", health.ObservedAt, "received_at", receivedAt)
		return
	}
	if err := bridge.mutate(func(document *stateDocument) error {
		for _, device := range document.Devices {
			if device["id"] == topicDevice {
				device["online"] = true
				device["last_heartbeat"] = receivedAt
				device["last_device_observed_at"] = health.ObservedAt
				device["firmware_version"] = health.FirmwareVersion
				device["active_config_version"] = health.ActiveConfigVersion
				return nil
			}
		}
		return errors.New("unknown_device")
	}); err == nil {
		bridge.notify("devices")
	}
}

func (bridge *Bridge) handleConfigAcknowledgement(_ paho.Client, message paho.Message) {
	topicDevice, validTopic := topicDeviceID(message.Topic(), "config/ack")
	var ack deviceprotocol.ConfigAcknowledgement
	if json.Unmarshal(message.Payload(), &ack) != nil || ack.Validate() != nil {
		bridge.logger.Warn("mqtt_config_ack_invalid")
		return
	}
	if !validTopic || ack.DeviceID != topicDevice {
		bridge.logger.Warn("mqtt_device_identity_mismatch", "topic", message.Topic(), "payload_device", ack.DeviceID)
		return
	}
	receivedAt := bridge.now()
	if err := bridge.mutate(func(document *stateDocument) error {
		for _, device := range document.Devices {
			if device["id"] == topicDevice {
				desiredVersion, _ := device["desired_config_version"].(string)
				if desiredVersion != "" && ack.ConfigVersion != desiredVersion {
					return errors.New("stale_config_ack")
				}
				if ack.Accepted {
					device["active_config_version"] = ack.ConfigVersion
				}
				device["last_config_result"] = map[string]any{
					"version": ack.ConfigVersion, "accepted": ack.Accepted, "detail": ack.Detail,
					"acknowledged_at": ack.AcknowledgedAt, "received_at": receivedAt,
				}
				return nil
			}
		}
		return errors.New("unknown_device")
	}); err == nil {
		bridge.notify("devices")
	}
}
