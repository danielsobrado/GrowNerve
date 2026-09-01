package command

import (
	"errors"
	"fmt"
	"time"
)

type State string

const (
	StatePending      State = "pending"
	StatePublished    State = "published"
	StateAcknowledged State = "acknowledged"
	StateApplied      State = "applied"
	StateRejected     State = "rejected"
	StateTimedOut     State = "timed_out"
	StateCancelled    State = "cancelled"
)

var ErrInvalidTransition = errors.New("invalid command transition")

type Request struct {
	Value     float64
	ExpiresAt time.Time
}

type SafetyContext struct {
	Controllable  bool
	Online        bool
	EmergencyStop bool
	Minimum       float64
	Maximum       float64
}

type SafetyError struct {
	Code   string
	Detail string
}

func (err *SafetyError) Error() string { return fmt.Sprintf("%s: %s", err.Code, err.Detail) }

func Validate(request Request, context SafetyContext, now time.Time) *SafetyError {
	if !context.Controllable {
		return &SafetyError{Code: "CHANNEL_NOT_CONTROLLABLE", Detail: "target channel cannot accept commands"}
	}
	if !context.Online {
		return &SafetyError{Code: "DEVICE_OFFLINE", Detail: "the physical provider is offline"}
	}
	if context.EmergencyStop {
		return &SafetyError{Code: "EMERGENCY_STOP_ACTIVE", Detail: "the emergency stop is latched"}
	}
	if !request.ExpiresAt.After(now) {
		return &SafetyError{Code: "COMMAND_EXPIRED", Detail: "the command has expired"}
	}
	if request.Value < context.Minimum || request.Value > context.Maximum {
		return &SafetyError{Code: "COMMAND_VALUE_OUT_OF_RANGE", Detail: "requested value is outside the channel safety limits"}
	}
	return nil
}

type Command struct {
	State State
}

var allowedTransitions = map[State]map[State]bool{
	StatePending:      {StatePublished: true, StateRejected: true, StateCancelled: true},
	StatePublished:    {StateAcknowledged: true, StateRejected: true, StateTimedOut: true},
	StateAcknowledged: {StateApplied: true, StateRejected: true, StateTimedOut: true},
}

func (command *Command) Transition(next State) error {
	if !allowedTransitions[command.State][next] {
		return ErrInvalidTransition
	}
	command.State = next
	return nil
}
