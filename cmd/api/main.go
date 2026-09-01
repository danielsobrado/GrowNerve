package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/farm"
	"github.com/jdanielsobrado/grownerve/internal/media"
	"github.com/jdanielsobrado/grownerve/internal/platform/audit"
	"github.com/jdanielsobrado/grownerve/internal/platform/auth"
	"github.com/jdanielsobrado/grownerve/internal/platform/config"
	"github.com/jdanielsobrado/grownerve/internal/platform/database"
	"github.com/jdanielsobrado/grownerve/internal/platform/httpx"
	platformmiddleware "github.com/jdanielsobrado/grownerve/internal/platform/middleware"
	mqttbridge "github.com/jdanielsobrado/grownerve/internal/platform/mqtt"
	"github.com/jdanielsobrado/grownerve/internal/platform/outbox"
	"github.com/jdanielsobrado/grownerve/internal/registry"
	"github.com/jdanielsobrado/grownerve/internal/runtime"
	"github.com/jdanielsobrado/grownerve/internal/telemetry"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	directory := os.Getenv("CONFIG_DIR")
	if directory == "" {
		directory = filepath.Clean("config")
	}
	cfg, err := config.Load(directory)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if err := validateRuntimeSecrets(cfg); err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	authenticator, err := buildAuthenticator(cfg)
	if err != nil {
		return fmt.Errorf("configure authentication: %w", err)
	}
	logger.Info("authentication_configured", "mode", authenticator.Mode(), "env", cfg.Env)

	pool, err := database.Open(context.Background(), os.Getenv(cfg.Postgres.URLEnv))
	if err != nil {
		return err
	}
	defer pool.Close()

	runtimeContext, cancelRuntime := context.WithCancel(context.Background())
	defer cancelRuntime()

	stateStore := farm.NewPostgresStore(pool)
	stateCommitter := farm.NewPostgresStateCommitter(pool)
	samples := telemetry.NewPostgresStore(pool)
	queue := outbox.NewPostgresStore(pool)
	recorder := audit.NewRecorder(pool, logger)
	recorder.Start(runtimeContext)

	mediaStore, err := media.NewFilesystemStore(cfg.Media.Path, cfg.Media.MaximumBytes)
	if err != nil {
		return fmt.Errorf("configure media storage: %w", err)
	}

	events := httpx.NewEventBroker()
	bridge := mqttbridge.NewBridge(cfg.MQTT.Broker, cfg.MQTT.ClientID, stateStore, logger,
		mqttbridge.WithTelemetryStore(samples), mqttbridge.WithNotifier(events),
		mqttbridge.WithCredentials(os.Getenv(cfg.MQTT.UsernameEnv), os.Getenv(cfg.MQTT.PasswordEnv)))
	bridge.Start(runtimeContext)

	publisher := farm.NewDurablePublisher(bridge, queue, logger)
	supervisor := runtime.New(stateStore, samples, logger, runtimeConfig(cfg),
		runtime.WithNotifier(events), runtime.WithConfigPublisher(bridge),
		runtime.WithAuditRecorder(recorder), runtime.WithOutbox(farm.NewOutboxWorker(queue, bridge, logger)))
	supervisor.Start(runtimeContext)

	health := httpx.NewHealthHandler(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			return err
		}
		if !bridge.Connected() {
			return errors.New("MQTT broker is unavailable")
		}
		return nil
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", health.Live)
	mux.HandleFunc("GET /health/ready", health.Ready)
	mux.HandleFunc("GET /version", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]string{"version": version})
	})
	mux.HandleFunc("GET /api/v1/stream", events.Stream)
	mux.Handle("/api/v1/", farm.NewHandler(stateStore,
		farm.WithStateCommitter(stateCommitter),
		farm.WithCommandPublisher(publisher), farm.WithTelemetry(samples), farm.WithMediaStore(mediaStore),
		farm.WithRegistry(registry.NewPostgresProjector(pool)),
		farm.WithNotifier(events), farm.WithAuthorizer(farm.RoleAuthorizer{}),
		farm.WithAuditRecorder(recorder), farm.WithLogger(logger)))

	authenticated := auth.Middleware(authenticator, []string{"/health", "/version"}, logger)(mux)
	handler := platformmiddleware.Chain(authenticated, platformmiddleware.Options{
		AllowedOrigins: cfg.Server.CORSAllowedOrigins, TrustedProxyCIDRs: cfg.Server.TrustedProxyCIDRs,
		Logger:     logger,
		ReadLimit:  platformmiddleware.RateLimit{Rate: cfg.Server.RateLimit.ReadPerSecond, Burst: cfg.Server.RateLimit.ReadBurst},
		WriteLimit: platformmiddleware.RateLimit{Rate: cfg.Server.RateLimit.WritePerSecond, Burst: cfg.Server.RateLimit.WriteBurst},
	})

	server := &http.Server{
		Addr: cfg.Server.Address, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 35 * time.Second,
		WriteTimeout: 0, IdleTimeout: 90 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("api_started", "address", cfg.Server.Address, "version", version)
		serverErrors <- server.ListenAndServe()
	}()

	signalContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case err = <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-signalContext.Done():
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownContext)
	cancelRuntime()
	recorder.Wait()
	return shutdownErr
}

func validateRuntimeSecrets(cfg config.Config) error {
	if cfg.Env != "production" {
		return nil
	}
	if strings.TrimSpace(os.Getenv(cfg.MQTT.UsernameEnv)) == "" || strings.TrimSpace(os.Getenv(cfg.MQTT.PasswordEnv)) == "" {
		return errors.New("production MQTT credential environment variables must contain non-empty values")
	}
	return nil
}

func runtimeConfig(cfg config.Config) runtime.Config {
	return runtime.Config{
		CommandSweepInterval: cfg.Runtime.CommandSweepInterval,
		AlertInterval:        cfg.Runtime.AlertInterval,
		RetentionInterval:    cfg.Runtime.RetentionInterval,
		ConfigSyncInterval:   cfg.Runtime.ConfigSyncInterval,
		OutboxInterval:       10 * time.Second,
		TelemetryRetention:   cfg.Telemetry.Retention,
		DeviceOfflineAfter:   cfg.Runtime.DeviceOfflineAfter,
	}
}

func buildAuthenticator(cfg config.Config) (auth.Authenticator, error) {
	switch cfg.Auth.Mode {
	case config.ModeLocal:
		return auth.NewLocalAuthenticator(os.Getenv(cfg.Auth.LocalAccountsEnv))
	case config.ModeOIDC:
		mapping := map[string]auth.Role{}
		for claim, role := range cfg.Auth.OIDC.RoleMapping {
			parsed, err := auth.ParseRole(role)
			if err != nil {
				return nil, fmt.Errorf("auth.oidc.role_mapping[%s]: %w", claim, err)
			}
			mapping[claim] = parsed
		}
		var fallback auth.Role
		if cfg.Auth.OIDC.DefaultRole != "" {
			parsed, err := auth.ParseRole(cfg.Auth.OIDC.DefaultRole)
			if err != nil {
				return nil, fmt.Errorf("auth.oidc.default_role: %w", err)
			}
			fallback = parsed
		}
		return auth.NewOIDCAuthenticator(auth.OIDCConfig{
			Issuer: cfg.Auth.OIDC.Issuer, Audience: cfg.Auth.OIDC.Audience,
			RoleClaim: cfg.Auth.OIDC.RoleClaim, RoleMapping: mapping, DefaultRole: fallback,
		}, nil)
	default:
		return auth.DevAuthenticator{}, nil
	}
}
