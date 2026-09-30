package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jdanielsobrado/grownerve/internal/farm"
)

// AdoptRequest turns a discovered device into a farm device. Channel kinds,
// value types, units and ranges always come from the provider's capability,
// never from the client.
type AdoptRequest struct {
	ZoneID string `json:"zone_id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	// AcknowledgeNoEdgeFailsafe must be true when any command capability is
	// adopted: a smart plug has no controller-side schedule or safe state to
	// fall back on if the server is lost (ADR-039).
	AcknowledgeNoEdgeFailsafe bool           `json:"acknowledge_no_edge_failsafe"`
	Channels                  []AdoptChannel `json:"channels"`
}

type AdoptChannel struct {
	CapabilityKey     string   `json:"capability_key"`
	Key               string   `json:"key"`
	Name              string   `json:"name"`
	EntityType        string   `json:"entity_type,omitempty"`
	EntityID          string   `json:"entity_id,omitempty"`
	StaleAfterSeconds int      `json:"stale_after_seconds,omitempty"`
	SafeMinimum       *float64 `json:"safe_minimum,omitempty"`
	SafeMaximum       *float64 `json:"safe_maximum,omitempty"`
}

type AdoptResult struct {
	Device   map[string]any   `json:"device"`
	Channels []map[string]any `json:"channels"`
}

// RequestError is a refusal the HTTP layer reports verbatim.
type RequestError struct {
	Status int
	Code   string
	Detail string
}

func (err *RequestError) Error() string { return err.Code + ": " + err.Detail }

func refuse(status int, code, format string, arguments ...any) error {
	return &RequestError{Status: status, Code: code, Detail: fmt.Sprintf(format, arguments...)}
}

var deviceTypes = []string{"controller", "light", "fan", "air_pump", "sensor"}

// entityTypes are the farm entities a channel may describe.
var entityTypes = []string{"facility", "zone", "reservoir", "grow_cycle", "plant_position", "device", "channel", "inventory_item"}

// Adopt binds a provider device to a new farm device and its channels, and
// registers them through the state committer so the relational registry (and
// therefore telemetry's foreign key) knows about them.
func (manager *Manager) Adopt(ctx context.Context, provider Provider, externalID, actor string, request AdoptRequest) (AdoptResult, error) {
	adapter, enabled := manager.adapters[provider]
	if !enabled {
		return AdoptResult{}, ErrProviderDisabled
	}
	var discovered *DiscoveredDevice
	for _, device := range adapter.Devices() {
		if device.ExternalID == externalID {
			discovered = &device
			break
		}
	}
	if discovered == nil {
		return AdoptResult{}, refuse(http.StatusNotFound, "UNKNOWN_EXTERNAL_DEVICE", "%s has no device %q", provider, externalID)
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		request.Name = discovered.Name
	}
	if request.Type == "" {
		request.Type = discovered.SuggestedType
	}
	if !slices.Contains(deviceTypes, request.Type) {
		return AdoptResult{}, refuse(http.StatusUnprocessableEntity, "INVALID_DEVICE_TYPE", "type must be one of %s", strings.Join(deviceTypes, ", "))
	}
	if len(request.Channels) == 0 {
		return AdoptResult{}, refuse(http.StatusUnprocessableEntity, "NO_CHANNELS", "choose at least one capability to adopt")
	}

	now := manager.deps.Now()
	deviceID := uuid.NewString()
	device := map[string]any{
		"id": deviceID, "zone_id": request.ZoneID, "name": request.Name, "type": request.Type,
		"online": false, "simulated": false, "last_heartbeat": "",
		"firmware_version": discovered.Firmware, "active_config_version": "",
		"integration": DeviceIntegration{
			Provider: provider, ExternalID: externalID, Manufacturer: discovered.Manufacturer,
			Model: discovered.Model, AdoptedAt: now,
		},
	}
	var channels, bindings []map[string]any
	seenKeys, seenCapabilities := map[string]bool{}, map[string]bool{}
	commands := false
	for _, choice := range request.Channels {
		capability, found := discovered.Capability(choice.CapabilityKey)
		if !found {
			return AdoptResult{}, refuse(http.StatusUnprocessableEntity, "UNKNOWN_CAPABILITY", "device %q has no capability %q", externalID, choice.CapabilityKey)
		}
		if seenCapabilities[capability.Key] {
			return AdoptResult{}, refuse(http.StatusUnprocessableEntity, "DUPLICATE_CAPABILITY", "capability %q was chosen twice", capability.Key)
		}
		seenCapabilities[capability.Key] = true
		key := strings.TrimSpace(choice.Key)
		if key == "" {
			key = capability.SuggestedChannelKey
		}
		if seenKeys[key] {
			return AdoptResult{}, refuse(http.StatusConflict, "CHANNEL_KEY_CONFLICT", "channel key %q is used twice", key)
		}
		seenKeys[key] = true
		name := strings.TrimSpace(choice.Name)
		if name == "" {
			name = capability.Label
		}
		entityType, entityID := choice.EntityType, choice.EntityID
		if entityType == "" {
			entityType, entityID = "device", deviceID
		}
		if !slices.Contains(entityTypes, entityType) || entityID == "" {
			return AdoptResult{}, refuse(http.StatusUnprocessableEntity, "INVALID_CHANNEL_ENTITY", "channel %q needs a valid entity type and id", key)
		}
		stale := choice.StaleAfterSeconds
		if stale <= 0 {
			stale = 300
			if discovered.PowerSource == "battery" {
				stale = 3600
			}
		}
		channelID := uuid.NewString()
		channel := map[string]any{
			"id": channelID, "device_id": deviceID, "entity_type": entityType, "entity_id": entityID,
			"key": key, "name": name, "kind": capability.Kind, "value_type": capability.ValueType,
			"unit": capability.Unit, "stale_after_seconds": stale, "integration_key": capability.Key,
		}
		if capability.Dimension != "" {
			channel["dimension"] = capability.Dimension
		}
		if capability.Minimum != nil {
			channel["minimum"] = *capability.Minimum
		}
		if capability.Maximum != nil {
			channel["maximum"] = *capability.Maximum
		}
		if capability.Kind == KindCommand {
			commands = true
			minimum, maximum := boundOr(capability.Minimum, 0), boundOr(capability.Maximum, 100)
			safeMinimum, safeMaximum := boundOr(choice.SafeMinimum, minimum), boundOr(choice.SafeMaximum, maximum)
			if safeMinimum < minimum || safeMaximum > maximum || safeMinimum > safeMaximum {
				return AdoptResult{}, refuse(http.StatusUnprocessableEntity, "INVALID_SAFE_RANGE",
					"safe range for %q must lie within %g..%g", key, minimum, maximum)
			}
			channel["safe_minimum"], channel["safe_maximum"] = safeMinimum, safeMaximum
		}
		channels = append(channels, channel)
		bindings = append(bindings, map[string]any{
			"id": uuid.NewString(), "channel_id": channelID, "device_id": deviceID,
			"valid_from": now, "integration_key": capability.Key,
		})
	}
	if commands && !request.AcknowledgeNoEdgeFailsafe {
		return AdoptResult{}, refuse(http.StatusUnprocessableEntity, "EDGE_FAILSAFE_NOT_ACKNOWLEDGED",
			"controllable integration devices have no controller-side failsafe; confirm acknowledge_no_edge_failsafe")
	}

	err := farm.CommitMutation(ctx, manager.deps.Store, manager.deps.Committer, func(state json.RawMessage) (json.RawMessage, error) {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(state, &object); err != nil {
			return nil, farm.ErrInvalidState
		}
		var parsed document
		if err := json.Unmarshal(state, &parsed); err != nil {
			return nil, farm.ErrInvalidState
		}
		if !slices.ContainsFunc(parsed.Zones, func(zone struct {
			ID string `json:"id"`
		}) bool {
			return zone.ID == request.ZoneID
		}) {
			return nil, refuse(http.StatusUnprocessableEntity, "UNKNOWN_ZONE", "zone %q does not exist", request.ZoneID)
		}
		for _, existing := range parsed.Devices {
			if existing.Integration != nil && existing.Integration.Provider == provider && existing.Integration.ExternalID == externalID {
				return nil, refuse(http.StatusConflict, "ALREADY_ADOPTED", "this device is already adopted as %s", existing.ID)
			}
		}
		for _, existing := range parsed.Channels {
			if seenKeys[existing.Key] {
				return nil, refuse(http.StatusConflict, "CHANNEL_KEY_CONFLICT", "channel key %q is already in use", existing.Key)
			}
		}
		additions := map[string]any{}
		for collection, records := range map[string][]map[string]any{
			"devices": {device}, "channels": channels, "channel_bindings": bindings,
		} {
			var current []json.RawMessage
			if raw, present := object[collection]; present && string(raw) != "null" {
				if err := json.Unmarshal(raw, &current); err != nil {
					return nil, farm.ErrInvalidState
				}
			}
			for _, record := range records {
				encoded, err := json.Marshal(record)
				if err != nil {
					return nil, err
				}
				current = append(current, encoded)
			}
			additions[collection] = current
		}
		return farm.ReplaceKeys(state, additions)
	})
	var refusal *RequestError
	switch {
	case errors.As(err, &refusal):
		return AdoptResult{}, err
	case errors.Is(err, farm.ErrNotFound):
		return AdoptResult{}, refuse(http.StatusConflict, "FARM_NOT_CONFIGURED", "configure a farm before adopting devices")
	case err != nil:
		return AdoptResult{}, err
	}

	manager.refreshIndex(ctx)
	kick(manager.livenessKick)
	manager.notify("state")
	if manager.deps.Audit != nil {
		capabilities := make([]string, 0, len(channels))
		for _, channel := range channels {
			capabilities = append(capabilities, channel["integration_key"].(string))
		}
		manager.deps.Audit.Record(ctx, farm.AuditEntry{
			Actor: actor, Action: "integration.device_adopted", TargetType: "device", TargetID: deviceID, OccurredAt: now,
			Detail: map[string]any{"provider": provider, "external_id": externalID, "capabilities": capabilities},
		})
	}
	return AdoptResult{Device: device, Channels: channels}, nil
}

func boundOr(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}
