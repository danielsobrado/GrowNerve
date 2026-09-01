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

func TestDraftVersionCanChangeAndEmptySetpointIsUnknown(t *testing.T) {
	version := Version{Status: StatusDraft}
	stages := []Stage{{Key: "seedling"}}
	if err := version.ReplaceStages(stages); err != nil {
		t.Fatal(err)
	}
	stages[0].Key = "changed-outside"
	if version.Stages[0].Key != "seedling" {
		t.Fatal("ReplaceStages did not copy its input")
	}
	if got := Evaluate(20, Setpoint{}); got != TargetUnknown {
		t.Fatalf("Evaluate() = %q", got)
	}
}
