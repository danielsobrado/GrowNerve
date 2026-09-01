package edge

import "time"

type Source string

const (
	SourceHardwareInterlock Source = "hardware_interlock"
	SourceLocalSafety       Source = "local_safety"
	SourceEmergency         Source = "emergency"
	SourceOverride          Source = "override"
	SourceAutomation        Source = "automation"
	SourceEssentialSchedule Source = "essential_schedule"
	SourceDefaultSafe       Source = "default_safe"
)

type Override struct {
	Value     float64
	ExpiresAt time.Time
}
type Policy struct {
	HardwareInterlock      *float64
	LocalSafetyLimit       *float64
	Emergency              *float64
	Override               *Override
	AutomationValue        *float64
	EssentialScheduleValue float64
	DefaultSafeValue       float64
}
type Resolution struct {
	Value  float64
	Source Source
}

func ResolveOutput(policy Policy, now time.Time) Resolution {
	if policy.HardwareInterlock != nil {
		return Resolution{*policy.HardwareInterlock, SourceHardwareInterlock}
	}
	if policy.LocalSafetyLimit != nil {
		return Resolution{*policy.LocalSafetyLimit, SourceLocalSafety}
	}
	if policy.Emergency != nil {
		return Resolution{*policy.Emergency, SourceEmergency}
	}
	if policy.Override != nil && policy.Override.ExpiresAt.After(now) {
		return Resolution{policy.Override.Value, SourceOverride}
	}
	if policy.AutomationValue != nil {
		return Resolution{*policy.AutomationValue, SourceAutomation}
	}
	if policy.EssentialScheduleValue != policy.DefaultSafeValue {
		return Resolution{policy.EssentialScheduleValue, SourceEssentialSchedule}
	}
	return Resolution{policy.DefaultSafeValue, SourceDefaultSafe}
}
