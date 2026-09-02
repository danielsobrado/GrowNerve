package config

import "time"

type Config struct {
	Env       string    `yaml:"env"`
	Server    Server    `yaml:"server"`
	Auth      Auth      `yaml:"auth"`
	Postgres  Postgres  `yaml:"postgres"`
	MQTT      MQTT      `yaml:"mqtt"`
	Telemetry Telemetry `yaml:"telemetry"`
	Runtime   Runtime   `yaml:"runtime"`
	Media     Media     `yaml:"media"`
}

type Server struct {
	Address            string        `yaml:"address"`
	ShutdownTimeout    time.Duration `yaml:"shutdown_timeout"`
	MaxHeaderBytes     int           `yaml:"max_header_bytes"`
	CORSAllowedOrigins []string      `yaml:"cors_allowed_origins"`
	TrustedProxyCIDRs  []string      `yaml:"trusted_proxy_cidrs"`
	RateLimit          RateLimit     `yaml:"rate_limit"`
}

type RateLimit struct {
	ReadPerSecond  float64 `yaml:"read_per_second"`
	ReadBurst      float64 `yaml:"read_burst"`
	WritePerSecond float64 `yaml:"write_per_second"`
	WriteBurst     float64 `yaml:"write_burst"`
}

type Auth struct {
	Mode             string `yaml:"mode"`
	LocalAccountsEnv string `yaml:"local_accounts_env"`
	OIDC             OIDC   `yaml:"oidc"`
}

type OIDC struct {
	Issuer      string            `yaml:"issuer"`
	Audience    string            `yaml:"audience"`
	RoleClaim   string            `yaml:"role_claim"`
	RoleMapping map[string]string `yaml:"role_mapping"`
	DefaultRole string            `yaml:"default_role"`
}

type Postgres struct {
	URLEnv string `yaml:"url_env"`
}

type MQTT struct {
	Broker      string `yaml:"broker"`
	ClientID    string `yaml:"client_id"`
	UsernameEnv string `yaml:"username_env"`
	PasswordEnv string `yaml:"password_env"`
}

type Telemetry struct {
	BatchSize     int           `yaml:"batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval"`
	Retention     time.Duration `yaml:"retention"`
}

type Runtime struct {
	CommandSweepInterval time.Duration `yaml:"command_sweep_interval"`
	AlertInterval        time.Duration `yaml:"alert_interval"`
	RetentionInterval    time.Duration `yaml:"retention_interval"`
	ConfigSyncInterval   time.Duration `yaml:"config_sync_interval"`
	DeviceOfflineAfter   time.Duration `yaml:"device_offline_after"`
}

type Media struct {
	Provider     string `yaml:"provider"`
	Path         string `yaml:"path"`
	MaximumBytes int64  `yaml:"maximum_bytes"`
}
