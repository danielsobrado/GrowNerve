package deviceprotocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"
)

const Version = 1

type Quality string

const (
	QualityGood        Quality = "good"
	QualitySuspect     Quality = "suspect"
	QualityStale       Quality = "stale"
	QualityCalibrating Quality = "calibrating"
	QualityFault       Quality = "fault"
	QualityUnknown     Quality = "unknown"
)

type Sample struct {
	ChannelID string  `json:"channelId"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
	Quality   Quality `json:"quality"`
}
type TelemetryEnvelope struct {
	ProtocolVersion int       `json:"protocolVersion"`
	DeviceID        string    `json:"deviceId"`
	BootID          string    `json:"bootId"`
	Sequence        uint64    `json:"sequence"`
	ObservedAt      time.Time `json:"observedAt"`
	Samples         []Sample  `json:"samples"`
}

func ParseTelemetry(payload []byte) (TelemetryEnvelope, error) {
	var envelope TelemetryEnvelope
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return envelope, fmt.Errorf("decode telemetry: %w", err)
	}
	if envelope.ProtocolVersion != Version {
		return envelope, fmt.Errorf("unsupported protocol version %d", envelope.ProtocolVersion)
	}
	if _, err := uuid.Parse(envelope.DeviceID); err != nil {
		return envelope, errors.New("deviceId must be a UUID")
	}
	if envelope.BootID == "" || envelope.ObservedAt.IsZero() || len(envelope.Samples) == 0 {
		return envelope, errors.New("bootId, observedAt, and samples are required")
	}
	qualities := []Quality{QualityGood, QualitySuspect, QualityStale, QualityCalibrating, QualityFault, QualityUnknown}
	for _, sample := range envelope.Samples {
		if _, err := uuid.Parse(sample.ChannelID); err != nil {
			return envelope, errors.New("sample channelId must be a UUID")
		}
		if sample.Unit == "" || math.IsNaN(sample.Value) || math.IsInf(sample.Value, 0) {
			return envelope, errors.New("sample value and unit are invalid")
		}
		if !slices.Contains(qualities, sample.Quality) {
			return envelope, fmt.Errorf("invalid sample quality %q", sample.Quality)
		}
	}
	return envelope, nil
}

type Command struct {
	ProtocolVersion int       `json:"protocolVersion"`
	CommandID       string    `json:"commandId"`
	TargetChannelID string    `json:"targetChannelId"`
	Type            string    `json:"type"`
	Value           any       `json:"value"`
	IssuedAt        time.Time `json:"issuedAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

func (command Command) Validate(now time.Time) error {
	if command.ProtocolVersion != Version {
		return errors.New("unsupported protocol version")
	}
	if _, err := uuid.Parse(command.CommandID); err != nil {
		return errors.New("commandId must be a UUID")
	}
	if _, err := uuid.Parse(command.TargetChannelID); err != nil {
		return errors.New("targetChannelId must be a UUID")
	}
	if command.Type != "set_percent" && command.Type != "set_boolean" {
		return errors.New("unsupported command type")
	}
	if command.IssuedAt.IsZero() || !command.ExpiresAt.After(now) {
		return errors.New("command is expired or missing time")
	}
	return nil
}

type Acknowledgement struct {
	ProtocolVersion int       `json:"protocolVersion"`
	CommandID       string    `json:"commandId"`
	DeviceID        string    `json:"deviceId"`
	Result          string    `json:"result"`
	ReasonCode      string    `json:"reasonCode,omitempty"`
	AppliedValue    any       `json:"appliedValue,omitempty"`
	AcknowledgedAt  time.Time `json:"acknowledgedAt"`
}

func (ack Acknowledgement) Validate() error {
	if ack.ProtocolVersion != Version {
		return errors.New("unsupported protocol version")
	}
	if _, err := uuid.Parse(ack.CommandID); err != nil {
		return errors.New("commandId must be a UUID")
	}
	if _, err := uuid.Parse(ack.DeviceID); err != nil {
		return errors.New("deviceId must be a UUID")
	}
	if !slices.Contains([]string{"accepted", "applied", "rejected", "failed"}, ack.Result) {
		return errors.New("invalid acknowledgement result")
	}
	if ack.AcknowledgedAt.IsZero() {
		return errors.New("acknowledgedAt is required")
	}
	return nil
}

type Health struct {
	ProtocolVersion     int       `json:"protocolVersion"`
	DeviceID            string    `json:"deviceId"`
	FirmwareVersion     string    `json:"firmwareVersion"`
	BootID              string    `json:"bootId"`
	UptimeSeconds       uint64    `json:"uptimeSeconds"`
	RSSI                int       `json:"rssi"`
	FreeHeap            uint64    `json:"freeHeap"`
	ActiveConfigVersion string    `json:"activeConfigVersion"`
	ObservedAt          time.Time `json:"observedAt"`
	SensorFaults        []string  `json:"sensorFaults"`
}
