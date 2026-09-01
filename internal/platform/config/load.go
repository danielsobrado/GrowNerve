package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func Load(directory string) (Config, error) {
	var result Config
	contents, err := os.ReadFile(filepath.Join(directory, "default.yaml"))
	if err != nil {
		return result, fmt.Errorf("read default configuration: %w", err)
	}
	if err := yaml.Unmarshal(contents, &result); err != nil {
		return result, fmt.Errorf("parse default configuration: %w", err)
	}
	applyEnvironment(&result)
	applyDefaults(&result)
	if err := result.Validate(); err != nil {
		return Config{}, err
	}
	return result, nil
}

func applyEnvironment(config *Config) {
	text := func(name string, target *string) {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			*target = value
		}
	}
	text("APP_ENV", &config.Env)
	text("APP_SERVER__ADDRESS", &config.Server.Address)
	text("APP_MQTT__BROKER", &config.MQTT.Broker)
	text("APP_AUTH__MODE", &config.Auth.Mode)
	text("APP_AUTH__OIDC__ISSUER", &config.Auth.OIDC.Issuer)
	text("APP_AUTH__OIDC__AUDIENCE", &config.Auth.OIDC.Audience)
	if value := strings.TrimSpace(os.Getenv("APP_SERVER__CORS_ALLOWED_ORIGINS")); value != "" {
		config.Server.CORSAllowedOrigins = splitList(value)
	}
	if value := strings.TrimSpace(os.Getenv("APP_TELEMETRY__RETENTION")); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			config.Telemetry.Retention = parsed
		}
	}
	if value := strings.TrimSpace(os.Getenv("APP_MEDIA__PATH")); value != "" {
		config.Media.Path = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_MEDIA__MAXIMUM_BYTES")); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			config.Media.MaximumBytes = parsed
		}
	}
}

func splitList(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

// applyDefaults fills values that have a safe answer, so a minimal configuration
// file still produces a complete, running system.
func applyDefaults(config *Config) {
	if config.Auth.Mode == "" {
		config.Auth.Mode = ModeDev
	}
	if config.Auth.LocalAccountsEnv == "" {
		config.Auth.LocalAccountsEnv = "GROWNERVE_LOCAL_ACCOUNTS"
	}
	if config.Auth.OIDC.RoleClaim == "" {
		config.Auth.OIDC.RoleClaim = "grownerve_role"
	}
	if config.Server.ShutdownTimeout == 0 {
		config.Server.ShutdownTimeout = 10 * time.Second
	}
	if config.Media.MaximumBytes == 0 {
		config.Media.MaximumBytes = 16 << 20
	}
	if config.Media.Path == "" {
		config.Media.Path = "./data/media"
	}
	defaults := map[*time.Duration]time.Duration{
		&config.Runtime.CommandSweepInterval: 5 * time.Second,
		&config.Runtime.AlertInterval:        15 * time.Second,
		&config.Runtime.RetentionInterval:    time.Hour,
		&config.Runtime.ConfigSyncInterval:   30 * time.Second,
		&config.Runtime.DeviceOfflineAfter:   2 * time.Minute,
	}
	for target, value := range defaults {
		if *target == 0 {
			*target = value
		}
	}
}

// Authentication modes.
const (
	ModeDev   = "dev"
	ModeLocal = "local"
	ModeOIDC  = "oidc"
)

func (config Config) Validate() error {
	if config.Env != "development" && config.Env != "test" && config.Env != "production" {
		return errors.New("env must be development, test, or production")
	}
	if strings.TrimSpace(config.Server.Address) == "" {
		return errors.New("server.address is required")
	}
	if strings.TrimSpace(config.Postgres.URLEnv) == "" {
		return errors.New("postgres.url_env is required")
	}
	if strings.TrimSpace(config.MQTT.Broker) == "" {
		return errors.New("mqtt.broker is required")
	}
	switch config.Auth.Mode {
	// An unset mode defaults to dev during loading. Production refuses both the
	// unset and the explicit dev value below, so the default can never make a
	// real deployment unauthenticated.
	case "", ModeDev, ModeLocal, ModeOIDC:
	default:
		return fmt.Errorf("auth.mode must be %s, %s, or %s", ModeDev, ModeLocal, ModeOIDC)
	}
	if config.Auth.Mode == ModeOIDC {
		if strings.TrimSpace(config.Auth.OIDC.Issuer) == "" || strings.TrimSpace(config.Auth.OIDC.Audience) == "" {
			return errors.New("auth.oidc requires both an issuer and an audience")
		}
	}
	if config.Telemetry.Retention < 0 {
		return errors.New("telemetry.retention cannot be negative")
	}
	return config.validateProduction()
}

// validateProduction refuses the shortcuts that are convenient in development
// and dangerous in a real deployment. A production process must not be able to
// start with authentication disabled or with an origin wildcard.
func (config Config) validateProduction() error {
	if config.Env != "production" {
		return nil
	}
	if config.Auth.Mode == "" || config.Auth.Mode == ModeDev {
		return errors.New("production cannot run with auth.mode: dev; configure local or oidc authentication")
	}
	for _, origin := range config.Server.CORSAllowedOrigins {
		if origin == "*" {
			return errors.New("production cannot use wildcard CORS")
		}
	}
	if config.Server.RateLimit.WritePerSecond <= 0 {
		return errors.New("production requires server.rate_limit.write_per_second above zero")
	}
	if strings.TrimSpace(config.MQTT.UsernameEnv) == "" || strings.TrimSpace(config.MQTT.PasswordEnv) == "" {
		return errors.New("production requires mqtt.username_env and mqtt.password_env; anonymous broker access is not permitted")
	}
	return nil
}
