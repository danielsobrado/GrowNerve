package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jdanielsobrado/grownerve/internal/alert"
	"github.com/jdanielsobrado/grownerve/internal/farm"
)

// SweepCommands moves commands past their expiry into the timed_out state. A
// command that is published and never acknowledged would otherwise sit as
// "in flight" forever, hiding the fact that the hardware never answered.
func (supervisor *Supervisor) SweepCommands(ctx context.Context) error {
	now := supervisor.now()
	var expired []string
	err := farm.Mutate(ctx, supervisor.store, func(state json.RawMessage) (json.RawMessage, error) {
		var current document
		if err := json.Unmarshal(state, &current); err != nil {
			return nil, err
		}
		expired = expired[:0]
		changed := false
		for _, command := range current.Commands {
			status, _ := command["status"].(string)
			if terminalCommandStates[status] {
				continue
			}
			expiresAt, known := parseTime(command["expires_at"])
			if !known || !now.After(expiresAt) {
				continue
			}
			command["status"] = "timed_out"
			command["updated_at"] = now
			command["reason_code"] = "COMMAND_EXPIRED"
			if id, ok := command["id"].(string); ok {
				expired = append(expired, id)
			}
			changed = true
		}
		if !changed {
			return state, nil
		}
		return farm.ReplaceKeys(state, map[string]any{"commands": current.Commands})
	})
	if errors.Is(err, farm.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, id := range expired {
		supervisor.logger.Warn("command_timed_out", "command", id)
		supervisor.record(ctx, farm.AuditEntry{
			Actor: "system", Action: "command.timed_out", TargetType: "command", TargetID: id,
			Detail: map[string]any{"reason": "no acknowledgement before expiry"},
		})
	}
	if len(expired) > 0 {
		supervisor.notify("commands")
	}
	return nil
}

// PruneTelemetry enforces the configured retention window. It is a no-op until a
// deployment chooses a policy, because silently discarding a grower's history
// would be worse than keeping it.
func (supervisor *Supervisor) PruneTelemetry(ctx context.Context) error {
	if supervisor.config.TelemetryRetention <= 0 {
		return nil
	}
	cutoff := supervisor.now().Add(-supervisor.config.TelemetryRetention)
	removed, err := supervisor.telemetry.Prune(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("prune telemetry: %w", err)
	}
	if removed > 0 {
		supervisor.logger.Info("telemetry_pruned", "rows", removed, "cutoff", cutoff)
	}
	return nil
}

// EvaluateAlerts marks stale devices offline, derives the active rule set from
// the running grows, and applies the resulting transitions to the document.
func (supervisor *Supervisor) EvaluateAlerts(ctx context.Context) error {
	now := supervisor.now()
	latest, err := supervisor.telemetry.Latest(ctx)
	if err != nil {
		return fmt.Errorf("read latest measurements: %w", err)
	}
	samples := make(map[string]alert.Sample, len(latest))
	for _, measurement := range latest {
		samples[measurement.ChannelID] = alert.Sample{
			Value: measurement.Value, ObservedAt: measurement.ObservedAt,
			Quality: string(measurement.Quality), Present: true,
		}
	}

	// The transitions are planned once, from a single consistent read. Planning
	// inside the write loop would be a side effect in a mutator that can run more
	// than once: the first pass would mark the alert open in the tracker, a write
	// conflict would discard it, and the retry would see nothing new to say — so
	// the alert would be silently lost rather than retried.
	state, _, err := supervisor.store.Load(ctx)
	if errors.Is(err, farm.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	var planning document
	if err := json.Unmarshal(state, &planning); err != nil {
		return err
	}
	supervisor.markStaleDevicesOffline(&planning, now)
	planned := supervisor.tracker.Plan(buildSnapshot(planning, samples), now)

	err = farm.Mutate(ctx, supervisor.store, func(state json.RawMessage) (json.RawMessage, error) {
		var current document
		if err := json.Unmarshal(state, &current); err != nil {
			return nil, err
		}
		// Both steps below are idempotent, which is what makes them safe to run
		// again on a retry.
		devicesChanged := supervisor.markStaleDevicesOffline(&current, now)
		alerts, alertsChanged := applyTransitions(current.Alerts, planned, now)
		if !devicesChanged && !alertsChanged {
			return state, nil
		}
		return farm.ReplaceKeys(state, map[string]any{"devices": current.Devices, "alerts": alerts})
	})
	if errors.Is(err, farm.ErrNotFound) {
		return nil
	}
	if err != nil {
		// The tracker is deliberately not advanced: the next pass re-plans the
		// same transitions and tries again.
		return err
	}
	supervisor.tracker.Commit(planned)
	if len(planned) > 0 {
		supervisor.notify("alerts")
	}
	return nil
}

// markStaleDevicesOffline is what makes offline detection work without the
// device cooperating: liveness is inferred from heartbeat age, not from a
// device politely announcing that it left.
func (supervisor *Supervisor) markStaleDevicesOffline(current *document, now time.Time) bool {
	if supervisor.config.DeviceOfflineAfter <= 0 {
		return false
	}
	changed := false
	for index := range current.Devices {
		device := &current.Devices[index]
		if !device.Online {
			continue
		}
		heartbeat, known := parseTime(device.LastHeartbeat)
		if !known || now.Sub(heartbeat) <= supervisor.config.DeviceOfflineAfter {
			continue
		}
		device.Online = false
		changed = true
		supervisor.logger.Warn("device_marked_offline", "device", device.ID, "last_heartbeat", device.LastHeartbeat)
	}
	return changed
}

// buildSnapshot derives the live rule set from the active grows: a channel is
// only range-checked against the stage its grow is actually in.
func buildSnapshot(current document, samples map[string]alert.Sample) alert.Snapshot {
	stagesByVersion := map[string]map[string]string{}
	for _, stage := range current.RecipeStages {
		byKey, found := stagesByVersion[stage.RecipeVersionID]
		if !found {
			byKey = map[string]string{}
			stagesByVersion[stage.RecipeVersionID] = byKey
		}
		byKey[stage.Key] = stage.ID
	}
	activeStages := map[string]bool{}
	for _, grow := range current.GrowCycles {
		if grow.Status != "active" {
			continue
		}
		if stageID, found := stagesByVersion[grow.RecipeVersionID][grow.StageKey]; found {
			activeStages[stageID] = true
		}
	}

	channelsByKey := map[string]channelRecord{}
	channelDevice := map[string]string{}
	for _, channel := range current.Channels {
		channelsByKey[channel.Key] = channel
		channelDevice[channel.ID] = channel.DeviceID
	}
	deviceOnline := map[string]bool{}
	deviceLabel := map[string]string{}
	for _, device := range current.Devices {
		deviceOnline[device.ID] = device.Online
		deviceLabel[device.ID] = device.Name
	}

	var rules []alert.Rule
	for _, setpoint := range current.Setpoints {
		if !activeStages[setpoint.StageID] {
			continue
		}
		channel, found := channelsByKey[setpoint.ChannelKey]
		if !found || channel.Kind != "measurement" {
			continue
		}
		// A setpoint in different units than its channel is a configuration
		// error, not a breach; evaluating it would compare unrelated numbers.
		if setpoint.Unit != "" && channel.Unit != "" && setpoint.Unit != channel.Unit {
			continue
		}
		stale := time.Duration(setpoint.StaleAfterSeconds) * time.Second
		if stale <= 0 {
			stale = time.Duration(channel.StaleAfterSeconds) * time.Second
		}
		duration := time.Duration(0)
		if setpoint.WarningDurationMinutes != nil {
			duration = time.Duration(*setpoint.WarningDurationMinutes) * time.Minute
		}
		rules = append(rules, alert.Rule{
			Key: setpoint.ChannelKey, EntityType: channel.EntityType, EntityID: channel.EntityID,
			ChannelID: channel.ID, Label: channel.Name, Severity: alert.SeverityWarning,
			Minimum: setpoint.Minimum, Maximum: setpoint.Maximum,
			Hysteresis: hysteresisFor(setpoint), Duration: duration, StaleAfter: stale,
		})
	}
	return alert.Snapshot{
		Rules: rules, Samples: samples, DeviceOnline: deviceOnline,
		ChannelDevice: channelDevice, DeviceLabel: deviceLabel,
	}
}

// hysteresisFor derives a recovery band from the target range. Two percent of
// the span is wide enough to absorb sensor noise without hiding a real
// excursion, and it scales with the quantity being measured.
func hysteresisFor(setpoint setpointRecord) float64 {
	if setpoint.Minimum == nil || setpoint.Maximum == nil {
		return 0
	}
	span := *setpoint.Maximum - *setpoint.Minimum
	if span <= 0 {
		return 0
	}
	return span * 0.02
}

// applyTransitions folds alert decisions into the stored alert collection,
// preserving acknowledgement state so a transition does not silently un-
// acknowledge an alert an operator has already seen.
func applyTransitions(alerts []map[string]any, transitions []alert.Transition, now time.Time) ([]map[string]any, bool) {
	changed := false
	indexOf := func(identity alert.Identity) int {
		for index, record := range alerts {
			if identityOf(record) != identity {
				continue
			}
			if status, _ := record["status"].(string); status == "resolved" {
				continue
			}
			return index
		}
		return -1
	}
	for _, transition := range transitions {
		existing := indexOf(transition.Identity)
		if transition.Action == alert.ActionResolve {
			if existing < 0 {
				continue
			}
			alerts[existing]["status"] = "resolved"
			alerts[existing]["resolved_at"] = now
			changed = true
			continue
		}
		if existing >= 0 {
			continue
		}
		alerts = append(alerts, map[string]any{
			"id": uuid.NewString(), "definition_key": transition.Identity.Key,
			"condition":   string(transition.Identity.Condition),
			"entity_type": transition.Identity.EntityType, "entity_id": transition.Identity.EntityID,
			"severity": string(transition.Severity), "status": "open",
			"title": transition.Title, "detail": transition.Detail, "opened_at": now,
		})
		changed = true
	}
	return alerts, changed
}

// SyncEdgeConfig publishes a controller's desired configuration whenever the
// version it reports differs from the version the farm intends. The message is
// retained, so a controller that reboots during a server outage still recovers
// its schedules from the broker.
func (supervisor *Supervisor) SyncEdgeConfig(ctx context.Context) error {
	if supervisor.publisher == nil {
		return nil
	}
	state, _, err := supervisor.store.Load(ctx)
	if errors.Is(err, farm.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	var current document
	if err := json.Unmarshal(state, &current); err != nil {
		return err
	}
	for _, device := range current.Devices {
		if len(device.DesiredConfig) == 0 || device.DesiredConfigVersion == "" {
			continue
		}
		if device.ActiveConfigVersion == device.DesiredConfigVersion {
			continue
		}
		supervisor.mu.Lock()
		alreadySent := supervisor.publishedConfig[device.ID] == device.DesiredConfigVersion
		supervisor.mu.Unlock()
		if alreadySent {
			continue
		}
		payload, err := json.Marshal(map[string]any{
			"protocolVersion": 1, "deviceId": device.ID,
			"configVersion": device.DesiredConfigVersion, "issuedAt": supervisor.now(),
			"config": device.DesiredConfig,
		})
		if err != nil {
			return err
		}
		if err := supervisor.publisher.PublishConfig(ctx, device.ID, payload); err != nil {
			supervisor.logger.Warn("edge_config_publish_failed", "device", device.ID, "error", err)
			continue
		}
		supervisor.mu.Lock()
		supervisor.publishedConfig[device.ID] = device.DesiredConfigVersion
		supervisor.mu.Unlock()
		supervisor.logger.Info("edge_config_published", "device", device.ID, "version", device.DesiredConfigVersion)
		supervisor.record(ctx, farm.AuditEntry{
			Actor: "system", Action: "edge_config.published", TargetType: "device", TargetID: device.ID,
			Detail: map[string]any{"config_version": device.DesiredConfigVersion},
		})
	}
	return nil
}
