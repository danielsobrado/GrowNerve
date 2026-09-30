package integration

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/farm"
	"github.com/jdanielsobrado/grownerve/internal/registry"
)

// Handler serves /api/v1/integrations. Browsing a provider's inventory needs a
// manager (a Home Assistant inventory describes a whole home); admitting new
// radios to a network needs an administrator.
type Handler struct {
	manager   *Manager
	authorize farm.Authorizer
	logger    *slog.Logger
	mux       *http.ServeMux
}

func NewHandler(manager *Manager, authorizer farm.Authorizer, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	handler := &Handler{manager: manager, authorize: authorizer, logger: logger, mux: http.NewServeMux()}
	handler.mux.HandleFunc("GET /api/v1/integrations", handler.list)
	handler.mux.HandleFunc("GET /api/v1/integrations/{provider}/devices", handler.devices)
	handler.mux.HandleFunc("POST /api/v1/integrations/{provider}/devices/{externalId}/adopt", handler.adopt)
	handler.mux.HandleFunc("POST /api/v1/integrations/{provider}/permit-join", handler.permitJoin)
	return handler
}

func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	handler.mux.ServeHTTP(writer, request)
}

func (handler *Handler) list(writer http.ResponseWriter, request *http.Request) {
	if !farm.Permit(handler.authorize, writer, request, farm.ActionRead) {
		return
	}
	writeJSON(writer, http.StatusOK, handler.manager.Statuses())
}

func (handler *Handler) provider(writer http.ResponseWriter, request *http.Request) (Provider, bool) {
	provider := Provider(request.PathValue("provider"))
	if !provider.Valid() {
		farm.WriteProblem(writer, request, http.StatusNotFound, "UNKNOWN_PROVIDER", "No integration provider has that name")
		return "", false
	}
	return provider, true
}

func (handler *Handler) devices(writer http.ResponseWriter, request *http.Request) {
	if !farm.Permit(handler.authorize, writer, request, farm.ActionManageIntegrations) {
		return
	}
	provider, ok := handler.provider(writer, request)
	if !ok {
		return
	}
	devices, err := handler.manager.Discovered(request.Context(), provider)
	if handler.writeError(writer, request, err) {
		return
	}
	if devices == nil {
		devices = []DiscoveredDevice{}
	}
	writeJSON(writer, http.StatusOK, devices)
}

func (handler *Handler) adopt(writer http.ResponseWriter, request *http.Request) {
	if !farm.Permit(handler.authorize, writer, request, farm.ActionManageIntegrations) {
		return
	}
	provider, ok := handler.provider(writer, request)
	if !ok {
		return
	}
	var body AdoptRequest
	if !decode(writer, request, &body) {
		return
	}
	result, err := handler.manager.Adopt(request.Context(), provider, request.PathValue("externalId"), farm.ActorOf(request), body)
	if handler.writeError(writer, request, err) {
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

// MaximumPermitJoinSeconds is the Zigbee specification's limit.
const MaximumPermitJoinSeconds = 254

func (handler *Handler) permitJoin(writer http.ResponseWriter, request *http.Request) {
	if !farm.Permit(handler.authorize, writer, request, farm.ActionPairDevices) {
		return
	}
	provider, ok := handler.provider(writer, request)
	if !ok {
		return
	}
	var body struct {
		Seconds int `json:"seconds"`
	}
	if !decode(writer, request, &body) {
		return
	}
	if body.Seconds < 0 || body.Seconds > MaximumPermitJoinSeconds {
		farm.WriteProblem(writer, request, http.StatusUnprocessableEntity, "INVALID_PERMIT_JOIN", "seconds must be between 0 (close) and 254")
		return
	}
	until, err := handler.manager.PermitJoin(request.Context(), provider, body.Seconds)
	if handler.writeError(writer, request, err) {
		return
	}
	handler.logger.Info("integration_permit_join", "provider", provider, "seconds", body.Seconds, "actor", farm.ActorOf(request))
	writeJSON(writer, http.StatusAccepted, map[string]time.Time{"until": until})
}

func (handler *Handler) writeError(writer http.ResponseWriter, request *http.Request, err error) bool {
	if err == nil {
		return false
	}
	var refusal *RequestError
	var invalid *registry.InvalidError
	switch {
	case errors.As(err, &refusal):
		farm.WriteProblem(writer, request, refusal.Status, refusal.Code, refusal.Detail)
	case errors.Is(err, ErrProviderDisabled):
		farm.WriteProblem(writer, request, http.StatusConflict, "INTEGRATION_DISABLED", "Enable this integration in the server configuration first")
	case errors.Is(err, ErrUnsupported):
		farm.WriteProblem(writer, request, http.StatusNotImplemented, "UNSUPPORTED_OPERATION", "This integration does not support that operation")
	case errors.Is(err, farm.ErrVersionConflict):
		farm.WriteProblem(writer, request, http.StatusConflict, "STATE_VERSION_CONFLICT", "Farm state kept changing; try again")
	case errors.As(err, &invalid):
		farm.WriteProblem(writer, request, http.StatusUnprocessableEntity, "INVALID_REGISTRY", invalid.Reason)
	default:
		handler.logger.Error("integration_request_failed", "path", request.URL.Path, "error", err)
		farm.WriteProblem(writer, request, http.StatusBadGateway, "INTEGRATION_FAILED", "The integration could not complete the request")
	}
	return true
}

func decode(writer http.ResponseWriter, request *http.Request, target any) bool {
	if !strings.EqualFold(strings.TrimSpace(strings.Split(request.Header.Get("Content-Type"), ";")[0]), "application/json") {
		farm.WriteProblem(writer, request, http.StatusUnsupportedMediaType, "CONTENT_TYPE_REQUIRED", "Content-Type must be application/json")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		farm.WriteProblem(writer, request, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be one valid JSON object")
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		farm.WriteProblem(writer, request, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be one valid JSON object")
		return false
	}
	return true
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
