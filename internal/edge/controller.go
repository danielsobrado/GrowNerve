package edge

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
)

type Controller struct {
	mu sync.Mutex

	deviceID   string
	config     deviceprotocol.EdgeSettings
	version    string
	overrides  map[string]Override
	emergency  bool
	interlocks map[string]float64
}

var ErrWrongDevice = errors.New("configuration is addressed to another device")

func NewController(deviceID string) *Controller {
	return &Controller{
		deviceID: deviceID, overrides: map[string]Override{}, interlocks: map[string]float64{},
	}
}

func (controller *Controller) ConfigVersion() string {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	return controller.version
}

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
		bounded := now.Add(time.Duration(seconds) * time.Second)
		if bounded.Before(expiry) {
			expiry = bounded
		}
	}
	controller.overrides[command.TargetChannelID] = Override{Value: value, ExpiresAt: expiry}
	return nil
}

func (controller *Controller) LatchEmergency() {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	controller.emergency = true
	controller.overrides = map[string]Override{}
}

func (controller *Controller) ClearEmergency() {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	controller.emergency = false
}

func (controller *Controller) SetInterlock(channelID string, value float64) {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	controller.interlocks[channelID] = value
}

func (controller *Controller) ClearInterlock(channelID string) {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	delete(controller.interlocks, channelID)
}

func (controller *Controller) Resolve(channelID string, now time.Time) Resolution {
	controller.mu.Lock()
	defer controller.mu.Unlock()

	policy := Policy{DefaultSafeValue: controller.config.SafeOutputs[channelID]}
	policy.EssentialScheduleValue = policy.DefaultSafeValue

	if value, latched := controller.interlocks[channelID]; latched {
		policy.HardwareInterlock = &value
	}
	if controller.emergency {
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

func (controller *Controller) essentialValue(channelID string, now time.Time, fallback float64) float64 {
	if period := controller.config.Photoperiod; period != nil && period.ChannelID == channelID {
		if withinWindow(period.OnHour, period.OnMinute, period.OffHour, period.OffMinute, now) {
			return 100
		}
		return 0
	}
	if schedule := controller.config.FanSchedule; schedule != nil && schedule.ChannelID == channelID {
		value := schedule.InactivePercent
		if withinWindow(schedule.OnHour, schedule.OnMinute, schedule.OffHour, schedule.OffMinute, now) {
			value = schedule.ActivePercent
		}
		if minimum := controller.config.FanMinimumPercent; minimum != nil && value < *minimum {
			value = *minimum
		}
		if value < fallback {
			value = fallback
		}
		return value
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

func withinPhotoperiod(period deviceprotocol.Photoperiod, now time.Time) bool {
	return withinWindow(period.OnHour, period.OnMinute, period.OffHour, period.OffMinute, now)
}

func withinWindow(onHour, onMinute, offHour, offMinute int, now time.Time) bool {
	minutes := now.Hour()*60 + now.Minute()
	on := onHour*60 + onMinute
	off := offHour*60 + offMinute
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
