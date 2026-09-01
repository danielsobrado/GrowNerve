package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	if err := result.Validate(); err != nil {
		return Config{}, err
	}
	return result, nil
}

func applyEnvironment(config *Config) {
	if value := strings.TrimSpace(os.Getenv("APP_ENV")); value != "" {
		config.Env = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_SERVER__ADDRESS")); value != "" {
		config.Server.Address = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_MQTT__BROKER")); value != "" {
		config.MQTT.Broker = value
	}
}

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
	if config.Env == "production" {
		for _, origin := range config.Server.CORSAllowedOrigins {
			if origin == "*" {
				return errors.New("production cannot use wildcard CORS")
			}
		}
	}
	return nil
}
