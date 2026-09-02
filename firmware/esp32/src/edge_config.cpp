#include "edge_config.h"

#include <ctype.h>
#include <math.h>

namespace {
constexpr const char *kNamespace = "grownerve";
constexpr const char *kRecordKey = "cfgrecord";
constexpr uint32_t kStorageMagic = 0x474E4346;  // GNCF
constexpr uint16_t kStorageVersion = 1;
constexpr size_t kMaximumConfigVersionLength = 64;

struct PersistedEdgeConfig {
  uint32_t magic = kStorageMagic;
  uint16_t storageVersion = kStorageVersion;
  EdgeSettings settings;
  char configVersion[kMaximumConfigVersionLength + 1] = {0};
};

bool validScheduleWindow(int onHour, int onMinute, int offHour, int offMinute) {
  if (onHour < 0 || onHour > 23 || offHour < 0 || offHour > 23 ||
      onMinute < 0 || onMinute > 59 || offMinute < 0 || offMinute > 59) {
    return false;
  }
  return onHour != offHour || onMinute != offMinute;
}

bool validPosixTimezone(const char *timezone) {
  if (timezone == nullptr) return false;
  const size_t length = strlen(timezone);
  if (length == 0 || length >= sizeof(EdgeSettings().timezonePosix)) return false;
  bool hasDigit = false;
  for (size_t index = 0; index < length; ++index) {
    const unsigned char value = static_cast<unsigned char>(timezone[index]);
    if (value == '/' || isspace(value)) return false;
    if (isdigit(value)) hasDigit = true;
  }
  return hasDigit;
}

bool knownSafeOutput(const char *channelId) {
  return strcmp(channelId, CHANNEL_LIGHT) == 0 || strcmp(channelId, CHANNEL_FAN) == 0 ||
         strcmp(channelId, CHANNEL_AIR_PUMP) == 0;
}

bool readRequiredInt(const JsonObjectConst &object, const char *key, int &out) {
  if (!object[key].is<int>()) return false;
  out = object[key].as<int>();
  return true;
}

bool readRequiredFloat(const JsonObjectConst &object, const char *key, float &out) {
  if (!object[key].is<float>()) return false;
  out = object[key].as<float>();
  return isfinite(out);
}
}  // namespace

bool parseEdgeSettings(const JsonObjectConst &config, EdgeSettings &out, String &error) {
  EdgeSettings parsed;

  const char *timezone = config["timezonePosix"] | "";
  if (strlen(timezone) > 0) {
    if (!config["timezonePosix"].is<const char *>() || !validPosixTimezone(timezone)) {
      error = "timezonePosix must be a POSIX TZ rule with an explicit offset";
      return false;
    }
    strncpy(parsed.timezonePosix, timezone, sizeof(parsed.timezonePosix) - 1);
  }

  if (config.containsKey("photoperiod")) {
    if (!config["photoperiod"].is<JsonObjectConst>()) {
      error = "photoperiod must be an object";
      return false;
    }
    JsonObjectConst period = config["photoperiod"];
    const char *channelId = period["channelId"] | "";
    if (strlen(timezone) == 0) {
      error = "timezonePosix is required for photoperiod";
      return false;
    }
    if (!period["channelId"].is<const char *>() || strlen(channelId) != 36) {
      error = "photoperiod channelId must be a UUID";
      return false;
    }
    parsed.photoperiod.configured = true;
    if (!readRequiredInt(period, "onHour", parsed.photoperiod.onHour) ||
        !readRequiredInt(period, "onMinute", parsed.photoperiod.onMinute) ||
        !readRequiredInt(period, "offHour", parsed.photoperiod.offHour) ||
        !readRequiredInt(period, "offMinute", parsed.photoperiod.offMinute)) {
      error = "photoperiod times must be integers";
      return false;
    }
    if (!validScheduleWindow(parsed.photoperiod.onHour, parsed.photoperiod.onMinute,
                             parsed.photoperiod.offHour, parsed.photoperiod.offMinute)) {
      error = "photoperiod times are invalid or define an empty window";
      return false;
    }
    strncpy(parsed.photoperiod.channelId, channelId, sizeof(parsed.photoperiod.channelId) - 1);
  }

  if (config.containsKey("fanMinimumPercent")) {
    if (!config["fanMinimumPercent"].is<float>()) {
      error = "fanMinimumPercent must be numeric";
      return false;
    }
    parsed.fanMinimumPercent = config["fanMinimumPercent"].as<float>();
    if (!isfinite(parsed.fanMinimumPercent) || parsed.fanMinimumPercent < 0 || parsed.fanMinimumPercent > 100) {
      error = "fanMinimumPercent must be finite and between 0 and 100";
      return false;
    }
    parsed.hasFanMinimum = true;
  }

  if (config.containsKey("fanSchedule")) {
    if (!config["fanSchedule"].is<JsonObjectConst>()) {
      error = "fanSchedule must be an object";
      return false;
    }
    JsonObjectConst schedule = config["fanSchedule"];
    const char *channelId = schedule["channelId"] | "";
    if (strlen(timezone) == 0) {
      error = "timezonePosix is required for fanSchedule";
      return false;
    }
    if (!schedule["channelId"].is<const char *>() || strlen(channelId) != 36) {
      error = "fanSchedule channelId must be a UUID";
      return false;
    }
    parsed.fanSchedule.configured = true;
    if (!readRequiredInt(schedule, "onHour", parsed.fanSchedule.onHour) ||
        !readRequiredInt(schedule, "onMinute", parsed.fanSchedule.onMinute) ||
        !readRequiredInt(schedule, "offHour", parsed.fanSchedule.offHour) ||
        !readRequiredInt(schedule, "offMinute", parsed.fanSchedule.offMinute)) {
      error = "fanSchedule times must be integers";
      return false;
    }
    if (!readRequiredFloat(schedule, "activePercent", parsed.fanSchedule.activePercent) ||
        !readRequiredFloat(schedule, "inactivePercent", parsed.fanSchedule.inactivePercent)) {
      error = "fanSchedule percentages must be finite numbers";
      return false;
    }
    if (!validScheduleWindow(parsed.fanSchedule.onHour, parsed.fanSchedule.onMinute,
                             parsed.fanSchedule.offHour, parsed.fanSchedule.offMinute)) {
      error = "fanSchedule times are invalid or define an empty window";
      return false;
    }
    if (parsed.fanSchedule.activePercent < 0 || parsed.fanSchedule.activePercent > 100 ||
        parsed.fanSchedule.inactivePercent < 0 || parsed.fanSchedule.inactivePercent > 100) {
      error = "fanSchedule percentages must be between 0 and 100";
      return false;
    }
    strncpy(parsed.fanSchedule.channelId, channelId, sizeof(parsed.fanSchedule.channelId) - 1);
  }

  if (config.containsKey("airPumpAlwaysOn")) {
    if (!config["airPumpAlwaysOn"].is<bool>()) {
      error = "airPumpAlwaysOn must be boolean";
      return false;
    }
    parsed.airPumpAlwaysOn = config["airPumpAlwaysOn"].as<bool>();
  }
  if (config.containsKey("telemetryIntervalSeconds")) {
    if (!config["telemetryIntervalSeconds"].is<uint32_t>()) {
      error = "telemetryIntervalSeconds must be a non-negative integer";
      return false;
    }
    const uint32_t value = config["telemetryIntervalSeconds"].as<uint32_t>();
    if (value > 3600U) {
      error = "telemetryIntervalSeconds must be between 0 and 3600";
      return false;
    }
    if (value > 0) parsed.telemetryIntervalSeconds = value;
  }
  if (config.containsKey("commandTimeoutSeconds")) {
    if (!config["commandTimeoutSeconds"].is<uint32_t>()) {
      error = "commandTimeoutSeconds must be a non-negative integer";
      return false;
    }
    const uint32_t value = config["commandTimeoutSeconds"].as<uint32_t>();
    if (value > 86400U) {
      error = "commandTimeoutSeconds must be between 0 and 86400";
      return false;
    }
    if (value > 0) parsed.commandTimeoutSeconds = value;
  }
  if (config.containsKey("safeOutputs")) {
    if (!config["safeOutputs"].is<JsonObjectConst>()) {
      error = "safeOutputs must be an object";
      return false;
    }
    JsonObjectConst safeOutputs = config["safeOutputs"];
    for (JsonPairConst entry : safeOutputs) {
      const char *channelId = entry.key().c_str();
      if (!knownSafeOutput(channelId)) {
        error = "safeOutputs contains an unknown channel";
        return false;
      }
      if (!entry.value().is<float>()) {
        error = "safeOutputs values must be numeric";
        return false;
      }
      const float value = entry.value().as<float>();
      if (!isfinite(value) || value < 0 || value > 100) {
        error = "safeOutputs values must be finite and between 0 and 100";
        return false;
      }
      if (strcmp(channelId, CHANNEL_LIGHT) == 0) parsed.safeLight = value;
      if (strcmp(channelId, CHANNEL_FAN) == 0) parsed.safeFan = value;
      if (strcmp(channelId, CHANNEL_AIR_PUMP) == 0) parsed.safeAirPump = value;
    }
  }

  out = parsed;
  return true;
}

bool saveEdgeSettings(Preferences &storage, const EdgeSettings &settings, const String &version) {
  if (version.length() == 0 || version.length() > kMaximumConfigVersionLength) return false;

  PersistedEdgeConfig record{};
  record.settings = settings;
  version.toCharArray(record.configVersion, sizeof(record.configVersion));

  if (!storage.begin(kNamespace, false)) return false;
  const size_t written = storage.putBytes(kRecordKey, &record, sizeof(record));
  storage.end();
  return written == sizeof(record);
}

bool loadEdgeSettings(Preferences &storage, EdgeSettings &settings, String &version) {
  if (!storage.begin(kNamespace, true)) return false;
  const size_t stored = storage.getBytesLength(kRecordKey);
  if (stored != sizeof(PersistedEdgeConfig)) {
    storage.end();
    return false;
  }

  PersistedEdgeConfig record{};
  const size_t read = storage.getBytes(kRecordKey, &record, sizeof(record));
  storage.end();
  if (read != sizeof(record) || record.magic != kStorageMagic || record.storageVersion != kStorageVersion) {
    return false;
  }
  record.configVersion[kMaximumConfigVersionLength] = '\0';
  if (record.configVersion[0] == '\0') return false;

  settings = record.settings;
  version = record.configVersion;
  return true;
}
