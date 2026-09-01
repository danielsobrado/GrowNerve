package farm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	commanddomain "github.com/jdanielsobrado/grownerve/internal/command"
	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
)

const maximumStateBytes = 32 << 20

type Handler struct {
	store     Store
	mu        sync.Mutex
	publisher CommandPublisher
}

type CommandPublisher interface {
	PublishCommand(context.Context, string, deviceprotocol.Command) error
}
type HandlerOption func(*Handler)

func WithCommandPublisher(publisher CommandPublisher) HandlerOption {
	return func(handler *Handler) { handler.publisher = publisher }
}

func NewHandler(store Store, options ...HandlerOption) http.Handler {
	handler := &Handler{store: store}
	for _, option := range options {
		option(handler)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/state", handler.getState)
	mux.HandleFunc("PUT /api/v1/state", handler.putState)
	mux.HandleFunc("GET /api/v1/overview", handler.getState)
	collections := map[string]string{
		"facilities": "facilities", "zones": "zones", "reservoirs": "reservoirs", "crops": "crops",
		"varieties": "varieties", "grow-cycles": "grow_cycles", "recipes": "recipes", "devices": "devices",
		"channels": "channels", "measurements": "measurements", "events": "events", "observations": "observations",
		"inventory": "inventory_items", "alerts": "alerts", "automation-rules": "automation_rules",
		"commands": "commands", "scene-layouts": "scene_layouts",
	}
	for resource, key := range collections {
		mux.HandleFunc("GET /api/v1/"+resource, handler.getCollection(key))
	}
	mux.HandleFunc("POST /api/v1/commands", handler.createCommand)
	return mux
}

type commandIntent struct {
	TargetChannelID string          `json:"targetChannelId"`
	Value           json.RawMessage `json:"value"`
	Reason          string          `json:"reason"`
	ExpiresAt       *time.Time      `json:"expiresAt"`
}

type channelState struct {
	ID          string   `json:"id"`
	DeviceID    string   `json:"device_id"`
	Kind        string   `json:"kind"`
	ValueType   string   `json:"value_type"`
	SafeMinimum *float64 `json:"safe_minimum"`
	SafeMaximum *float64 `json:"safe_maximum"`
}
type deviceState struct {
	ID     string `json:"id"`
	Online bool   `json:"online"`
}
type commandState struct {
	Channels []channelState    `json:"channels"`
	Devices  []deviceState     `json:"devices"`
	Commands []json.RawMessage `json:"commands"`
	Events   []json.RawMessage `json:"events"`
}

func (handler *Handler) createCommand(writer http.ResponseWriter, request *http.Request) {
	if request.Header.Get("Content-Type") != "application/json" {
		writeProblem(writer, request, http.StatusUnsupportedMediaType, "CONTENT_TYPE_REQUIRED", "Content-Type must be application/json")
		return
	}
	var intent commandIntent
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&intent) != nil || intent.TargetChannelID == "" || strings.TrimSpace(intent.Reason) == "" {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_COMMAND", "targetChannelId, value, and reason are required")
		return
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	rawState, err := handler.store.Load(request.Context())
	if err != nil {
		writeProblem(writer, request, http.StatusConflict, "FARM_NOT_CONFIGURED", "Configure a farm before issuing commands")
		return
	}
	var state commandState
	if json.Unmarshal(rawState, &state) != nil {
		writeProblem(writer, request, http.StatusInternalServerError, "STATE_INVALID", "Stored farm state is invalid")
		return
	}
	idempotencyKey := request.Header.Get("Idempotency-Key")
	if idempotencyKey != "" {
		for _, existing := range state.Commands {
			var record map[string]any
			if json.Unmarshal(existing, &record) == nil && record["idempotency_key"] == idempotencyKey {
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write(existing)
				return
			}
		}
	}
	var channel *channelState
	for index := range state.Channels {
		if state.Channels[index].ID == intent.TargetChannelID {
			channel = &state.Channels[index]
			break
		}
	}
	if channel == nil {
		writeProblem(writer, request, http.StatusNotFound, "UNKNOWN_CHANNEL", "Target channel does not exist")
		return
	}
	online := false
	for _, device := range state.Devices {
		if device.ID == channel.DeviceID {
			online = device.Online
			break
		}
	}
	var numeric float64
	if channel.ValueType == "boolean" {
		var value bool
		if json.Unmarshal(intent.Value, &value) != nil {
			writeProblem(writer, request, http.StatusBadRequest, "INVALID_COMMAND_VALUE", "Boolean channel requires a boolean value")
			return
		}
		if value {
			numeric = 1
		}
	} else if json.Unmarshal(intent.Value, &numeric) != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_COMMAND_VALUE", "Numeric channel requires a number")
		return
	}
	minimum, maximum := -1e12, 1e12
	if channel.SafeMinimum != nil {
		minimum = *channel.SafeMinimum
	}
	if channel.SafeMaximum != nil {
		maximum = *channel.SafeMaximum
	}
	now := time.Now().UTC()
	expiresAt := now.Add(30 * time.Second)
	if intent.ExpiresAt != nil {
		expiresAt = *intent.ExpiresAt
	}
	safetyError := commanddomain.Validate(commanddomain.Request{Value: numeric, ExpiresAt: expiresAt}, commanddomain.SafetyContext{Controllable: channel.Kind == "command", Online: online, Minimum: minimum, Maximum: maximum}, now)
	status := "pending"
	reasonCode := ""
	if safetyError != nil {
		status = "rejected"
		reasonCode = safetyError.Code
	}
	record := map[string]any{"id": uuid.NewString(), "target_channel_id": intent.TargetChannelID, "command_type": "set_percent", "value": json.RawMessage(intent.Value), "reason": intent.Reason, "status": status, "requested_at": now, "updated_at": now, "simulated": false, "idempotency_key": idempotencyKey}
	if channel.ValueType == "boolean" {
		record["command_type"] = "set_boolean"
	}
	if reasonCode != "" {
		record["reason_code"] = reasonCode
	}
	encoded, _ := json.Marshal(record)
	state.Commands = append(state.Commands, encoded)
	var stateObject map[string]json.RawMessage
	_ = json.Unmarshal(rawState, &stateObject)
	stateObject["commands"], _ = json.Marshal(state.Commands)
	nextState, _ := json.Marshal(stateObject)
	if err := handler.store.Save(request.Context(), nextState); err != nil {
		writeProblem(writer, request, http.StatusInternalServerError, "COMMAND_WRITE_FAILED", "Command could not be persisted")
		return
	}
	if safetyError == nil && handler.publisher != nil {
		protocolCommand := deviceprotocol.Command{ProtocolVersion: deviceprotocol.Version, CommandID: record["id"].(string), TargetChannelID: intent.TargetChannelID, Type: record["command_type"].(string), Value: json.RawMessage(intent.Value), IssuedAt: now, ExpiresAt: expiresAt}
		if publishErr := handler.publisher.PublishCommand(request.Context(), channel.DeviceID, protocolCommand); publishErr == nil {
			record["status"] = "published"
			record["updated_at"] = time.Now().UTC()
			encoded, _ = json.Marshal(record)
			state.Commands[len(state.Commands)-1] = encoded
			stateObject["commands"], _ = json.Marshal(state.Commands)
			nextState, _ = json.Marshal(stateObject)
			_ = handler.store.Save(request.Context(), nextState)
		}
	}
	writer.Header().Set("Content-Type", "application/json")
	if safetyError != nil {
		writer.WriteHeader(http.StatusUnprocessableEntity)
	} else {
		writer.WriteHeader(http.StatusAccepted)
	}
	_, _ = writer.Write(encoded)
}

func (handler *Handler) getCollection(key string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		state, err := handler.store.Load(request.Context())
		if errors.Is(err, ErrNotFound) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte("[]"))
			return
		}
		if err != nil {
			writeProblem(writer, request, http.StatusInternalServerError, "STATE_READ_FAILED", "Farm state is temporarily unavailable")
			return
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(state, &object) != nil {
			writeProblem(writer, request, http.StatusInternalServerError, "STATE_INVALID", "Stored farm state is invalid")
			return
		}
		collection, found := object[key]
		if !found {
			collection = json.RawMessage("[]")
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(collection)
	}
}

func (handler *Handler) getState(writer http.ResponseWriter, request *http.Request) {
	state, err := handler.store.Load(request.Context())
	if errors.Is(err, ErrNotFound) {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		writeProblem(writer, request, http.StatusInternalServerError, "STATE_READ_FAILED", "Farm state is temporarily unavailable")
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("ETag", stateETag(state))
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(state)
}

func (handler *Handler) putState(writer http.ResponseWriter, request *http.Request) {
	if mediaType := strings.ToLower(strings.TrimSpace(strings.Split(request.Header.Get("Content-Type"), ";")[0])); mediaType != "application/json" {
		writeProblem(writer, request, http.StatusUnsupportedMediaType, "CONTENT_TYPE_REQUIRED", "Content-Type must be application/json")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, maximumStateBytes))
	if err != nil {
		writeProblem(writer, request, http.StatusRequestEntityTooLarge, "STATE_TOO_LARGE", "Farm state exceeds the configured request limit")
		return
	}
	var object map[string]json.RawMessage
	if len(body) == 0 || json.Unmarshal(body, &object) != nil || object == nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_FARM_STATE", "Farm state must be a valid JSON object")
		return
	}
	if expected := request.Header.Get("If-Match"); expected != "" {
		current, loadErr := handler.store.Load(request.Context())
		if loadErr == nil && stateETag(current) != expected {
			writeProblem(writer, request, http.StatusConflict, "STATE_VERSION_CONFLICT", "Farm state has changed; reload before saving")
			return
		}
	}
	if err := handler.store.Save(request.Context(), json.RawMessage(body)); err != nil {
		writeProblem(writer, request, http.StatusInternalServerError, "STATE_WRITE_FAILED", "Farm state could not be persisted")
		return
	}
	writer.Header().Set("ETag", stateETag(body))
	writer.WriteHeader(http.StatusNoContent)
}

func writeProblem(writer http.ResponseWriter, request *http.Request, status int, code, detail string) {
	writer.Header().Set("Content-Type", "application/problem+json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"type": "https://grownerve.local/problems/" + strings.ToLower(code), "title": http.StatusText(status), "status": status, "detail": detail, "instance": request.URL.Path, "code": code, "correlationId": request.Header.Get("X-Correlation-ID")})
}
