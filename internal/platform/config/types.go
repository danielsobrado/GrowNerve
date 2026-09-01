package config

import "time"

type Config struct {
	Env       string    `yaml:"env"`
	Server    Server    `yaml:"server"`
	Postgres  Postgres  `yaml:"postgres"`
	MQTT      MQTT      `yaml:"mqtt"`
	Telemetry Telemetry `yaml:"telemetry"`
	Media     Media     `yaml:"media"`
}

type Server struct {
	Address            string        `yaml:"address"`
	ShutdownTimeout    time.Duration `yaml:"shutdown_timeout"`
	CORSAllowedOrigins []string      `yaml:"cors_allowed_origins"`
}

type Postgres struct {
	URLEnv string `yaml:"url_env"`
}

type MQTT struct {
	Broker   string `yaml:"broker"`
	ClientID string `yaml:"client_id"`
}

type Telemetry struct {
	BatchSize     int           `yaml:"batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval"`
}

type Media struct {
	Provider string `yaml:"provider"`
	Path     string `yaml:"path"`
}
