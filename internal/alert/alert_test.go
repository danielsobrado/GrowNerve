package alert

import (
	"testing"
	"time"
)

func pointer(value float64) *float64 { return &value }

var origin = time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)

func temperatureRule() Rule {
	return Rule{
		Key: "air_temperature_range", EntityType: "zone", EntityID: "zone-1",
		ChannelID: "channel-1", Label: "Air temperature", Severity: SeverityWarning,
		Minimum: pointer(18), Maximum: pointer(26), Hysteresis: 1,
		Duration: 5 * time.Minute, StaleAfter: 10 * time.Minute,
	}
}

func snapshotAt(value float64, observed time.Time) Snapshot {
	return Snapshot{
		Rules:         []Rule{temperatureRule()},
		Samples:       map[string]Sample{"channel-1": {Value: value, ObservedAt: observed, Quality: "good", Present: true}},
		DeviceOnline:  map[string]bool{"device-1": true},
		ChannelDevice: map[string]string{"channel-1": "device-1"},
		DeviceLabel:   map[string]string{"device-1": "Tent controller"},
	}
}

func find(transitions []Transition, condition Condition) (Transition, bool) {
	for _, transition := range transitions {
		if transition.Identity.Condition == condition {
			return transition, true
		}
	}
	return Transition{}, false
}

func TestBreachMustPersistBeforeAlertOpens(t *testing.T) {
	tracker := NewTracker()

	// A single high sample must not open an alert; the duration has not elapsed.
	if transitions := tracker.Evaluate(snapshotAt(30, origin), origin); len(transitions) != 0 {
		t.Fatalf("spike opened an alert immediately: %+v", transitions)
	}

	// Still breaching, but not yet for long enough.
	later := origin.Add(4 * time.Minute)
	if transitions := tracker.Evaluate(snapshotAt(30, later), later); len(transitions) != 0 {
		t.Fatalf("alert opened before the duration elapsed: %+v", transitions)
	}

	elapsed := origin.Add(6 * time.Minute)
	transitions := tracker.Evaluate(snapshotAt(30, elapsed), elapsed)
	opened, found := find(transitions, ConditionAboveRange)
	if !found || opened.Action != ActionOpen {
		t.Fatalf("alert did not open after the duration elapsed: %+v", transitions)
	}
	if opened.Severity != SeverityWarning {
		t.Fatalf("severity = %q", opened.Severity)
	}
}

func TestPersistentBreachIsNotDuplicated(t *testing.T) {
	tracker := NewTracker()
	for offset := 0; offset <= 6; offset++ {
		at := origin.Add(time.Duration(offset) * time.Minute)
		tracker.Evaluate(snapshotAt(30, at), at)
	}
	if tracker.OpenCount() != 1 {
		t.Fatalf("open alerts = %d, want 1", tracker.OpenCount())
	}

	// Twenty more evaluations of the same fault must not create more alerts.
	for offset := 7; offset < 27; offset++ {
		at := origin.Add(time.Duration(offset) * time.Minute)
		if transitions := tracker.Evaluate(snapshotAt(30, at), at); len(transitions) != 0 {
			t.Fatalf("duplicate transition at minute %d: %+v", offset, transitions)
		}
	}
}

func TestHysteresisPreventsFlapping(t *testing.T) {
	tracker := NewTracker()
	open := origin.Add(6 * time.Minute)
	for _, at := range []time.Time{origin, open} {
		tracker.Evaluate(snapshotAt(30, at), at)
	}
	if tracker.OpenCount() != 1 {
		t.Fatal("alert did not open")
	}

	// 25.5 is back inside the 26 maximum but within the 1.0 hysteresis band, so
	// the alert must hold rather than clear and immediately reopen.
	held := open.Add(time.Minute)
	if transitions := tracker.Evaluate(snapshotAt(25.5, held), held); len(transitions) != 0 {
		t.Fatalf("alert cleared inside the hysteresis band: %+v", transitions)
	}

	// 24.5 has recovered past the band, so it clears.
	recovered := held.Add(time.Minute)
	transitions := tracker.Evaluate(snapshotAt(24.5, recovered), recovered)
	resolved, found := find(transitions, ConditionAboveRange)
	if !found || resolved.Action != ActionResolve {
		t.Fatalf("alert did not clear after recovering past the band: %+v", transitions)
	}
	if tracker.OpenCount() != 0 {
		t.Fatalf("open alerts after recovery = %d", tracker.OpenCount())
	}
}

func TestStaleReadingOpensImmediatelyAndSuppressesRangeChecks(t *testing.T) {
	tracker := NewTracker()
	rule := temperatureRule()
	rule.Duration = 0
	snapshot := snapshotAt(30, origin.Add(-30*time.Minute))
	snapshot.Rules = []Rule{rule}

	transitions := tracker.Evaluate(snapshot, origin)
	stale, found := find(transitions, ConditionSensorStale)
	if !found || stale.Action != ActionOpen {
		t.Fatalf("stale reading did not alert: %+v", transitions)
	}
	// A stale value must not also be range-checked: the number is not current.
	if _, alsoHigh := find(transitions, ConditionAboveRange); alsoHigh {
		t.Fatal("stale channel was also range-checked")
	}
}

func TestOfflineDeviceReplacesChannelAlerts(t *testing.T) {
	tracker := NewTracker()
	rule := temperatureRule()
	rule.Duration = 0
	snapshot := snapshotAt(30, origin)
	snapshot.Rules = []Rule{rule}
	snapshot.DeviceOnline["device-1"] = false

	transitions := tracker.Evaluate(snapshot, origin)
	offline, found := find(transitions, ConditionDeviceOffline)
	if !found || offline.Action != ActionOpen {
		t.Fatalf("offline device did not alert: %+v", transitions)
	}
	if offline.Title != "Tent controller is offline" {
		t.Fatalf("title = %q", offline.Title)
	}
	// An offline provider explains the missing data, so reporting staleness too
	// would be noise.
	if _, alsoStale := find(transitions, ConditionSensorStale); alsoStale {
		t.Fatal("offline device also raised a staleness alert")
	}
}

func TestFaultQualityIsNotRangeChecked(t *testing.T) {
	tracker := NewTracker()
	rule := temperatureRule()
	rule.Duration = 0
	snapshot := snapshotAt(999, origin)
	snapshot.Rules = []Rule{rule}
	snapshot.Samples["channel-1"] = Sample{Value: 999, ObservedAt: origin, Quality: "fault", Present: true}

	transitions := tracker.Evaluate(snapshot, origin)
	if _, ranged := find(transitions, ConditionAboveRange); ranged {
		t.Fatal("a faulted reading was range-checked")
	}
	if _, flagged := find(transitions, ConditionSensorStale); !flagged {
		t.Fatalf("a faulted reading raised no alert: %+v", transitions)
	}
}

func TestRestoredAlertsAreNotReopened(t *testing.T) {
	tracker := NewTracker()
	identity := Identity{Key: "air_temperature_range", EntityType: "zone", EntityID: "zone-1", Condition: ConditionAboveRange}
	tracker.Restore([]Identity{identity})

	at := origin.Add(time.Hour)
	if transitions := tracker.Evaluate(snapshotAt(30, at), at); len(transitions) != 0 {
		t.Fatalf("restart reopened an existing alert: %+v", transitions)
	}
}

func TestRuleDisappearingResolvesItsAlert(t *testing.T) {
	tracker := NewTracker()
	rule := temperatureRule()
	rule.Duration = 0
	snapshot := snapshotAt(30, origin)
	snapshot.Rules = []Rule{rule}
	tracker.Evaluate(snapshot, origin)
	if tracker.OpenCount() != 1 {
		t.Fatal("alert did not open")
	}

	// The rule is removed, for example because the recipe stage advanced.
	empty := Snapshot{Rules: nil}
	transitions := tracker.Evaluate(empty, origin.Add(time.Minute))
	if len(transitions) != 1 || transitions[0].Action != ActionResolve {
		t.Fatalf("removed rule left its alert open: %+v", transitions)
	}
}
