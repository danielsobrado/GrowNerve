// Command device-simulator stands in for an ESP32 controller. It speaks the
// real protocol and runs the same precedence logic as the firmware, so the
// server's telemetry, command, and configuration paths can be exercised without
// hardware. It is a development aid: simulator success is never commissioning
// evidence.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/edge"
)

const defaultDeviceID = "01990a20-6a00-7000-8000-000000000020"

var defaultChannels = []string{
	"01990a20-6a00-7000-8000-000000000031",
	"01990a20-6a00-7000-8000-000000000032",
	"01990a20-6a00-7000-8000-000000000033",
	"01990a20-6a00-7000-8000-000000000034",
}

type simulatedCommandResult struct {
	result       string
	reason       string
	appliedValue any
}

type simulatedCommandReplay struct {
	mu      sync.Mutex
	results map[string]simulatedCommandResult
}

func newSimulatedCommandReplay() *simulatedCommandReplay {
	return &simulatedCommandReplay{results: map[string]simulatedCommandResult{}}
}

func (replay *simulatedCommandReplay) apply(controller *edge.Controller, command deviceprotocol.Command, now time.Time) simulatedCommandResult {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if result, found := replay.results[command.CommandID]; found {
		return result
	}
	result := simulatedCommandResult{result: "applied", appliedValue: command.Value}
	if err := controller.ApplyCommand(command, now); err != nil {
		result.result = "rejected"
		result.reason = rejectionCode(err)
		result.appliedValue = nil
	}
	replay.results[command.CommandID] = result
	return result
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	broker := flag.String("broker", "tcp://127.0.0.1:1883", "MQTT broker URL")
	deviceID := flag.String("device-id", defaultDeviceID, "controller UUID")
	interval := flag.Duration("interval", 10*time.Second, "telemetry interval")
	username := flag.String("username", os.Getenv("GROWNERVE_MQTT_USERNAME"), "broker username")
	password := flag.String("password", os.Getenv("GROWNERVE_MQTT_PASSWORD"), "broker password")
	flag.Parse()
	if _, err := uuid.Parse(*deviceID); err != nil {
		return fmt.Errorf("device-id: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	bootID := uuid.NewString()
	controller := edge.NewController(*deviceID)
	replay := newSimulatedCommandReplay()

	options := paho.NewClientOptions().AddBroker(*broker).
		SetClientID("grownerve-simulator-" + (*deviceID)[len(*deviceID)-8:]).SetAutoReconnect(true)
	if *username != "" {
		options.SetUsername(*username).SetPassword(*password)
	}
	options.SetOnConnectHandler(func(client paho.Client) {
		subscribeCommands(client, controller, replay, *deviceID)
		subscribeConfig(client, controller, *deviceID)
	})

	client := paho.NewClient(options)
	token := client.Connect()
	token.Wait()
	if token.Error() != nil {
		return token.Error()
	}
	defer client.Disconnect(250)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	publisher := &telemetryPublisher{client: client, deviceID: *deviceID, bootID: bootID, controller: controller, started: time.Now()}
	publisher.publish()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			publisher.publish()
		}
	}
}

func subscribeCommands(client paho.Client, controller *edge.Controller, replay *simulatedCommandReplay, deviceID string) {
	topic := fmt.Sprintf("grownerve/v1/devices/%s/commands", deviceID)
	client.Subscribe(topic, 1, func(_ paho.Client, message paho.Message) {
		var command deviceprotocol.Command
		if json.Unmarshal(message.Payload(), &command) != nil {
			return
		}
		now := time.Now().UTC()
		result := replay.apply(controller, command, now)
		ack := deviceprotocol.Acknowledgement{
			ProtocolVersion: deviceprotocol.Version, CommandID: command.CommandID, DeviceID: deviceID,
			Result: result.result, ReasonCode: result.reason, AppliedValue: result.appliedValue, AcknowledgedAt: now,
		}
		payload, _ := json.Marshal(ack)
		client.Publish(fmt.Sprintf("grownerve/v1/devices/%s/acks", deviceID), 1, false, payload)
	})
}

func rejectionCode(err error) string {
	if strings.Contains(err.Error(), "emergency") {
		return "EMERGENCY_STOP_ACTIVE"
	}
	return "INVALID_OR_EXPIRED_COMMAND"
}

// subscribeConfig adopts retained configuration and reports the result, which is
// the only signal the server has that a schedule change actually landed.
func subscribeConfig(client paho.Client, controller *edge.Controller, deviceID string) {
	topic := fmt.Sprintf("grownerve/v1/devices/%s/config", deviceID)
	client.Subscribe(topic, 1, func(_ paho.Client, message paho.Message) {
		var config deviceprotocol.EdgeConfig
		accepted, detail := true, ""
		if err := json.Unmarshal(message.Payload(), &config); err != nil {
			accepted, detail = false, "unreadable configuration"
		} else if err := controller.ApplyConfig(config); err != nil {
			accepted, detail = false, err.Error()
		}
		ack := deviceprotocol.ConfigAcknowledgement{
			ProtocolVersion: deviceprotocol.Version, DeviceID: deviceID,
			ConfigVersion: config.ConfigVersion, Accepted: accepted, Detail: detail,
			AcknowledgedAt: time.Now().UTC(),
		}
		payload, _ := json.Marshal(ack)
		client.Publish(topic+"/ack", 1, false, payload)
	})
}

type telemetryPublisher struct {
	client     paho.Client
	deviceID   string
	bootID     string
	controller *edge.Controller
	started    time.Time
	sequence   uint64
}

func (publisher *telemetryPublisher) publish() {
	publisher.sequence++
	now := time.Now().UTC()
	wave := math.Sin(float64(publisher.sequence) / 8)

	envelope := deviceprotocol.TelemetryEnvelope{
		ProtocolVersion: deviceprotocol.Version, DeviceID: publisher.deviceID, BootID: publisher.bootID,
		Sequence: publisher.sequence, ObservedAt: now,
		Samples: []deviceprotocol.Sample{
			{ChannelID: defaultChannels[0], Value: 22.2 + wave, Unit: "degC", Quality: deviceprotocol.QualityGood},
			{ChannelID: defaultChannels[1], Value: 67 + wave*3, Unit: "%RH", Quality: deviceprotocol.QualityGood},
			{ChannelID: defaultChannels[2], Value: 20.5 + wave*.5, Unit: "degC", Quality: deviceprotocol.QualityGood},
			{ChannelID: defaultChannels[3], Value: 72 - float64(publisher.sequence%20)*.05, Unit: "%", Quality: deviceprotocol.QualityGood},
		},
	}
	payload, _ := json.Marshal(envelope)
	publisher.client.Publish(fmt.Sprintf("grownerve/v1/devices/%s/telemetry", publisher.deviceID), 1, false, payload)

	// The reported configuration version is whatever the controller is actually
	// running, so a rejected configuration is visible to the server as a device
	// that never adopted it.
	health := deviceprotocol.Health{
		ProtocolVersion: deviceprotocol.Version, DeviceID: publisher.deviceID, FirmwareVersion: "0.1.0-sim",
		BootID: publisher.bootID, UptimeSeconds: uint64(time.Since(publisher.started).Seconds()),
		RSSI: -48, FreeHeap: 180000, ActiveConfigVersion: publisher.controller.ConfigVersion(),
		ObservedAt: now, SensorFaults: []string{},
	}
	healthPayload, _ := json.Marshal(health)
	publisher.client.Publish(fmt.Sprintf("grownerve/v1/devices/%s/health", publisher.deviceID), 1, false, healthPayload)
}
