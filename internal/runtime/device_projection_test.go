package runtime

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestOfflineTransitionPreservesDeviceMetadata(t *testing.T) {
	state := `{"devices":[{
		"id":"device-1","zone_id":"zone-1","name":"Controller","type":"controller","online":true,"simulated":false,
		"output_percent":42,"state":true,"last_heartbeat":"2026-09-01T00:00:00Z","last_device_observed_at":"2026-09-01T00:00:01Z",
		"firmware_version":"1.2.3","active_config_version":"v2","desired_config_version":"v3","desired_config":{},
		"last_config_result":{"version":"v2","accepted":true}
	}],"channels":[],"setpoints":[],"recipe_stages":[],"grow_cycles":[],"alerts":[],"commands":[]}`
	now := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
	supervisor, store, _ := testSupervisor(t, state, WithClock(func() time.Time { return now }))

	if err := supervisor.EvaluateAlerts(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, _, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`"online":false`, `"simulated":false`, `"output_percent":42`, `"state":true`,
		`"last_device_observed_at":"2026-09-01T00:00:01Z"`, `"firmware_version":"1.2.3"`,
		`"desired_config_version":"v3"`, `"last_config_result":{"accepted":true,"version":"v2"}`,
	} {
		if !strings.Contains(string(result), expected) {
			t.Fatalf("runtime projection lost %s: %s", expected, result)
		}
	}
}
