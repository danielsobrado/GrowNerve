package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	directory := t.TempDir()
	contents := []byte("env: development\nserver:\n  address: 127.0.0.1:8080\npostgres:\n  url_env: GROWNERVE_DATABASE_URL\nmqtt:\n  broker: tcp://127.0.0.1:1883\n")
	if err := os.WriteFile(filepath.Join(directory, "default.yaml"), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_SERVER__ADDRESS", "127.0.0.1:9090")
	config, err := Load(directory)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.Server.Address != "127.0.0.1:9090" {
		t.Fatalf("Address = %q", config.Server.Address)
	}
}

func TestProductionRejectsWildcardCORS(t *testing.T) {
	config := Config{Env: "production", Server: Server{Address: ":8080", CORSAllowedOrigins: []string{"*"}}, Postgres: Postgres{URLEnv: "GROWNERVE_DATABASE_URL"}, MQTT: MQTT{Broker: "tcp://mqtt:1883"}}
	if err := config.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want production CORS error")
	}
}

func TestValidateRequiredConfiguration(t *testing.T) {
	valid := Config{Env: "development", Server: Server{Address: ":8080"}, Postgres: Postgres{URLEnv: "DATABASE_URL"}, MQTT: MQTT{Broker: "tcp://mqtt:1883"}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid configuration: %v", err)
	}
	tests := []Config{
		{Env: "unknown", Server: valid.Server, Postgres: valid.Postgres, MQTT: valid.MQTT},
		{Env: "development", Postgres: valid.Postgres, MQTT: valid.MQTT},
		{Env: "development", Server: valid.Server, MQTT: valid.MQTT},
		{Env: "development", Server: valid.Server, Postgres: valid.Postgres},
	}
	for _, config := range tests {
		if err := config.Validate(); err == nil {
			t.Fatalf("Validate(%+v) succeeded", config)
		}
	}
}

func TestLoadReportsReadAndParseErrors(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("missing file succeeded")
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "default.yaml"), []byte("env: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(directory); err == nil {
		t.Fatal("invalid YAML succeeded")
	}
}
