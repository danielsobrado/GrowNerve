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

// productionConfig is a minimally valid production configuration; each test
// below breaks exactly one rule so the failure is unambiguous.
func productionConfig() Config {
	return Config{
		Env:      "production",
		Server:   Server{Address: ":8080", CORSAllowedOrigins: []string{"https://farm.example"}, RateLimit: RateLimit{WritePerSecond: 5}},
		Auth:     Auth{Mode: ModeLocal, LocalAccountsEnv: "GROWNERVE_LOCAL_ACCOUNTS"},
		Postgres: Postgres{URLEnv: "GROWNERVE_DATABASE_URL"},
		MQTT:     MQTT{Broker: "tls://mqtt:8883", UsernameEnv: "MQTT_USER", PasswordEnv: "MQTT_PASSWORD"},
	}
}

func TestProductionRefusesUnsafeDefaults(t *testing.T) {
	if err := productionConfig().Validate(); err != nil {
		t.Fatalf("valid production configuration rejected: %v", err)
	}

	tests := map[string]func(*Config){
		"development authentication": func(config *Config) { config.Auth.Mode = ModeDev },
		"unset authentication":       func(config *Config) { config.Auth.Mode = "" },
		"wildcard CORS":              func(config *Config) { config.Server.CORSAllowedOrigins = []string{"*"} },
		"no write rate limit":        func(config *Config) { config.Server.RateLimit.WritePerSecond = 0 },
		"anonymous broker":           func(config *Config) { config.MQTT.UsernameEnv = "" },
		"no broker password":         func(config *Config) { config.MQTT.PasswordEnv = "" },
	}
	for name, breakRule := range tests {
		t.Run(name, func(t *testing.T) {
			config := productionConfig()
			breakRule(&config)
			if err := config.Validate(); err == nil {
				t.Fatal("production started with an unsafe configuration")
			}
		})
	}
}

func TestOIDCRequiresIssuerAndAudience(t *testing.T) {
	config := productionConfig()
	config.Auth = Auth{Mode: ModeOIDC}
	if err := config.Validate(); err == nil {
		t.Fatal("oidc mode accepted without an issuer or audience")
	}
	config.Auth.OIDC = OIDC{Issuer: "https://id.example", Audience: "grownerve"}
	if err := config.Validate(); err != nil {
		t.Fatalf("complete oidc configuration rejected: %v", err)
	}
}

func TestDevelopmentDefaultsAreFilledOnLoad(t *testing.T) {
	directory := t.TempDir()
	contents := []byte("env: development\nserver:\n  address: 127.0.0.1:8080\npostgres:\n  url_env: GROWNERVE_DATABASE_URL\nmqtt:\n  broker: tcp://127.0.0.1:1883\n")
	if err := os.WriteFile(filepath.Join(directory, "default.yaml"), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	if config.Auth.Mode != ModeDev {
		t.Fatalf("auth mode = %q, want the dev default", config.Auth.Mode)
	}
	if config.Runtime.AlertInterval == 0 || config.Runtime.DeviceOfflineAfter == 0 {
		t.Fatalf("runtime intervals were left at zero: %+v", config.Runtime)
	}
	if config.Media.MaximumBytes == 0 {
		t.Fatal("media upload limit was left unbounded")
	}
}

// TestShippedConfigurationFilesLoad is the test whose absence let a
// `retention: 0` slip into the default file: it parsed as an integer and the
// server refused to start. Every configuration file in the repository is loaded
// here, so a file that cannot be read fails the build rather than the process.
func TestShippedConfigurationFilesLoad(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..", "..")
	for _, name := range []string{"default.yaml", "production.example.yaml"} {
		t.Run(name, func(t *testing.T) {
			source := filepath.Join(repositoryRoot, "config", name)
			contents, err := os.ReadFile(source)
			if err != nil {
				t.Fatalf("read %s: %v", source, err)
			}
			// Load only reads default.yaml, so each file is staged under that
			// name to exercise the real loading path.
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "default.yaml"), contents, 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := Load(directory)
			if err != nil {
				t.Fatalf("%s does not load: %v", name, err)
			}
			if config.Server.Address == "" || config.Postgres.URLEnv == "" || config.MQTT.Broker == "" {
				t.Fatalf("%s loaded but is incomplete: %+v", name, config)
			}
		})
	}
}

// TestProductionExampleIsActuallyValidForProduction guards the example against
// drifting out of step with the rules it is meant to demonstrate.
func TestProductionExampleIsActuallyValidForProduction(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "config", "production.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "default.yaml"), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := Load(directory)
	if err != nil {
		t.Fatalf("the production example would not start a production server: %v", err)
	}
	if config.Env != "production" {
		t.Fatalf("the production example is not marked production: %q", config.Env)
	}
	if config.Auth.Mode == ModeDev {
		t.Fatal("the production example uses development authentication")
	}
}
