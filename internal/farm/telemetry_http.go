package farm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

type TelemetryReader interface {
	History(ctx context.Context, query telemetry.Query) ([]telemetry.Measurement, error)
	Downsampled(ctx context.Context, query telemetry.Query) ([]telemetry.Bucket, error)
	Latest(ctx context.Context) ([]telemetry.Measurement, error)
	Recent(ctx context.Context, limit int) ([]telemetry.Measurement, error)
}

func WithTelemetry(reader TelemetryReader) HandlerOption {
	return func(handler *Handler) { handler.telemetry = reader }
}

const projectionWindow = 2000

func (handler *Handler) getMeasurements(writer http.ResponseWriter, request *http.Request) {
	if !handler.permit(writer, request, ActionRead) {
		return
	}
	if handler.telemetry == nil {
		writeProblem(writer, request, http.StatusServiceUnavailable, "TELEMETRY_UNAVAILABLE", "Measurement history is not configured")
		return
	}
	query := request.URL.Query()
	channelID := query.Get("channelId")
	if channelID == "" {
		writeProblem(writer, request, http.StatusBadRequest, "CHANNEL_REQUIRED", "channelId is required")
		return
	}
	from, err := optionalTime(query.Get("from"))
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_TIME_RANGE", "from must be an RFC 3339 timestamp")
		return
	}
	to, err := optionalTime(query.Get("to"))
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_TIME_RANGE", "to must be an RFC 3339 timestamp")
		return
	}
	limit, err := optionalInt(query.Get("limit"))
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_LIMIT", "limit must be a positive whole number")
		return
	}
	bucket, err := optionalInt(query.Get("bucketSeconds"))
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_BUCKET", "bucketSeconds must be a positive whole number")
		return
	}
	specification := telemetry.Query{ChannelID: channelID, From: from, To: to, Limit: limit, BucketSeconds: bucket}

	if bucket > 0 {
		buckets, err := handler.telemetry.Downsampled(request.Context(), specification)
		if err != nil {
			writeProblem(writer, request, http.StatusInternalServerError, "TELEMETRY_READ_FAILED", "Measurement history is temporarily unavailable")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"channelId": channelID, "bucketSeconds": bucket, "buckets": buckets})
		return
	}
	measurements, err := handler.telemetry.History(request.Context(), specification)
	if err != nil {
		writeProblem(writer, request, http.StatusInternalServerError, "TELEMETRY_READ_FAILED", "Measurement history is temporarily unavailable")
		return
	}
	if measurements == nil {
		measurements = []telemetry.Measurement{}
	}
	writeJSON(writer, http.StatusOK, map[string]any{"channelId": channelID, "measurements": measurements})
}

func (handler *Handler) getLatestMeasurements(writer http.ResponseWriter, request *http.Request) {
	if !handler.permit(writer, request, ActionRead) {
		return
	}
	if handler.telemetry == nil {
		writeJSON(writer, http.StatusOK, []telemetry.Measurement{})
		return
	}
	latest, err := handler.telemetry.Latest(request.Context())
	if err != nil {
		writeProblem(writer, request, http.StatusInternalServerError, "TELEMETRY_READ_FAILED", "Measurement history is temporarily unavailable")
		return
	}
	if latest == nil {
		latest = []telemetry.Measurement{}
	}
	writeJSON(writer, http.StatusOK, latest)
}

func (handler *Handler) projectMeasurements(ctx context.Context, state json.RawMessage) json.RawMessage {
	if handler.telemetry == nil {
		return state
	}
	recent, err := handler.telemetry.Recent(ctx, projectionWindow)
	if err != nil {
		return state
	}
	if recent == nil {
		recent = []telemetry.Measurement{}
	}
	projected, err := ReplaceKeys(state, map[string]any{"measurements": recent})
	if err != nil {
		return state
	}
	return projected
}

func optionalTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}

func optionalInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, errInvalidNumber
	}
	return parsed, nil
}

var errInvalidNumber = errorString("value must be a non-negative whole number")

type errorString string

func (err errorString) Error() string { return string(err) }

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

type TelemetryWriter interface {
	Append(ctx context.Context, measurements []telemetry.Measurement) (int, error)
}

// adoptMeasurements is the non-transactional compatibility path used by test
// stores. Production uses PostgresStateCommitter so history, registry and state
// commit atomically. Adoption never strips history after an append failure.
func (handler *Handler) adoptMeasurements(ctx context.Context, object map[string]json.RawMessage, body []byte) ([]byte, error) {
	raw, present := object["measurements"]
	if !present {
		return body, nil
	}
	var submitted []telemetry.Measurement
	if err := json.Unmarshal(raw, &submitted); err != nil {
		return nil, telemetry.ErrInvalidMeasurement
	}
	if len(submitted) > 0 {
		writer, writable := handler.telemetry.(TelemetryWriter)
		if !writable {
			return nil, errors.New("measurement store is read-only")
		}
		if _, err := writer.Append(ctx, submitted); err != nil {
			return nil, err
		}
	}
	stripped, err := ReplaceKeys(body, map[string]any{"measurements": []telemetry.Measurement{}})
	if err != nil {
		return nil, err
	}
	return stripped, nil
}
