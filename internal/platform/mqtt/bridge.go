package mqtt

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
	"github.com/google/uuid"
	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/farm"
)

type Bridge struct {
	client paho.Client
	store  farm.Store
	logger *slog.Logger
	mu     sync.Mutex
}

func NewBridge(broker, clientID string, store farm.Store, logger *slog.Logger) *Bridge {
	bridge := &Bridge{store: store, logger: logger}
	options := paho.NewClientOptions().AddBroker(broker).SetClientID(clientID).SetAutoReconnect(true).SetConnectRetry(true).SetConnectRetryInterval(5 * time.Second).SetOrderMatters(false)
	options.SetOnConnectHandler(func(client paho.Client) {
		for topic, handler := range map[string]paho.MessageHandler{
			"grownerve/v1/devices/+/telemetry": bridge.handleTelemetry,
			"grownerve/v1/devices/+/acks":      bridge.handleAcknowledgement,
			"grownerve/v1/devices/+/health":    bridge.handleHealth,
		} {
			if token := client.Subscribe(topic, 1, handler); token.Wait() && token.Error() != nil {
				logger.Error("mqtt_subscribe_failed", "topic", topic, "error", token.Error())
			}
		}
		logger.Info("mqtt_connected", "broker", broker)
	})
	options.SetConnectionLostHandler(func(_ paho.Client, err error) { logger.Warn("mqtt_connection_lost", "error", err) })
	bridge.client = paho.NewClient(options)
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

func (bridge *Bridge) PublishCommand(ctx context.Context, deviceID string, command deviceprotocol.Command) error {
	if !bridge.client.IsConnected() {
		return errors.New("MQTT broker is unavailable")
	}
	if err := command.Validate(time.Now().UTC()); err != nil {
		return err
	}
	payload, err := json.Marshal(command)
	if err != nil {
		return err
	}
	token := bridge.client.Publish(fmt.Sprintf("grownerve/v1/devices/%s/commands", deviceID), 1, false, payload)
	for !token.WaitTimeout(100 * time.Millisecond) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return token.Error()
}

type stateDocument struct {
	Devices  []map[string]any `json:"devices"`
	Channels []struct {
		ID       string `json:"id"`
		DeviceID string `json:"device_id"`
		Unit     string `json:"unit"`
	} `json:"channels"`
	Measurements []json.RawMessage `json:"measurements"`
	Commands     []map[string]any  `json:"commands"`
}

func (bridge *Bridge) mutate(mutator func(*stateDocument) error) {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	state, err := bridge.store.Load(context.Background())
	if err != nil {
		bridge.logger.Warn("mqtt_state_load_failed", "error", err)
		return
	}
	var document stateDocument
	if json.Unmarshal(state, &document) != nil {
		bridge.logger.Warn("mqtt_state_invalid")
		return
	}
	if err := mutator(&document); err != nil {
		bridge.logger.Warn("mqtt_message_rejected", "reason", err)
		return
	}
	var object map[string]json.RawMessage
	_ = json.Unmarshal(state, &object)
	object["devices"], _ = json.Marshal(document.Devices)
	object["measurements"], _ = json.Marshal(document.Measurements)
	object["commands"], _ = json.Marshal(document.Commands)
	next, _ := json.Marshal(object)
	if err := bridge.store.Save(context.Background(), next); err != nil {
		bridge.logger.Error("mqtt_state_save_failed", "error", err)
	}
}

func (bridge *Bridge) handleTelemetry(_ paho.Client, message paho.Message) {
	envelope, err := deviceprotocol.ParseTelemetry(message.Payload())
	if err != nil {
		bridge.logger.Warn("mqtt_telemetry_invalid", "error", err)
		return
	}
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
			record, _ := json.Marshal(map[string]any{"id": uuid.NewString(), "channel_id": sample.ChannelID, "observed_at": envelope.ObservedAt, "received_at": time.Now().UTC(), "value": sample.Value, "unit": sample.Unit, "quality": sample.Quality, "sequence": envelope.Sequence, "source_device_id": envelope.DeviceID})
			document.Measurements = append(document.Measurements, record)
		}
		if len(document.Measurements) > 5000 {
			document.Measurements = document.Measurements[len(document.Measurements)-5000:]
		}
		return nil
	})
}

func (bridge *Bridge) handleAcknowledgement(_ paho.Client, message paho.Message) {
	var ack deviceprotocol.Acknowledgement
	if json.Unmarshal(message.Payload(), &ack) != nil || ack.Validate() != nil {
		bridge.logger.Warn("mqtt_ack_invalid")
		return
	}
	bridge.mutate(func(document *stateDocument) error {
		for _, command := range document.Commands {
			if command["id"] == ack.CommandID {
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
		}
		return errors.New("unknown_command")
	})
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
}
