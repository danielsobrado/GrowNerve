// Package zigbee2mqtt reads and controls Zigbee devices through a
// Zigbee2MQTT bridge (https://www.zigbee2mqtt.io) over MQTT.
//
// Devices are identified by IEEE address, which survives renames; topics use
// the friendly name, which is re-resolved from bridge/devices on every update.
package zigbee2mqtt

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/jdanielsobrado/grownerve/internal/integration"
)

// Expose access bits.
const (
	accessPublished = 1
	accessSet       = 2
)

type bridgeDevice struct {
	IEEEAddress     string      `json:"ieee_address"`
	FriendlyName    string      `json:"friendly_name"`
	Type            string      `json:"type"`
	Disabled        bool        `json:"disabled"`
	PowerSource     string      `json:"power_source"`
	SoftwareBuildID string      `json:"software_build_id"`
	Manufacturer    string      `json:"manufacturer"`
	ModelID         string      `json:"model_id"`
	Definition      *definition `json:"definition"`
}

type definition struct {
	Model       string   `json:"model"`
	Vendor      string   `json:"vendor"`
	Description string   `json:"description"`
	Exposes     []expose `json:"exposes"`
}

type expose struct {
	Type     string   `json:"type"`
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Property string   `json:"property"`
	Access   int      `json:"access"`
	Unit     string   `json:"unit"`
	Category string   `json:"category"`
	ValueMin *float64 `json:"value_min"`
	ValueMax *float64 `json:"value_max"`
	ValueOn  any      `json:"value_on"`
	ValueOff any      `json:"value_off"`
	Features []expose `json:"features"`
}

// property is how one adopted capability is read from and written to a
// device's state payload.
type property struct {
	boolean  bool
	valueOn  any
	valueOff any
	// levelMax > 0 scales a 0..levelMax level (brightness) to percent.
	levelMax float64
	unit     integration.Unit
	settable bool
}

type device struct {
	ieee       string
	name       string
	discovered integration.DiscoveredDevice
	properties map[string]property
}

var diagnosticProperties = map[string]bool{
	"linkquality": true, "battery": true, "voltage": true, "battery_voltage": true, "battery_low": true, "update_available": true,
}

// parseDevices reads the retained bridge/devices list. The coordinator,
// disabled devices and devices Zigbee2MQTT cannot interpret are skipped.
func parseDevices(payload []byte) ([]*device, error) {
	var listed []bridgeDevice
	if err := json.Unmarshal(payload, &listed); err != nil {
		return nil, fmt.Errorf("decode bridge/devices: %w", err)
	}
	var devices []*device
	for _, entry := range listed {
		if entry.Type == "Coordinator" || entry.Disabled || entry.Definition == nil || entry.IEEEAddress == "" || entry.FriendlyName == "" {
			continue
		}
		parsed := &device{ieee: entry.IEEEAddress, name: entry.FriendlyName, properties: map[string]property{}}
		parsed.discovered = integration.DiscoveredDevice{
			Provider: integration.Zigbee2MQTT, ExternalID: entry.IEEEAddress, Name: entry.FriendlyName,
			Manufacturer: firstNonEmpty(entry.Definition.Vendor, entry.Manufacturer),
			Model:        firstNonEmpty(entry.Definition.Model, entry.ModelID),
			Firmware:     entry.SoftwareBuildID, PowerSource: powerSource(entry.PowerSource),
		}
		parents := map[string]bool{}
		for _, exposed := range entry.Definition.Exposes {
			parsed.addExpose(exposed, "", parents)
		}
		switch {
		case parents["light"]:
			parsed.discovered.SuggestedType = "light"
		case parents["fan"]:
			parsed.discovered.SuggestedType = "fan"
		case parsed.hasCommand():
			parsed.discovered.SuggestedType = "controller"
		default:
			parsed.discovered.SuggestedType = "sensor"
		}
		devices = append(devices, parsed)
	}
	return devices, nil
}

func (parsed *device) hasCommand() bool {
	for _, capability := range parsed.discovered.Capabilities {
		if capability.Kind == integration.KindCommand {
			return true
		}
	}
	return false
}

func (parsed *device) addExpose(exposed expose, parent string, parents map[string]bool) {
	switch exposed.Type {
	case "light", "switch", "fan", "lock", "cover", "climate":
		parents[exposed.Type] = true
		for _, feature := range exposed.Features {
			parsed.addExpose(feature, exposed.Type, parents)
		}
		return
	case "binary", "numeric":
	default:
		// enum, text, composite (colour) and list values have no channel shape.
		return
	}
	key := exposed.Property
	if key == "" || exposed.Access&accessPublished == 0 || exposed.Category == "config" {
		return
	}
	if _, duplicate := parsed.properties[key]; duplicate {
		return
	}
	settable := exposed.Access&accessSet != 0
	capability := integration.Capability{
		Key: key, Label: label(exposed), SuggestedChannelKey: integration.Slug(parsed.name) + "." + integration.Slug(key),
		Diagnostic: exposed.Category == "diagnostic" || diagnosticProperties[key],
	}
	zero, one, hundred := 0.0, 1.0, 100.0
	switch {
	case exposed.Type == "binary":
		entry := property{boolean: true, valueOn: exposed.ValueOn, valueOff: exposed.ValueOff, unit: integration.Unit{Canonical: integration.UnitBoolean}}
		capability.ValueType, capability.Unit, capability.Minimum, capability.Maximum = "boolean", integration.UnitBoolean, &zero, &one
		capability.Kind = integration.KindState
		// Only on/off outputs are controllable; other settable binaries (child
		// lock, LED indicator) are device settings, not grow outputs.
		if settable && strings.HasPrefix(key, "state") && (parent == "switch" || parent == "light" || parent == "fan" || parent == "") {
			capability.Kind = integration.KindCommand
			entry.settable = true
		}
		parsed.properties[key] = entry
	case exposed.Name == "brightness" && parent == "light":
		levelMax := 254.0
		if exposed.ValueMax != nil && *exposed.ValueMax > 0 {
			levelMax = *exposed.ValueMax
		}
		capability.Kind, capability.ValueType, capability.Unit, capability.Dimension = integration.KindState, "number", "%", "ratio"
		capability.Minimum, capability.Maximum = &zero, &hundred
		if settable {
			capability.Kind = integration.KindCommand
		}
		parsed.properties[key] = property{levelMax: levelMax, unit: integration.Unit{Canonical: "%"}, settable: settable}
	default:
		unit := integration.CanonicalUnit(exposed.Unit, key)
		capability.Kind, capability.ValueType, capability.Unit, capability.Dimension = integration.KindMeasurement, "number", unit.Canonical, unit.Dimension
		if exposed.ValueMin != nil {
			minimum := unit.Convert(*exposed.ValueMin)
			capability.Minimum = &minimum
		}
		if exposed.ValueMax != nil {
			maximum := unit.Convert(*exposed.ValueMax)
			capability.Maximum = &maximum
		}
		parsed.properties[key] = property{unit: unit}
	}
	parsed.discovered.Capabilities = append(parsed.discovered.Capabilities, capability)
}

// readings turns one state payload into canonical readings for every
// property the device exposes. Absent or unreadable values are skipped.
func (parsed *device) readings(payload []byte) ([]integration.Reading, error) {
	var state map[string]any
	if err := json.Unmarshal(payload, &state); err != nil {
		return nil, fmt.Errorf("decode state of %s: %w", parsed.name, err)
	}
	var readings []integration.Reading
	for key, entry := range parsed.properties {
		raw, present := state[key]
		if !present || raw == nil {
			continue
		}
		value, ok := entry.read(raw)
		if !ok {
			continue
		}
		readings = append(readings, integration.Reading{ExternalID: parsed.ieee, Key: key, Value: value, Unit: entry.unit.Canonical})
	}
	return readings, nil
}

func (entry property) read(raw any) (float64, bool) {
	if entry.boolean {
		switch {
		case entry.valueOn != nil && equalValue(raw, entry.valueOn):
			return 1, true
		case entry.valueOff != nil && equalValue(raw, entry.valueOff):
			return 0, true
		}
		switch typed := raw.(type) {
		case bool:
			if typed {
				return 1, true
			}
			return 0, true
		case string:
			switch strings.ToUpper(typed) {
			case "ON", "TRUE", "OPEN", "LOCK":
				return 1, true
			case "OFF", "FALSE", "CLOSED", "CLOSE", "UNLOCK":
				return 0, true
			}
		}
		return 0, false
	}
	number, ok := raw.(float64)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	if entry.levelMax > 0 {
		return math.Round(number/entry.levelMax*1000) / 10, true
	}
	return entry.unit.Convert(number), true
}

// setPayload builds the <friendly_name>/set body for an absolute target.
func (entry property) setPayload(key string, command integration.Command) (map[string]any, error) {
	if !entry.settable {
		return nil, fmt.Errorf("%s is not controllable", key)
	}
	switch {
	case entry.boolean && command.Boolean != nil:
		on, off := entry.valueOn, entry.valueOff
		if on == nil {
			on = "ON"
		}
		if off == nil {
			off = "OFF"
		}
		if *command.Boolean {
			return map[string]any{key: on}, nil
		}
		return map[string]any{key: off}, nil
	case entry.levelMax > 0 && command.Percent != nil:
		percent := math.Max(0, math.Min(100, *command.Percent))
		return map[string]any{key: math.Round(percent / 100 * entry.levelMax)}, nil
	}
	return nil, fmt.Errorf("%s does not accept that command type", key)
}

func equalValue(left, right any) bool {
	leftText, _ := json.Marshal(left)
	rightText, _ := json.Marshal(right)
	return string(leftText) == string(rightText)
}

// parseOnline reads bridge/state and <device>/availability, which are JSON
// objects in Zigbee2MQTT 2.x and bare strings in 1.x.
func parseOnline(payload []byte) (bool, bool) {
	text := strings.TrimSpace(string(payload))
	var object struct {
		State string `json:"state"`
	}
	if json.Unmarshal(payload, &object) == nil && object.State != "" {
		text = object.State
	}
	switch strings.ToLower(strings.Trim(text, `"`)) {
	case "online":
		return true, true
	case "offline":
		return false, true
	}
	return false, false
}

func label(exposed expose) string {
	if exposed.Label != "" {
		return exposed.Label
	}
	name := firstNonEmpty(exposed.Name, exposed.Property)
	name = strings.ReplaceAll(name, "_", " ")
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func powerSource(value string) string {
	lower := strings.ToLower(value)
	switch {
	case strings.Contains(lower, "battery"):
		return "battery"
	case lower == "":
		return ""
	default:
		return "mains"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
