package edge

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
)

// Controller is the reference implementation of what an ESP32 must do on its
// own. It holds the last configuration it accepted, applies manual overrides
// until they expire, and resolves every output through the documented
// precedence order — so essential behaviour continues when the server is gone.
//
// The ESP32 firmware in firmware/esp32 mirrors this logic; keeping an
// executable copy here is what lets the server-loss behaviour be tested in CI
// rather than only on a bench.
type Controller struct {
	mu sync.Mutex

	deviceID  string
	config    deviceprotocol.EdgeSettings
	version   string
	overrides map[string]Override
	// emergency latches until an operator clears it deliberately.
	emergency bool
	// interlocks are hardware-level refusals keyed by channel.
	interlocks map[string]float64
}

// ErrWrongDevice reports configuration addressed to another controller.
var ErrWrongDevice = errors.New("configuration is addressed to another device")

func NewController(deviceID string) *Controller {
	return &Controller{
		deviceID: deviceID, overrides: map[string]Override{}, interlocks: map[string]float64{},
	}
}

// ConfigVersion reports the configuration the controller is actually running,
// which is what it publishes in its health messages.
func (controller *Controller) ConfigVersion() string {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	return controller.version
}

// ApplyConfig adopts a configuration after validating it. A controller that
// rejects a configuration keeps running the last one it accepted rather than
// falling back to nothing.
func (controller *Controller) ApplyConfig(config deviceprotocol.EdgeConfig) error {
	if err := config.Validate(); err != nil {
		return err
	}
	if !strings.EqualFold(config.DeviceID, controller.deviceID) {
		return ErrWrongDevice
	}
	controller.mu.Lock()
	defer controller.mu.Unlock()
	controller.config = config.Config
	controller.version = config.ConfigVersion
	return nil
}

// ApplyCommand records a manual override with an expiry. An override that is
// never renewed lapses on its own, so a server that disappears mid-override
// cannot leave an output latched indefinitely.
func (controller *Controller) ApplyCommand(command deviceprotocol.Command, now time.Time) error {
	if err := command.Validate(now); err != nil {
		return err
	}
	value, numeric := numericValue(command.Value)
	if !numeric {
		return errors.New("command value is not a number or boolean")
	}
	controller.mu.Lock()
	defer controller.mu.Unlock()
	if controller.emergency {
		return errors.New("emergency stop is latched")
	}
	expiry := command.ExpiresAt
	if seconds := controller.config.CommandTimeoutSeconds; seconds > 0 {
		// The controller's own timeout wins when it is stricter, so a server
		// cannot request an override that outlives what the device permits.
		bounded := now.Add(time.Duration(seconds) * time.Second)
		if bounded.Before(expiry) {
			expiry = bounded
		}
	}
	controller.overrides[command.TargetChannelID] = Override{Value: value, ExpiresAt: expiry}
	return nil
}

// LatchEmergency stops all non-essential output and refuses further commands
// until it is cleared deliberately.
func (controller *Controller) LatchEmergency() {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	controller.emergency = true
	// Overrides are dropped rather than kept: resuming from an emergency must
	// not silently restore whatever was running when it was triggered.
	controller.overrides = map[string]Override{}
}

// ClearEmergency releases the latch. It is deliberately explicit, because
// clearing an emergency is an operator decision, never an automatic recovery.
func (controller *Controller) ClearEmergency() {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	controller.emergency = false
}

// SetInterlock records a hardware-level refusal for one channel, which outranks
// every other input.
func (controller *Controller) SetInterlock(channelID string, value float64) {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	controller.interlocks[channelID] = value
}

// ClearInterlock releases a hardware refusal.
func (controller *Controller) ClearInterlock(channelID string) {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	delete(controller.interlocks, channelID)
}

// Resolve computes the output for one channel, reporting which input decided it.
// This is what the controller does on every cycle, with or without a server.
func (controller *Controller) Resolve(channelID string, now time.Time) Resolution {
	controller.mu.Lock()
	defer controller.mu.Unlock()

	policy := Policy{DefaultSafeValue: controller.config.SafeOutputs[channelID]}
	policy.EssentialScheduleValue = policy.DefaultSafeValue

	if value, latched := controller.interlocks[channelID]; latched {
		policy.HardwareInterlock = &value
	}
	if controller.emergency {
		// The emergency value is the channel's safe state, not zero: for the air
		// pump, stopping is the dangerous option.
		emergency := controller.essentialValue(channelID, now, policy.DefaultSafeValue)
		policy.Emergency = &emergency
	}
	if override, present := controller.overrides[channelID]; present {
		if override.ExpiresAt.After(now) {
			policy.Override = &override
		} else {
			delete(controller.overrides, channelID)
		}
	}
	policy.EssentialScheduleValue = controller.essentialValue(channelID, now, policy.DefaultSafeValue)
	return ResolveOutput(policy, now)
}

// essentialValue is what the controller runs from its own persisted schedules,
// with no server involved.
func (controller *Controller) essentialValue(channelID string, now time.Time, fallback float64) float64 {
	if period := controller.config.Photoperiod; period != nil && period.ChannelID == channelID {
		if withinPhotoperiod(*period, now) {
			return 100
		}
		return 0
	}
	if minimum := controller.config.FanMinimumPercent; minimum != nil && *minimum > fallback {
		if _, isSafe := controller.config.SafeOutputs[channelID]; isSafe {
			return *minimum
		}
	}
	if always := controller.config.AirPumpAlwaysOn; always != nil && *always {
		if value, isSafe := controller.config.SafeOutputs[channelID]; isSafe && value > 0 {
			return value
		}
	}
	return fallback
}

// withinPhotoperiod evaluates a daily window on the local clock, handling a
// schedule that crosses midnight.
func withinPhotoperiod(period deviceprotocol.Photoperiod, now time.Time) bool {
	minutes := now.Hour()*60 + now.Minute()
	on := period.OnHour*60 + period.OnMinute
	off := period.OffHour*60 + period.OffMinute
	if on == off {
		return false
	}
	if on < off {
		return minutes >= on && minutes < off
	}
	return minutes >= on || minutes < off
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case bool:
		if typed {
			return 100, true
		}
		return 0, true
	}
	return 0, false
}
