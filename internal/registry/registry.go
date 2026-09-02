// Package registry projects the identities that relational data references —
// facilities, devices, and logical channels — out of the farm configuration
// document and into their tables.
//
// It exists because the two halves of the split in ADR-032 have to meet
// somewhere: measurements are relational and carry a foreign key to
// device_channels, so a channel that lives only in the document has nothing for
// telemetry to reference. Projecting the registry is what makes that foreign key
// meaningful, and it is what lets the database reject telemetry from a channel
// nobody configured.
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Document is the slice of farm state the registry needs.
type Document struct {
	Facilities []Facility `json:"facilities"`
	Devices    []Device   `json:"devices"`
	Channels   []Channel  `json:"channels"`
}

type Facility struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

type Device struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Online bool   `json:"online"`
}

type Channel struct {
	ID                string   `json:"id"`
	DeviceID          string   `json:"device_id"`
	EntityType        string   `json:"entity_type"`
	EntityID          string   `json:"entity_id"`
	Key               string   `json:"key"`
	Name              string   `json:"name"`
	Kind              string   `json:"kind"`
	ValueType         string   `json:"value_type"`
	Unit              string   `json:"unit"`
	Dimension         string   `json:"dimension"`
	Minimum           *float64 `json:"minimum"`
	Maximum           *float64 `json:"maximum"`
	SafeMinimum       *float64 `json:"safe_minimum"`
	SafeMaximum       *float64 `json:"safe_maximum"`
	StaleAfterSeconds int      `json:"stale_after_seconds"`
}

// InvalidError reports a registry the database will not accept. It is returned
// to the client rather than logged, because a configuration whose channels
// cannot be stored would silently discard every measurement they produce.
type InvalidError struct{ Reason string }

func (err *InvalidError) Error() string { return err.Reason }

// Projector writes the registry.
type Projector interface {
	Project(ctx context.Context, document Document) error
}

// Parse extracts the registry from a farm state document.
func Parse(state json.RawMessage) (Document, error) {
	var document Document
	if err := json.Unmarshal(state, &document); err != nil {
		return document, err
	}
	return document, nil
}

// Validate checks the invariants the schema enforces, so a bad configuration is
// refused with a readable message instead of a driver error.
func (document Document) Validate() error {
	if len(document.Facilities) == 0 && (len(document.Devices) > 0 || len(document.Channels) > 0) {
		return &InvalidError{"devices and channels require at least one facility"}
	}
	kinds := map[string]bool{"measurement": true, "state": true, "command": true, "counter": true}
	valueTypes := map[string]bool{"number": true, "boolean": true, "enum": true}
	deviceIDs := make(map[string]bool, len(document.Devices))
	for _, device := range document.Devices {
		if device.ID == "" {
			return &InvalidError{"every device needs an id"}
		}
		deviceIDs[device.ID] = true
	}
	seenKeys := map[string]string{}
	for _, channel := range document.Channels {
		if channel.ID == "" || channel.Key == "" {
			return &InvalidError{"every channel needs an id and a key"}
		}
		if channel.DeviceID != "" && !deviceIDs[channel.DeviceID] {
			return &InvalidError{fmt.Sprintf("channel %q references unknown device %q", channel.Key, channel.DeviceID)}
		}
		if !kinds[channel.Kind] {
			return &InvalidError{fmt.Sprintf("channel %q has unsupported kind %q", channel.Key, channel.Kind)}
		}
		if !valueTypes[channel.ValueType] {
			return &InvalidError{fmt.Sprintf("channel %q has unsupported value type %q", channel.Key, channel.ValueType)}
		}
		// The schema enforces one channel per key per facility. Catching it here
		// names the offending key instead of surfacing a constraint violation.
		if existing, duplicate := seenKeys[channel.Key]; duplicate && existing != channel.ID {
			return &InvalidError{fmt.Sprintf("channel key %q is used by more than one channel", channel.Key)}
		}
		seenKeys[channel.Key] = channel.ID
	}
	return nil
}

// StaleAfter returns the channel's staleness window, defaulting to ten minutes
// so a channel configured without one still has a bound.
func (channel Channel) StaleAfter() time.Duration {
	if channel.StaleAfterSeconds <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(channel.StaleAfterSeconds) * time.Second
}

// DeviceStatus maps the document's liveness flag onto the schema's vocabulary.
func (device Device) DeviceStatus() string {
	if device.Online {
		return "online"
	}
	return "offline"
}

// DeviceType falls back to a generic type rather than failing, because the
// document's type vocabulary is broader than the registry needs.
func (device Device) DeviceType() string {
	if device.Type == "" {
		return "controller"
	}
	return device.Type
}
