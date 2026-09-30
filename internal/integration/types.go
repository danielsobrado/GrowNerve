// Package integration connects third-party device ecosystems (Zigbee through
// Zigbee2MQTT, Matter, Home Assistant) to the farm without changing how the farm
// treats a device.
//
// Adapters translate a provider's protocol into provider-neutral readings and
// commands. The Manager binds those to the farm's own devices and channels,
// records telemetry, keeps device liveness, and turns reflected device state
// into command acknowledgements. Commands for integration devices still pass
// the command handler's safety validation, the outbox and the sweeper: the
// Router only decides which transport carries them (ADR-038).
package integration

import (
	"context"
	"time"
)

// Provider names an integration. The values are stored in the farm document.
type Provider string

const (
	Zigbee2MQTT   Provider = "zigbee2mqtt"
	Matter        Provider = "matter"
	HomeAssistant Provider = "home_assistant"
)

// Providers lists every provider the server knows, enabled or not.
var Providers = []Provider{Zigbee2MQTT, Matter, HomeAssistant}

func (provider Provider) Valid() bool {
	for _, known := range Providers {
		if provider == known {
			return true
		}
	}
	return false
}

// Capability kinds match channel kinds in the farm document.
const (
	KindMeasurement = "measurement"
	KindState       = "state"
	KindCommand     = "command"
)

// UnitBoolean is the unit of every on/off channel. Measurements must carry a
// unit, so boolean state is stored as 0 or 1 in "bool".
const UnitBoolean = "bool"

// Capability is one reading or output a discovered device offers. Adopting a
// device turns chosen capabilities into channels.
type Capability struct {
	// Key identifies the capability within its device and becomes the channel's
	// integration_key: a Zigbee2MQTT expose property, a Matter attribute path, a
	// Home Assistant entity.
	Key       string `json:"key"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	ValueType string `json:"value_type"`
	// Unit is already canonical (degC, %RH, %, bool, lx, ppm, W, …).
	Unit                string   `json:"unit"`
	Dimension           string   `json:"dimension,omitempty"`
	Minimum             *float64 `json:"minimum,omitempty"`
	Maximum             *float64 `json:"maximum,omitempty"`
	SuggestedChannelKey string   `json:"suggested_channel_key"`
	// Diagnostic capabilities (battery, link quality) are offered unticked.
	Diagnostic bool   `json:"diagnostic,omitempty"`
	Note       string `json:"note,omitempty"`
}

// DiscoveredDevice is a device the provider knows about, adopted or not.
type DiscoveredDevice struct {
	Provider        Provider     `json:"provider"`
	ExternalID      string       `json:"external_id"`
	Name            string       `json:"name"`
	Manufacturer    string       `json:"manufacturer,omitempty"`
	Model           string       `json:"model,omitempty"`
	Firmware        string       `json:"firmware,omitempty"`
	PowerSource     string       `json:"power_source,omitempty"`
	SuggestedType   string       `json:"suggested_type"`
	Available       bool         `json:"available"`
	Capabilities    []Capability `json:"capabilities"`
	AdoptedDeviceID string       `json:"adopted_device_id,omitempty"`
}

// Capability looks up one capability by key.
func (device DiscoveredDevice) Capability(key string) (Capability, bool) {
	for _, capability := range device.Capabilities {
		if capability.Key == key {
			return capability, true
		}
	}
	return Capability{}, false
}

// Reading is one value a provider reported. Unit is canonical.
type Reading struct {
	ExternalID string
	Key        string
	Value      float64
	Unit       string
	ObservedAt time.Time
}

// Connection states reported in Status.
const (
	StateDisabled   = "disabled"
	StateConnecting = "connecting"
	StateConnected  = "connected"
	StateDegraded   = "degraded"
	StateError      = "error"
)

// Status is a provider's connection health as shown to operators.
type Status struct {
	Provider        Provider   `json:"provider"`
	Enabled         bool       `json:"enabled"`
	State           string     `json:"state"`
	Detail          string     `json:"detail,omitempty"`
	Since           time.Time  `json:"since"`
	DeviceCount     int        `json:"device_count"`
	AdoptedCount    int        `json:"adopted_count"`
	PermitJoin      bool       `json:"permit_join_supported"`
	PermitJoinUntil *time.Time `json:"permit_join_until,omitempty"`
}

// Sink receives what adapters observe. Implementations must not block for
// long: adapters call it from their receive loops.
type Sink interface {
	Readings(provider Provider, readings []Reading)
	// AvailabilityChanged asks for an immediate liveness pass.
	AvailabilityChanged(provider Provider)
	DevicesChanged(provider Provider)
	StatusChanged(provider Provider)
}

// Command is an absolute target for one capability. It is never a toggle, so
// executing it twice (an outbox redelivery) is harmless.
type Command struct {
	ID         string
	ExternalID string
	Key        string
	Boolean    *bool
	Percent    *float64
	ExpiresAt  time.Time
}

// Outcome results. "published" means the provider took the request but nothing
// is known about the device yet; "accepted" means the provider confirmed it
// will act; "applied" must only be reported when the device itself confirmed
// the new state (ADR-022).
const (
	OutcomePublished = "published"
	OutcomeAccepted  = "accepted"
	OutcomeApplied   = "applied"
	OutcomeRejected  = "rejected"
)

type Outcome struct {
	Result     string
	ReasonCode string
}

// Adapter is one provider connection.
type Adapter interface {
	Provider() Provider
	// Start connects in the background and keeps reconnecting until ctx ends.
	Start(ctx context.Context, sink Sink)
	Status() Status
	Devices() []DiscoveredDevice
	// Available reports whether the provider currently believes the device is
	// reachable. It is false whenever the adapter itself is disconnected.
	Available(externalID string) bool
	// Execute sends a command. An error is transient (the outbox retries it
	// until the command expires); a rejected Outcome is final.
	Execute(ctx context.Context, command Command) (Outcome, error)
}

// PermitJoiner is implemented by adapters that can open their network to new
// devices (Zigbee).
type PermitJoiner interface {
	PermitJoin(ctx context.Context, seconds int) (time.Time, error)
}
