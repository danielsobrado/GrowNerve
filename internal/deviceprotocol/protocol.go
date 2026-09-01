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
	if command.IssuedAt.After(now.Add(time.Minute)) {
		return errors.New("command issuedAt is too far in the future")
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

type EdgeConfig struct {
	ProtocolVersion int          `json:"protocolVersion"`
	DeviceID        string       `json:"deviceId"`
	ConfigVersion   string       `json:"configVersion"`
	IssuedAt        time.Time    `json:"issuedAt"`
	Config          EdgeSettings `json:"config"`
}

type EdgeSettings struct {
	// TimezonePOSIX is persisted on the controller and applied through tzset.
	// It is a POSIX TZ rule such as UTC0, GST-4, or PHT-8; IANA names such as
	// Asia/Dubai are deliberately rejected because the ESP32 C runtime does not
	// interpret them as zoneinfo identifiers.
	TimezonePOSIX            string             `json:"timezonePosix,omitempty"`
	Photoperiod              *Photoperiod       `json:"photoperiod,omitempty"`
	FanMinimumPercent        *float64           `json:"fanMinimumPercent,omitempty"`
	FanSchedule              *FanSchedule       `json:"fanSchedule,omitempty"`
	AirPumpAlwaysOn          *bool              `json:"airPumpAlwaysOn,omitempty"`
	SafeOutputs              map[string]float64 `json:"safeOutputs,omitempty"`
	TelemetryIntervalSeconds int                `json:"telemetryIntervalSeconds,omitempty"`
	CommandTimeoutSeconds    int                `json:"commandTimeoutSeconds,omitempty"`
}

type Photoperiod struct {
	OnHour    int    `json:"onHour"`
	OnMinute  int    `json:"onMinute"`
	OffHour   int    `json:"offHour"`
	OffMinute int    `json:"offMinute"`
	ChannelID string `json:"channelId"`
}

// FanSchedule expresses a daily active window while preserving the configured
// fan floor. Percentages are output targets, not raw PWM values.
type FanSchedule struct {
	ChannelID       string  `json:"channelId"`
	OnHour          int     `json:"onHour"`
	OnMinute        int     `json:"onMinute"`
	OffHour         int     `json:"offHour"`
	OffMinute       int     `json:"offMinute"`
	ActivePercent   float64 `json:"activePercent"`
	InactivePercent float64 `json:"inactivePercent"`
}

func validateWindow(onHour, onMinute, offHour, offMinute int) error {
	for _, value := range []int{onHour, offHour} {
		if value < 0 || value > 23 {
			return errors.New("schedule hours must be between 0 and 23")
		}
	}
	for _, value := range []int{onMinute, offMinute} {
		if value < 0 || value > 59 {
			return errors.New("schedule minutes must be between 0 and 59")
		}
	}
	if onHour == offHour && onMinute == offMinute {
		return errors.New("schedule start and end must differ")
	}
	return nil
}

func validatePOSIXTimezone(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("timezonePosix is required for wall-clock schedules")
	}
	if len(value) > 63 {
		return errors.New("timezonePosix is too long")
	}
	if strings.ContainsAny(value, "/\r\n\t ") {
		return errors.New("timezonePosix must be a POSIX TZ rule, not an IANA timezone name")
	}
	if !strings.ContainsAny(value, "0123456789") {
		return errors.New("timezonePosix must include an explicit UTC offset")
	}
	return nil
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
	if config.Config.TimezonePOSIX != "" {
		if err := validatePOSIXTimezone(config.Config.TimezonePOSIX); err != nil {
			return err
		}
	}
	if (config.Config.Photoperiod != nil || config.Config.FanSchedule != nil) && strings.TrimSpace(config.Config.TimezonePOSIX) == "" {
		return errors.New("timezonePosix is required for wall-clock schedules")
	}
	if period := config.Config.Photoperiod; period != nil {
		if _, err := uuid.Parse(period.ChannelID); err != nil {
			return errors.New("photoperiod channelId must be a UUID")
		}
		if err := validateWindow(period.OnHour, period.OnMinute, period.OffHour, period.OffMinute); err != nil {
			return fmt.Errorf("photoperiod: %w", err)
		}
	}
	if minimum := config.Config.FanMinimumPercent; minimum != nil && (*minimum < 0 || *minimum > 100 || math.IsNaN(*minimum) || math.IsInf(*minimum, 0)) {
		return errors.New("fanMinimumPercent must be between 0 and 100")
	}
	if schedule := config.Config.FanSchedule; schedule != nil {
		if _, err := uuid.Parse(schedule.ChannelID); err != nil {
			return errors.New("fanSchedule channelId must be a UUID")
		}
		if err := validateWindow(schedule.OnHour, schedule.OnMinute, schedule.OffHour, schedule.OffMinute); err != nil {
			return fmt.Errorf("fanSchedule: %w", err)
		}
		if schedule.ActivePercent < 0 || schedule.ActivePercent > 100 || schedule.InactivePercent < 0 || schedule.InactivePercent > 100 ||
			math.IsNaN(schedule.ActivePercent) || math.IsNaN(schedule.InactivePercent) || math.IsInf(schedule.ActivePercent, 0) || math.IsInf(schedule.InactivePercent, 0) {
			return errors.New("fanSchedule percentages must be between 0 and 100")
		}
	}
	for channelID, value := range config.Config.SafeOutputs {
		if _, err := uuid.Parse(channelID); err != nil {
			return errors.New("safeOutputs keys must be channel UUIDs")
		}
		if value < 0 || value > 100 || math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("safeOutputs values must be between 0 and 100")
		}
	}
	if config.Config.TelemetryIntervalSeconds < 0 || config.Config.TelemetryIntervalSeconds > 3600 {
		return errors.New("telemetryIntervalSeconds must be between 0 and 3600")
	}
	if config.Config.CommandTimeoutSeconds < 0 || config.Config.CommandTimeoutSeconds > 86400 {
		return errors.New("commandTimeoutSeconds must be between 0 and 86400")
	}
	return nil
}

type ConfigAcknowledgement struct {
	ProtocolVersion int       `json:"protocolVersion"`
	DeviceID        string    `json:"deviceId"`
	ConfigVersion   string    `json:"configVersion"`
	Accepted        bool      `json:"accepted"`
	Detail          string    `json:"detail,omitempty"`
	AcknowledgedAt  time.Time `json:"acknowledgedAt"`
}

func (ack ConfigAcknowledgement) Validate() error {
	if ack.ProtocolVersion != Version {
		return errors.New("unsupported protocol version")
	}
	if _, err := uuid.Parse(ack.DeviceID); err != nil {
		return errors.New("deviceId must be a UUID")
	}
	if strings.TrimSpace(ack.ConfigVersion) == "" || ack.AcknowledgedAt.IsZero() {
		return errors.New("configVersion and acknowledgedAt are required")
	}
	return nil
}
