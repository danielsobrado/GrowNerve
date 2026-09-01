// GrowNerve ESP32 reference controller.
//
// Contract, in order of importance:
//   1. Essential outputs keep running when the server is unreachable.
//   2. Configuration survives a reboot.
//   3. A watchdog resets a wedged controller into a safe state.
//   4. Manual overrides expire on their own; nothing latches indefinitely.
//   5. Outputs fail to their safe value, which for aeration means running.
//
// Sensor drivers are deliberately left as one clearly marked function. Wiring,
// calibration, and fail-state behaviour are commissioning decisions per
// installation, and guessing them in firmware would be the wrong kind of
// convenience. See docs/23-development-and-operations.md.

#include <Arduino.h>
#include <ArduinoJson.h>
#include <Preferences.h>
#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <PubSubClient.h>
#include <esp_task_wdt.h>
#include <time.h>

#include "secrets.h"
#include "edge_config.h"
#include "edge_policy.h"

namespace {

constexpr int kProtocolVersion = 1;
constexpr const char *kFirmwareVersion = "0.1.0";

// Output pins. Confirm every one of these against the actual wiring before
// energising anything; a wrong pin here drives the wrong relay.
constexpr uint8_t kLightRelayPin = 26;
constexpr uint8_t kFanPwmPin = 27;
constexpr uint8_t kAirPumpRelayPin = 25;
constexpr uint8_t kEmergencyInputPin = 34;

constexpr uint8_t kFanPwmChannel = 0;
constexpr uint32_t kFanPwmFrequency = 25000;
constexpr uint8_t kFanPwmResolution = 8;

// The watchdog is longer than the slowest normal cycle and far shorter than a
// grow cares about, so a wedged controller resets in seconds rather than hours.
constexpr uint32_t kWatchdogSeconds = 30;
constexpr uint32_t kReconnectIntervalMillis = 5000;

WiFiClientSecure secureClient;
PubSubClient mqtt(secureClient);
Preferences storage;

EdgeSettings settings;
String activeConfigVersion;
String bootId;

struct Override {
  bool active = false;
  float value = 0;
  uint32_t expiresAtMillis = 0;
};

Override lightOverride;
Override fanOverride;
Override airPumpOverride;

bool emergencyLatched = false;
uint32_t lastTelemetryMillis = 0;
uint32_t lastReconnectAttemptMillis = 0;

String topicFor(const char *suffix) {
  return String("grownerve/v1/devices/") + DEVICE_ID + "/" + suffix;
}

// ---------------------------------------------------------------------------
// Outputs
// ---------------------------------------------------------------------------

Override *overrideFor(const char *channelId) {
  if (strcmp(channelId, CHANNEL_LIGHT) == 0) return &lightOverride;
  if (strcmp(channelId, CHANNEL_FAN) == 0) return &fanOverride;
  if (strcmp(channelId, CHANNEL_AIR_PUMP) == 0) return &airPumpOverride;
  return nullptr;
}

// essentialValue is what this controller runs from its own persisted schedules,
// with no server involved.
float essentialValue(const char *channelId, const struct tm &localTime, float safeValue) {
  if (settings.photoperiod.configured && strcmp(channelId, settings.photoperiod.channelId) == 0) {
    return withinPhotoperiod(settings.photoperiod.onHour, settings.photoperiod.onMinute,
                             settings.photoperiod.offHour, settings.photoperiod.offMinute,
                             localTime.tm_hour, localTime.tm_min)
               ? 100.0f
               : 0.0f;
  }
  if (strcmp(channelId, CHANNEL_FAN) == 0 && settings.hasFanMinimum && settings.fanMinimumPercent > safeValue) {
    return settings.fanMinimumPercent;
  }
  if (strcmp(channelId, CHANNEL_AIR_PUMP) == 0 && settings.airPumpAlwaysOn) {
    return safeValue > 0 ? safeValue : 100.0f;
  }
  return safeValue;
}

OutputResolution resolveChannel(const char *channelId, float safeValue, const struct tm &localTime, uint32_t nowMillis) {
  OutputPolicy policy;
  policy.defaultSafeValue = safeValue;
  policy.essentialScheduleValue = essentialValue(channelId, localTime, safeValue);

  if (emergencyLatched) {
    policy.emergencyLatched = true;
    // The emergency value is the channel's safe state, not zero. Stopping
    // aeration would turn a safety action into a crop loss.
    policy.emergencyValue = policy.essentialScheduleValue;
    if (strcmp(channelId, CHANNEL_LIGHT) == 0) policy.emergencyValue = 0;
  }

  Override *pending = overrideFor(channelId);
  if (pending != nullptr && pending->active) {
    if ((int32_t)(pending->expiresAtMillis - nowMillis) > 0) {
      policy.hasOverride = true;
      policy.overrideValue = pending->value;
      policy.overrideExpiresAtMillis = pending->expiresAtMillis;
    } else {
      pending->active = false;
    }
  }
  return resolveOutput(policy, nowMillis);
}

void driveOutputs(const struct tm &localTime, uint32_t nowMillis) {
  const OutputResolution light = resolveChannel(CHANNEL_LIGHT, settings.safeLight, localTime, nowMillis);
  const OutputResolution fan = resolveChannel(CHANNEL_FAN, settings.safeFan, localTime, nowMillis);
  const OutputResolution airPump = resolveChannel(CHANNEL_AIR_PUMP, settings.safeAirPump, localTime, nowMillis);

  digitalWrite(kLightRelayPin, light.value > 50 ? HIGH : LOW);
  digitalWrite(kAirPumpRelayPin, airPump.value > 50 ? HIGH : LOW);
  ledcWrite(kFanPwmChannel, (uint32_t)(constrain(fan.value, 0.0f, 100.0f) * 255.0f / 100.0f));
}

// failSafeOutputs is what runs before the network exists and after a fault: the
// state a controller must hold when it knows nothing else.
void failSafeOutputs() {
  digitalWrite(kLightRelayPin, LOW);
  digitalWrite(kAirPumpRelayPin, settings.safeAirPump > 50 ? HIGH : LOW);
  ledcWrite(kFanPwmChannel, (uint32_t)(constrain(settings.safeFan, 0.0f, 100.0f) * 255.0f / 100.0f));
}

// ---------------------------------------------------------------------------
// Sensors
// ---------------------------------------------------------------------------

struct Reading {
  float value;
  const char *unit;
  const char *quality;
};

// readSensors is the one function a commissioning engineer must complete for a
// specific build. Until real drivers are wired in, every channel reports the
// "unknown" quality so the server treats the values as untrustworthy rather than
// charting invented numbers.
void readSensors(Reading &airTemperature, Reading &humidity, Reading &waterTemperature, Reading &waterLevel) {
  airTemperature = {0.0f, "degC", "unknown"};
  humidity = {0.0f, "%RH", "unknown"};
  waterTemperature = {0.0f, "degC", "unknown"};
  waterLevel = {0.0f, "%", "unknown"};
}

// ---------------------------------------------------------------------------
// MQTT
// ---------------------------------------------------------------------------

void publishConfigAck(const String &version, bool accepted, const String &detail) {
  JsonDocument document;
  document["protocolVersion"] = kProtocolVersion;
  document["deviceId"] = DEVICE_ID;
  document["configVersion"] = version;
  document["accepted"] = accepted;
  if (detail.length() > 0) document["detail"] = detail;

  char buffer[320];
  serializeJson(document, buffer, sizeof(buffer));
  mqtt.publish(topicFor("config/ack").c_str(), buffer, false);
}

void handleConfig(const JsonDocument &document) {
  const String version = document["configVersion"] | "";
  String error;
  if (document["protocolVersion"].as<int>() != kProtocolVersion) {
    publishConfigAck(version, false, "unsupported protocol version");
    return;
  }
  if (strcmp(document["deviceId"] | "", DEVICE_ID) != 0) {
    // Configuration addressed to another controller is ignored outright.
    return;
  }
  if (version.length() == 0) {
    publishConfigAck(version, false, "configVersion is required");
    return;
  }
  EdgeSettings parsed;
  if (!parseEdgeSettings(document["config"].as<JsonObjectConst>(), parsed, error)) {
    // The running configuration is kept: a bad update must not leave the
    // controller with nothing.
    publishConfigAck(version, false, error);
    return;
  }
  settings = parsed;
  activeConfigVersion = version;
  saveEdgeSettings(storage, settings, activeConfigVersion);
  publishConfigAck(version, true, "");
  Serial.printf("adopted configuration %s\n", version.c_str());
}

void publishCommandAck(const char *commandId, const char *result, const char *reasonCode) {
  JsonDocument document;
  document["protocolVersion"] = kProtocolVersion;
  document["commandId"] = commandId;
  document["deviceId"] = DEVICE_ID;
  document["result"] = result;
  if (strlen(reasonCode) > 0) document["reasonCode"] = reasonCode;

  char buffer[320];
  serializeJson(document, buffer, sizeof(buffer));
  mqtt.publish(topicFor("acks").c_str(), buffer, false);
}

void handleCommand(const JsonDocument &document) {
  const char *commandId = document["commandId"] | "";
  const char *channelId = document["targetChannelId"] | "";
  if (strlen(commandId) == 0 || strlen(channelId) == 0) return;

  if (document["protocolVersion"].as<int>() != kProtocolVersion) {
    publishCommandAck(commandId, "rejected", "UNSUPPORTED_PROTOCOL_VERSION");
    return;
  }
  if (emergencyLatched) {
    publishCommandAck(commandId, "rejected", "EMERGENCY_STOP_ACTIVE");
    return;
  }
  Override *target = overrideFor(channelId);
  if (target == nullptr) {
    publishCommandAck(commandId, "rejected", "UNKNOWN_CHANNEL");
    return;
  }

  float value = 0;
  if (document["value"].is<bool>()) {
    value = document["value"].as<bool>() ? 100.0f : 0.0f;
  } else if (document["value"].is<float>()) {
    value = constrain(document["value"].as<float>(), 0.0f, 100.0f);
  } else {
    publishCommandAck(commandId, "rejected", "INVALID_COMMAND_VALUE");
    return;
  }

  // The override is bounded by this controller's own timeout, so a server
  // asking for a very long override cannot leave an output latched.
  target->active = true;
  target->value = value;
  target->expiresAtMillis = millis() + settings.commandTimeoutSeconds * 1000UL;
  publishCommandAck(commandId, "applied", "");
}

void onMessage(char *topic, uint8_t *payload, unsigned int length) {
  JsonDocument document;
  if (deserializeJson(document, payload, length) != DeserializationError::Ok) {
    Serial.println("discarded an unreadable message");
    return;
  }
  const String subject(topic);
  if (subject.endsWith("/config")) {
    handleConfig(document);
    return;
  }
  if (subject.endsWith("/commands")) {
    handleCommand(document);
  }
}

void publishTelemetry(const char *timestamp, uint32_t sequence) {
  Reading airTemperature, humidity, waterTemperature, waterLevel;
  readSensors(airTemperature, humidity, waterTemperature, waterLevel);

  JsonDocument document;
  document["protocolVersion"] = kProtocolVersion;
  document["deviceId"] = DEVICE_ID;
  document["bootId"] = bootId;
  document["sequence"] = sequence;
  document["observedAt"] = timestamp;

  JsonArray samples = document["samples"].to<JsonArray>();
  const struct {
    const char *channelId;
    const Reading *reading;
  } channels[] = {
      {CHANNEL_AIR_TEMPERATURE, &airTemperature},
      {CHANNEL_HUMIDITY, &humidity},
      {CHANNEL_WATER_TEMPERATURE, &waterTemperature},
      {CHANNEL_WATER_LEVEL, &waterLevel},
  };
  for (const auto &channel : channels) {
    JsonObject sample = samples.add<JsonObject>();
    sample["channelId"] = channel.channelId;
    sample["value"] = channel.reading->value;
    sample["unit"] = channel.reading->unit;
    sample["quality"] = channel.reading->quality;
  }

  char buffer[1024];
  serializeJson(document, buffer, sizeof(buffer));
  mqtt.publish(topicFor("telemetry").c_str(), buffer, false);
}

void publishHealth(const char *timestamp) {
  JsonDocument document;
  document["protocolVersion"] = kProtocolVersion;
  document["deviceId"] = DEVICE_ID;
  document["firmwareVersion"] = kFirmwareVersion;
  document["bootId"] = bootId;
  document["uptimeSeconds"] = millis() / 1000UL;
  document["rssi"] = WiFi.RSSI();
  document["freeHeap"] = ESP.getFreeHeap();
  // The version reported is the one actually running, so a rejected
  // configuration shows on the server as a device that never adopted it.
  document["activeConfigVersion"] = activeConfigVersion;
  document["observedAt"] = timestamp;
  document["sensorFaults"].to<JsonArray>();

  char buffer[512];
  serializeJson(document, buffer, sizeof(buffer));
  mqtt.publish(topicFor("health").c_str(), buffer, false);
}

bool connectBroker() {
  if (millis() - lastReconnectAttemptMillis < kReconnectIntervalMillis) return false;
  lastReconnectAttemptMillis = millis();

  Serial.println("connecting to broker");
  if (!mqtt.connect(MQTT_USERNAME, MQTT_USERNAME, MQTT_PASSWORD)) {
    Serial.printf("broker connect failed, state %d\n", mqtt.state());
    return false;
  }
  // Both subscriptions use QoS 1, and the retained configuration arrives
  // immediately on subscribe. That is what lets a controller recover its
  // schedules after rebooting while the server is down.
  mqtt.subscribe(topicFor("config").c_str(), 1);
  mqtt.subscribe(topicFor("commands").c_str(), 1);
  Serial.println("broker connected");
  return true;
}

void connectWiFi() {
  WiFi.mode(WIFI_STA);
  WiFi.setSleep(false);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  // The loop is bounded: the controller must return to driving its outputs
  // rather than blocking forever on a network that is not coming back.
  const uint32_t deadline = millis() + 20000;
  while (WiFi.status() != WL_CONNECTED && millis() < deadline) {
    esp_task_wdt_reset();
    delay(250);
  }
  Serial.println(WiFi.status() == WL_CONNECTED ? "wifi connected" : "wifi unavailable; running locally");
}

bool currentLocalTime(struct tm &out, char *iso, size_t isoSize) {
  if (!getLocalTime(&out, 50)) return false;
  strftime(iso, isoSize, "%Y-%m-%dT%H:%M:%SZ", &out);
  return true;
}

}  // namespace

void setup() {
  Serial.begin(115200);

  pinMode(kLightRelayPin, OUTPUT);
  pinMode(kAirPumpRelayPin, OUTPUT);
  pinMode(kEmergencyInputPin, INPUT_PULLUP);
  ledcSetup(kFanPwmChannel, kFanPwmFrequency, kFanPwmResolution);
  ledcAttachPin(kFanPwmPin, kFanPwmChannel);

  // Outputs are driven to their safe state before anything else runs, so a
  // failure during startup leaves known state rather than floating pins.
  failSafeOutputs();

  esp_task_wdt_init(kWatchdogSeconds, true);
  esp_task_wdt_add(nullptr);

  bootId = String(ESP.getEfuseMac(), HEX) + "-" + String(millis(), HEX);

  if (loadEdgeSettings(storage, settings, activeConfigVersion)) {
    Serial.printf("restored configuration %s from flash\n", activeConfigVersion.c_str());
  } else {
    Serial.println("no stored configuration; running safe defaults");
  }
  failSafeOutputs();

  connectWiFi();
  configTime(0, 0, "pool.ntp.org", "time.nist.gov");
  secureClient.setCACert(MQTT_CA_CERTIFICATE);
  mqtt.setServer(MQTT_HOST, MQTT_PORT);
  mqtt.setBufferSize(2048);
  mqtt.setCallback(onMessage);
}

void loop() {
  esp_task_wdt_reset();

  // The emergency input is read every cycle and latches. Clearing it is a
  // deliberate operator action at the hardware, never an automatic recovery.
  if (digitalRead(kEmergencyInputPin) == LOW) {
    if (!emergencyLatched) Serial.println("emergency input latched");
    emergencyLatched = true;
    lightOverride.active = fanOverride.active = airPumpOverride.active = false;
  }

  if (WiFi.status() != WL_CONNECTED) {
    connectWiFi();
  } else if (!mqtt.connected()) {
    connectBroker();
  } else {
    mqtt.loop();
  }

  struct tm localTime;
  char timestamp[32];
  const bool haveClock = currentLocalTime(localTime, timestamp, sizeof(timestamp));

  if (haveClock) {
    // Schedules run from the local clock, which is why the controller keeps
    // working with no server: nothing in this path needs the network.
    driveOutputs(localTime, millis());
  } else {
    // Without a trustworthy clock a photoperiod cannot be evaluated, so the
    // controller holds safe outputs rather than guessing.
    failSafeOutputs();
  }

  static uint32_t sequence = 0;
  const uint32_t interval = settings.telemetryIntervalSeconds * 1000UL;
  if (mqtt.connected() && haveClock && millis() - lastTelemetryMillis >= interval) {
    lastTelemetryMillis = millis();
    publishTelemetry(timestamp, ++sequence);
    publishHealth(timestamp);
  }

  delay(100);
}
