package integration

import (
	"math"
	"sync"
	"time"
)

// percentTolerance absorbs provider quantisation: Zigbee and Matter levels are
// 0-254, so a requested 50% comes back as 49.6%.
const percentTolerance = 1.0

type pendingCommand struct {
	commandID  string
	deviceID   string
	provider   Provider
	externalID string
	key        string
	boolean    *bool
	percent    *float64
	expiresAt  time.Time
}

// appliedValue is what the device reported, recorded on the command.
func (pending pendingCommand) appliedValue(reading Reading) any {
	if pending.boolean != nil {
		return reading.Value != 0
	}
	return reading.Value
}

func (pending pendingCommand) satisfiedBy(reading Reading) bool {
	switch {
	case pending.boolean != nil:
		return (reading.Value != 0) == *pending.boolean
	case pending.percent != nil:
		return math.Abs(reading.Value-*pending.percent) <= percentTolerance
	}
	return false
}

// tracker holds commands waiting for the device to reflect the requested
// state. It is in memory on purpose: after a restart the pending commands
// simply time out, which is the safe reading of "no confirmation".
type tracker struct {
	mu      sync.Mutex
	pending map[string]pendingCommand
}

func newTracker() *tracker { return &tracker{pending: map[string]pendingCommand{}} }

func (tracker *tracker) add(command pendingCommand) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	tracker.pending[command.commandID] = command
}

func (tracker *tracker) remove(commandID string) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	delete(tracker.pending, commandID)
}

// match removes and returns the commands the reading confirms, and forgets
// commands that expired unconfirmed (the sweeper times those out).
func (tracker *tracker) match(provider Provider, reading Reading, now time.Time) []pendingCommand {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	var matched []pendingCommand
	for id, command := range tracker.pending {
		if !command.expiresAt.After(now) {
			delete(tracker.pending, id)
			continue
		}
		if command.provider != provider || command.externalID != reading.ExternalID || command.key != reading.Key {
			continue
		}
		if command.satisfiedBy(reading) {
			delete(tracker.pending, id)
			matched = append(matched, command)
		}
	}
	return matched
}
