// Persisted controller configuration.
//
// The configuration is written to NVS the moment it is accepted, so a reboot
// during a network outage recovers the schedules from flash rather than waiting
// for a server that may not come back.
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

struct EdgeSettings {
  Photoperiod photoperiod;
  bool hasFanMinimum = false;
  float fanMinimumPercent = 0;
  bool airPumpAlwaysOn = true;
  uint32_t telemetryIntervalSeconds = 10;
  uint32_t commandTimeoutSeconds = 300;

  float safeLight = 0;
  float safeFan = 30;
  // Aeration defaults to running. A controller that has never been configured
  // must still keep a deep-water crop alive.
  float safeAirPump = 100;
};

// Applies a decoded configuration document, rejecting anything malformed so the
// controller keeps running the last configuration it accepted.
bool parseEdgeSettings(const JsonObjectConst &config, EdgeSettings &out, String &error);

// Persists and restores the accepted configuration and its version.
void saveEdgeSettings(Preferences &storage, const EdgeSettings &settings, const String &version);
bool loadEdgeSettings(Preferences &storage, EdgeSettings &settings, String &version);
