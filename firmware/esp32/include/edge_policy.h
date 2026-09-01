// Output precedence for the GrowNerve controller.
//
// This is a direct port of internal/edge in the server repository. The two are
// kept in step deliberately: the Go copy is what CI exercises, and this copy is
// what actually drives the relays. If you change one, change both.
#pragma once

#include <Arduino.h>

enum class OutputSource : uint8_t {
  HardwareInterlock,
  LocalSafety,
  Emergency,
  Override,
  Automation,
  EssentialSchedule,
  DefaultSafe,
};

struct OutputPolicy {
  bool hasHardwareInterlock = false;
  float hardwareInterlock = 0;

  bool hasLocalSafetyLimit = false;
  float localSafetyLimit = 0;

  bool emergencyLatched = false;
  float emergencyValue = 0;

  bool hasOverride = false;
  float overrideValue = 0;
  uint32_t overrideExpiresAtMillis = 0;

  bool hasAutomation = false;
  float automationValue = 0;

  float essentialScheduleValue = 0;
  float defaultSafeValue = 0;
};

struct OutputResolution {
  float value;
  OutputSource source;
};

// Resolves one output. The order is fixed and must not be reordered for
// convenience: a hardware interlock outranks everything, and the safe default is
// the floor rather than zero, because for aeration "off" is the dangerous state.
inline OutputResolution resolveOutput(const OutputPolicy &policy, uint32_t nowMillis) {
  if (policy.hasHardwareInterlock) {
    return {policy.hardwareInterlock, OutputSource::HardwareInterlock};
  }
  if (policy.hasLocalSafetyLimit) {
    return {policy.localSafetyLimit, OutputSource::LocalSafety};
  }
  if (policy.emergencyLatched) {
    return {policy.emergencyValue, OutputSource::Emergency};
  }
  // Comparison is written as a subtraction so it stays correct when millis()
  // wraps, which it does roughly every 49 days of uptime.
  if (policy.hasOverride && (int32_t)(policy.overrideExpiresAtMillis - nowMillis) > 0) {
    return {policy.overrideValue, OutputSource::Override};
  }
  if (policy.hasAutomation) {
    return {policy.automationValue, OutputSource::Automation};
  }
  if (policy.essentialScheduleValue != policy.defaultSafeValue) {
    return {policy.essentialScheduleValue, OutputSource::EssentialSchedule};
  }
  return {policy.defaultSafeValue, OutputSource::DefaultSafe};
}

// A daily window on the controller's local clock, handling a schedule that
// crosses midnight.
inline bool withinPhotoperiod(int onHour, int onMinute, int offHour, int offMinute, int hour, int minute) {
  const int minutesNow = hour * 60 + minute;
  const int on = onHour * 60 + onMinute;
  const int off = offHour * 60 + offMinute;
  if (on == off) {
    return false;
  }
  if (on < off) {
    return minutesNow >= on && minutesNow < off;
  }
  return minutesNow >= on || minutesNow < off;
}
