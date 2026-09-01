// Copy to secrets.h and fill in. secrets.h is git-ignored; never commit a real
// credential, and provision one broker identity per controller so a compromised
// device cannot publish telemetry attributed to another.
#pragma once

#define WIFI_SSID "your-network"
#define WIFI_PASSWORD "your-password"

// Broker. Use the TLS port in any real deployment.
#define MQTT_HOST "192.168.1.10"
#define MQTT_PORT 8883
#define MQTT_USERNAME "device-00000000-0000-0000-0000-000000000000"
#define MQTT_PASSWORD "per-device-password"

// This controller's identity. It must match the device row on the server and
// the broker ACL entry.
#define DEVICE_ID "00000000-0000-0000-0000-000000000000"

// Channel identifiers, from the server's channel registry.
#define CHANNEL_AIR_TEMPERATURE "00000000-0000-0000-0000-000000000031"
#define CHANNEL_HUMIDITY        "00000000-0000-0000-0000-000000000032"
#define CHANNEL_WATER_TEMPERATURE "00000000-0000-0000-0000-000000000033"
#define CHANNEL_WATER_LEVEL     "00000000-0000-0000-0000-000000000034"
#define CHANNEL_LIGHT           "00000000-0000-0000-0000-000000000041"
#define CHANNEL_FAN             "00000000-0000-0000-0000-000000000042"
#define CHANNEL_AIR_PUMP        "00000000-0000-0000-0000-000000000043"

// The CA that signed the broker certificate, in PEM form.
static const char MQTT_CA_CERTIFICATE[] = R"(-----BEGIN CERTIFICATE-----
replace with your broker CA certificate
-----END CERTIFICATE-----)";
