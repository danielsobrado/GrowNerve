package farm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	commanddomain "github.com/jdanielsobrado/grownerve/internal/command"
	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/media"
	"github.com/jdanielsobrado/grownerve/internal/registry"
)

const maximumStateBytes = 32 << 20

type Handler struct {
	store     Store
	publisher CommandPublisher
	notifier  Notifier
	authorize Authorizer
	audit     AuditRecorder
	telemetry TelemetryReader
	registry  RegistryProjector
	media     media.Store
	logger    *slog.Logger
	now       func() time.Time
}

type CommandPublisher interface {
	PublishCommand(context.Context, string, deviceprotocol.Command) error
}

// Notifier receives a change hint after every accepted write so live-update
// transports can invalidate without polling.
type Notifier interface {
	Notify(topic string)
}

// Authorizer decides whether the request may perform an action. It is separate
// from safety validation: a caller can be authorized and still be refused by an
// interlock, and vice versa.
type Authorizer interface {
	Authorize(request *http.Request, action string) error
}

// AuditRecorder durably records security-relevant actions.
type AuditRecorder interface {
	Record(ctx context.Context, entry AuditEntry)
}

// AuditEntry is one security-relevant action.
type AuditEntry struct {
	Actor         string         `json:"actor"`
	Action        string         `json:"action"`
	TargetType    string         `json:"target_type"`
	TargetID      string         `json:"target_id"`
	OccurredAt    time.Time      `json:"occurred_at"`
	CorrelationID string         `json:"correlation_id"`
	Detail        map[string]any `json:"detail,omitempty"`
}

type HandlerOption func(*Handler)

func WithCommandPublisher(publisher CommandPublisher) HandlerOption {
	return func(handler *Handler) { handler.publisher = publisher }
}

func WithNotifier(notifier Notifier) HandlerOption {
	return func(handler *Handler) { handler.notifier = notifier }
}

func WithAuthorizer(authorizer Authorizer) HandlerOption {
	return func(handler *Handler) { handler.authorize = authorizer }
}

func WithAuditRecorder(recorder AuditRecorder) HandlerOption {
	return func(handler *Handler) { handler.audit = recorder }
}

// RegistryProjector keeps the relational identities that telemetry references in
// step with the configuration document.
type RegistryProjector interface {
	Project(ctx context.Context, document registry.Document) error
}

// WithRegistry projects facilities, devices, and channels into their tables
// whenever configuration is written. Without it, telemetry has no channel row to
// reference and every measurement is rejected by the database.
func WithRegistry(projector RegistryProjector) HandlerOption {
	return func(handler *Handler) { handler.registry = projector }
}

// WithLogger supplies the logger used for degraded-path diagnostics.
func WithLogger(logger *slog.Logger) HandlerOption {
	return func(handler *Handler) { handler.logger = logger }
}

func WithClock(now func() time.Time) HandlerOption {
	return func(handler *Handler) { handler.now = now }
}

func NewHandler(store Store, options ...HandlerOption) http.Handler {
	handler := &Handler{store: store, now: func() time.Time { return time.Now().UTC() }}
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
		"channels": "channels", "events": "events", "observations": "observations",
		"inventory": "inventory_items", "alerts": "alerts", "automation-rules": "automation_rules",
		"commands": "commands", "scene-layouts": "scene_layouts",
	}
	for resource, key := range collections {
		mux.HandleFunc("GET /api/v1/"+resource, handler.getCollection(key))
	}
	mux.HandleFunc("POST /api/v1/commands", handler.createCommand)
	mux.HandleFunc("GET /api/v1/measurements", handler.getLatestMeasurements)
	mux.HandleFunc("GET /api/v1/measurements/latest", handler.getLatestMeasurements)
	mux.HandleFunc("GET /api/v1/measurements/history", handler.getMeasurements)
	mux.HandleFunc("POST /api/v1/media", handler.uploadMedia)
	mux.HandleFunc("GET /api/v1/media/{id}", handler.downloadMedia)
	return mux
}

func (handler *Handler) permit(writer http.ResponseWriter, request *http.Request, action string) bool {
	if handler.authorize == nil {
		return true
	}
	if err := handler.authorize.Authorize(request, action); err != nil {
		writeAuthorizationProblem(writer, request, err)
		return false
	}
	return true
}

func (handler *Handler) notify(topic string) {
	if handler.notifier != nil {
		handler.notifier.Notify(topic)
	}
}

func (handler *Handler) record(ctx context.Context, entry AuditEntry) {
	if handler.audit == nil {
		return
	}
	if entry.OccurredAt.IsZero() {
		entry.OccurredAt = handler.now()
	}
	handler.audit.Record(ctx, entry)
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

// commandOutcome is the decision reached for one command attempt.
type commandOutcome struct {
	record   map[string]any
	encoded  []byte
	deviceID string
	expires  time.Time
	replayed bool
	safety   *commanddomain.SafetyError
}

func (handler *Handler) createCommand(writer http.ResponseWriter, request *http.Request) {
	if !handler.permit(writer, request, ActionIssueCommand) {
		return
	}
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
	idempotencyKey := request.Header.Get("Idempotency-Key")

	var outcome commandOutcome
	var problem *problemDetails
	err := Mutate(request.Context(), handler.store, func(rawState json.RawMessage) (json.RawMessage, error) {
		outcome, problem = commandOutcome{}, nil
		next, decided := handler.decideCommand(rawState, intent, idempotencyKey, &outcome, &problem)
		if !decided {
			return nil, errCommandRefused
		}
		return next, nil
	})
	switch {
	case errors.Is(err, errCommandRefused) && outcome.replayed:
		// A retry of an already-accepted command returns the original record so
		// the client cannot create a second actuation by retrying.
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(outcome.encoded)
		return
	case errors.Is(err, errCommandRefused) && problem != nil:
		writeProblem(writer, request, problem.status, problem.code, problem.detail)
		return
	case errors.Is(err, ErrNotFound):
		writeProblem(writer, request, http.StatusConflict, "FARM_NOT_CONFIGURED", "Configure a farm before issuing commands")
		return
	case err != nil:
		writeProblem(writer, request, http.StatusInternalServerError, "COMMAND_WRITE_FAILED", "Command could not be persisted")
		return
	}

	handler.record(request.Context(), AuditEntry{
		Actor: ActorOf(request), Action: "command.requested", TargetType: "channel", TargetID: intent.TargetChannelID,
		CorrelationID: request.Header.Get("X-Correlation-ID"),
		Detail:        map[string]any{"reason": intent.Reason, "status": outcome.record["status"], "rejection": outcome.record["reason_code"]},
	})

	if outcome.safety == nil && handler.publisher != nil {
		handler.publishAccepted(request.Context(), intent, &outcome)
	}
	handler.notify("commands")

	writer.Header().Set("Content-Type", "application/json")
	if outcome.safety != nil {
		writer.WriteHeader(http.StatusUnprocessableEntity)
	} else {
		writer.WriteHeader(http.StatusAccepted)
	}
	_, _ = writer.Write(outcome.encoded)
}

var errCommandRefused = errors.New("command refused")

type problemDetails struct {
	status int
	code   string
	detail string
}

// decideCommand validates one attempt against a loaded state document and
// returns the next document to persist. It reports false when the request must
// be refused or replayed rather than written.
func (handler *Handler) decideCommand(rawState json.RawMessage, intent commandIntent, idempotencyKey string, outcome *commandOutcome, problem **problemDetails) (json.RawMessage, bool) {
	var state commandState
	if json.Unmarshal(rawState, &state) != nil {
		*problem = &problemDetails{http.StatusInternalServerError, "STATE_INVALID", "Stored farm state is invalid"}
		return nil, false
	}
	if idempotencyKey != "" {
		for _, existing := range state.Commands {
			var record map[string]any
			if json.Unmarshal(existing, &record) == nil && record["idempotency_key"] == idempotencyKey {
				outcome.replayed, outcome.encoded = true, existing
				return nil, false
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
		*problem = &problemDetails{http.StatusNotFound, "UNKNOWN_CHANNEL", "Target channel does not exist"}
		return nil, false
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
			*problem = &problemDetails{http.StatusBadRequest, "INVALID_COMMAND_VALUE", "Boolean channel requires a boolean value"}
			return nil, false
		}
		if value {
			numeric = 1
		}
	} else if json.Unmarshal(intent.Value, &numeric) != nil {
		*problem = &problemDetails{http.StatusBadRequest, "INVALID_COMMAND_VALUE", "Numeric channel requires a number"}
		return nil, false
	}
	minimum, maximum := -1e12, 1e12
	if channel.SafeMinimum != nil {
		minimum = *channel.SafeMinimum
	}
	if channel.SafeMaximum != nil {
		maximum = *channel.SafeMaximum
	}
	now := handler.now()
	expiresAt := now.Add(defaultCommandTTL)
	if intent.ExpiresAt != nil {
		expiresAt = *intent.ExpiresAt
	}
	safetyError := commanddomain.Validate(
		commanddomain.Request{Value: numeric, ExpiresAt: expiresAt},
		commanddomain.SafetyContext{Controllable: channel.Kind == "command", Online: online, Minimum: minimum, Maximum: maximum},
		now,
	)
	status, reasonCode := "pending", ""
	if safetyError != nil {
		status, reasonCode = "rejected", safetyError.Code
	}
	record := map[string]any{
		"id": uuid.NewString(), "target_channel_id": intent.TargetChannelID, "command_type": "set_percent",
		"value": json.RawMessage(intent.Value), "reason": intent.Reason, "status": status,
		"requested_at": now, "updated_at": now, "expires_at": expiresAt, "simulated": false,
		"idempotency_key": idempotencyKey,
	}
	if channel.ValueType == "boolean" {
		record["command_type"] = "set_boolean"
	}
	if reasonCode != "" {
		record["reason_code"] = reasonCode
	}
	encoded, _ := json.Marshal(record)
	state.Commands = append(state.Commands, encoded)
	next, err := replaceKey(rawState, "commands", state.Commands)
	if err != nil {
		*problem = &problemDetails{http.StatusInternalServerError, "STATE_INVALID", "Stored farm state is invalid"}
		return nil, false
	}
	outcome.record, outcome.encoded, outcome.deviceID, outcome.expires, outcome.safety = record, encoded, channel.DeviceID, expiresAt, safetyError
	return next, true
}

// defaultCommandTTL bounds how long an unacknowledged command stays actionable.
const defaultCommandTTL = 30 * time.Second

// publishAccepted publishes a persisted command and promotes it to "published"
// only after the broker accepts it, so the durable record never claims delivery
// that did not happen.
func (handler *Handler) publishAccepted(ctx context.Context, intent commandIntent, outcome *commandOutcome) {
	protocolCommand := deviceprotocol.Command{
		ProtocolVersion: deviceprotocol.Version, CommandID: outcome.record["id"].(string),
		TargetChannelID: intent.TargetChannelID, Type: outcome.record["command_type"].(string),
		Value: json.RawMessage(intent.Value), IssuedAt: handler.now(), ExpiresAt: outcome.expires,
	}
	if err := handler.publisher.PublishCommand(ctx, outcome.deviceID, protocolCommand); err != nil {
		return
	}
	outcome.record["status"] = "published"
	outcome.record["updated_at"] = handler.now()
	encoded, _ := json.Marshal(outcome.record)
	commandID := outcome.record["id"].(string)
	_ = Mutate(ctx, handler.store, func(rawState json.RawMessage) (json.RawMessage, error) {
		var state commandState
		if err := json.Unmarshal(rawState, &state); err != nil {
			return nil, err
		}
		for index, existing := range state.Commands {
			var record map[string]any
			if json.Unmarshal(existing, &record) == nil && record["id"] == commandID {
				state.Commands[index] = encoded
				return replaceKey(rawState, "commands", state.Commands)
			}
		}
		return rawState, nil
	})
	outcome.encoded = encoded
}

// replaceKey rewrites one top-level key of the state document while preserving
// every other key exactly as stored.
func replaceKey(state json.RawMessage, key string, value any) (json.RawMessage, error) {
	return ReplaceKeys(state, map[string]any{key: value})
}

// ReplaceKeys rewrites the named top-level keys of the state document, leaving
// every other key byte-identical. Background jobs use it so updating alerts or
// commands cannot drop a collection the writer did not model.
func ReplaceKeys(state json.RawMessage, values map[string]any) (json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(state, &object); err != nil {
		return nil, err
	}
	for key, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		object[key] = encoded
	}
	return json.Marshal(object)
}

func (handler *Handler) getCollection(key string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !handler.permit(writer, request, ActionRead) {
			return
		}
		state, _, err := handler.store.Load(request.Context())
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
	if !handler.permit(writer, request, ActionRead) {
		return
	}
	state, version, err := handler.store.Load(request.Context())
	if errors.Is(err, ErrNotFound) {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		writeProblem(writer, request, http.StatusInternalServerError, "STATE_READ_FAILED", "Farm state is temporarily unavailable")
		return
	}
	// The document holds configuration; measurements live in their own store and
	// are merged back in so the whole-state contract keeps working.
	state = handler.projectMeasurements(request.Context(), state)
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("ETag", versionETag(version))
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(state)
}

func (handler *Handler) putState(writer http.ResponseWriter, request *http.Request) {
	if !handler.permit(writer, request, ActionWriteState) {
		return
	}
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
	// The registry is projected before the document is stored. A configuration
	// whose channels cannot be persisted would silently discard every
	// measurement they produce, so it is refused rather than accepted.
	if !handler.projectRegistry(writer, request, body) {
		return
	}
	// Measurements are served into the state projection but never stored in it.
	// Without this, a client that reads the whole state and writes it back would
	// copy history into the configuration document on every save.
	if handler.telemetry != nil {
		body = handler.adoptMeasurements(request.Context(), object, body)
	}
	expected := AnyVersion
	if header := request.Header.Get("If-Match"); header != "" {
		version, valid := parseETag(header)
		if !valid {
			writeProblem(writer, request, http.StatusConflict, "STATE_VERSION_CONFLICT", "Farm state has changed; reload before saving")
			return
		}
		expected = version
	}
	version, err := handler.store.Save(request.Context(), json.RawMessage(body), expected)
	if errors.Is(err, ErrVersionConflict) {
		writer.Header().Set("ETag", versionETag(version))
		writeProblem(writer, request, http.StatusConflict, "STATE_VERSION_CONFLICT", "Farm state has changed; reload before saving")
		return
	}
	if err != nil {
		writeProblem(writer, request, http.StatusInternalServerError, "STATE_WRITE_FAILED", "Farm state could not be persisted")
		return
	}
	handler.notify("state")
	writer.Header().Set("ETag", versionETag(version))
	writer.WriteHeader(http.StatusNoContent)
}

// projectRegistry syncs the relational identities and reports whether the write
// may continue.
func (handler *Handler) projectRegistry(writer http.ResponseWriter, request *http.Request, body []byte) bool {
	if handler.registry == nil {
		return true
	}
	document, err := registry.Parse(body)
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_FARM_STATE", "Farm state must be a valid JSON object")
		return false
	}
	if err := handler.registry.Project(request.Context(), document); err != nil {
		var invalid *registry.InvalidError
		if errors.As(err, &invalid) {
			writeProblem(writer, request, http.StatusUnprocessableEntity, "INVALID_REGISTRY", invalid.Reason)
			return false
		}
		if handler.logger != nil {
			handler.logger.Error("registry_projection_failed", "error", err)
		}
		writeProblem(writer, request, http.StatusInternalServerError, "REGISTRY_WRITE_FAILED", "Devices and channels could not be persisted")
		return false
	}
	return true
}

func writeProblem(writer http.ResponseWriter, request *http.Request, status int, code, detail string) {
	writer.Header().Set("Content-Type", "application/problem+json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"type": "https://grownerve.local/problems/" + strings.ToLower(code), "title": http.StatusText(status), "status": status, "detail": detail, "instance": request.URL.Path, "code": code, "correlationId": request.Header.Get("X-Correlation-ID")})
}
