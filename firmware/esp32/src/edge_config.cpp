#include "edge_config.h"

namespace {
constexpr const char *kNamespace = "grownerve";
constexpr const char *kVersionKey = "cfgver";
constexpr const char *kSettingsKey = "cfgblob";
}  // namespace

bool parseEdgeSettings(const JsonObjectConst &config, EdgeSettings &out, String &error) {
  EdgeSettings parsed;

  if (config["photoperiod"].is<JsonObjectConst>()) {
    JsonObjectConst period = config["photoperiod"];
    const char *channelId = period["channelId"] | "";
    if (strlen(channelId) != 36) {
      error = "photoperiod channelId must be a UUID";
      return false;
    }
    parsed.photoperiod.configured = true;
    parsed.photoperiod.onHour = period["onHour"] | 0;
    parsed.photoperiod.onMinute = period["onMinute"] | 0;
    parsed.photoperiod.offHour = period["offHour"] | 0;
    parsed.photoperiod.offMinute = period["offMinute"] | 0;
    if (parsed.photoperiod.onHour < 0 || parsed.photoperiod.onHour > 23 ||
        parsed.photoperiod.offHour < 0 || parsed.photoperiod.offHour > 23 ||
        parsed.photoperiod.onMinute < 0 || parsed.photoperiod.onMinute > 59 ||
        parsed.photoperiod.offMinute < 0 || parsed.photoperiod.offMinute > 59) {
      error = "photoperiod times are out of range";
      return false;
    }
    strncpy(parsed.photoperiod.channelId, channelId, sizeof(parsed.photoperiod.channelId) - 1);
  }

  if (config["fanMinimumPercent"].is<float>()) {
    parsed.fanMinimumPercent = config["fanMinimumPercent"];
    if (parsed.fanMinimumPercent < 0 || parsed.fanMinimumPercent > 100) {
      error = "fanMinimumPercent must be between 0 and 100";
      return false;
    }
    parsed.hasFanMinimum = true;
  }
  if (config["airPumpAlwaysOn"].is<bool>()) {
    parsed.airPumpAlwaysOn = config["airPumpAlwaysOn"];
  }
  if (config["telemetryIntervalSeconds"].is<uint32_t>()) {
    const uint32_t interval = config["telemetryIntervalSeconds"];
    // A pathological interval would either flood the broker or make the device
    // look offline, so it is clamped rather than trusted.
    parsed.telemetryIntervalSeconds = constrain(interval, 1U, 3600U);
  }
  if (config["commandTimeoutSeconds"].is<uint32_t>()) {
    parsed.commandTimeoutSeconds = constrain((uint32_t)config["commandTimeoutSeconds"], 1U, 86400U);
  }
  if (config["safeOutputs"].is<JsonObjectConst>()) {
    JsonObjectConst safeOutputs = config["safeOutputs"];
    for (JsonPairConst entry : safeOutputs) {
      const float value = entry.value().as<float>();
      if (strcmp(entry.key().c_str(), CHANNEL_LIGHT) == 0) parsed.safeLight = value;
      if (strcmp(entry.key().c_str(), CHANNEL_FAN) == 0) parsed.safeFan = value;
      if (strcmp(entry.key().c_str(), CHANNEL_AIR_PUMP) == 0) parsed.safeAirPump = value;
    }
  }

  out = parsed;
  return true;
}

void saveEdgeSettings(Preferences &storage, const EdgeSettings &settings, const String &version) {
  storage.begin(kNamespace, false);
  storage.putBytes(kSettingsKey, &settings, sizeof(settings));
  storage.putString(kVersionKey, version);
  storage.end();
}

bool loadEdgeSettings(Preferences &storage, EdgeSettings &settings, String &version) {
  storage.begin(kNamespace, true);
  const size_t stored = storage.getBytesLength(kSettingsKey);
  bool restored = false;
  // A struct of a different size means the firmware changed shape since the
  // configuration was written. Discarding it is safer than reinterpreting bytes
  // that no longer mean what they did.
  if (stored == sizeof(settings)) {
    storage.getBytes(kSettingsKey, &settings, sizeof(settings));
    version = storage.getString(kVersionKey, "");
    restored = version.length() > 0;
  }
  storage.end();
  return restored;
}
