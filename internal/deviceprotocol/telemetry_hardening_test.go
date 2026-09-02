package deviceprotocol

import (
	"fmt"
	"strings"
	"testing"
)

func telemetryPayload(sequence uint64, bootID, unit string, samples string) []byte {
	return []byte(fmt.Sprintf(`{"protocolVersion":1,"deviceId":"%s","bootId":"%s","sequence":%d,"observedAt":"2026-09-02T03:00:00Z","samples":%s}`,
		deviceID, bootID, sequence, samples))
}

func validTelemetrySamples(unit string) string {
	return fmt.Sprintf(`[{"channelId":"%s","value":22.4,"unit":"%s","quality":"good"}]`, channelID, unit)
}

func TestTelemetryRejectsSequenceOutsidePersistenceRange(t *testing.T) {
	payload := telemetryPayload(MaximumTelemetrySequence+1, "boot", "degC", validTelemetrySamples("degC"))
	if _, err := ParseTelemetry(payload); err == nil {
		t.Fatal("telemetry sequence that cannot fit int64 was accepted")
	}
}

func TestTelemetryRejectsOversizedIdentifiersAndBatches(t *testing.T) {
	longBoot := strings.Repeat("b", MaximumTelemetryBootID+1)
	if _, err := ParseTelemetry(telemetryPayload(1, longBoot, "degC", validTelemetrySamples("degC"))); err == nil {
		t.Fatal("oversized boot id was accepted")
	}

	longUnit := strings.Repeat("u", MaximumTelemetryUnitLength+1)
	if _, err := ParseTelemetry(telemetryPayload(1, "boot", longUnit, validTelemetrySamples(longUnit))); err == nil {
		t.Fatal("oversized unit was accepted")
	}

	samples := make([]string, MaximumTelemetrySamples+1)
	for index := range samples {
		samples[index] = fmt.Sprintf(`{"channelId":"%s","value":%d,"unit":"degC","quality":"good"}`, channelID, index)
	}
	if _, err := ParseTelemetry(telemetryPayload(1, "boot", "degC", "["+strings.Join(samples, ",")+"]")); err == nil {
		t.Fatal("oversized telemetry batch was accepted")
	}
}

func TestTelemetryRejectsTrailingJSON(t *testing.T) {
	payload := append(telemetryPayload(1, "boot", "degC", validTelemetrySamples("degC")), []byte(` {}`)...)
	if _, err := ParseTelemetry(payload); err == nil {
		t.Fatal("telemetry with a second JSON value was accepted")
	}
}
