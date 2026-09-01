package deviceprotocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
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

// EdgeConfig is the configuration a controller persists and keeps applying when
// the server is unreachable. It is delivered retained, so a controller that
// reboots during an outage recovers its schedules from the broker.
type EdgeConfig struct {
	ProtocolVersion int          `json:"protocolVersion"`
	DeviceID        string       `json:"deviceId"`
	ConfigVersion   string       `json:"configVersion"`
	IssuedAt        time.Time    `json:"issuedAt"`
	Config          EdgeSettings `json:"config"`
}

// EdgeSettings are the essential behaviours a controller must be able to run
// alone. Anything that needs server judgement deliberately does not appear here.
type EdgeSettings struct {
	// Photoperiod drives the light on a local clock.
	Photoperiod *Photoperiod `json:"photoperiod,omitempty"`
	// FanMinimumPercent is the floor the circulation fan never drops below.
	FanMinimumPercent *float64 `json:"fanMinimumPercent,omitempty"`
	// AirPumpAlwaysOn keeps reservoir aeration running, which is the one output
	// whose failure kills a deep-water crop fastest.
	AirPumpAlwaysOn *bool `json:"airPumpAlwaysOn,omitempty"`
	// SafeOutputs are the values every output falls back to when nothing else
	// applies, keyed by channel id.
	SafeOutputs map[string]float64 `json:"safeOutputs,omitempty"`
	// TelemetryIntervalSeconds paces local sampling.
	TelemetryIntervalSeconds int `json:"telemetryIntervalSeconds,omitempty"`
	// CommandTimeoutSeconds bounds how long a manual override survives without
	// being renewed, so a lost server cannot leave an output latched forever.
	CommandTimeoutSeconds int `json:"commandTimeoutSeconds,omitempty"`
}

// Photoperiod is a daily light schedule on the controller's local clock. OffHour
// may be less than OnHour, which expresses a schedule that crosses midnight.
type Photoperiod struct {
	OnHour    int    `json:"onHour"`
	OnMinute  int    `json:"onMinute"`
	OffHour   int    `json:"offHour"`
	OffMinute int    `json:"offMinute"`
	ChannelID string `json:"channelId"`
}

func (config EdgeConfig) Validate() error {
	if config.ProtocolVersion != Version {
		return errors.New("unsupported protocol version")
	}
	if _, err := uuid.Parse(config.DeviceID); err != nil {
		return errors.New("deviceId must be a UUID")
	}
	if strings.TrimSpace(config.ConfigVersion) == "" {
		return errors.New("configVersion is required")
	}
	if config.IssuedAt.IsZero() {
		return errors.New("issuedAt is required")
	}
	if period := config.Config.Photoperiod; period != nil {
		if _, err := uuid.Parse(period.ChannelID); err != nil {
			return errors.New("photoperiod channelId must be a UUID")
		}
		for _, value := range []int{period.OnHour, period.OffHour} {
			if value < 0 || value > 23 {
				return errors.New("photoperiod hours must be between 0 and 23")
			}
		}
		for _, value := range []int{period.OnMinute, period.OffMinute} {
			if value < 0 || value > 59 {
				return errors.New("photoperiod minutes must be between 0 and 59")
			}
		}
	}
	if minimum := config.Config.FanMinimumPercent; minimum != nil && (*minimum < 0 || *minimum > 100) {
		return errors.New("fanMinimumPercent must be between 0 and 100")
	}
	return nil
}

// ConfigAcknowledgement reports whether a controller adopted a configuration. It
// is the only trustworthy evidence that a schedule change reached the hardware.
type ConfigAcknowledgement struct {
	ProtocolVersion int       `json:"protocolVersion"`
	DeviceID        string    `json:"deviceId"`
	ConfigVersion   string    `json:"configVersion"`
	Accepted        bool      `json:"accepted"`
	Detail          string    `json:"detail,omitempty"`
	AcknowledgedAt  time.Time `json:"acknowledgedAt"`
}
