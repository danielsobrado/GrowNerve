package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
)

// ValidatingConfigPublisher prevents an invalid retained configuration from
// reaching a controller. Device-side validation remains mandatory as a second
// safety boundary.
type ValidatingConfigPublisher struct {
	next ConfigPublisher
}

func NewValidatingConfigPublisher(next ConfigPublisher) *ValidatingConfigPublisher {
	return &ValidatingConfigPublisher{next: next}
}

func (publisher *ValidatingConfigPublisher) PublishConfig(ctx context.Context, deviceID string, payload []byte) error {
	if publisher == nil || publisher.next == nil {
		return errors.New("edge config publisher is not configured")
	}
	var config deviceprotocol.EdgeConfig
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("decode edge config: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return err
	}
	if !strings.EqualFold(config.DeviceID, deviceID) {
		return errors.New("edge config deviceId does not match publish target")
	}
	if err := config.Validate(); err != nil {
		return fmt.Errorf("validate edge config: %w", err)
	}
	return publisher.next.PublishConfig(ctx, deviceID, payload)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode edge config trailer: %w", err)
	}
	return errors.New("edge config contains more than one JSON value")
}
