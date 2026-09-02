// GrowNerve ESP32 reference controller.
#include <Arduino.h>
#include <ArduinoJson.h>
#include <Preferences.h>
#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <PubSubClient.h>
#include <ctype.h>
#include <esp_task_wdt.h>
#include <time.h>

#include "secrets.h"
#include "command_replay.h"
#include "edge_config.h"
#include "edge_policy.h"
#include "hardware_config.h"

namespace {

constexpr int kProtocolVersion = 1;
constexpr const char *kFirmwareVersion = "0.1.6";
constexpr uint8_t kFanPwmChannel = 0;
constexpr uint32_t kFanPwmFrequency = 25000;
constexpr uint8_t kFanPwmResolution = 8;
constexpr uint32_t kWatchdogSeconds = 30;
constexpr uint32_t kReconnectIntervalMillis = 5000;
constexpr uint32_t kMaximumCommandLifetimeSeconds = 300;
constexpr uint32_t kMaximumFutureClockSkewSeconds = 60;
constexpr time_t kMinimumTrustedEpoch = 1704067200;  // 2024-01-01 UTC

WiFiClientSecure secureClient;
PubSubClient mqtt(secureClient);
Preferences storage;
CommandReplayCache commandReplay;

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

void applyTimezone() {
  setenv("TZ", settings.timezonePosix[0] == '\0' ? "UTC0" : settings.timezonePosix, 1);
  tzset();
}

bool trustedClock(time_t &now) {
  now = time(nullptr);
  return now >= kMinimumTrustedEpoch;
}

bool utcTimestamp(char *iso, size_t isoSize) {
  time_t now;
  if (!trustedClock(now)) return false;
  struct tm utcTime;
  gmtime_r(&now, &utcTime);
  return strftime(iso, isoSize, "%Y-%m-%dT%H:%M:%SZ", &utcTime) > 0;
}

bool currentTimes(struct tm &localTime, char *iso, size_t isoSize) {
  time_t now;
  if (!trustedClock(now)) return false;
  localtime_r(&now, &localTime);
  struct tm utcTime;
  gmtime_r(&now, &utcTime);
  return strftime(iso, isoSize, "%Y-%m-%dT%H:%M:%SZ", &utcTime) > 0;
}

bool validTimestampSuffix(const char *text) {
  const size_t length = strlen(text);
  if (length == 20) return text[19] == 'Z';
  if (length < 22 || text[19] != '.' || text[length - 1] != 'Z') return false;
  for (size_t index = 20; index + 1 < length; ++index) {
    if (!isdigit(static_cast<unsigned char>(text[index]))) return false;
  }
  return true;
}

bool parseUtcTimestamp(const char *text, time_t &out) {
  if (text == nullptr || strlen(text) < 20 || !validTimestampSuffix(text)) return false;
  if (text[4] != '-' || text[7] != '-' || text[10] != 'T' || text[13] != ':' || text[16] != ':') return false;

  int year, month, day, hour, minute, second;
  if (sscanf(text, "%d-%d-%dT%d:%d:%d", &year, &month, &day, &hour, &minute, &second) != 6) return false;
  if (year < 1970 || year > 2100 || month < 1 || month > 12 || day < 1 || day > 31 ||
      hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 60) {
    return false;
  }

  struct tm parsed = {};
  parsed.tm_year = year - 1900;
  parsed.tm_mon = month - 1;
  parsed.tm_mday = day;
  parsed.tm_hour = hour;
  parsed.tm_min = minute;
  parsed.tm_sec = second;
  out = timegm(&parsed);
  if (out <= 0) return false;

  struct tm normalized;
  gmtime_r(&out, &normalized);
  return normalized.tm_year == year - 1900 && normalized.tm_mon == month - 1 &&
         normalized.tm_mday == day && normalized.tm_hour == hour && normalized.tm_min == minute &&
         normalized.tm_sec == second;
}

Override *overrideFor(const char *channelId) {
  if (strcmp(channelId, CHANNEL_LIGHT) == 0) return &lightOverride;
  if (strcmp(channelId, CHANNEL_FAN) == 0) return &fanOverride;
  if (strcmp(channelId, CHANNEL_AIR_PUMP) == 0) return &airPumpOverride;
  return nullptr;
}

bool withinWindow(int onHour, int onMinute, int offHour, int offMinute, const struct tm &localTime) {
  const int minutes = localTime.tm_hour * 60 + localTime.tm_min;
  const int on = onHour * 60 + onMinute;
  const int off = offHour * 60 + offMinute;
  if (on == off) return false;
  if (on < off) return minutes >= on && minutes < off;
  return minutes >= on || minutes < off;
}

float essentialValue(const char *channelId, const struct tm &localTime, float safeValue) {
  if (settings.photoperiod.configured && strcmp(channelId, settings.photoperiod.channelId) == 0) {
    return withinWindow(settings.photoperiod.onHour, settings.photoperiod.onMinute,
                        settings.photoperiod.offHour, settings.photoperiod.offMinute, localTime)
               ? 100.0f
               : 0.0f;
  }
  if (settings.fanSchedule.configured && strcmp(channelId, settings.fanSchedule.channelId) == 0) {
    float scheduled = withinWindow(settings.fanSchedule.onHour, settings.fanSchedule.onMinute,
                                   settings.fanSchedule.offHour, settings.fanSchedule.offMinute, localTime)
                          ? settings.fanSchedule.activePercent
                          : settings.fanSchedule.inactivePercent;
    if (settings.hasFanMinimum && scheduled < settings.fanMinimumPercent) scheduled = settings.fanMinimumPercent;
    return max(safeValue, scheduled);
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

void writeRelay(uint8_t pin, bool on, bool activeHigh) {
  const bool electricalHigh = on == activeHigh;
  digitalWrite(pin, electricalHigh ? HIGH : LOW);
}

void driveOutputs(const struct tm &localTime, uint32_t nowMillis) {
  const OutputResolution light = resolveChannel(CHANNEL_LIGHT, settings.safeLight, localTime, nowMillis);
  const OutputResolution fan = resolveChannel(CHANNEL_FAN, settings.safeFan, localTime, nowMillis);
  const OutputResolution airPump = resolveChannel(CHANNEL_AIR_PUMP, settings.safeAirPump, localTime, nowMillis);

  writeRelay(GN_LIGHT_RELAY_PIN, light.value > 50, GN_LIGHT_RELAY_ACTIVE_HIGH);
  writeRelay(GN_AIR_PUMP_RELAY_PIN, airPump.value > 50, GN_AIR_PUMP_RELAY_ACTIVE_HIGH);
  ledcWrite(kFanPwmChannel, (uint32_t)(constrain(fan.value, 0.0f, 100.0f) * 255.0f / 100.0f));
}

void failSafeOutputs() {
  writeRelay(GN_LIGHT_RELAY_PIN, false, GN_LIGHT_RELAY_ACTIVE_HIGH);
  writeRelay(GN_AIR_PUMP_RELAY_PIN, settings.safeAirPump > 50, GN_AIR_PUMP_RELAY_ACTIVE_HIGH);
  ledcWrite(kFanPwmChannel, (uint32_t)(constrain(settings.safeFan, 0.0f, 100.0f) * 255.0f / 100.0f));
}

struct Reading {
  float value;
  const char *unit;
  const char *quality;
};

void readSensors(Reading &airTemperature, Reading &humidity, Reading &waterTemperature, Reading &waterLevel) {
  airTemperature = {0.0f, "degC", "unknown"};
  humidity = {0.0f, "%RH", "unknown"};
  waterTemperature = {0.0f, "degC", "unknown"};
  waterLevel = {0.0f, "%", "unknown"};
}

void addAcknowledgedAt(JsonDocument &document) {
  char timestamp[32];
  if (utcTimestamp(timestamp, sizeof(timestamp))) {
    document["acknowledgedAt"] = timestamp;
  } else {
    document["acknowledgedAt"] = "1970-01-01T00:00:01Z";
  }
}

void publishConfigAck(const String &version, bool accepted, const String &detail) {
  JsonDocument document;
  document["protocolVersion"] = kProtocolVersion;
  document["deviceId"] = DEVICE_ID;
  document["configVersion"] = version;
  document["accepted"] = accepted;
  if (detail.length() > 0) document["detail"] = detail;
  addAcknowledgedAt(document);

  char buffer[384];
  serializeJson(document, buffer, sizeof(buffer));
  mqtt.publish(topicFor("config/ack").c_str(), buffer, false);
}

void handleConfig(const JsonDocument &document) {
  const String version = document["configVersion"] | "";
  if (document["protocolVersion"].as<int>() != kProtocolVersion) {
    publishConfigAck(version, false, "unsupported protocol version");
    return;
  }
  if (strcmp(document["deviceId"] | "", DEVICE_ID) != 0) return;
  if (version.length() == 0) {
    publishConfigAck(version, false, "configVersion is required");
    return;
  }
  if (!document["config"].is<JsonObjectConst>()) {
    publishConfigAck(version, false, "config must be an object");
    return;
  }

  EdgeSettings parsed;
  String error;
  if (!parseEdgeSettings(document["config"].as<JsonObjectConst>(), parsed, error)) {
    publishConfigAck(version, false, error);
    return;
  }
  if (!saveEdgeSettings(storage, parsed, version)) {
    publishConfigAck(version, false, "configuration persistence failed");
    return;
  }

  settings = parsed;
  activeConfigVersion = version;
  applyTimezone();
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
  addAcknowledgedAt(document);

  char buffer[384];
  serializeJson(document, buffer, sizeof(buffer));
  mqtt.publish(topicFor("acks").c_str(), buffer, false);
}

void completeCommand(const char *commandId, const char *result, const char *reasonCode) {
  if (!commandReplay.remember(storage, commandId, result, reasonCode)) {
    Serial.printf("command replay result was not persisted: %s\n", commandId);
  }
  publishCommandAck(commandId, result, reasonCode);
}

const char *validateCommandPolicy(const char *channelId, const char *type, float value) {
  if (strcmp(channelId, CHANNEL_LIGHT) == 0) {
    return strcmp(type, "set_boolean") == 0 ? nullptr : "INVALID_COMMAND_TYPE";
  }
  if (strcmp(channelId, CHANNEL_FAN) == 0) {
    if (strcmp(type, "set_percent") != 0) return "INVALID_COMMAND_TYPE";
    if (settings.hasFanMinimum && value < settings.fanMinimumPercent) return "LOCAL_SAFETY_LIMIT";
    return nullptr;
  }
  if (strcmp(channelId, CHANNEL_AIR_PUMP) == 0) {
    if (strcmp(type, "set_boolean") != 0) return "INVALID_COMMAND_TYPE";
    if (settings.airPumpAlwaysOn && value <= 50.0f) return "LOCAL_SAFETY_LIMIT";
    return nullptr;
  }
  return "UNKNOWN_CHANNEL";
}

void handleCommand(const JsonDocument &document) {
  const char *commandId = document["commandId"] | "";
  const char *channelId = document["targetChannelId"] | "";
  const char *type = document["type"] | "";
  if (strlen(commandId) != 36 || strlen(channelId) == 0) return;

  CommandReplayResult replay;
  if (commandReplay.find(commandId, replay)) {
    publishCommandAck(commandId, replay.result.c_str(), replay.reasonCode.c_str());
    return;
  }

  if (document["protocolVersion"].as<int>() != kProtocolVersion) {
    completeCommand(commandId, "rejected", "UNSUPPORTED_PROTOCOL_VERSION");
    return;
  }
  if (emergencyLatched) {
    completeCommand(commandId, "rejected", "EMERGENCY_STOP_ACTIVE");
    return;
  }
  Override *target = overrideFor(channelId);
  if (target == nullptr) {
    completeCommand(commandId, "rejected", "UNKNOWN_CHANNEL");
    return;
  }

  time_t nowEpoch;
  if (!trustedClock(nowEpoch)) {
    completeCommand(commandId, "rejected", "CLOCK_UNAVAILABLE");
    return;
  }
  time_t issuedAt;
  time_t expiresAt;
  if (!parseUtcTimestamp(document["issuedAt"] | "", issuedAt) || !parseUtcTimestamp(document["expiresAt"] | "", expiresAt)) {
    completeCommand(commandId, "rejected", "INVALID_COMMAND_TIME");
    return;
  }
  if (expiresAt <= issuedAt) {
    completeCommand(commandId, "rejected", "INVALID_COMMAND_TIME");
    return;
  }
  if (expiresAt - issuedAt > kMaximumCommandLifetimeSeconds) {
    completeCommand(commandId, "rejected", "COMMAND_TTL_TOO_LONG");
    return;
  }
  if (expiresAt <= nowEpoch) {
    completeCommand(commandId, "rejected", "COMMAND_EXPIRED");
    return;
  }
  if (issuedAt > nowEpoch + kMaximumFutureClockSkewSeconds) {
    completeCommand(commandId, "rejected", "COMMAND_FROM_FUTURE");
    return;
  }
  if (issuedAt < nowEpoch - kMaximumCommandLifetimeSeconds) {
    completeCommand(commandId, "rejected", "COMMAND_STALE");
    return;
  }

  float value = 0;
  if (strcmp(type, "set_boolean") == 0 && document["value"].is<bool>()) {
    value = document["value"].as<bool>() ? 100.0f : 0.0f;
  } else if (strcmp(type, "set_percent") == 0 && document["value"].is<float>()) {
    value = document["value"].as<float>();
    if (!isfinite(value) || value < 0 || value > 100) {
      completeCommand(commandId, "rejected", "COMMAND_VALUE_OUT_OF_RANGE");
      return;
    }
  } else {
    completeCommand(commandId, "rejected", "INVALID_COMMAND_VALUE");
    return;
  }

  if (const char *reason = validateCommandPolicy(channelId, type, value); reason != nullptr) {
    completeCommand(commandId, "rejected", reason);
    return;
  }

  const uint32_t remainingSeconds = (uint32_t)(expiresAt - nowEpoch);
  const uint32_t configuredLimit = settings.commandTimeoutSeconds > 0 ? settings.commandTimeoutSeconds : kMaximumCommandLifetimeSeconds;
  const uint32_t localLimit = min(configuredLimit, kMaximumCommandLifetimeSeconds);
  const uint32_t durationSeconds = min(remainingSeconds, localLimit);
  if (durationSeconds == 0) {
    completeCommand(commandId, "rejected", "COMMAND_EXPIRED");
    return;
  }

  target->active = true;
  target->value = value;
  target->expiresAtMillis = millis() + durationSeconds * 1000UL;
  completeCommand(commandId, "applied", "");
}

void onMessage(char *topic, uint8_t *payload, unsigned int length) {
  JsonDocument document;
  if (deserializeJson(document, payload, length) != DeserializationError::Ok) {
    Serial.println("discarded an unreadable message");
    return;
  }
  const String subject(topic);
  if (subject == topicFor("config")) {
    handleConfig(document);
    return;
  }
  if (subject == topicFor("commands")) handleCommand(document);
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
  if (!mqtt.connect(DEVICE_ID, MQTT_USERNAME, MQTT_PASSWORD)) {
    Serial.printf("broker connect failed, state %d\n", mqtt.state());
    return false;
  }
  if (!mqtt.subscribe(topicFor("config").c_str(), 1) || !mqtt.subscribe(topicFor("commands").c_str(), 1)) {
    Serial.println("broker subscription failed");
    mqtt.disconnect();
    return false;
  }
  Serial.println("broker connected");
  return true;
}

void connectWiFi() {
  WiFi.mode(WIFI_STA);
  WiFi.setSleep(false);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  const uint32_t deadline = millis() + 20000;
  while (WiFi.status() != WL_CONNECTED && millis() < deadline) {
    esp_task_wdt_reset();
    delay(250);
  }
  Serial.println(WiFi.status() == WL_CONNECTED ? "wifi connected" : "wifi unavailable; running locally");
}

}  // namespace

void setup() {
  Serial.begin(115200);

  pinMode(GN_LIGHT_RELAY_PIN, OUTPUT);
  pinMode(GN_AIR_PUMP_RELAY_PIN, OUTPUT);
  pinMode(GN_EMERGENCY_INPUT_PIN, INPUT_PULLUP);
  ledcSetup(kFanPwmChannel, kFanPwmFrequency, kFanPwmResolution);
  ledcAttachPin(GN_FAN_PWM_PIN, kFanPwmChannel);

  failSafeOutputs();
  esp_task_wdt_init(kWatchdogSeconds, true);
  esp_task_wdt_add(nullptr);
  bootId = String(ESP.getEfuseMac(), HEX) + "-" + String(millis(), HEX);

  if (loadEdgeSettings(storage, settings, activeConfigVersion)) {
    Serial.printf("restored configuration %s from flash\n", activeConfigVersion.c_str());
  } else {
    Serial.println("no stored configuration; running safe defaults");
  }
  commandReplay.load(storage);
  applyTimezone();
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

  const bool emergencyActive = digitalRead(GN_EMERGENCY_INPUT_PIN) == (GN_EMERGENCY_ACTIVE_HIGH ? HIGH : LOW);
  if (emergencyActive) {
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
  const bool haveClock = currentTimes(localTime, timestamp, sizeof(timestamp));
  if (haveClock) {
    driveOutputs(localTime, millis());
  } else {
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
