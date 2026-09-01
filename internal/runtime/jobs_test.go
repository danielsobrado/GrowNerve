package runtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/farm"
	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

var testNow = time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC)

func testSupervisor(t *testing.T, state string, options ...Option) (*Supervisor, *farm.MemoryStore, *telemetry.MemoryStore) {
	t.Helper()
	store := farm.NewMemoryStore()
	if _, err := store.Save(context.Background(), json.RawMessage(state), farm.AnyVersion); err != nil {
		t.Fatal(err)
	}
	samples := telemetry.NewMemoryStore(0)
	config := DefaultConfig()
	options = append([]Option{WithClock(func() time.Time { return testNow })}, options...)
	return New(store, samples, slog.New(slog.NewTextHandler(io.Discard, nil)), config, options...), store, samples
}

func loadDocument(t *testing.T, store *farm.MemoryStore) map[string]any {
	t.Helper()
	state, _, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(state, &object); err != nil {
		t.Fatal(err)
	}
	return object
}

func TestSweepCommandsExpiresUnacknowledgedCommands(t *testing.T) {
	state := `{"commands":[
		{"id":"expired","status":"published","expires_at":"2026-04-12T08:59:00Z"},
		{"id":"live","status":"published","expires_at":"2026-04-12T09:05:00Z"},
		{"id":"done","status":"applied","expires_at":"2026-04-12T08:00:00Z"}
	],"devices":[],"channels":[],"alerts":[]}`
	supervisor, store, _ := testSupervisor(t, state)

	if err := supervisor.SweepCommands(context.Background()); err != nil {
		t.Fatal(err)
	}
	commands := loadDocument(t, store)["commands"].([]any)
	byID := map[string]map[string]any{}
	for _, entry := range commands {
		record := entry.(map[string]any)
		byID[record["id"].(string)] = record
	}
	if byID["expired"]["status"] != "timed_out" {
		t.Fatalf("expired command status = %v", byID["expired"]["status"])
	}
	if byID["expired"]["reason_code"] != "COMMAND_EXPIRED" {
		t.Fatalf("expired command reason = %v", byID["expired"]["reason_code"])
	}
	if byID["live"]["status"] != "published" {
		t.Fatalf("unexpired command was swept: %v", byID["live"]["status"])
	}
	// A command that already reached a terminal state must not be reopened.
	if byID["done"]["status"] != "applied" {
		t.Fatalf("applied command was swept: %v", byID["done"]["status"])
	}
}

func TestSweepCommandsPreservesUnmodelledCollections(t *testing.T) {
	state := `{"commands":[{"id":"expired","status":"published","expires_at":"2026-04-12T08:00:00Z"}],
		"harvests":[{"id":"h1","mass_g":420}],"devices":[],"channels":[],"alerts":[]}`
	supervisor, store, _ := testSupervisor(t, state)
	if err := supervisor.SweepCommands(context.Background()); err != nil {
		t.Fatal(err)
	}
	document := loadDocument(t, store)
	harvests, present := document["harvests"].([]any)
	if !present || len(harvests) != 1 {
		t.Fatalf("a background job dropped a collection it does not model: %v", document["harvests"])
	}
}

// alertState wires one active grow, one published stage with a temperature
// target, one channel and one online device.
const alertState = `{
  "devices":[{"id":"device-1","name":"Tent controller","online":true,"last_heartbeat":"2026-04-12T08:59:30Z"}],
  "channels":[{"id":"channel-1","device_id":"device-1","entity_type":"zone","entity_id":"zone-1","key":"air_temperature","name":"Air temperature","kind":"measurement","unit":"degC","stale_after_seconds":600}],
  "recipe_stages":[{"id":"stage-1","recipe_version_id":"version-1","key":"vegetative","name":"Vegetative"}],
  "grow_cycles":[{"id":"grow-1","recipe_version_id":"version-1","status":"active","stage_key":"vegetative","name":"Lettuce"}],
  "setpoints":[{"id":"setpoint-1","stage_id":"stage-1","channel_key":"air_temperature","unit":"degC","minimum":18,"maximum":26,"stale_after_seconds":600}],
  "alerts":[],"commands":[]
}`

func TestAlertEvaluationOpensAndResolvesAgainstStoredState(t *testing.T) {
	supervisor, store, samples := testSupervisor(t, alertState)
	ctx := context.Background()

	appendSample := func(value float64, at time.Time) {
		if _, err := samples.Append(ctx, []telemetry.Measurement{{
			ChannelID: "channel-1", ObservedAt: at, Value: value, Unit: "degC", Quality: telemetry.QualityGood,
		}}); err != nil {
			t.Fatal(err)
		}
	}

	appendSample(22, testNow)
	if err := supervisor.EvaluateAlerts(ctx); err != nil {
		t.Fatal(err)
	}
	if alerts := loadDocument(t, store)["alerts"].([]any); len(alerts) != 0 {
		t.Fatalf("an in-range reading raised an alert: %v", alerts)
	}

	appendSample(31, testNow.Add(time.Second))
	if err := supervisor.EvaluateAlerts(ctx); err != nil {
		t.Fatal(err)
	}
	alerts := loadDocument(t, store)["alerts"].([]any)
	if len(alerts) != 1 {
		t.Fatalf("out-of-range reading produced %d alerts, want 1: %v", len(alerts), alerts)
	}
	opened := alerts[0].(map[string]any)
	if opened["status"] != "open" || opened["condition"] != "ABOVE_RANGE" {
		t.Fatalf("alert = %v", opened)
	}
	if !strings.Contains(opened["detail"].(string), "26.00 maximum") {
		t.Fatalf("alert detail does not name the breached limit: %v", opened["detail"])
	}

	// Repeated evaluation of the same fault must not create a second alert.
	for pass := 0; pass < 5; pass++ {
		if err := supervisor.EvaluateAlerts(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if alerts := loadDocument(t, store)["alerts"].([]any); len(alerts) != 1 {
		t.Fatalf("repeated evaluation duplicated the alert: %d", len(alerts))
	}

	appendSample(21, testNow.Add(2*time.Second))
	if err := supervisor.EvaluateAlerts(ctx); err != nil {
		t.Fatal(err)
	}
	resolved := loadDocument(t, store)["alerts"].([]any)[0].(map[string]any)
	if resolved["status"] != "resolved" {
		t.Fatalf("recovered reading left the alert open: %v", resolved)
	}
}

func TestStaleHeartbeatMarksDeviceOfflineAndAlerts(t *testing.T) {
	supervisor, store, samples := testSupervisor(t, alertState)
	ctx := context.Background()
	if _, err := samples.Append(ctx, []telemetry.Measurement{{
		ChannelID: "channel-1", ObservedAt: testNow, Value: 22, Unit: "degC", Quality: telemetry.QualityGood,
	}}); err != nil {
		t.Fatal(err)
	}
	// Advance past the offline threshold without a new heartbeat.
	supervisor.now = func() time.Time { return testNow.Add(10 * time.Minute) }

	if err := supervisor.EvaluateAlerts(ctx); err != nil {
		t.Fatal(err)
	}
	document := loadDocument(t, store)
	device := document["devices"].([]any)[0].(map[string]any)
	if device["online"] != false {
		t.Fatalf("device with a stale heartbeat is still online: %v", device)
	}
	alerts := document["alerts"].([]any)
	if len(alerts) != 1 || alerts[0].(map[string]any)["condition"] != "DEVICE_OFFLINE" {
		t.Fatalf("offline device did not alert: %v", alerts)
	}
}

func TestOnlySetpointsForTheActiveStageAreEvaluated(t *testing.T) {
	// The grow is planned, not active, so its targets must not alert.
	state := strings.Replace(alertState, `"status":"active"`, `"status":"planned"`, 1)
	supervisor, store, samples := testSupervisor(t, state)
	ctx := context.Background()
	if _, err := samples.Append(ctx, []telemetry.Measurement{{
		ChannelID: "channel-1", ObservedAt: testNow, Value: 99, Unit: "degC", Quality: telemetry.QualityGood,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := supervisor.EvaluateAlerts(ctx); err != nil {
		t.Fatal(err)
	}
	if alerts := loadDocument(t, store)["alerts"].([]any); len(alerts) != 0 {
		t.Fatalf("a planned grow's targets were evaluated: %v", alerts)
	}
}

func TestRetentionIsDisabledUntilAPolicyIsChosen(t *testing.T) {
	supervisor, _, samples := testSupervisor(t, alertState)
	ctx := context.Background()
	if _, err := samples.Append(ctx, []telemetry.Measurement{{
		ChannelID: "channel-1", ObservedAt: testNow.Add(-365 * 24 * time.Hour), Value: 22, Unit: "degC", Quality: telemetry.QualityGood,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := supervisor.PruneTelemetry(ctx); err != nil {
		t.Fatal(err)
	}
	if remaining, _ := samples.Recent(ctx, 10); len(remaining) != 1 {
		t.Fatal("history was discarded with no retention policy configured")
	}

	supervisor.config.TelemetryRetention = 24 * time.Hour
	if err := supervisor.PruneTelemetry(ctx); err != nil {
		t.Fatal(err)
	}
	if remaining, _ := samples.Recent(ctx, 10); len(remaining) != 0 {
		t.Fatalf("retention did not prune old samples: %d remain", len(remaining))
	}
}

type recordingConfigPublisher struct{ sent map[string][]byte }

func (publisher *recordingConfigPublisher) PublishConfig(_ context.Context, deviceID string, payload []byte) error {
	if publisher.sent == nil {
		publisher.sent = map[string][]byte{}
	}
	publisher.sent[deviceID] = payload
	return nil
}

func TestEdgeConfigIsPublishedOnceUntilTheDeviceAdoptsIt(t *testing.T) {
	state := `{"devices":[{"id":"device-1","name":"Tent controller","online":true,
		"active_config_version":"c1","desired_config_version":"c2","desired_config":{"photoperiod":{"onHour":6,"offHour":24}}}],
		"channels":[],"alerts":[],"commands":[]}`
	publisher := &recordingConfigPublisher{}
	supervisor, store, _ := testSupervisor(t, state, WithConfigPublisher(publisher))
	ctx := context.Background()

	if err := supervisor.SyncEdgeConfig(ctx); err != nil {
		t.Fatal(err)
	}
	payload, sent := publisher.sent["device-1"]
	if !sent {
		t.Fatal("a device with a pending configuration received nothing")
	}
	if !strings.Contains(string(payload), `"configVersion":"c2"`) || !strings.Contains(string(payload), `"onHour":6`) {
		t.Fatalf("payload = %s", payload)
	}

	// Re-running must not republish the same version.
	publisher.sent = nil
	if err := supervisor.SyncEdgeConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if len(publisher.sent) != 0 {
		t.Fatal("the same configuration version was published twice")
	}

	// Once the device reports the version as active, nothing further is sent.
	updated := strings.Replace(state, `"active_config_version":"c1"`, `"active_config_version":"c2"`, 1)
	if _, err := store.Save(ctx, json.RawMessage(updated), farm.AnyVersion); err != nil {
		t.Fatal(err)
	}
	supervisor.publishedConfig = map[string]string{}
	if err := supervisor.SyncEdgeConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if len(publisher.sent) != 0 {
		t.Fatal("configuration was republished after the device adopted it")
	}
}

// conflictingStore fails the first N saves with a version conflict, standing in
// for a busy document that another writer keeps winning.
type conflictingStore struct {
	inner     *farm.MemoryStore
	failures  int
	attempted int
}

func (store *conflictingStore) Load(ctx context.Context) (json.RawMessage, int64, error) {
	return store.inner.Load(ctx)
}

func (store *conflictingStore) Save(ctx context.Context, state json.RawMessage, expected int64) (int64, error) {
	store.attempted++
	if store.attempted <= store.failures {
		return 0, farm.ErrVersionConflict
	}
	return store.inner.Save(ctx, state, expected)
}

// TestAlertSurvivesAWriteConflict is the regression guard for a bug found by
// running the real server: the alert transition was planned inside the write
// loop, so a conflict discarded it and the retry — seeing the tracker already
// marked it open — emitted nothing. The alert was lost permanently while the
// condition was still breaching.
func TestAlertSurvivesAWriteConflict(t *testing.T) {
	inner := farm.NewMemoryStore()
	if _, err := inner.Save(context.Background(), json.RawMessage(alertState), farm.AnyVersion); err != nil {
		t.Fatal(err)
	}
	store := &conflictingStore{inner: inner, failures: 3}

	samples := telemetry.NewMemoryStore(0)
	supervisor := New(store, samples, slog.New(slog.NewTextHandler(io.Discard, nil)), DefaultConfig(),
		WithClock(func() time.Time { return testNow }))

	ctx := context.Background()
	if _, err := samples.Append(ctx, []telemetry.Measurement{{
		ChannelID: "channel-1", ObservedAt: testNow, Value: 31, Unit: "degC", Quality: telemetry.QualityGood,
	}}); err != nil {
		t.Fatal(err)
	}

	if err := supervisor.EvaluateAlerts(ctx); err != nil {
		t.Fatal(err)
	}
	alerts := loadDocument(t, inner)["alerts"].([]any)
	if len(alerts) != 1 {
		t.Fatalf("the alert was lost to a write conflict: %d alerts stored", len(alerts))
	}
	if alerts[0].(map[string]any)["condition"] != "ABOVE_RANGE" {
		t.Fatalf("alert = %v", alerts[0])
	}
}

// TestAlertIsRetriedWhenTheWriteNeverSucceeds proves the tracker is not advanced
// by a failed write, so the next pass tries again.
func TestAlertIsRetriedWhenTheWriteNeverSucceeds(t *testing.T) {
	inner := farm.NewMemoryStore()
	if _, err := inner.Save(context.Background(), json.RawMessage(alertState), farm.AnyVersion); err != nil {
		t.Fatal(err)
	}
	// Enough failures to exhaust every retry inside one evaluation pass.
	store := &conflictingStore{inner: inner, failures: 1000}

	samples := telemetry.NewMemoryStore(0)
	supervisor := New(store, samples, slog.New(slog.NewTextHandler(io.Discard, nil)), DefaultConfig(),
		WithClock(func() time.Time { return testNow }))
	ctx := context.Background()
	if _, err := samples.Append(ctx, []telemetry.Measurement{{
		ChannelID: "channel-1", ObservedAt: testNow, Value: 31, Unit: "degC", Quality: telemetry.QualityGood,
	}}); err != nil {
		t.Fatal(err)
	}

	if err := supervisor.EvaluateAlerts(ctx); err == nil {
		t.Fatal("a persistently failing write reported success")
	}
	if loaded := loadDocument(t, inner)["alerts"].([]any); len(loaded) != 0 {
		t.Fatalf("an alert was stored despite every write failing: %v", loaded)
	}

	// The store recovers; the same condition must still produce the alert.
	store.failures = 0
	if err := supervisor.EvaluateAlerts(ctx); err != nil {
		t.Fatal(err)
	}
	if loaded := loadDocument(t, inner)["alerts"].([]any); len(loaded) != 1 {
		t.Fatalf("the alert was not retried after the store recovered: %d stored", len(loaded))
	}
}
