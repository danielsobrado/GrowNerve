// Package runtime holds the server's continuously running work: expiring
// commands, evaluating alerts, enforcing telemetry retention, and keeping edge
// controllers in step with their configuration. These jobs are what make the
// server a runtime rather than a store with an HTTP interface in front of it.
package runtime

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"
)

// document is the narrow projection of the farm state these jobs need. Every
// write goes back through farm.ReplaceKeys so collections modelled here cannot
// clobber collections that are not.
type document struct {
	Devices        []deviceRecord   `json:"devices"`
	Channels       []channelRecord  `json:"channels"`
	Setpoints      []setpointRecord `json:"setpoints"`
	RecipeStages   []stageRecord    `json:"recipe_stages"`
	GrowCycles     []growRecord     `json:"grow_cycles"`
	Alerts         []map[string]any `json:"alerts"`
	Commands       []map[string]any `json:"commands"`
	AutomationRule []map[string]any `json:"automation_rules"`
}

type deviceRecord struct {
	ID                   string          `json:"id"`
	ZoneID               string          `json:"zone_id"`
	Name                 string          `json:"name"`
	Type                 string          `json:"type"`
	Online               bool            `json:"online"`
	Simulated            bool            `json:"simulated"`
	OutputPercent        *float64        `json:"output_percent,omitempty"`
	State                *bool           `json:"state,omitempty"`
	LastHeartbeat        string          `json:"last_heartbeat"`
	LastDeviceObservedAt string          `json:"last_device_observed_at,omitempty"`
	FirmwareVersion      string          `json:"firmware_version"`
	ActiveConfigVersion  string          `json:"active_config_version"`
	DesiredConfig        json.RawMessage `json:"desired_config,omitempty"`
	DesiredConfigVersion string          `json:"desired_config_version,omitempty"`
	LastConfigResult     map[string]any  `json:"last_config_result,omitempty"`

	// unmodelled carries every field this projection does not name (for
	// example an integration binding) so writing devices back cannot drop it.
	unmodelled map[string]json.RawMessage
}

// deviceFields is the set of JSON names deviceRecord models itself.
var deviceFields = func() map[string]bool {
	fields := map[string]bool{}
	recordType := reflect.TypeOf(deviceRecord{})
	for index := range recordType.NumField() {
		name, _, _ := strings.Cut(recordType.Field(index).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			fields[name] = true
		}
	}
	return fields
}()

func (record *deviceRecord) UnmarshalJSON(data []byte) error {
	type plain deviceRecord
	if err := json.Unmarshal(data, (*plain)(record)); err != nil {
		return err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	record.unmodelled = map[string]json.RawMessage{}
	for key, value := range all {
		if !deviceFields[key] {
			record.unmodelled[key] = value
		}
	}
	return nil
}

func (record deviceRecord) MarshalJSON() ([]byte, error) {
	type plain deviceRecord
	modelled, err := json.Marshal(plain(record))
	if err != nil || len(record.unmodelled) == 0 {
		return modelled, err
	}
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(modelled, &merged); err != nil {
		return nil, err
	}
	for key, value := range record.unmodelled {
		merged[key] = value
	}
	return json.Marshal(merged)
}

type channelRecord struct {
	ID                string   `json:"id"`
	DeviceID          string   `json:"device_id"`
	EntityType        string   `json:"entity_type"`
	EntityID          string   `json:"entity_id"`
	Key               string   `json:"key"`
	Name              string   `json:"name"`
	Kind              string   `json:"kind"`
	Unit              string   `json:"unit"`
	SafeMinimum       *float64 `json:"safe_minimum"`
	SafeMaximum       *float64 `json:"safe_maximum"`
	StaleAfterSeconds int      `json:"stale_after_seconds"`
}

type setpointRecord struct {
	ID                     string   `json:"id"`
	StageID                string   `json:"stage_id"`
	ChannelKey             string   `json:"channel_key"`
	Unit                   string   `json:"unit"`
	Minimum                *float64 `json:"minimum"`
	Maximum                *float64 `json:"maximum"`
	WarningDurationMinutes *int     `json:"warning_duration_minutes"`
	StaleAfterSeconds      int      `json:"stale_after_seconds"`
}

type stageRecord struct {
	ID              string `json:"id"`
	RecipeVersionID string `json:"recipe_version_id"`
	Key             string `json:"key"`
	Name            string `json:"name"`
}

type growRecord struct {
	ID              string `json:"id"`
	RecipeVersionID string `json:"recipe_version_id"`
	Status          string `json:"status"`
	StageKey        string `json:"stage_key"`
	Name            string `json:"name"`
}

// terminalCommandStates are the states a command can never leave. A sweeper or a
// late acknowledgement must not resurrect one.
var terminalCommandStates = map[string]bool{
	"applied": true, "rejected": true, "timed_out": true, "cancelled": true,
}

// parseTime reads a timestamp written by either the Go server or the browser
// runtime, tolerating the formats both emit.
func parseTime(value any) (time.Time, bool) {
	text, isText := value.(string)
	if !isText || text == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}
