package farm_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/jdanielsobrado/grownerve/internal/farm"
	"github.com/jdanielsobrado/grownerve/internal/platform/database/dbtest"
)

const (
	commitFacilityID = "01990a20-6a00-7000-8000-000000000201"
	commitDeviceID   = "01990a20-6a00-7000-8000-000000000202"
	commitChannelID  = "01990a20-6a00-7000-8000-000000000203"
	unknownChannelID = "01990a20-6a00-7000-8000-0000000002ff"
)

func stateDocument(name string, measurementChannel string) json.RawMessage {
	measurements := "[]"
	if measurementChannel != "" {
		measurements = fmt.Sprintf(`[{"channel_id":"%s","observed_at":"2026-09-01T12:00:00Z","sequence":1,"value":22.5,"unit":"degC","quality":"good","source_device_id":"%s"}]`, measurementChannel, commitDeviceID)
	}
	return json.RawMessage(fmt.Sprintf(`{
		"facilities":[{"id":"%s","name":%q,"timezone":"UTC"}],
		"devices":[{"id":"%s","name":"Controller","type":"controller","online":true}],
		"channels":[{"id":"%s","entity_type":"facility","entity_id":"%s","key":"air.temperature","name":"Air temperature","kind":"measurement","value_type":"number","unit":"degC","stale_after_seconds":60}],
		"measurements":%s
	}`, commitFacilityID, name, commitDeviceID, commitChannelID, commitFacilityID, measurements))
}

// storedFacilityName decodes the document rather than matching its text:
// PostgreSQL returns JSONB re-serialised, with its own spacing and key order.
func storedFacilityName(t *testing.T, state json.RawMessage) string {
	t.Helper()
	var document struct {
		Facilities []struct {
			Name string `json:"name"`
		} `json:"facilities"`
	}
	if err := json.Unmarshal(state, &document); err != nil || len(document.Facilities) != 1 {
		t.Fatalf("stored state is not the expected document (%v): %s", err, state)
	}
	return document.Facilities[0].Name
}

func TestPostgresStateCommitterRejectsStaleWriteWithoutProjectionSideEffects(t *testing.T) {
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool, "browser_compatible_states", "facilities", "devices", "device_channels", "measurements", "latest_measurements")
	committer := farm.NewPostgresStateCommitter(pool)
	store := farm.NewPostgresStore(pool)
	ctx := context.Background()

	initialVersion, err := committer.CommitState(ctx, stateDocument("Initial", commitChannelID), farm.NoVersion)
	if err != nil {
		t.Fatal(err)
	}
	winnerVersion, err := committer.CommitState(ctx, stateDocument("Winner", ""), initialVersion)
	if err != nil {
		t.Fatal(err)
	}
	if winnerVersion <= initialVersion {
		t.Fatalf("winner version = %d, initial = %d", winnerVersion, initialVersion)
	}

	version, err := committer.CommitState(ctx, stateDocument("Loser", ""), initialVersion)
	if !errors.Is(err, farm.ErrVersionConflict) {
		t.Fatalf("stale commit = version %d, err %v; want ErrVersionConflict", version, err)
	}
	if version != winnerVersion {
		t.Fatalf("conflict reported version %d, want current %d", version, winnerVersion)
	}

	state, storedVersion, err := store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if storedVersion != winnerVersion || storedFacilityName(t, state) != "Winner" {
		t.Fatalf("stale commit changed stored state: version=%d state=%s", storedVersion, state)
	}

	var facilityName string
	if err := pool.QueryRow(ctx, `SELECT name FROM facilities WHERE id=$1`, commitFacilityID).Scan(&facilityName); err != nil {
		t.Fatal(err)
	}
	if facilityName != "Winner" {
		t.Fatalf("stale commit projected facility name %q", facilityName)
	}

	var measurements int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM measurements`).Scan(&measurements); err != nil {
		t.Fatal(err)
	}
	if measurements != 1 {
		t.Fatalf("measurement count = %d, want the initial imported sample only", measurements)
	}
}

func TestPostgresStateCommitterRollsBackStateAndRegistryWhenTelemetryImportFails(t *testing.T) {
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool, "browser_compatible_states", "facilities", "devices", "device_channels", "measurements", "latest_measurements")
	committer := farm.NewPostgresStateCommitter(pool)
	store := farm.NewPostgresStore(pool)
	ctx := context.Background()

	version, err := committer.CommitState(ctx, stateDocument("Stable", commitChannelID), farm.NoVersion)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := committer.CommitState(ctx, stateDocument("Must Roll Back", unknownChannelID), version); err == nil {
		t.Fatal("commit with telemetry for an unknown channel succeeded")
	}

	state, storedVersion, err := store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if storedVersion != version || storedFacilityName(t, state) != "Stable" {
		t.Fatalf("failed telemetry import changed state: version=%d state=%s", storedVersion, state)
	}

	var facilityName string
	if err := pool.QueryRow(ctx, `SELECT name FROM facilities WHERE id=$1`, commitFacilityID).Scan(&facilityName); err != nil {
		t.Fatal(err)
	}
	if facilityName != "Stable" {
		t.Fatalf("failed telemetry import changed registry to %q", facilityName)
	}

	var measurements int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM measurements`).Scan(&measurements); err != nil {
		t.Fatal(err)
	}
	if measurements != 1 {
		t.Fatalf("failed telemetry import changed history count to %d", measurements)
	}
}
