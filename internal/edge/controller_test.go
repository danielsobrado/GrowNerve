package edge

import (
	"testing"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
)

const (
	testDevice     = "01990a20-6a00-7000-8000-000000000020"
	lightChannel   = "01990a20-6a00-7000-8000-000000000041"
	fanChannel     = "01990a20-6a00-7000-8000-000000000042"
	airPumpChannel = "01990a20-6a00-7000-8000-000000000043"
)

func value(v float64) *float64 { return &v }
func flag(v bool) *bool        { return &v }

func pilotConfig() deviceprotocol.EdgeConfig {
	return deviceprotocol.EdgeConfig{
		ProtocolVersion: deviceprotocol.Version, DeviceID: testDevice,
		ConfigVersion: "pilot-v1", IssuedAt: time.Now().UTC(),
		Config: deviceprotocol.EdgeSettings{
			Photoperiod:       &deviceprotocol.Photoperiod{OnHour: 6, OffHour: 24 % 24, OnMinute: 0, OffMinute: 0, ChannelID: lightChannel},
			FanMinimumPercent: value(30),
			AirPumpAlwaysOn:   flag(true),
			SafeOutputs: map[string]float64{
				lightChannel: 0, fanChannel: 30, airPumpChannel: 100,
			},
			CommandTimeoutSeconds: 300,
		},
	}
}

func configuredController(t *testing.T) *Controller {
	t.Helper()
	controller := NewController(testDevice)
	if err := controller.ApplyConfig(pilotConfig()); err != nil {
		t.Fatal(err)
	}
	return controller
}

func at(hour, minute int) time.Time {
	return time.Date(2026, 6, 15, hour, minute, 0, 0, time.UTC)
}

// TestEssentialOperationContinuesWithoutAServer is the software half of the
// phase 8 exit criterion: once configured, the controller keeps running its
// schedules with no server involved at all.
func TestEssentialOperationContinuesWithoutAServer(t *testing.T) {
	controller := configuredController(t)

	// The photoperiod runs 06:00 to 24:00 on the controller's own clock.
	if resolution := controller.Resolve(lightChannel, at(9, 0)); resolution.Value != 100 || resolution.Source != SourceEssentialSchedule {
		t.Fatalf("light during the photoperiod = %+v", resolution)
	}
	if resolution := controller.Resolve(lightChannel, at(3, 0)); resolution.Value != 0 {
		t.Fatalf("light outside the photoperiod = %+v", resolution)
	}

	// Aeration is the output whose failure kills a deep-water crop fastest, so
	// it must hold without any server contact.
	if resolution := controller.Resolve(airPumpChannel, at(3, 0)); resolution.Value != 100 {
		t.Fatalf("air pump stopped with no server: %+v", resolution)
	}

	// The fan never drops below its configured floor.
	if resolution := controller.Resolve(fanChannel, at(3, 0)); resolution.Value < 30 {
		t.Fatalf("fan fell below its minimum: %+v", resolution)
	}
}

// TestRebootRecoversFromRetainedConfiguration models a controller restarting
// during an outage: the broker's retained configuration is all it has.
func TestRebootRecoversFromRetainedConfiguration(t *testing.T) {
	rebooted := NewController(testDevice)
	// Before configuration, every output sits at its safe default rather than
	// at whatever it was last commanded to.
	if resolution := rebooted.Resolve(lightChannel, at(9, 0)); resolution.Source != SourceDefaultSafe {
		t.Fatalf("an unconfigured controller did not start from safe defaults: %+v", resolution)
	}
	if err := rebooted.ApplyConfig(pilotConfig()); err != nil {
		t.Fatal(err)
	}
	if resolution := rebooted.Resolve(airPumpChannel, at(9, 0)); resolution.Value != 100 {
		t.Fatalf("aeration did not resume after reboot: %+v", resolution)
	}
	if rebooted.ConfigVersion() != "pilot-v1" {
		t.Fatalf("config version = %q", rebooted.ConfigVersion())
	}
}

func TestOverrideExpiresWithoutTheServerRenewingIt(t *testing.T) {
	controller := configuredController(t)
	now := at(3, 0)
	command := deviceprotocol.Command{
		ProtocolVersion: deviceprotocol.Version, CommandID: "01990a20-6a00-7000-8000-000000000051",
		TargetChannelID: lightChannel, Type: "set_percent", Value: float64(80),
		IssuedAt: now, ExpiresAt: now.Add(2 * time.Minute),
	}
	if err := controller.ApplyCommand(command, now); err != nil {
		t.Fatal(err)
	}
	if resolution := controller.Resolve(lightChannel, now.Add(time.Minute)); resolution.Value != 80 || resolution.Source != SourceOverride {
		t.Fatalf("override was not applied: %+v", resolution)
	}
	// The server vanishes. The override must lapse on its own rather than
	// latching the output indefinitely.
	if resolution := controller.Resolve(lightChannel, now.Add(5*time.Minute)); resolution.Source == SourceOverride {
		t.Fatalf("an unrenewed override outlived its expiry: %+v", resolution)
	}
}

func TestControllerTimeoutOutranksAGenerousServerExpiry(t *testing.T) {
	controller := configuredController(t)
	now := at(3, 0)
	command := deviceprotocol.Command{
		ProtocolVersion: deviceprotocol.Version, CommandID: "01990a20-6a00-7000-8000-000000000052",
		TargetChannelID: fanChannel, Type: "set_percent", Value: float64(90),
		IssuedAt: now, ExpiresAt: now.Add(24 * time.Hour),
	}
	if err := controller.ApplyCommand(command, now); err != nil {
		t.Fatal(err)
	}
	// The configured 300-second timeout is stricter than the server's 24 hours,
	// so the controller's own bound is what applies.
	if resolution := controller.Resolve(fanChannel, now.Add(10*time.Minute)); resolution.Source == SourceOverride {
		t.Fatalf("a server-supplied expiry overrode the controller's own limit: %+v", resolution)
	}
}

func TestEmergencyStopLatchesAndRefusesCommands(t *testing.T) {
	controller := configuredController(t)
	now := at(9, 0)
	controller.LatchEmergency()

	command := deviceprotocol.Command{
		ProtocolVersion: deviceprotocol.Version, CommandID: "01990a20-6a00-7000-8000-000000000053",
		TargetChannelID: lightChannel, Type: "set_percent", Value: float64(100),
		IssuedAt: now, ExpiresAt: now.Add(time.Minute),
	}
	if err := controller.ApplyCommand(command, now); err == nil {
		t.Fatal("a command was accepted while the emergency stop was latched")
	}
	if resolution := controller.Resolve(lightChannel, now); resolution.Source != SourceEmergency {
		t.Fatalf("emergency did not take precedence: %+v", resolution)
	}
	// Aeration's safe state is running, so an emergency must not stop it.
	if resolution := controller.Resolve(airPumpChannel, now); resolution.Value != 100 {
		t.Fatalf("the emergency stop cut aeration: %+v", resolution)
	}

	controller.ClearEmergency()
	if err := controller.ApplyCommand(command, now); err != nil {
		t.Fatalf("commands were still refused after the emergency cleared: %v", err)
	}
}

func TestHardwareInterlockOutranksEverything(t *testing.T) {
	controller := configuredController(t)
	now := at(9, 0)
	command := deviceprotocol.Command{
		ProtocolVersion: deviceprotocol.Version, CommandID: "01990a20-6a00-7000-8000-000000000054",
		TargetChannelID: lightChannel, Type: "set_percent", Value: float64(100),
		IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	if err := controller.ApplyCommand(command, now); err != nil {
		t.Fatal(err)
	}
	controller.SetInterlock(lightChannel, 0)
	resolution := controller.Resolve(lightChannel, now)
	if resolution.Source != SourceHardwareInterlock || resolution.Value != 0 {
		t.Fatalf("an interlock was overridden by a command: %+v", resolution)
	}
}

func TestConfigurationForAnotherDeviceIsRefused(t *testing.T) {
	controller := configuredController(t)
	foreign := pilotConfig()
	foreign.DeviceID = "01990a20-6a00-7000-8000-0000000000ff"
	foreign.ConfigVersion = "someone-elses-v2"
	if err := controller.ApplyConfig(foreign); err == nil {
		t.Fatal("a controller adopted another device's configuration")
	}
	if controller.ConfigVersion() != "pilot-v1" {
		t.Fatalf("the rejected configuration replaced the running one: %q", controller.ConfigVersion())
	}
}

func TestInvalidConfigurationLeavesTheRunningOneInPlace(t *testing.T) {
	controller := configuredController(t)
	for name, mutate := range map[string]func(*deviceprotocol.EdgeConfig){
		"no version":     func(config *deviceprotocol.EdgeConfig) { config.ConfigVersion = "" },
		"bad protocol":   func(config *deviceprotocol.EdgeConfig) { config.ProtocolVersion = 99 },
		"bad hour":       func(config *deviceprotocol.EdgeConfig) { config.Config.Photoperiod.OnHour = 30 },
		"bad fan floor":  func(config *deviceprotocol.EdgeConfig) { config.Config.FanMinimumPercent = value(150) },
		"bad channel id": func(config *deviceprotocol.EdgeConfig) { config.Config.Photoperiod.ChannelID = "light" },
	} {
		t.Run(name, func(t *testing.T) {
			broken := pilotConfig()
			broken.ConfigVersion = "broken-v2"
			mutate(&broken)
			if err := controller.ApplyConfig(broken); err == nil {
				t.Fatal("an invalid configuration was adopted")
			}
			if controller.ConfigVersion() != "pilot-v1" {
				t.Fatalf("running configuration was replaced by an invalid one: %q", controller.ConfigVersion())
			}
			// The controller must keep working on the last good configuration.
			if resolution := controller.Resolve(airPumpChannel, at(3, 0)); resolution.Value != 100 {
				t.Fatalf("aeration stopped after rejecting a bad configuration: %+v", resolution)
			}
		})
	}
}

func TestPhotoperiodCrossingMidnight(t *testing.T) {
	controller := NewController(testDevice)
	config := pilotConfig()
	// 18:00 to 02:00 spans midnight, which is how a night-cycle schedule is
	// expressed.
	config.Config.Photoperiod = &deviceprotocol.Photoperiod{OnHour: 18, OffHour: 2, ChannelID: lightChannel}
	if err := controller.ApplyConfig(config); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		hour int
		on   bool
	}{{19, true}, {23, true}, {0, true}, {1, true}, {2, false}, {10, false}, {17, false}} {
		if got := controller.Resolve(lightChannel, at(testCase.hour, 0)).Value == 100; got != testCase.on {
			t.Fatalf("at %02d:00 light on = %v, want %v", testCase.hour, got, testCase.on)
		}
	}
}
