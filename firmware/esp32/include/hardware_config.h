#pragma once

#include <stdint.h>

// Reference pin map. Change this file for a different controller PCB rather than
// scattering hardware assumptions through the control logic.
constexpr uint8_t GN_LIGHT_RELAY_PIN = 26;
constexpr uint8_t GN_FAN_PWM_PIN = 27;
constexpr uint8_t GN_AIR_PUMP_RELAY_PIN = 25;
constexpr uint8_t GN_EMERGENCY_INPUT_PIN = 33;

// Set these to match the actual relay board. A wrong polarity can turn a load on
// when software asks for its safe OFF state.
constexpr bool GN_LIGHT_RELAY_ACTIVE_HIGH = true;
constexpr bool GN_AIR_PUMP_RELAY_ACTIVE_HIGH = true;

// Reference emergency wiring is fail-safe: a normally-closed contact connects
// GPIO33 to GND while healthy. INPUT_PULLUP therefore reads HIGH when the button
// is pressed, the contact opens, or the wire breaks.
constexpr bool GN_EMERGENCY_ACTIVE_HIGH = true;
