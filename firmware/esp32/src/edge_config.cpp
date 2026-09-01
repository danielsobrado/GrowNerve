#include "edge_config.h"

namespace {
constexpr const char *kNamespace = "grownerve";
constexpr const char *kVersionKey = "cfgver";
constexpr const char *kSettingsKey = "cfgblob";

bool validScheduleWindow(int onHour, int onMinute, int offHour, int offMinute) {
  return onHour >= 0 && onHour <= 23 && offHour >= 0 && offHour <= 23 &&
         onMinute >= 0 && onMinute <= 59 && offMinute >= 0 && offMinute <= 59;
}
}  // namespace

bool parseEdgeSettings(const JsonObjectConst &config, EdgeSettings &out, String &error) {
  EdgeSettings parsed;

  const char *timezone = config["timezonePosix"] | "";
  if (strlen(timezone) > 0) {
    if (strlen(timezone) >= sizeof(parsed.timezonePosix)) {
      error = "timezonePosix is too long";
      return false;
    }
    strncpy(parsed.timezonePosix, timezone, sizeof(parsed.timezonePosix) - 1);
  }

  if (config["photoperiod"].is<JsonObjectConst>()) {
    JsonObjectConst period = config["photoperiod"];
    const char *channelId = period["channelId"] | "";
    if (strlen(timezone) == 0) {
      error = "timezonePosix is required for photoperiod";
      return false;
    }
    if (strlen(channelId) != 36) {
      error = "photoperiod channelId must be a UUID";
      return false;
    }
    parsed.photoperiod.configured = true;
    parsed.photoperiod.onHour = period["onHour"] | 0;
    parsed.photoperiod.onMinute = period["onMinute"] | 0;
    parsed.photoperiod.offHour = period["offHour"] | 0;
    parsed.photoperiod.offMinute = period["offMinute"] | 0;
    if (!validScheduleWindow(parsed.photoperiod.onHour, parsed.photoperiod.onMinute,
                             parsed.photoperiod.offHour, parsed.photoperiod.offMinute)) {
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

  if (config["fanSchedule"].is<JsonObjectConst>()) {
    JsonObjectConst schedule = config["fanSchedule"];
    const char *channelId = schedule["channelId"] | "";
    if (strlen(timezone) == 0) {
      error = "timezonePosix is required for fanSchedule";
      return false;
    }
    if (strlen(channelId) != 36) {
      error = "fanSchedule channelId must be a UUID";
      return false;
    }
    parsed.fanSchedule.configured = true;
    parsed.fanSchedule.onHour = schedule["onHour"] | 0;
    parsed.fanSchedule.onMinute = schedule["onMinute"] | 0;
    parsed.fanSchedule.offHour = schedule["offHour"] | 0;
    parsed.fanSchedule.offMinute = schedule["offMinute"] | 0;
    parsed.fanSchedule.activePercent = schedule["activePercent"] | 0.0f;
    parsed.fanSchedule.inactivePercent = schedule["inactivePercent"] | 0.0f;
    if (!validScheduleWindow(parsed.fanSchedule.onHour, parsed.fanSchedule.onMinute,
                             parsed.fanSchedule.offHour, parsed.fanSchedule.offMinute)) {
      error = "fanSchedule times are out of range";
      return false;
    }
    if (parsed.fanSchedule.activePercent < 0 || parsed.fanSchedule.activePercent > 100 ||
        parsed.fanSchedule.inactivePercent < 0 || parsed.fanSchedule.inactivePercent > 100) {
      error = "fanSchedule percentages must be between 0 and 100";
      return false;
    }
    strncpy(parsed.fanSchedule.channelId, channelId, sizeof(parsed.fanSchedule.channelId) - 1);
  }

  if (config["airPumpAlwaysOn"].is<bool>()) {
    parsed.airPumpAlwaysOn = config["airPumpAlwaysOn"];
  }
  if (config["telemetryIntervalSeconds"].is<uint32_t>()) {
    parsed.telemetryIntervalSeconds = constrain((uint32_t)config["telemetryIntervalSeconds"], 1U, 3600U);
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
  if (stored == sizeof(settings)) {
    storage.getBytes(kSettingsKey, &settings, sizeof(settings));
    version = storage.getString(kVersionKey, "");
    restored = version.length() > 0;
  }
  storage.end();
  return restored;
}
