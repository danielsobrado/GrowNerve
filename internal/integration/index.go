package integration

import (
	"encoding/json"
	"time"
)

// DeviceIntegration is the binding stored on a farm device. Its presence is
// what makes a device integration-managed.
type DeviceIntegration struct {
	Provider     Provider  `json:"provider"`
	ExternalID   string    `json:"external_id"`
	Manufacturer string    `json:"manufacturer,omitempty"`
	Model        string    `json:"model,omitempty"`
	AdoptedAt    time.Time `json:"adopted_at"`
}

// document is the slice of farm state the integration layer reads.
type document struct {
	Facilities []struct {
		ID string `json:"id"`
	} `json:"facilities"`
	Zones []struct {
		ID string `json:"id"`
	} `json:"zones"`
	Devices []struct {
		ID          string             `json:"id"`
		Integration *DeviceIntegration `json:"integration"`
	} `json:"devices"`
	Channels []struct {
		ID             string `json:"id"`
		DeviceID       string `json:"device_id"`
		Key            string `json:"key"`
		Kind           string `json:"kind"`
		ValueType      string `json:"value_type"`
		Unit           string `json:"unit"`
		IntegrationKey string `json:"integration_key"`
	} `json:"channels"`
}

type externalRef struct {
	provider   Provider
	externalID string
}

type boundDevice struct {
	ID         string
	Provider   Provider
	ExternalID string
}

type boundChannel struct {
	ID        string
	DeviceID  string
	Key       string
	Kind      string
	ValueType string
	Unit      string
}

// bindingIndex is a read-only snapshot of which farm devices and channels are
// bound to which provider devices and capabilities. Readings are resolved
// against it instead of rewriting the farm document per reading.
type bindingIndex struct {
	devices     map[externalRef]boundDevice
	byID        map[string]boundDevice
	channels    map[string][]boundChannel
	byChannelID map[string]boundChannel
}

func channelSlot(deviceID, key string) string { return deviceID + "\x00" + key }

func emptyIndex() *bindingIndex {
	return &bindingIndex{
		devices: map[externalRef]boundDevice{}, byID: map[string]boundDevice{},
		channels: map[string][]boundChannel{}, byChannelID: map[string]boundChannel{},
	}
}

func buildIndex(state json.RawMessage) (*bindingIndex, error) {
	var parsed document
	if err := json.Unmarshal(state, &parsed); err != nil {
		return nil, err
	}
	index := emptyIndex()
	for _, device := range parsed.Devices {
		if device.Integration == nil || !device.Integration.Provider.Valid() || device.Integration.ExternalID == "" {
			continue
		}
		bound := boundDevice{ID: device.ID, Provider: device.Integration.Provider, ExternalID: device.Integration.ExternalID}
		index.devices[externalRef{bound.Provider, bound.ExternalID}] = bound
		index.byID[bound.ID] = bound
	}
	for _, channel := range parsed.Channels {
		if channel.IntegrationKey == "" {
			continue
		}
		if _, bound := index.byID[channel.DeviceID]; !bound {
			continue
		}
		entry := boundChannel{
			ID: channel.ID, DeviceID: channel.DeviceID, Key: channel.IntegrationKey,
			Kind: channel.Kind, ValueType: channel.ValueType, Unit: channel.Unit,
		}
		slot := channelSlot(channel.DeviceID, channel.IntegrationKey)
		index.channels[slot] = append(index.channels[slot], entry)
		index.byChannelID[channel.ID] = entry
	}
	return index, nil
}

func (index *bindingIndex) adoptedCount(provider Provider) int {
	count := 0
	for ref := range index.devices {
		if ref.provider == provider {
			count++
		}
	}
	return count
}
