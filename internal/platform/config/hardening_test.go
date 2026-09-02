package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeMinimalConfig(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	contents := []byte("env: development\nserver:\n  address: 127.0.0.1:8080\npostgres:\n  url_env: GROWNERVE_DATABASE_URL\nmqtt:\n  broker: tcp://127.0.0.1:1883\n")
	if err := os.WriteFile(filepath.Join(directory, "default.yaml"), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestLoadRejectsMalformedEnvironmentOverrides(t *testing.T) {
	for name, testCase := range map[string]struct {
		variable string
		value    string
	}{
		"retention":   {variable: "APP_TELEMETRY__RETENTION", value: "not-a-duration"},
		"media bytes": {variable: "APP_MEDIA__MAXIMUM_BYTES", value: "many"},
	} {
		t.Run(name, func(t *testing.T) {
			directory := writeMinimalConfig(t)
			t.Setenv(testCase.variable, testCase.value)
			if _, err := Load(directory); err == nil {
				t.Fatal("malformed environment override was silently ignored")
			}
		})
	}
}

func TestValidateRejectsExplicitlyDisabledRuntimeJobs(t *testing.T) {
	config := Config{
		Env:      "development",
		Server:   Server{Address: ":8080"},
		Postgres: Postgres{URLEnv: "DATABASE_URL"},
		MQTT:     MQTT{Broker: "tcp://mqtt:1883"},
	}
	for name, mutate := range map[string]func(*Config){
		"command sweep": func(config *Config) { config.Runtime.CommandSweepInterval = -time.Second },
		"alerting":      func(config *Config) { config.Runtime.AlertInterval = -time.Second },
		"config sync":   func(config *Config) { config.Runtime.ConfigSyncInterval = -time.Second },
		"offline check": func(config *Config) { config.Runtime.DeviceOfflineAfter = -time.Second },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := config
			mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("negative runtime interval was accepted")
			}
		})
	}
}

func TestProductionOIDCIssuerRequiresHTTPS(t *testing.T) {
	config := productionConfig()
	config.Auth = Auth{Mode: ModeOIDC, OIDC: OIDC{Issuer: "http://id.example", Audience: "grownerve"}}
	if err := config.Validate(); err == nil {
		t.Fatal("production OIDC accepted a plaintext issuer")
	}
}

func TestLoadFillsTelemetryAndRuntimeDefaults(t *testing.T) {
	config, err := Load(writeMinimalConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if config.Telemetry.BatchSize <= 0 || config.Telemetry.FlushInterval <= 0 {
		t.Fatalf("telemetry defaults were not applied: %+v", config.Telemetry)
	}
	if config.Runtime.CommandSweepInterval <= 0 || config.Runtime.ConfigSyncInterval <= 0 || config.Runtime.DeviceOfflineAfter <= 0 {
		t.Fatalf("runtime defaults were not applied: %+v", config.Runtime)
	}
}
