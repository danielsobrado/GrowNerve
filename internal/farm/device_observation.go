package farm

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// DeviceObservation is what a transport learned about one device. Nil fields
// are left untouched.
type DeviceObservation struct {
	Online *bool
	// Heartbeat refreshes last_heartbeat to the receipt time. Liveness is
	// always judged from server receipt time, never the device's clock.
	Heartbeat       bool
	ObservedAt      *time.Time
	State           *bool
	OutputPercent   *float64
	FirmwareVersion *string
}

// ObserveDevices applies observations for many devices in one document write.
// Devices are edited as generic objects so fields this code does not know
// about survive. Unknown device IDs are ignored. It reports whether anything
// changed; an unchanged document is not written.
func ObserveDevices(ctx context.Context, store Store, observations map[string]DeviceObservation, receivedAt time.Time) (bool, error) {
	if len(observations) == 0 {
		return false, nil
	}
	changed := false
	err := Mutate(ctx, store, func(state json.RawMessage) (json.RawMessage, error) {
		changed = false
		var document struct {
			Devices []map[string]any `json:"devices"`
		}
		if err := json.Unmarshal(state, &document); err != nil {
			return nil, ErrInvalidState
		}
		for _, device := range document.Devices {
			id, _ := device["id"].(string)
			observation, found := observations[id]
			if !found {
				continue
			}
			set := func(key string, value any) {
				encoded, _ := json.Marshal(value)
				existing, _ := json.Marshal(device[key])
				if string(encoded) != string(existing) {
					device[key] = value
					changed = true
				}
			}
			if observation.Online != nil {
				set("online", *observation.Online)
			}
			if observation.Heartbeat {
				set("last_heartbeat", receivedAt)
			}
			if observation.ObservedAt != nil {
				set("last_device_observed_at", observation.ObservedAt.UTC())
			}
			if observation.State != nil {
				set("state", *observation.State)
			}
			if observation.OutputPercent != nil {
				set("output_percent", *observation.OutputPercent)
			}
			if observation.FirmwareVersion != nil {
				set("firmware_version", *observation.FirmwareVersion)
			}
		}
		if !changed {
			return nil, errNothingObserved
		}
		return ReplaceKeys(state, map[string]any{"devices": document.Devices})
	})
	if errors.Is(err, errNothingObserved) {
		return false, nil
	}
	return changed, err
}

// errNothingObserved aborts the mutation so an unchanged document is not
// rewritten (and its version not bumped) on every liveness tick.
var errNothingObserved = errors.New("no device observation changed state")
