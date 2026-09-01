package farm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

var sampleOrigin = time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

func telemetryHandler(t *testing.T) (http.Handler, *MemoryStore, *telemetry.MemoryStore) {
	t.Helper()
	store := NewMemoryStore()
	if _, err := store.Save(context.Background(), json.RawMessage(`{"facilities":[],"measurements":[]}`), AnyVersion); err != nil {
		t.Fatal(err)
	}
	samples := telemetry.NewMemoryStore(0)
	return NewHandler(store, WithTelemetry(samples)), store, samples
}

func seed(t *testing.T, samples *telemetry.MemoryStore, count int) {
	t.Helper()
	batch := make([]telemetry.Measurement, 0, count)
	for index := 0; index < count; index++ {
		batch = append(batch, telemetry.Measurement{
			ChannelID: "channel-1", ObservedAt: sampleOrigin.Add(time.Duration(index) * time.Minute),
			Value: float64(20 + index%5), Unit: "degC", Quality: telemetry.QualityGood,
		})
	}
	if _, err := samples.Append(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryIsBoundedAndFiltered(t *testing.T) {
	handler, _, samples := telemetryHandler(t)
	seed(t, samples, 300)

	from := sampleOrigin.Format(time.RFC3339)
	to := sampleOrigin.Add(10 * time.Minute).Format(time.RFC3339)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/measurements/history?channelId=channel-1&from="+from+"&to="+to, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		Measurements []telemetry.Measurement `json:"measurements"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Measurements) != 10 {
		t.Fatalf("range returned %d samples, want 10", len(payload.Measurements))
	}

	// An unreasonable limit is clamped rather than honoured.
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/measurements/history?channelId=channel-1&limit=999999", nil))
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Measurements) > telemetry.MaximumLimit {
		t.Fatalf("limit was not clamped: %d rows", len(payload.Measurements))
	}
}

func TestHistoryRejectsMissingChannelAndBadRange(t *testing.T) {
	handler, _, _ := telemetryHandler(t)
	for _, testCase := range []struct {
		name, query string
		want        int
	}{
		{"no channel", "", http.StatusBadRequest},
		{"bad from", "?channelId=c&from=yesterday", http.StatusBadRequest},
		{"bad limit", "?channelId=c&limit=-4", http.StatusBadRequest},
		{"bad bucket", "?channelId=c&bucketSeconds=abc", http.StatusBadRequest},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/measurements/history"+testCase.query, nil))
			if response.Code != testCase.want {
				t.Fatalf("status = %d, want %d", response.Code, testCase.want)
			}
		})
	}
}

func TestDownsampledHistoryAggregates(t *testing.T) {
	handler, _, samples := telemetryHandler(t)
	seed(t, samples, 60)

	from := sampleOrigin.Format(time.RFC3339)
	to := sampleOrigin.Add(time.Hour).Format(time.RFC3339)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/measurements/history?channelId=channel-1&bucketSeconds=600&from="+from+"&to="+to, nil))
	var payload struct {
		Buckets []telemetry.Bucket `json:"buckets"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Buckets) != 6 {
		t.Fatalf("600s buckets over an hour = %d, want 6", len(payload.Buckets))
	}
	for _, bucket := range payload.Buckets {
		if bucket.Samples != 10 {
			t.Fatalf("bucket holds %d samples, want 10", bucket.Samples)
		}
		if bucket.Minimum > bucket.Average || bucket.Average > bucket.Maximum {
			t.Fatalf("aggregate is inconsistent: %+v", bucket)
		}
	}
}

// TestStateWriteDoesNotStoreMeasurements is the guard against telemetry creeping
// back into the configuration document. A client that reads the whole state and
// writes it back must not copy history into storage.
func TestStateWriteDoesNotStoreMeasurements(t *testing.T) {
	handler, store, samples := telemetryHandler(t)
	seed(t, samples, 5)

	read := httptest.NewRecorder()
	handler.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/api/v1/state", nil))
	if !strings.Contains(read.Body.String(), `"channel_id":"channel-1"`) {
		t.Fatalf("state projection carried no measurements: %s", read.Body.String())
	}

	request := httptest.NewRequest(http.MethodPut, "/api/v1/state", strings.NewReader(read.Body.String()))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("If-Match", read.Header().Get("ETag"))
	write := httptest.NewRecorder()
	handler.ServeHTTP(write, request)
	if write.Code != http.StatusNoContent {
		t.Fatalf("write status = %d: %s", write.Code, write.Body.String())
	}

	stored, _, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Measurements []json.RawMessage `json:"measurements"`
	}
	if err := json.Unmarshal(stored, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Measurements) != 0 {
		t.Fatalf("round-tripping the state stored %d measurements in the document", len(document.Measurements))
	}
}

// TestSubmittedMeasurementsAreAdoptedNotDropped covers importing a browser farm
// into the server: the history has to survive the move.
func TestSubmittedMeasurementsAreAdoptedNotDropped(t *testing.T) {
	handler, _, samples := telemetryHandler(t)
	body := `{"facilities":[],"measurements":[
		{"channel_id":"channel-9","observed_at":"2026-05-04T12:00:00Z","value":21.5,"unit":"degC","quality":"good"}
	]}`
	request := httptest.NewRequest(http.MethodPut, "/api/v1/state", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("import status = %d: %s", response.Code, response.Body.String())
	}

	stored, err := samples.History(context.Background(), telemetry.Query{
		ChannelID: "channel-9", From: sampleOrigin.Add(-time.Hour), To: sampleOrigin.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].Value != 21.5 {
		t.Fatalf("imported history was lost: %+v", stored)
	}
}

func TestLatestMeasurementsReportOnePerChannel(t *testing.T) {
	handler, _, samples := telemetryHandler(t)
	seed(t, samples, 20)
	if _, err := samples.Append(context.Background(), []telemetry.Measurement{
		{ChannelID: "channel-2", ObservedAt: sampleOrigin, Value: 55, Unit: "percent", Quality: telemetry.QualityGood},
	}); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/measurements/latest", nil))
	var latest []telemetry.Measurement
	if err := json.Unmarshal(response.Body.Bytes(), &latest); err != nil {
		t.Fatal(err)
	}
	if len(latest) != 2 {
		t.Fatalf("latest returned %d rows, want one per channel", len(latest))
	}
}
