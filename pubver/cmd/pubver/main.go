package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pubver/internal/config"
	"pubver/internal/httpapi"
	"pubver/internal/repository"
	"pubver/internal/repository/postgres"
	"pubver/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	repo, cleanup, err := newVerificationRepository(ctx, cfg)
	if err != nil {
		logger.Error("build repository", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	verificationService := service.NewVerificationService(repo, logger, cfg.JWTEncKey)

	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(
			logger,
			cfg.RequestTimeout,
			httpapi.RateLimitConfig{
				Enabled:         cfg.RateLimit.Enabled,
				RequestsPerSec:  cfg.RateLimit.RequestsPerSec,
				Burst:           cfg.RateLimit.Burst,
				VisitorTTL:      cfg.RateLimit.VisitorTTL,
				CleanupInterval: cfg.RateLimit.CleanupInterval,
			},
			verificationService,
		),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("public verification api started", "addr", cfg.HTTPAddr)

	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			logger.Error("http server crashed", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown http server", "error", err)
		os.Exit(1)
	}

	logger.Info("public verification api stopped")
}

func newVerificationRepository(ctx context.Context, cfg config.Config) (repository.VerificationRepository, func(), error) {
	pool, err := newDBPool(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}

	return postgres.NewVerificationRepository(pool), pool.Close, nil
}

func newLogger(level string) *slog.Logger {
	var logLevel slog.Level

	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
}

func newDBPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	poolConfig.MaxConns = cfg.DBMaxConns

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
