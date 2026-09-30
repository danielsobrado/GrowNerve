package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/farm"
	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

const (
	storeTimeout         = 5 * time.Second
	executeTimeout       = 5 * time.Second
	indexRefreshInterval = 15 * time.Second
	executedMemory       = 10 * time.Minute
	minimumKickSpacing   = time.Second
)

// Dependencies are the farm services the Manager works through.
type Dependencies struct {
	Store     farm.Store
	Committer farm.StateCommitter
	Telemetry telemetry.Store
	Notifier  farm.Notifier
	Audit     farm.AuditRecorder
	Logger    *slog.Logger
	Now       func() time.Time
	// LivenessInterval is how often available adopted devices get a heartbeat.
	LivenessInterval time.Duration
}

// Manager binds adapters to the farm.
type Manager struct {
	deps     Dependencies
	adapters map[Provider]Adapter
	tracker  *tracker

	mu       sync.Mutex
	index    *bindingIndex
	executed map[string]time.Time

	livenessKick chan struct{}
	indexKick    chan struct{}
}

func NewManager(deps Dependencies, adapters ...Adapter) *Manager {
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Now().UTC() }
	}
	if deps.Logger == nil {
		deps.Logger = slog.New(slog.DiscardHandler)
	}
	if deps.LivenessInterval <= 0 {
		deps.LivenessInterval = 30 * time.Second
	}
	manager := &Manager{
		deps: deps, adapters: map[Provider]Adapter{}, tracker: newTracker(),
		index: emptyIndex(), executed: map[string]time.Time{},
		livenessKick: make(chan struct{}, 1), indexKick: make(chan struct{}, 1),
	}
	for _, adapter := range adapters {
		manager.adapters[adapter.Provider()] = adapter
	}
	return manager
}

// Enabled reports whether any adapter is configured.
func (manager *Manager) Enabled() bool { return len(manager.adapters) > 0 }

// Start launches the adapters and the Manager's background loops.
func (manager *Manager) Start(ctx context.Context) {
	manager.refreshIndex(ctx)
	for _, adapter := range manager.adapters {
		adapter.Start(ctx, manager)
	}
	go manager.loop(ctx, "integration_index", indexRefreshInterval, manager.indexKick, manager.refreshIndex)
	go manager.loop(ctx, "integration_liveness", manager.deps.LivenessInterval, manager.livenessKick, manager.livenessPass)
}

func (manager *Manager) loop(ctx context.Context, name string, interval time.Duration, kick <-chan struct{}, run func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-kick:
			if wait := minimumKickSpacing - time.Since(last); wait > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(wait):
				}
			}
		}
		last = time.Now()
		manager.guard(name, func() { run(ctx) })
	}
}

// guard keeps one misbehaving pass from taking the API process down with it.
func (manager *Manager) guard(name string, run func()) {
	defer func() {
		if recovered := recover(); recovered != nil {
			manager.deps.Logger.Error("integration_panic", "job", name, "panic", recovered, "stack", string(debug.Stack()))
		}
	}()
	run()
}

func kick(channel chan struct{}) {
	select {
	case channel <- struct{}{}:
	default:
	}
}

// Notify implements farm.Notifier: a state write may have adopted, renamed or
// removed bound devices, so the binding index is rebuilt.
func (manager *Manager) Notify(topic string) {
	if topic == "state" {
		kick(manager.indexKick)
	}
}

func (manager *Manager) notify(topic string) {
	if manager.deps.Notifier != nil {
		manager.deps.Notifier.Notify(topic)
	}
}

func (manager *Manager) refreshIndex(ctx context.Context) {
	loadContext, cancel := context.WithTimeout(ctx, storeTimeout)
	defer cancel()
	state, _, err := manager.deps.Store.Load(loadContext)
	if errors.Is(err, farm.ErrNotFound) {
		manager.setIndex(emptyIndex())
		return
	}
	if err != nil {
		manager.deps.Logger.Warn("integration_index_load_failed", "error", err)
		return
	}
	index, err := buildIndex(state)
	if err != nil {
		manager.deps.Logger.Warn("integration_index_invalid", "error", err)
		return
	}
	manager.setIndex(index)
}

func (manager *Manager) setIndex(index *bindingIndex) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.index = index
}

func (manager *Manager) currentIndex() *bindingIndex {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.index
}

// --- Sink -------------------------------------------------------------------

func (manager *Manager) AvailabilityChanged(Provider) { kick(manager.livenessKick) }

func (manager *Manager) DevicesChanged(Provider) { manager.notify("integrations") }

func (manager *Manager) StatusChanged(Provider) {
	manager.notify("integrations")
	kick(manager.livenessKick)
}

// Readings records values for bound channels and confirms pending commands.
// Values for capabilities nobody adopted are dropped here, not stored.
func (manager *Manager) Readings(provider Provider, readings []Reading) {
	manager.guard("integration_readings", func() { manager.record(provider, readings) })
}

func (manager *Manager) record(provider Provider, readings []Reading) {
	now := manager.deps.Now()
	for _, reading := range readings {
		for _, pending := range manager.tracker.match(provider, reading, now) {
			manager.acknowledge(pending.deviceID, pending.commandID, OutcomeApplied, "", pending.appliedValue(reading))
		}
	}

	index := manager.currentIndex()
	var measurements []telemetry.Measurement
	observations := map[string]farm.DeviceObservation{}
	for _, reading := range readings {
		device, bound := index.devices[externalRef{provider, reading.ExternalID}]
		if !bound {
			continue
		}
		observedAt := reading.ObservedAt
		if observedAt.IsZero() || observedAt.After(now.Add(deviceprotocol.MaximumFutureClockSkew)) {
			observedAt = now
		}
		for _, channel := range index.channels[channelSlot(device.ID, reading.Key)] {
			value, convertible := ConvertTo(reading.Value, reading.Unit, channel.Unit)
			if !convertible {
				manager.deps.Logger.Warn("integration_unit_mismatch", "provider", provider, "device", device.ID,
					"channel", channel.ID, "reading_unit", reading.Unit, "channel_unit", channel.Unit)
				continue
			}
			measurements = append(measurements, telemetry.Measurement{
				ChannelID: channel.ID, ObservedAt: observedAt, ReceivedAt: now, Value: value,
				Unit: channel.Unit, Quality: telemetry.QualityGood, SourceDeviceID: device.ID,
			})
			if channel.Kind != KindCommand {
				continue
			}
			observation := observations[device.ID]
			if channel.ValueType == "boolean" {
				state := value != 0
				observation.State = &state
			} else if channel.Unit == "%" {
				percent := value
				observation.OutputPercent = &percent
			}
			observations[device.ID] = observation
		}
	}
	if len(measurements) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	if _, err := manager.deps.Telemetry.Append(ctx, measurements); err != nil {
		manager.deps.Logger.Error("integration_telemetry_append_failed", "provider", provider, "error", err)
		return
	}
	manager.notify("measurements")
	if changed, err := farm.ObserveDevices(ctx, manager.deps.Store, observations, now); err != nil {
		manager.deps.Logger.Warn("integration_device_state_failed", "provider", provider, "error", err)
	} else if changed {
		manager.notify("devices")
	}
}

// livenessPass gives every available adopted device a heartbeat and takes
// unavailable ones offline immediately. If this loop stops, heartbeats stop
// and the runtime supervisor takes the devices offline: the fail-closed
// backstop does not depend on this code being healthy.
func (manager *Manager) livenessPass(ctx context.Context) {
	index := manager.currentIndex()
	if len(index.byID) == 0 {
		return
	}
	observations := make(map[string]farm.DeviceObservation, len(index.byID))
	for _, device := range index.byID {
		adapter := manager.adapters[device.Provider]
		available := adapter != nil && adapter.Available(device.ExternalID)
		observations[device.ID] = farm.DeviceObservation{Online: &available, Heartbeat: available}
	}
	writeContext, cancel := context.WithTimeout(ctx, storeTimeout)
	defer cancel()
	changed, err := farm.ObserveDevices(writeContext, manager.deps.Store, observations, manager.deps.Now())
	if err != nil && !errors.Is(err, farm.ErrNotFound) {
		manager.deps.Logger.Warn("integration_liveness_failed", "error", err)
		return
	}
	if changed {
		manager.notify("devices")
	}
}

// --- Commands ---------------------------------------------------------------

// Deliver sends a command if deviceID is integration-managed. handled is false
// for any other device, which then belongs to the MQTT transport.
func (manager *Manager) Deliver(ctx context.Context, deviceID string, command deviceprotocol.Command) (bool, error) {
	state, _, err := manager.deps.Store.Load(ctx)
	if errors.Is(err, farm.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		// Unknown ownership: fail rather than guess. The outbox retries it.
		return true, fmt.Errorf("resolve command transport: %w", err)
	}
	var parsed document
	if err := json.Unmarshal(state, &parsed); err != nil {
		return true, fmt.Errorf("resolve command transport: %w", err)
	}
	var binding *DeviceIntegration
	for _, device := range parsed.Devices {
		if device.ID == deviceID {
			binding = device.Integration
			break
		}
	}
	if binding == nil {
		return false, nil
	}
	now := manager.deps.Now()
	if err := command.Validate(now); err != nil {
		return true, err
	}
	if manager.executedRecently(command.CommandID, now) {
		return true, nil
	}
	integrationKey := ""
	for _, channel := range parsed.Channels {
		if channel.ID == command.TargetChannelID && channel.DeviceID == deviceID {
			integrationKey = channel.IntegrationKey
			break
		}
	}
	if integrationKey == "" {
		manager.acknowledge(deviceID, command.CommandID, OutcomeRejected, "CHANNEL_NOT_BOUND", nil)
		return true, nil
	}
	adapter := manager.adapters[binding.Provider]
	if adapter == nil {
		manager.acknowledge(deviceID, command.CommandID, OutcomeRejected, "INTEGRATION_DISABLED", nil)
		return true, nil
	}
	if !adapter.Available(binding.ExternalID) {
		return true, fmt.Errorf("%s device %s is unavailable", binding.Provider, binding.ExternalID)
	}

	target := Command{ID: command.CommandID, ExternalID: binding.ExternalID, Key: integrationKey, ExpiresAt: command.ExpiresAt}
	pending := pendingCommand{
		commandID: command.CommandID, deviceID: deviceID, provider: binding.Provider,
		externalID: binding.ExternalID, key: integrationKey, expiresAt: command.ExpiresAt,
	}
	raw, _ := json.Marshal(command.Value)
	switch command.Type {
	case "set_boolean":
		var value bool
		if json.Unmarshal(raw, &value) != nil {
			manager.acknowledge(deviceID, command.CommandID, OutcomeRejected, "INVALID_COMMAND_VALUE", nil)
			return true, nil
		}
		target.Boolean, pending.boolean = &value, &value
	default:
		var value float64
		if json.Unmarshal(raw, &value) != nil {
			manager.acknowledge(deviceID, command.CommandID, OutcomeRejected, "INVALID_COMMAND_VALUE", nil)
			return true, nil
		}
		target.Percent, pending.percent = &value, &value
	}

	// Track before executing: a fast device can report the new state before
	// Execute returns.
	manager.tracker.add(pending)
	executeContext, cancel := context.WithTimeout(ctx, executeTimeout)
	defer cancel()
	outcome, err := adapter.Execute(executeContext, target)
	if err != nil {
		manager.tracker.remove(command.CommandID)
		return true, err
	}
	manager.markExecuted(command.CommandID, now)
	switch outcome.Result {
	case OutcomeApplied:
		manager.tracker.remove(command.CommandID)
		manager.acknowledge(deviceID, command.CommandID, OutcomeApplied, "", command.Value)
	case OutcomeAccepted:
		manager.acknowledge(deviceID, command.CommandID, OutcomeAccepted, "", nil)
	case OutcomeRejected:
		manager.tracker.remove(command.CommandID)
		reason := outcome.ReasonCode
		if reason == "" {
			reason = "PROVIDER_REJECTED"
		}
		manager.acknowledge(deviceID, command.CommandID, OutcomeRejected, reason, nil)
	}
	return true, nil
}

func (manager *Manager) executedRecently(commandID string, now time.Time) bool {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	at, found := manager.executed[commandID]
	return found && now.Sub(at) < executedMemory
}

func (manager *Manager) markExecuted(commandID string, now time.Time) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	for id, at := range manager.executed {
		if now.Sub(at) >= executedMemory {
			delete(manager.executed, id)
		}
	}
	manager.executed[commandID] = now
}

func (manager *Manager) acknowledge(deviceID, commandID, result, reason string, applied any) {
	now := manager.deps.Now()
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	err := farm.ApplyCommandAcknowledgement(ctx, manager.deps.Store, deviceID, deviceprotocol.Acknowledgement{
		ProtocolVersion: deviceprotocol.Version, CommandID: commandID, DeviceID: deviceID,
		Result: result, ReasonCode: reason, AppliedValue: applied, AcknowledgedAt: now,
	}, now)
	if err != nil {
		if !errors.Is(err, farm.ErrCommandAlreadyFinal) {
			manager.deps.Logger.Warn("integration_ack_rejected", "device", deviceID, "command", commandID, "result", result, "reason", err)
		}
		return
	}
	manager.notify("commands")
}

// --- Inventory ---------------------------------------------------------------

// Statuses reports every known provider, including disabled ones.
func (manager *Manager) Statuses() []Status {
	index := manager.currentIndex()
	statuses := make([]Status, 0, len(Providers))
	for _, provider := range Providers {
		adapter, enabled := manager.adapters[provider]
		if !enabled {
			statuses = append(statuses, Status{Provider: provider, State: StateDisabled})
			continue
		}
		status := adapter.Status()
		status.Provider, status.Enabled = provider, true
		status.DeviceCount = len(adapter.Devices())
		status.AdoptedCount = index.adoptedCount(provider)
		_, status.PermitJoin = adapter.(PermitJoiner)
		statuses = append(statuses, status)
	}
	return statuses
}

// ErrProviderDisabled reports a request for a provider that is not configured.
var ErrProviderDisabled = errors.New("integration provider is not enabled")

// Discovered lists the provider's devices, marking the adopted ones.
func (manager *Manager) Discovered(ctx context.Context, provider Provider) ([]DiscoveredDevice, error) {
	adapter, enabled := manager.adapters[provider]
	if !enabled {
		return nil, ErrProviderDisabled
	}
	manager.refreshIndex(ctx)
	index := manager.currentIndex()
	devices := adapter.Devices()
	for position := range devices {
		if bound, adopted := index.devices[externalRef{provider, devices[position].ExternalID}]; adopted {
			devices[position].AdoptedDeviceID = bound.ID
		}
	}
	return devices, nil
}

// PermitJoin opens the provider's network for new devices.
func (manager *Manager) PermitJoin(ctx context.Context, provider Provider, seconds int) (time.Time, error) {
	adapter, enabled := manager.adapters[provider]
	if !enabled {
		return time.Time{}, ErrProviderDisabled
	}
	joiner, supported := adapter.(PermitJoiner)
	if !supported {
		return time.Time{}, ErrUnsupported
	}
	until, err := joiner.PermitJoin(ctx, seconds)
	if err == nil {
		manager.notify("integrations")
	}
	return until, err
}

// ErrUnsupported reports an operation the provider does not offer.
var ErrUnsupported = errors.New("operation is not supported by this provider")
