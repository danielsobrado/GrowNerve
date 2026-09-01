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
)

const defaultDeviceID = "01990a20-6a00-7000-8000-000000000020"

var defaultChannels = []string{"01990a20-6a00-7000-8000-000000000031", "01990a20-6a00-7000-8000-000000000032", "01990a20-6a00-7000-8000-000000000033", "01990a20-6a00-7000-8000-000000000034"}

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
	flag.Parse()
	if _, err := uuid.Parse(*deviceID); err != nil {
		return fmt.Errorf("device-id: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	bootID := uuid.NewString()
	var applied sync.Map
	options := paho.NewClientOptions().AddBroker(*broker).SetClientID("grownerve-simulator-" + (*deviceID)[len(*deviceID)-8:]).SetAutoReconnect(true)
	options.SetOnConnectHandler(func(client paho.Client) {
		client.Subscribe("grownerve/v1/devices/+/commands", 1, func(_ paho.Client, message paho.Message) {
			var command deviceprotocol.Command
			if json.Unmarshal(message.Payload(), &command) != nil {
				return
			}
			parts := strings.Split(message.Topic(), "/")
			targetDeviceID := parts[len(parts)-2]
			result, reason := "applied", ""
			if err := command.Validate(time.Now().UTC()); err != nil {
				result, reason = "rejected", "INVALID_OR_EXPIRED_COMMAND"
			}
			if _, loaded := applied.LoadOrStore(command.CommandID, true); loaded {
				result = "applied"
				reason = "DUPLICATE_ALREADY_APPLIED"
			}
			if value, ok := command.Value.(float64); ok && (value < 0 || value > 100) {
				result, reason = "rejected", "VALUE_OUT_OF_RANGE"
			}
			ack := deviceprotocol.Acknowledgement{ProtocolVersion: 1, CommandID: command.CommandID, DeviceID: targetDeviceID, Result: result, ReasonCode: reason, AppliedValue: command.Value, AcknowledgedAt: time.Now().UTC()}
			payload, _ := json.Marshal(ack)
			client.Publish(fmt.Sprintf("grownerve/v1/devices/%s/acks", targetDeviceID), 1, false, payload)
		})
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
	sequence := uint64(0)
	started := time.Now()
	publish := func() {
		sequence++
		now := time.Now().UTC()
		wave := math.Sin(float64(sequence) / 8)
		envelope := deviceprotocol.TelemetryEnvelope{ProtocolVersion: 1, DeviceID: *deviceID, BootID: bootID, Sequence: sequence, ObservedAt: now, Samples: []deviceprotocol.Sample{
			{ChannelID: defaultChannels[0], Value: 22.2 + wave, Unit: "degC", Quality: deviceprotocol.QualityGood},
			{ChannelID: defaultChannels[1], Value: 67 + wave*3, Unit: "%RH", Quality: deviceprotocol.QualityGood},
			{ChannelID: defaultChannels[2], Value: 20.5 + wave*.5, Unit: "degC", Quality: deviceprotocol.QualityGood},
			{ChannelID: defaultChannels[3], Value: 72 - float64(sequence%20)*.05, Unit: "%", Quality: deviceprotocol.QualityGood},
		}}
		payload, _ := json.Marshal(envelope)
		client.Publish(fmt.Sprintf("grownerve/v1/devices/%s/telemetry", *deviceID), 1, false, payload)
		health := deviceprotocol.Health{ProtocolVersion: 1, DeviceID: *deviceID, FirmwareVersion: "0.1.0-sim", BootID: bootID, UptimeSeconds: uint64(time.Since(started).Seconds()), RSSI: -48, FreeHeap: 180000, ActiveConfigVersion: "pilot-v1", ObservedAt: now, SensorFaults: []string{}}
		healthPayload, _ := json.Marshal(health)
		client.Publish(fmt.Sprintf("grownerve/v1/devices/%s/health", *deviceID), 1, false, healthPayload)
	}
	publish()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			publish()
		}
	}
}
