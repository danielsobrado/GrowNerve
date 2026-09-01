// Package alert decides when a farm condition deserves an operator's attention.
// The decision logic is pure: it takes a snapshot of the farm and the tracker's
// memory of previous evaluations, and returns the transitions to apply. Nothing
// here reads a database, so every rule can be tested against explicit inputs.
package alert

import (
	"fmt"
	"sort"
	"time"
)

// Condition names why an alert exists. It is part of the stable domain
// vocabulary, so wording changes must not change these values.
type Condition string

const (
	ConditionBelowRange    Condition = "BELOW_RANGE"
	ConditionAboveRange    Condition = "ABOVE_RANGE"
	ConditionSensorStale   Condition = "SENSOR_DATA_STALE"
	ConditionDeviceOffline Condition = "DEVICE_OFFLINE"
)

// Severity mirrors the schema's alert severities.
type Severity string

const (
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Rule is one continuously evaluated condition.
type Rule struct {
	// Key identifies the rule. Together with the entity it forms the
	// deduplication identity, so one persistent fault yields one alert.
	Key        string
	EntityType string
	EntityID   string
	ChannelID  string
	Label      string
	Severity   Severity

	Minimum *float64
	Maximum *float64
	// Hysteresis is the margin a value must recover by before the alert clears.
	// Without it a value sitting exactly on a limit flaps open and closed.
	Hysteresis float64
	// Duration is how long a breach must persist before the alert opens, which
	// suppresses single-sample spikes.
	Duration time.Duration
	// StaleAfter is how old the newest sample may be before the channel counts
	// as stale. Zero disables the staleness rule for this channel.
	StaleAfter time.Duration
}

// Sample is the newest reading for a channel, if any.
type Sample struct {
	Value      float64
	ObservedAt time.Time
	Quality    string
	Present    bool
}

// Snapshot is everything one evaluation pass needs.
type Snapshot struct {
	Rules   []Rule
	Samples map[string]Sample
	// DeviceOnline reports liveness per device, keyed by device id.
	DeviceOnline map[string]bool
	// ChannelDevice maps a channel to the device that provides it.
	ChannelDevice map[string]string
	// DeviceLabel supplies human-readable device names for alert text.
	DeviceLabel map[string]string
}

// Action is what to do with an alert identity.
type Action string

const (
	ActionOpen    Action = "open"
	ActionResolve Action = "resolve"
)

// Transition is one decision to apply to the alert store.
type Transition struct {
	Identity   Identity
	Action     Action
	Condition  Condition
	Severity   Severity
	Title      string
	Detail     string
	OccurredAt time.Time
}

// Identity deduplicates alerts. One rule breaching continuously produces a
// single alert rather than one per evaluation.
type Identity struct {
	Key        string    `json:"definition_key"`
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	Condition  Condition `json:"condition"`
}

func (identity Identity) String() string {
	return fmt.Sprintf("%s|%s|%s|%s", identity.Key, identity.EntityType, identity.EntityID, identity.Condition)
}

// Tracker remembers when each condition started breaching so the duration and
// hysteresis rules can be applied across evaluations.
type Tracker struct {
	breachingSince map[string]time.Time
	open           map[string]bool
}

func NewTracker() *Tracker {
	return &Tracker{breachingSince: map[string]time.Time{}, open: map[string]bool{}}
}

// Restore seeds the tracker from alerts already open in the store, so a server
// restart does not reopen every existing alert or lose their identities.
func (tracker *Tracker) Restore(open []Identity) {
	for _, identity := range open {
		tracker.open[identity.String()] = true
	}
}

// OpenCount reports how many alerts the tracker believes are open.
func (tracker *Tracker) OpenCount() int { return len(tracker.open) }

// Evaluate plans and commits in one step. It is the convenient form for tests
// and for callers that persist synchronously; a caller whose write can fail
// should use Plan and Commit so a failed write does not lose the transition.
func (tracker *Tracker) Evaluate(snapshot Snapshot, now time.Time) []Transition {
	transitions := tracker.Plan(snapshot, now)
	tracker.Commit(transitions)
	return transitions
}

// Plan returns the transitions this pass would apply, without recording that
// they happened. It is deliberately separate from Commit: persisting an alert
// can fail, and a tracker that had already marked the alert open would never
// emit it again — the alert would be lost rather than retried.
//
// Plan does advance breach timers, because the duration rule measures how long a
// condition has been true regardless of whether a write succeeded.
func (tracker *Tracker) Plan(snapshot Snapshot, now time.Time) []Transition {
	var transitions []Transition
	seen := map[string]bool{}

	isOpen := func(identity Identity) bool { return tracker.open[identity.String()] }
	for _, rule := range snapshot.Rules {
		for _, candidate := range evaluateRule(rule, snapshot, now, isOpen) {
			key := candidate.Identity.String()
			seen[key] = true
			transitions = append(transitions, tracker.plan(rule, candidate, now)...)
		}
	}

	// Anything the tracker still holds open that no rule reported this pass has
	// recovered; clear it so a fixed condition does not linger.
	for key, isOpen := range tracker.open {
		if !isOpen || seen[key] {
			continue
		}
		identity, ok := parseIdentity(key)
		if !ok {
			continue
		}
		transitions = append(transitions, Transition{
			Identity: identity, Action: ActionResolve, Condition: identity.Condition,
			Title: "Condition cleared", Detail: "The condition is no longer reported.", OccurredAt: now,
		})
	}

	sort.SliceStable(transitions, func(i, j int) bool {
		return transitions[i].Identity.String() < transitions[j].Identity.String()
	})
	return transitions
}

// candidate is a breach reported by a rule before duration and hysteresis are
// applied.
type candidate struct {
	Identity  Identity
	Breaching bool
	Title     string
	Detail    string
}

// Commit records the transitions as applied. Call it only after the transitions
// have been durably stored.
func (tracker *Tracker) Commit(transitions []Transition) {
	for _, transition := range transitions {
		key := transition.Identity.String()
		switch transition.Action {
		case ActionOpen:
			tracker.open[key] = true
		case ActionResolve:
			delete(tracker.open, key)
			delete(tracker.breachingSince, key)
		}
	}
}

func (tracker *Tracker) plan(rule Rule, reported candidate, now time.Time) []Transition {
	key := reported.Identity.String()
	if !reported.Breaching {
		delete(tracker.breachingSince, key)
		if tracker.open[key] {
			return []Transition{{
				Identity: reported.Identity, Action: ActionResolve, Condition: reported.Identity.Condition,
				Severity: rule.Severity, Title: reported.Title, Detail: reported.Detail, OccurredAt: now,
			}}
		}
		return nil
	}
	if tracker.open[key] {
		return nil
	}
	since, tracking := tracker.breachingSince[key]
	if !tracking {
		tracker.breachingSince[key] = now
		since = now
	}
	if now.Sub(since) < rule.Duration {
		return nil
	}
	return []Transition{{
		Identity: reported.Identity, Action: ActionOpen, Condition: reported.Identity.Condition,
		Severity: rule.Severity, Title: reported.Title, Detail: reported.Detail, OccurredAt: now,
	}}
}

// evaluateRule reports the current state of every condition a rule watches.
// isOpen supplies whether each condition is already alerting, which is what
// makes hysteresis directional: the threshold to clear is stricter than the
// threshold to open.
func evaluateRule(rule Rule, snapshot Snapshot, now time.Time, isOpen func(Identity) bool) []candidate {
	identity := func(condition Condition) Identity {
		return Identity{Key: rule.Key, EntityType: rule.EntityType, EntityID: rule.EntityID, Condition: condition}
	}
	label := rule.Label
	if label == "" {
		label = rule.ChannelID
	}

	// Device liveness is checked first: an offline provider explains a stale
	// channel, so reporting both would be noise.
	if deviceID, bound := snapshot.ChannelDevice[rule.ChannelID]; bound {
		if online, known := snapshot.DeviceOnline[deviceID]; known && !online {
			name := snapshot.DeviceLabel[deviceID]
			if name == "" {
				name = deviceID
			}
			return []candidate{{
				Identity: identity(ConditionDeviceOffline), Breaching: true,
				Title:  fmt.Sprintf("%s is offline", name),
				Detail: fmt.Sprintf("%s stopped reporting, so %s cannot be trusted.", name, label),
			}}
		}
	}

	sample, present := snapshot.Samples[rule.ChannelID]
	if !present || !sample.Present {
		if rule.StaleAfter <= 0 {
			return nil
		}
		return []candidate{{
			Identity: identity(ConditionSensorStale), Breaching: true,
			Title:  fmt.Sprintf("%s has no readings", label),
			Detail: "No measurement has been received for this channel.",
		}}
	}
	if rule.StaleAfter > 0 && now.Sub(sample.ObservedAt) > rule.StaleAfter {
		return []candidate{{
			Identity: identity(ConditionSensorStale), Breaching: true,
			Title: fmt.Sprintf("%s is stale", label),
			Detail: fmt.Sprintf("The newest reading is %s old, beyond the %s limit.",
				now.Sub(sample.ObservedAt).Round(time.Second), rule.StaleAfter),
		}}
	}

	// A quality flag the device set is authoritative: a sensor reporting a fault
	// must not be range-checked as though its number meant something.
	if sample.Quality == "fault" || sample.Quality == "unknown" {
		return []candidate{{
			Identity: identity(ConditionSensorStale), Breaching: true,
			Title:  fmt.Sprintf("%s reports a fault", label),
			Detail: fmt.Sprintf("The device flagged this reading as %q, so it cannot be evaluated.", sample.Quality),
		}}
	}

	candidates := []candidate{{
		Identity: identity(ConditionSensorStale), Breaching: false,
		Title: fmt.Sprintf("%s is reporting again", label), Detail: "Fresh readings have resumed.",
	}}
	if rule.Minimum != nil {
		low := identity(ConditionBelowRange)
		// Opening needs the value below the limit; clearing needs it back above
		// the limit by the hysteresis band. A value between the two thresholds
		// keeps whichever state it is already in, which is what stops flapping.
		threshold := *rule.Minimum
		if isOpen(low) {
			threshold += rule.Hysteresis
		}
		candidates = append(candidates, candidate{
			Identity: low, Breaching: sample.Value < threshold,
			Title:  fmt.Sprintf("%s below target", label),
			Detail: fmt.Sprintf("Reading %.2f is below the %.2f minimum.", sample.Value, *rule.Minimum),
		})
	}
	if rule.Maximum != nil {
		high := identity(ConditionAboveRange)
		threshold := *rule.Maximum
		if isOpen(high) {
			threshold -= rule.Hysteresis
		}
		candidates = append(candidates, candidate{
			Identity: high, Breaching: sample.Value > threshold,
			Title:  fmt.Sprintf("%s above target", label),
			Detail: fmt.Sprintf("Reading %.2f is above the %.2f maximum.", sample.Value, *rule.Maximum),
		})
	}
	return candidates
}

func parseIdentity(key string) (Identity, bool) {
	var identity Identity
	parts := splitN(key, '|', 4)
	if len(parts) != 4 {
		return identity, false
	}
	identity.Key, identity.EntityType, identity.EntityID, identity.Condition = parts[0], parts[1], parts[2], Condition(parts[3])
	return identity, true
}

func splitN(value string, separator byte, count int) []string {
	parts := make([]string, 0, count)
	start := 0
	for index := 0; index < len(value) && len(parts) < count-1; index++ {
		if value[index] == separator {
			parts = append(parts, value[start:index])
			start = index + 1
		}
	}
	return append(parts, value[start:])
}
