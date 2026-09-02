package farm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jdanielsobrado/grownerve/internal/farm"
	"github.com/jdanielsobrado/grownerve/internal/platform/database/dbtest"
)

const replacementChannelID = "01990a20-6a00-7000-8000-000000000204"

func replacementState(channelID string, includeMeasurement bool) json.RawMessage {
	measurements := "[]"
	if includeMeasurement {
		measurements = fmt.Sprintf(`[{"channel_id":"%s","observed_at":"2026-09-01T12:00:00Z","sequence":1,"value":22.5,"unit":"degC","quality":"good","source_device_id":"%s"}]`, channelID, commitDeviceID)
	}
	return json.RawMessage(fmt.Sprintf(`{
		"facilities":[{"id":"%s","name":"Farm","timezone":"UTC"}],
		"devices":[{"id":"%s","name":"Controller","type":"controller","online":true}],
		"channels":[{"id":"%s","entity_type":"facility","entity_id":"%s","key":"air.temperature","name":"Air temperature","kind":"measurement","value_type":"number","unit":"degC","stale_after_seconds":60}],
		"measurements":%s
	}`, commitFacilityID, commitDeviceID, channelID, commitFacilityID, measurements))
}

func TestStateReplacementCanReuseChannelKeyWithoutDeletingHistory(t *testing.T) {
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool, "browser_compatible_states", "facilities", "devices", "device_channels", "measurements", "latest_measurements")
	committer := farm.NewPostgresStateCommitter(pool)
	ctx := context.Background()

	version, err := committer.CommitState(ctx, replacementState(commitChannelID, true), farm.NoVersion)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := committer.CommitState(ctx, replacementState(replacementChannelID, false), version); err != nil {
		t.Fatalf("replacement with a new channel identity failed: %v", err)
	}

	var measurements int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM measurements WHERE channel_id=$1`, commitChannelID).Scan(&measurements); err != nil {
		t.Fatal(err)
	}
	if measurements != 1 {
		t.Fatalf("historical measurements = %d, want 1", measurements)
	}

	rows, err := pool.Query(ctx, `SELECT id::text, key FROM device_channels WHERE facility_id=$1 ORDER BY id`, commitFacilityID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	keys := map[string]string{}
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			t.Fatal(err)
		}
		keys[id] = key
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if keys[replacementChannelID] != "air.temperature" {
		t.Fatalf("replacement channel key = %q", keys[replacementChannelID])
	}
	if !strings.HasPrefix(keys[commitChannelID], "retired:"+commitChannelID+":") {
		t.Fatalf("historical channel key was not retired safely: %q", keys[commitChannelID])
	}
}
