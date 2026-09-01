package httpx

import (
	"encoding/json"
	"net/http"
)

type ReadinessCheck func() error

type HealthHandler struct {
	readiness ReadinessCheck
}

func NewHealthHandler(readiness ReadinessCheck) *HealthHandler {
	return &HealthHandler{readiness: readiness}
}

func (handler *HealthHandler) Live(writer http.ResponseWriter, _ *http.Request) {
	writeHealth(writer, http.StatusOK, "ok", nil)
}

func (handler *HealthHandler) Ready(writer http.ResponseWriter, _ *http.Request) {
	if handler.readiness != nil {
		if err := handler.readiness(); err != nil {
			writeHealth(writer, http.StatusServiceUnavailable, "unavailable", map[string]string{"database": err.Error()})
			return
		}
	}
	writeHealth(writer, http.StatusOK, "ok", nil)
}

func writeHealth(writer http.ResponseWriter, status int, state string, checks map[string]string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"status": state, "checks": checks})
}
