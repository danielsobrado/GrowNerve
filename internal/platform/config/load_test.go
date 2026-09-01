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
