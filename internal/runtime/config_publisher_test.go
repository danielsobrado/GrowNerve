package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
)

const (
	publisherDeviceID  = "01990a20-6a00-7000-8000-000000000301"
	publisherChannelID = "01990a20-6a00-7000-8000-000000000302"
)

type capturedConfigPublisher struct {
	calls    int
	deviceID string
	payload  []byte
}

func (publisher *capturedConfigPublisher) PublishConfig(_ context.Context, deviceID string, payload []byte) error {
	publisher.calls++
	publisher.deviceID = deviceID
	publisher.payload = append([]byte(nil), payload...)
	return nil
}

func edgeConfigPayload(t *testing.T, mutate func(*deviceprotocol.EdgeConfig)) []byte {
	t.Helper()
	config := deviceprotocol.EdgeConfig{
		ProtocolVersion: deviceprotocol.Version,
		DeviceID:        publisherDeviceID,
		ConfigVersion:   "edge-v2",
		IssuedAt:        time.Now().UTC(),
		Config: deviceprotocol.EdgeSettings{
			TimezonePOSIX: "GST-4",
			Photoperiod: &deviceprotocol.Photoperiod{
				OnHour: 6, OffHour: 23, ChannelID: publisherChannelID,
			},
			SafeOutputs: map[string]float64{publisherChannelID: 0},
		},
	}
	if mutate != nil {
		mutate(&config)
	}
	payload, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestValidatingConfigPublisherForwardsOnlyValidConfig(t *testing.T) {
	next := &capturedConfigPublisher{}
	publisher := NewValidatingConfigPublisher(next)
	payload := edgeConfigPayload(t, nil)
	if err := publisher.PublishConfig(context.Background(), publisherDeviceID, payload); err != nil {
		t.Fatal(err)
	}
	if next.calls != 1 || next.deviceID != publisherDeviceID || string(next.payload) != string(payload) {
		t.Fatalf("forwarded config = calls:%d device:%q payload:%s", next.calls, next.deviceID, next.payload)
	}
}

func TestValidatingConfigPublisherRejectsUnsafeOrMisdirectedConfig(t *testing.T) {
	for name, testCase := range map[string]struct {
		deviceID string
		payload  []byte
	}{
		"IANA timezone": {
			deviceID: publisherDeviceID,
			payload: edgeConfigPayload(t, func(config *deviceprotocol.EdgeConfig) {
				config.Config.TimezonePOSIX = "Asia/Dubai"
			}),
		},
		"wrong target": {
			deviceID: "01990a20-6a00-7000-8000-000000000399",
			payload:  edgeConfigPayload(t, nil),
		},
		"unsafe output": {
			deviceID: publisherDeviceID,
			payload: edgeConfigPayload(t, func(config *deviceprotocol.EdgeConfig) {
				config.Config.SafeOutputs[publisherChannelID] = 101
			}),
		},
		"version too long": {
			deviceID: publisherDeviceID,
			payload: edgeConfigPayload(t, func(config *deviceprotocol.EdgeConfig) {
				config.ConfigVersion = strings.Repeat("v", maximumPersistedConfigVersionLength+1)
			}),
		},
	} {
		t.Run(name, func(t *testing.T) {
			next := &capturedConfigPublisher{}
			publisher := NewValidatingConfigPublisher(next)
			if err := publisher.PublishConfig(context.Background(), testCase.deviceID, testCase.payload); err == nil {
				t.Fatal("invalid configuration was published")
			}
			if next.calls != 0 {
				t.Fatalf("invalid configuration reached downstream publisher %d times", next.calls)
			}
		})
	}
}

func TestValidatingConfigPublisherRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	for _, payload := range [][]byte{
		[]byte(`{"protocolVersion":1,"deviceId":"` + publisherDeviceID + `","configVersion":"v","issuedAt":"2026-09-01T12:00:00Z","config":{},"unexpected":true}`),
		append(edgeConfigPayload(t, nil), []byte(` {}`)...),
	} {
		next := &capturedConfigPublisher{}
		publisher := NewValidatingConfigPublisher(next)
		if err := publisher.PublishConfig(context.Background(), publisherDeviceID, payload); err == nil {
			t.Fatal("malformed configuration envelope was published")
		}
		if next.calls != 0 {
			t.Fatal("malformed configuration reached downstream publisher")
		}
	}
}
