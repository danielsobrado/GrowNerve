package runtime

import (
	"context"
	"testing"
	"time"
)

type countingConfigPublisher struct {
	calls int
}

func (publisher *countingConfigPublisher) PublishConfig(context.Context, string, []byte) error {
	publisher.calls++
	return nil
}

func TestEdgeConfigRetriesAfterSyncIntervalWithoutAcknowledgement(t *testing.T) {
	now := testNow
	state := `{"devices":[{"id":"device-1","active_config_version":"v1","desired_config_version":"v2","desired_config":{}}],"channels":[],"alerts":[],"commands":[]}`
	publisher := &countingConfigPublisher{}
	supervisor, _, _ := testSupervisor(t, state,
		WithClock(func() time.Time { return now }),
		WithConfigPublisher(publisher),
	)
	ctx := context.Background()

	if err := supervisor.SyncEdgeConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if publisher.calls != 1 {
		t.Fatalf("initial publications = %d, want 1", publisher.calls)
	}

	if err := supervisor.SyncEdgeConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if publisher.calls != 1 {
		t.Fatalf("duplicate call inside sync interval republished %d times", publisher.calls)
	}

	now = now.Add(supervisor.config.ConfigSyncInterval)
	if err := supervisor.SyncEdgeConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if publisher.calls != 2 {
		t.Fatalf("unacknowledged configuration was not retried: %d publications", publisher.calls)
	}
}
