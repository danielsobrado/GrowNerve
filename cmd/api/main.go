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
	"syscall"
	"time"

	"github.com/jdanielsobrado/grownerve/internal/farm"
	"github.com/jdanielsobrado/grownerve/internal/platform/config"
	"github.com/jdanielsobrado/grownerve/internal/platform/database"
	"github.com/jdanielsobrado/grownerve/internal/platform/httpx"
	platformmiddleware "github.com/jdanielsobrado/grownerve/internal/platform/middleware"
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	pool, err := database.Open(context.Background(), os.Getenv(cfg.Postgres.URLEnv))
	if err != nil {
		return err
	}
	defer pool.Close()
	health := httpx.NewHealthHandler(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pool.Ping(ctx)
	})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", health.Live)
	mux.HandleFunc("GET /health/ready", health.Ready)
	mux.HandleFunc("GET /version", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]string{"version": version})
	})
	mux.Handle("/api/v1/", farm.NewHandler(farm.NewPostgresStore(pool)))
	handler := platformmiddleware.Chain(mux, platformmiddleware.Options{AllowedOrigins: cfg.Server.CORSAllowedOrigins, Logger: logger})
	server := &http.Server{Addr: cfg.Server.Address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 35 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 90 * time.Second}
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
	return server.Shutdown(shutdownContext)
}
