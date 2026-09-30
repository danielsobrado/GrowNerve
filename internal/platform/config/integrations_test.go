package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestZigbee2MQTTDefaultsAndEnvironment(t *testing.T) {
	directory := t.TempDir()
	contents := []byte("env: development\nserver:\n  address: 127.0.0.1:8080\npostgres:\n  url_env: GROWNERVE_DATABASE_URL\nmqtt:\n  broker: tcp://127.0.0.1:1883\n")
	if err := os.WriteFile(filepath.Join(directory, "default.yaml"), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_INTEGRATIONS__ZIGBEE2MQTT__ENABLED", "true")
	t.Setenv("APP_INTEGRATIONS__ZIGBEE2MQTT__BASE_TOPIC", "z2m")
	loaded, err := Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	zigbee := loaded.Integrations.Zigbee2MQTT
	if !zigbee.Enabled || zigbee.BaseTopic != "z2m" || zigbee.ClientID == "" || zigbee.AvailabilityFallback != 90*time.Minute {
		t.Fatalf("zigbee2mqtt = %+v", zigbee)
	}
	if loaded.Integrations.LivenessInterval != 30*time.Second {
		t.Fatalf("liveness interval = %v", loaded.Integrations.LivenessInterval)
	}

	t.Setenv("APP_INTEGRATIONS__ZIGBEE2MQTT__ENABLED", "maybe")
	if _, err := Load(directory); err == nil {
		t.Fatal("a non-boolean enable flag was accepted")
	}
}

func TestZigbee2MQTTValidation(t *testing.T) {
	base := func() Config {
		return Config{
			Env: "development", Server: Server{Address: ":8080"}, Postgres: Postgres{URLEnv: "DATABASE_URL"},
			MQTT:    MQTT{Broker: "tcp://mqtt:1883"},
			Runtime: Runtime{DeviceOfflineAfter: 2 * time.Minute},
			Integrations: Integrations{LivenessInterval: 30 * time.Second, Zigbee2MQTT: Zigbee2MQTT{
				Enabled: true, BaseTopic: "zigbee2mqtt",
			}},
		}
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("valid configuration: %v", err)
	}
	cases := map[string]func(*Config){
		"liveness":   func(config *Config) { config.Integrations.LivenessInterval = 90 * time.Second },
		"wildcard":   func(config *Config) { config.Integrations.Zigbee2MQTT.BaseTopic = "zigbee2mqtt/#" },
		"slash":      func(config *Config) { config.Integrations.Zigbee2MQTT.BaseTopic = "zigbee2mqtt/" },
		"broker":     func(config *Config) { config.Integrations.Zigbee2MQTT.Broker = "http://broker" },
		"production": func(config *Config) { config.Integrations.Zigbee2MQTT.Broker = "tls://zigbee:8883"; *config = productionLike(*config) },
	}
	for name, mutate := range cases {
		config := base()
		mutate(&config)
		if err := config.Validate(); err == nil || (name == "production" && !strings.Contains(err.Error(), "zigbee2mqtt")) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
}

// productionLike upgrades a development configuration to one that passes every
// other production rule, so a failure isolates the integration check.
func productionLike(config Config) Config {
	config.Env = "production"
	config.Auth.Mode = ModeLocal
	config.Server.RateLimit.WritePerSecond = 1
	config.MQTT.UsernameEnv, config.MQTT.PasswordEnv = "MQTT_USER", "MQTT_PASSWORD"
	return config
}
