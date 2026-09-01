package edge

import (
	"testing"
	"time"
)

func TestResolveOutputUsesDocumentedPrecedence(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	policy := Policy{DefaultSafeValue: 0, EssentialScheduleValue: 35, AutomationValue: pointer(55), Override: &Override{Value: 70, ExpiresAt: now.Add(time.Hour)}}
	if got := ResolveOutput(policy, now); got.Value != 70 || got.Source != SourceOverride { t.Fatalf("override = %#v", got) }
	policy.Emergency = pointer(0)
	if got := ResolveOutput(policy, now); got.Value != 0 || got.Source != SourceEmergency { t.Fatalf("emergency = %#v", got) }
	policy.HardwareInterlock = pointer(25)
	if got := ResolveOutput(policy, now); got.Value != 25 || got.Source != SourceHardwareInterlock { t.Fatalf("interlock = %#v", got) }
}

func TestExpiredOverrideFallsBackDeterministically(t *testing.T) {
	now := time.Now()
	policy := Policy{DefaultSafeValue: 0, EssentialScheduleValue: 35, Override: &Override{Value: 80, ExpiresAt: now.Add(-time.Second)}}
	if got := ResolveOutput(policy, now); got.Value != 35 || got.Source != SourceEssentialSchedule { t.Fatalf("result = %#v", got) }
}

func pointer(value float64) *float64 { return &value }
