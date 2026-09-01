// Persisted controller configuration.
#pragma once

#include <Arduino.h>
#include <ArduinoJson.h>
#include <Preferences.h>

struct Photoperiod {
  bool configured = false;
  int onHour = 0;
  int onMinute = 0;
  int offHour = 0;
  int offMinute = 0;
  char channelId[40] = {0};
};

struct FanSchedule {
  bool configured = false;
  int onHour = 0;
  int onMinute = 0;
  int offHour = 0;
  int offMinute = 0;
  float activePercent = 0;
  float inactivePercent = 0;
  char channelId[40] = {0};
};

struct EdgeSettings {
  char timezonePosix[64] = "UTC0";
  Photoperiod photoperiod;
  FanSchedule fanSchedule;
  bool hasFanMinimum = false;
  float fanMinimumPercent = 0;
  bool airPumpAlwaysOn = true;
  uint32_t telemetryIntervalSeconds = 10;
  uint32_t commandTimeoutSeconds = 300;

  float safeLight = 0;
  float safeFan = 30;
  float safeAirPump = 100;
};

bool parseEdgeSettings(const JsonObjectConst &config, EdgeSettings &out, String &error);
void saveEdgeSettings(Preferences &storage, const EdgeSettings &settings, const String &version);
bool loadEdgeSettings(Preferences &storage, EdgeSettings &settings, String &version);
