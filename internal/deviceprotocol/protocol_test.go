package deviceprotocol

import (
	"math"
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
	command.ExpiresAt = now.Add(time.Minute)
	command.IssuedAt = now.Add(MaximumFutureClockSkew + time.Second)
	if err := command.Validate(now); err == nil {
		t.Fatal("future-dated command accepted")
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

func TestCommandRejectsStaleAndOverlongWindows(t *testing.T) {
	now := time.Date(2026, 9, 2, 2, 0, 0, 0, time.UTC)
	base := Command{ProtocolVersion: Version, CommandID: deviceID, TargetChannelID: channelID, Type: "set_percent", Value: 50}

	valid := base
	valid.IssuedAt = now.Add(-MaximumCommandLifetime)
	valid.ExpiresAt = now.Add(time.Second)
	if err := valid.Validate(now); err != nil {
		t.Fatalf("boundary command rejected: %v", err)
	}

	overlong := base
	overlong.IssuedAt = now
	overlong.ExpiresAt = now.Add(MaximumCommandLifetime + time.Second)
	if err := overlong.Validate(now); err == nil {
		t.Fatal("command with excessive lifetime was accepted")
	}

	stale := base
	stale.IssuedAt = now.Add(-MaximumCommandLifetime - time.Second)
	stale.ExpiresAt = now.Add(time.Second)
	if err := stale.Validate(now); err == nil {
		t.Fatal("stale command was accepted")
	}

	backwards := base
	backwards.IssuedAt = now.Add(time.Minute)
	backwards.ExpiresAt = now
	if err := backwards.Validate(now); err == nil {
		t.Fatal("command expiring before issuance was accepted")
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

func validEdgeConfig() EdgeConfig {
	minimum := 30.0
	return EdgeConfig{
		ProtocolVersion: Version,
		DeviceID:        deviceID,
		ConfigVersion:   "v2",
		IssuedAt:        time.Now().UTC(),
		Config: EdgeSettings{
			TimezonePOSIX:     "GST-4",
			Photoperiod:       &Photoperiod{OnHour: 6, OffHour: 23, ChannelID: channelID},
			FanMinimumPercent: &minimum,
			SafeOutputs:       map[string]float64{channelID: 0},
			TelemetryIntervalSeconds: 10,
			CommandTimeoutSeconds:    300,
		},
	}
}

func TestEdgeConfigAcceptsExplicitPOSIXTimezoneAndBoundedOutputs(t *testing.T) {
	config := validEdgeConfig()
	if err := config.Validate(); err != nil {
		t.Fatalf("valid edge configuration: %v", err)
	}
}

func TestEdgeConfigRejectsIANAZoneForPOSIXField(t *testing.T) {
	config := validEdgeConfig()
	config.Config.TimezonePOSIX = "Asia/Dubai"
	if err := config.Validate(); err == nil {
		t.Fatal("IANA timezone was accepted as a POSIX TZ rule")
	}
}

func TestEdgeConfigRejectsAmbiguousOrUnsafeScheduleValues(t *testing.T) {
	for name, mutate := range map[string]func(*EdgeConfig){
		"zero schedule window": func(config *EdgeConfig) {
			config.Config.Photoperiod.OnHour = 6
			config.Config.Photoperiod.OffHour = 6
		},
		"safe output over 100": func(config *EdgeConfig) {
			config.Config.SafeOutputs[channelID] = 101
		},
		"safe output NaN": func(config *EdgeConfig) {
			config.Config.SafeOutputs[channelID] = math.NaN()
		},
		"safe output non UUID": func(config *EdgeConfig) {
			config.Config.SafeOutputs = map[string]float64{"fan": 30}
		},
		"telemetry interval too large": func(config *EdgeConfig) {
			config.Config.TelemetryIntervalSeconds = 3601
		},
		"command timeout too large": func(config *EdgeConfig) {
			config.Config.CommandTimeoutSeconds = 86401
		},
	} {
		t.Run(name, func(t *testing.T) {
			config := validEdgeConfig()
			mutate(&config)
			if err := config.Validate(); err == nil {
				t.Fatal("unsafe edge configuration was accepted")
			}
		})
	}
}
