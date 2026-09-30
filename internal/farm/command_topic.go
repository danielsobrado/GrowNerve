package farm

import "strings"

const (
	deviceTopicPrefix  = "grownerve/v1/devices/"
	commandTopicSuffix = "/commands"
)

// CommandTopic is where a device's commands are published and, when the
// transport is down, the outbox key they are queued under. Integration devices
// share the format so the outbox can hold and replay their commands too.
func CommandTopic(deviceID string) string {
	return deviceTopicPrefix + deviceID + commandTopicSuffix
}

// ParseCommandTopic recovers the device a queued command is addressed to.
func ParseCommandTopic(topic string) (string, bool) {
	if !strings.HasPrefix(topic, deviceTopicPrefix) || !strings.HasSuffix(topic, commandTopicSuffix) {
		return "", false
	}
	deviceID := strings.TrimSuffix(strings.TrimPrefix(topic, deviceTopicPrefix), commandTopicSuffix)
	if deviceID == "" || strings.Contains(deviceID, "/") {
		return "", false
	}
	return deviceID, true
}
