package farm

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
)

// Reasons an acknowledgement is refused. They are returned rather than logged
// so each transport decides how loudly to report a misbehaving device.
var (
	ErrUnknownCommand        = errors.New("unknown_command")
	ErrCommandDeviceMismatch = errors.New("command_device_mismatch")
	ErrCommandAlreadyFinal   = errors.New("command_already_final")
	ErrCommandExpiryInvalid  = errors.New("command_expiry_invalid")
)

// ApplyCommandAcknowledgement records a device's answer to a command. It is the
// single place command results enter the farm document, whichever transport
// carried them: an ESP32 over MQTT or an integration adapter reporting that a
// third-party device reflected the requested state.
//
// The device must own the command's target channel, final results are never
// rewritten, and an answer that arrives after the command expired marks it
// timed out instead of applied.
func ApplyCommandAcknowledgement(ctx context.Context, store Store, deviceID string, ack deviceprotocol.Acknowledgement, receivedAt time.Time) error {
	return Mutate(ctx, store, func(state json.RawMessage) (json.RawMessage, error) {
		var document struct {
			Channels []struct {
				ID       string `json:"id"`
				DeviceID string `json:"device_id"`
			} `json:"channels"`
			Commands []map[string]any `json:"commands"`
		}
		if err := json.Unmarshal(state, &document); err != nil {
			return nil, ErrInvalidState
		}
		for _, command := range document.Commands {
			if command["id"] != ack.CommandID {
				continue
			}
			targetChannel, _ := command["target_channel_id"].(string)
			owned := false
			for _, channel := range document.Channels {
				if channel.ID == targetChannel && channel.DeviceID == deviceID {
					owned = true
					break
				}
			}
			if !owned {
				return nil, ErrCommandDeviceMismatch
			}
			if CommandStatusFinal(command["status"]) {
				return nil, ErrCommandAlreadyFinal
			}
			expiresAt, valid := StoredTime(command["expires_at"])
			if !valid {
				return nil, ErrCommandExpiryInvalid
			}
			command["acknowledged_at"] = ack.AcknowledgedAt
			command["updated_at"] = receivedAt
			if !expiresAt.After(receivedAt) {
				command["status"] = "timed_out"
				command["reason_code"] = "COMMAND_EXPIRED"
				return ReplaceKeys(state, map[string]any{"commands": document.Commands})
			}
			switch ack.Result {
			case "applied":
				command["status"] = "applied"
			case "accepted":
				command["status"] = "acknowledged"
			default:
				command["status"] = "rejected"
			}
			command["reason_code"] = ack.ReasonCode
			if ack.AppliedValue != nil {
				command["applied_value"] = ack.AppliedValue
			}
			return ReplaceKeys(state, map[string]any{"commands": document.Commands})
		}
		return nil, ErrUnknownCommand
	})
}

// CommandStatusFinal reports whether a stored command status can no longer
// change.
func CommandStatusFinal(status any) bool {
	switch status {
	case "applied", "rejected", "timed_out", "cancelled":
		return true
	}
	return false
}

// StoredTime reads a timestamp as it appears in the farm document: a string
// after a JSON round trip, or a time.Time when set in the same process.
func StoredTime(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC(), !typed.IsZero()
	case string:
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
			if parsed, err := time.Parse(layout, typed); err == nil {
				return parsed.UTC(), true
			}
		}
	}
	return time.Time{}, false
}
