package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
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
	if err := applyEnvironment(&result); err != nil {
		return Config{}, err
	}
	applyDefaults(&result)
	if err := result.Validate(); err != nil {
		return Config{}, err
	}
	return result, nil
}

func applyEnvironment(config *Config) error {
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
	if value := strings.TrimSpace(os.Getenv("APP_SERVER__TRUSTED_PROXY_CIDRS")); value != "" {
		config.Server.TrustedProxyCIDRs = splitList(value)
	}
	if value := strings.TrimSpace(os.Getenv("APP_TELEMETRY__RETENTION")); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("APP_TELEMETRY__RETENTION must be a duration: %w", err)
		}
		config.Telemetry.Retention = parsed
	}
	if value := strings.TrimSpace(os.Getenv("APP_MEDIA__PATH")); value != "" {
		config.Media.Path = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_MEDIA__MAXIMUM_BYTES")); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("APP_MEDIA__MAXIMUM_BYTES must be a whole number: %w", err)
		}
		config.Media.MaximumBytes = parsed
	}
	return nil
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
	if config.Telemetry.BatchSize == 0 {
		config.Telemetry.BatchSize = 200
	}
	if config.Telemetry.FlushInterval == 0 {
		config.Telemetry.FlushInterval = time.Second
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

const (
	ModeDev   = "dev"
	ModeLocal = "local"
	ModeOIDC  = "oidc"
)

func rejectNegativeDuration(name string, value time.Duration) error {
	if value < 0 {
		return fmt.Errorf("%s cannot be negative", name)
	}
	return nil
}

func validateOrigin(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return fmt.Errorf("invalid CORS origin %q", value)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("CORS origin %q must use http or https", value)
	}
	return nil
}

func (config Config) Validate() error {
	if config.Env != "development" && config.Env != "test" && config.Env != "production" {
		return errors.New("env must be development, test, or production")
	}
	if strings.TrimSpace(config.Server.Address) == "" {
		return errors.New("server.address is required")
	}
	if err := rejectNegativeDuration("server.shutdown_timeout", config.Server.ShutdownTimeout); err != nil {
		return err
	}
	if strings.TrimSpace(config.Postgres.URLEnv) == "" {
		return errors.New("postgres.url_env is required")
	}
	if strings.TrimSpace(config.MQTT.Broker) == "" {
		return errors.New("mqtt.broker is required")
	}
	for _, origin := range config.Server.CORSAllowedOrigins {
		if origin != "*" {
			if err := validateOrigin(origin); err != nil {
				return err
			}
		}
	}
	for _, cidr := range config.Server.TrustedProxyCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("server.trusted_proxy_cidrs contains invalid CIDR %q", cidr)
		}
	}
	for name, value := range map[string]float64{
		"server.rate_limit.read_per_second":  config.Server.RateLimit.ReadPerSecond,
		"server.rate_limit.read_burst":       config.Server.RateLimit.ReadBurst,
		"server.rate_limit.write_per_second": config.Server.RateLimit.WritePerSecond,
		"server.rate_limit.write_burst":      config.Server.RateLimit.WriteBurst,
	} {
		if value < 0 {
			return fmt.Errorf("%s cannot be negative", name)
		}
	}
	switch config.Auth.Mode {
	case "", ModeDev, ModeLocal, ModeOIDC:
	default:
		return fmt.Errorf("auth.mode must be %s, %s, or %s", ModeDev, ModeLocal, ModeOIDC)
	}
	if config.Auth.Mode == ModeOIDC {
		if strings.TrimSpace(config.Auth.OIDC.Issuer) == "" || strings.TrimSpace(config.Auth.OIDC.Audience) == "" {
			return errors.New("auth.oidc requires both an issuer and an audience")
		}
	}
	if config.Telemetry.BatchSize < 0 {
		return errors.New("telemetry.batch_size cannot be negative")
	}
	if err := rejectNegativeDuration("telemetry.flush_interval", config.Telemetry.FlushInterval); err != nil {
		return err
	}
	if config.Telemetry.Retention < 0 {
		return errors.New("telemetry.retention cannot be negative")
	}
	for name, value := range map[string]time.Duration{
		"runtime.command_sweep_interval": config.Runtime.CommandSweepInterval,
		"runtime.alert_interval":         config.Runtime.AlertInterval,
		"runtime.retention_interval":     config.Runtime.RetentionInterval,
		"runtime.config_sync_interval":   config.Runtime.ConfigSyncInterval,
		"runtime.device_offline_after":   config.Runtime.DeviceOfflineAfter,
	} {
		if err := rejectNegativeDuration(name, value); err != nil {
			return err
		}
	}
	if config.Media.MaximumBytes < 0 {
		return errors.New("media.maximum_bytes cannot be negative")
	}
	return config.validateProduction()
}

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
	if config.Auth.Mode == ModeOIDC {
		issuer, err := url.Parse(config.Auth.OIDC.Issuer)
		if err != nil || issuer.Scheme != "https" || issuer.Host == "" {
			return errors.New("production OIDC issuer must be an absolute HTTPS URL")
		}
	}
	return nil
}
