package runtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/alert"
	"github.com/jdanielsobrado/grownerve/internal/farm"
	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

// Config tunes the background jobs. Every interval is explicit so an operator
// can see and change the cadence rather than discovering it in code.
type Config struct {
	CommandSweepInterval time.Duration
	AlertInterval        time.Duration
	RetentionInterval    time.Duration
	ConfigSyncInterval   time.Duration
	OutboxInterval       time.Duration
	// TelemetryRetention is how long measurements are kept. Zero disables
	// pruning, which is the safe default for a deployment that has not chosen a
	// retention policy yet.
	TelemetryRetention time.Duration
	// DeviceOfflineAfter is how long a device may go without a heartbeat before
	// it is treated as offline.
	DeviceOfflineAfter time.Duration
}

// DefaultConfig is tuned for the reference installation: fast enough that an
// operator sees a fault promptly, slow enough not to churn the document.
func DefaultConfig() Config {
	return Config{
		CommandSweepInterval: 5 * time.Second,
		AlertInterval:        15 * time.Second,
		RetentionInterval:    time.Hour,
		ConfigSyncInterval:   30 * time.Second,
		OutboxInterval:       10 * time.Second,
		TelemetryRetention:   0,
		DeviceOfflineAfter:   2 * time.Minute,
	}
}

// ConfigPublisher delivers retained edge configuration to a controller.
type ConfigPublisher interface {
	PublishConfig(ctx context.Context, deviceID string, payload []byte) error
}

// Supervisor runs the server's periodic work.
type Supervisor struct {
	store     farm.Store
	telemetry telemetry.Store
	notifier  farm.Notifier
	publisher ConfigPublisher
	outbox    Draining
	audit     farm.AuditRecorder
	logger    *slog.Logger
	config    Config

	tracker *alert.Tracker
	now     func() time.Time

	mu              sync.Mutex
	publishedConfig map[string]string
}

type Option func(*Supervisor)

func WithNotifier(notifier farm.Notifier) Option {
	return func(supervisor *Supervisor) { supervisor.notifier = notifier }
}

// Draining is the outbox worker's contract: publish whatever is queued.
type Draining interface {
	Drain(ctx context.Context) error
}

// WithOutbox drains queued command publications on a schedule so a command that
// could not reach the broker when it was issued still gets there.
func WithOutbox(worker Draining) Option {
	return func(supervisor *Supervisor) { supervisor.outbox = worker }
}

func WithConfigPublisher(publisher ConfigPublisher) Option {
	return func(supervisor *Supervisor) { supervisor.publisher = publisher }
}

func WithAuditRecorder(recorder farm.AuditRecorder) Option {
	return func(supervisor *Supervisor) { supervisor.audit = recorder }
}

func WithClock(now func() time.Time) Option {
	return func(supervisor *Supervisor) { supervisor.now = now }
}

func New(store farm.Store, samples telemetry.Store, logger *slog.Logger, config Config, options ...Option) *Supervisor {
	supervisor := &Supervisor{
		store: store, telemetry: samples, logger: logger, config: config,
		tracker: alert.NewTracker(), now: func() time.Time { return time.Now().UTC() },
		publishedConfig: map[string]string{},
	}
	for _, option := range options {
		option(supervisor)
	}
	return supervisor
}

// Start launches every job. Each runs on its own ticker so a slow job cannot
// delay an unrelated one, and all of them stop when the context is cancelled.
func (supervisor *Supervisor) Start(ctx context.Context) {
	supervisor.restoreTracker(ctx)
	jobs := []struct {
		name     string
		interval time.Duration
		run      func(context.Context) error
	}{
		{"command_sweep", supervisor.config.CommandSweepInterval, supervisor.SweepCommands},
		{"alert_evaluation", supervisor.config.AlertInterval, supervisor.EvaluateAlerts},
		{"telemetry_retention", supervisor.config.RetentionInterval, supervisor.PruneTelemetry},
		{"edge_config_sync", supervisor.config.ConfigSyncInterval, supervisor.SyncEdgeConfig},
		{"outbox_drain", supervisor.config.OutboxInterval, supervisor.DrainOutbox},
	}
	for _, job := range jobs {
		if job.interval <= 0 {
			continue
		}
		go supervisor.loop(ctx, job.name, job.interval, job.run)
	}
}

func (supervisor *Supervisor) loop(ctx context.Context, name string, interval time.Duration, run func(context.Context) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := run(ctx); err != nil {
				// A failing job is logged and retried on the next tick rather
				// than terminating the loop, so one bad document cannot stop
				// alerting for the rest of the process lifetime.
				supervisor.logger.Warn("background_job_failed", "job", name, "error", err)
			}
		}
	}
}

// restoreTracker seeds alert state from the document so a restart neither
// reopens existing alerts nor forgets them.
func (supervisor *Supervisor) restoreTracker(ctx context.Context) {
	state, _, err := supervisor.store.Load(ctx)
	if err != nil {
		return
	}
	var current document
	if json.Unmarshal(state, &current) != nil {
		return
	}
	var open []alert.Identity
	for _, record := range current.Alerts {
		status, _ := record["status"].(string)
		if status == "resolved" {
			continue
		}
		open = append(open, identityOf(record))
	}
	supervisor.tracker.Restore(open)
}

func identityOf(record map[string]any) alert.Identity {
	text := func(key string) string { value, _ := record[key].(string); return value }
	condition := text("condition")
	if condition == "" {
		condition = text("definition_key")
	}
	return alert.Identity{
		Key: text("definition_key"), EntityType: text("entity_type"),
		EntityID: text("entity_id"), Condition: alert.Condition(condition),
	}
}

// DrainOutbox publishes queued messages. It is a no-op when no outbox is wired.
func (supervisor *Supervisor) DrainOutbox(ctx context.Context) error {
	if supervisor.outbox == nil {
		return nil
	}
	return supervisor.outbox.Drain(ctx)
}

func (supervisor *Supervisor) notify(topic string) {
	if supervisor.notifier != nil {
		supervisor.notifier.Notify(topic)
	}
}

func (supervisor *Supervisor) record(ctx context.Context, entry farm.AuditEntry) {
	if supervisor.audit == nil {
		return
	}
	if entry.OccurredAt.IsZero() {
		entry.OccurredAt = supervisor.now()
	}
	supervisor.audit.Record(ctx, entry)
}
