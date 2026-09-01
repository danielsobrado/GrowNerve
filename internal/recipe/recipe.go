package recipe

import "errors"

var ErrVersionImmutable = errors.New("published recipe versions are immutable")

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
)

type Stage struct {
	Key       string
	Setpoints []Setpoint
}

type Version struct {
	Status Status
	Stages []Stage
}

func (version *Version) ReplaceStages(stages []Stage) error {
	if version.Status == StatusPublished {
		return ErrVersionImmutable
	}
	version.Stages = append([]Stage(nil), stages...)
	return nil
}

type Setpoint struct {
	Minimum *float64
	Maximum *float64
}

type TargetStatus string

const (
	TargetLow     TargetStatus = "low"
	TargetInRange TargetStatus = "in_range"
	TargetHigh    TargetStatus = "high"
	TargetUnknown TargetStatus = "unknown"
)

func Evaluate(value float64, setpoint Setpoint) TargetStatus {
	if setpoint.Minimum != nil && value < *setpoint.Minimum {
		return TargetLow
	}
	if setpoint.Maximum != nil && value > *setpoint.Maximum {
		return TargetHigh
	}
	if setpoint.Minimum == nil && setpoint.Maximum == nil {
		return TargetUnknown
	}
	return TargetInRange
}
