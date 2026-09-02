package command

import (
	"testing"
	"time"
)

func TestValidateRejectsUnsafeCommand(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	checks := []struct {
		name string
		ctx  SafetyContext
		code string
	}{
		{name: "offline", ctx: SafetyContext{Controllable: true, Online: false}, code: "DEVICE_OFFLINE"},
		{name: "emergency stop", ctx: SafetyContext{Controllable: true, Online: true, EmergencyStop: true}, code: "EMERGENCY_STOP_ACTIVE"},
		{name: "outside range", ctx: SafetyContext{Controllable: true, Online: true, Minimum: 0, Maximum: 100}, code: "COMMAND_VALUE_OUT_OF_RANGE"},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			value := 50.0
			if check.name == "outside range" {
				value = 101
			}
			err := Validate(Request{Value: value, ExpiresAt: now.Add(time.Minute)}, check.ctx, now)
			if err == nil || err.Code != check.code {
				t.Fatalf("Validate() error = %#v, want code %q", err, check.code)
			}
		})
	}
}

func TestValidateRejectsExcessiveTTL(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	context := SafetyContext{
		Controllable: true,
		Online:       true,
		Minimum:      0,
		Maximum:      100,
		MaximumTTL:   MaximumTTL,
	}
	if err := Validate(Request{Value: 50, ExpiresAt: now.Add(MaximumTTL)}, context, now); err != nil {
		t.Fatalf("maximum permitted TTL rejected: %v", err)
	}
	if err := Validate(Request{Value: 50, ExpiresAt: now.Add(MaximumTTL + time.Second)}, context, now); err == nil || err.Code != "COMMAND_TTL_TOO_LONG" {
		t.Fatalf("excessive TTL error = %#v", err)
	}
}

func TestStateMachineRejectsInvalidTransition(t *testing.T) {
	command := Command{State: StatePending}
	if err := command.Transition(StateApplied); err != ErrInvalidTransition {
		t.Fatalf("Transition() error = %v, want %v", err, ErrInvalidTransition)
	}
}

func TestValidateCommonSafetyPaths(t *testing.T) {
	now := time.Now()
	base := SafetyContext{Controllable: true, Online: true, Minimum: 0, Maximum: 100}
	if err := Validate(Request{Value: 50, ExpiresAt: now.Add(time.Minute)}, base, now); err != nil {
		t.Fatalf("valid command rejected: %v", err)
	}
	checks := []struct {
		context SafetyContext
		request Request
		code    string
	}{
		{context: SafetyContext{Online: true}, request: Request{ExpiresAt: now.Add(time.Minute)}, code: "CHANNEL_NOT_CONTROLLABLE"},
		{context: base, request: Request{ExpiresAt: now.Add(-time.Second)}, code: "COMMAND_EXPIRED"},
	}
	for _, check := range checks {
		if err := Validate(check.request, check.context, now); err == nil || err.Code != check.code {
			t.Fatalf("Validate() = %v, want %s", err, check.code)
		}
	}
	if got := (&SafetyError{Code: "TEST", Detail: "detail"}).Error(); got != "TEST: detail" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestStateMachineValidTransitions(t *testing.T) {
	command := Command{State: StatePending}
	for _, state := range []State{StatePublished, StateAcknowledged, StateApplied} {
		if err := command.Transition(state); err != nil {
			t.Fatalf("Transition(%s): %v", state, err)
		}
	}
}
