package recipe

import "testing"

func TestPublishedVersionCannotBeChanged(t *testing.T) {
	version := Version{Status: StatusPublished, Stages: []Stage{{Key: "seedling"}}}
	if err := version.ReplaceStages([]Stage{{Key: "vegetative"}}); err != ErrVersionImmutable {
		t.Fatalf("ReplaceStages() error = %v, want %v", err, ErrVersionImmutable)
	}
}

func TestEvaluateSetpoint(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  TargetStatus
	}{
		{name: "low", value: 17, want: TargetLow},
		{name: "inside", value: 21, want: TargetInRange},
		{name: "high", value: 25, want: TargetHigh},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Evaluate(test.value, Setpoint{Minimum: pointer(18), Maximum: pointer(24)})
			if got != test.want {
				t.Fatalf("Evaluate() = %q, want %q", got, test.want)
			}
		})
	}
}

func pointer(value float64) *float64 { return &value }
