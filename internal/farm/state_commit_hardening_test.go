package farm

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jdanielsobrado/grownerve/internal/registry"
	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

func TestImportedMeasurementCannotClaimAnotherKnownDevice(t *testing.T) {
	const channelID = "01990a20-6a00-7000-8000-000000000301"
	const ownerID = "01990a20-6a00-7000-8000-000000000302"
	const foreignID = "01990a20-6a00-7000-8000-000000000303"
	document := registry.Document{
		Devices: []registry.Device{{ID: ownerID}, {ID: foreignID}},
		Channels: []registry.Channel{{ID: channelID, DeviceID: ownerID, Key: "air.temperature", Kind: "measurement", ValueType: "number", Unit: "degC"}},
	}
	object := map[string]json.RawMessage{
		"measurements": json.RawMessage(`[{"channel_id":"` + channelID + `","observed_at":"2026-09-02T04:00:00Z","sequence":1,"value":22,"unit":"degC","quality":"good","source_device_id":"` + foreignID + `"}]`),
	}

	if _, err := importedMeasurements(object, document); !errors.Is(err, telemetry.ErrInvalidMeasurement) {
		t.Fatalf("cross-device measurement import error = %v, want ErrInvalidMeasurement", err)
	}
}
