package deviceprotocol

import (
	"testing"
	"time"
)

const deviceID = "01990a20-6a00-7000-8000-000000000001"
const channelID = "01990a20-6a00-7000-8000-000000000002"

func TestParseTelemetry(t *testing.T) {
	payload := []byte(`{"protocolVersion":1,"deviceId":"` + deviceID + `","bootId":"boot-1","sequence":4,"observedAt":"2026-09-01T10:00:00Z","samples":[{"channelId":"` + channelID + `","value":22.4,"unit":"degC","quality":"good"}]}`)
	envelope, err := ParseTelemetry(payload)
	if err != nil {
		t.Fatalf("ParseTelemetry(): %v", err)
	}
	if envelope.DeviceID != deviceID || len(envelope.Samples) != 1 {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestParseTelemetryRejectsInvalidNetworkInput(t *testing.T) {
	tests := [][]byte{
		[]byte(`not-json`),
		[]byte(`{"protocolVersion":2,"deviceId":"` + deviceID + `","bootId":"boot","observedAt":"2026-09-01T10:00:00Z","samples":[{"channelId":"` + channelID + `","value":1,"unit":"degC","quality":"good"}]}`),
		[]byte(`{"protocolVersion":1,"deviceId":"bad","bootId":"boot","observedAt":"2026-09-01T10:00:00Z","samples":[]}`),
		[]byte(`{"protocolVersion":1,"deviceId":"` + deviceID + `","bootId":"boot","observedAt":"2026-09-01T10:00:00Z","samples":[{"channelId":"` + channelID + `","value":1,"unit":"degC","quality":"invented"}]}`),
	}
	for _, payload := range tests {
		if _, err := ParseTelemetry(payload); err == nil {
			t.Fatalf("ParseTelemetry(%s) succeeded", payload)
		}
	}
}

func TestCommandAndAcknowledgementValidation(t *testing.T) {
	now := time.Now().UTC()
	command := Command{ProtocolVersion: 1, CommandID: deviceID, TargetChannelID: channelID, Type: "set_percent", Value: 45, IssuedAt: now, ExpiresAt: now.Add(time.Minute)}
	if err := command.Validate(now); err != nil {
		t.Fatalf("valid command: %v", err)
	}
	command.ExpiresAt = now.Add(-time.Second)
	if err := command.Validate(now); err == nil {
		t.Fatal("expired command accepted")
	}
	ack := Acknowledgement{ProtocolVersion: 1, CommandID: deviceID, DeviceID: channelID, Result: "applied", AcknowledgedAt: now}
	if err := ack.Validate(); err != nil {
		t.Fatalf("valid ack: %v", err)
	}
	ack.Result = "unknown"
	if err := ack.Validate(); err == nil {
		t.Fatal("unknown ack result accepted")
	}
}

func TestProtocolValidationBoundaries(t *testing.T) {
	now := time.Now().UTC()
	commands := []Command{
		{ProtocolVersion: 2, CommandID: deviceID, TargetChannelID: channelID, Type: "set_percent", IssuedAt: now, ExpiresAt: now.Add(time.Minute)},
		{ProtocolVersion: 1, CommandID: "bad", TargetChannelID: channelID, Type: "set_percent", IssuedAt: now, ExpiresAt: now.Add(time.Minute)},
		{ProtocolVersion: 1, CommandID: deviceID, TargetChannelID: "bad", Type: "set_percent", IssuedAt: now, ExpiresAt: now.Add(time.Minute)},
		{ProtocolVersion: 1, CommandID: deviceID, TargetChannelID: channelID, Type: "dose_forever", IssuedAt: now, ExpiresAt: now.Add(time.Minute)},
	}
	for _, command := range commands {
		if err := command.Validate(now); err == nil {
			t.Fatalf("Validate(%+v) succeeded", command)
		}
	}
	acks := []Acknowledgement{
		{ProtocolVersion: 2, CommandID: deviceID, DeviceID: channelID, Result: "applied", AcknowledgedAt: now},
		{ProtocolVersion: 1, CommandID: "bad", DeviceID: channelID, Result: "applied", AcknowledgedAt: now},
		{ProtocolVersion: 1, CommandID: deviceID, DeviceID: "bad", Result: "applied", AcknowledgedAt: now},
		{ProtocolVersion: 1, CommandID: deviceID, DeviceID: channelID, Result: "applied"},
	}
	for _, ack := range acks {
		if err := ack.Validate(); err == nil {
			t.Fatalf("Validate(%+v) succeeded", ack)
		}
	}
}
