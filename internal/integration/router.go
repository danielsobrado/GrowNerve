package integration

import (
	"context"
	"encoding/json"

	"github.com/jdanielsobrado/grownerve/internal/deviceprotocol"
	"github.com/jdanielsobrado/grownerve/internal/farm"
)

// Transport is what the MQTT bridge offers the command path.
type Transport interface {
	farm.CommandPublisher
	farm.RawPublisher
}

// Router stands where the MQTT bridge used to stand in the command path: the
// durable publisher and the outbox worker hand it every command, and it sends
// integration-managed devices' commands to their adapter and everything else
// to the bridge. Because it sits behind DurablePublisher, a command for an
// unreachable Zigbee plug is queued and retried exactly like one for an
// ESP32 behind a down broker.
type Router struct {
	fallback Transport
	manager  *Manager
}

func NewRouter(fallback Transport, manager *Manager) *Router {
	return &Router{fallback: fallback, manager: manager}
}

func (router *Router) PublishCommand(ctx context.Context, deviceID string, command deviceprotocol.Command) error {
	if router.manager != nil && router.manager.Enabled() {
		if handled, err := router.manager.Deliver(ctx, deviceID, command); handled {
			return err
		}
	}
	return router.fallback.PublishCommand(ctx, deviceID, command)
}

func (router *Router) PublishRaw(ctx context.Context, topic string, payload []byte) error {
	if deviceID, isCommand := farm.ParseCommandTopic(topic); isCommand && router.manager != nil && router.manager.Enabled() {
		var command deviceprotocol.Command
		if json.Unmarshal(payload, &command) == nil {
			if handled, err := router.manager.Deliver(ctx, deviceID, command); handled {
				return err
			}
		}
	}
	return router.fallback.PublishRaw(ctx, topic, payload)
}

// FanOut delivers every notification to each notifier.
func FanOut(notifiers ...farm.Notifier) farm.Notifier { return fanOut(notifiers) }

type fanOut []farm.Notifier

func (notifiers fanOut) Notify(topic string) {
	for _, notifier := range notifiers {
		if notifier != nil {
			notifier.Notify(topic)
		}
	}
}
