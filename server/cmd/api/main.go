package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/ba-reynolds/gaggle/docs" // Imports Swagger docs
	"github.com/ba-reynolds/gaggle/internal/api"
	"github.com/ba-reynolds/gaggle/internal/auth"
	"github.com/ba-reynolds/gaggle/internal/cache"
	"github.com/ba-reynolds/gaggle/internal/metrics"
	"github.com/ba-reynolds/gaggle/internal/service"
	"github.com/ba-reynolds/gaggle/internal/store"
	"github.com/ba-reynolds/gaggle/pkg/config"
	"github.com/ba-reynolds/gaggle/pkg/logger"
)

// @title         Gaggle API
// @version       1.0
// @description   API for Gaggle, a social microblogging app
// @host          localhost:2021
// @BasePath      /api/v1
// @schemes       http
// @securityDefinitions.apikey ApiKeyAuth
// @in            header
// @name          Authorization
func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.NewLogger(cfg.LoggingConfig.Filename)
	if err != nil {
		slog.Error("failed to initialize logger", "error", err)
		os.Exit(1)
	}
	slog.SetDefault(log) // Set the default logger for convenience

	// Initialize storage
	db, err := store.NewPostgresStorage(cfg.DBConfig)
	if err != nil {
		log.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize Redis cache (optional: the app degrades gracefully if absent)
	rdb := cache.New(cfg.RedisConfig.Address, cfg.RedisConfig.Password, cfg.RedisConfig.DB, cfg.RedisConfig.FeedCacheTTL, log)
	if !rdb.Ping(context.Background()) {
		rdb = nil
	}
	defer func() {
		if rdb != nil {
			rdb.Close()
		}
	}()

	authenticator := auth.NewJWTAuthenticator(cfg.AuthConfig)
	store := store.NewStore(db, log, cfg.AppConfig.MediaDir)
	svc := service.NewService(store, log, authenticator, cfg.AppConfig)

	// Background metrics sampling: host stats recorded 24/7 so the admin
	// dashboard has history (not just live values), plus hourly pruning of old
	// page_views/host_metrics_samples rows. It stops with the process.
	samplerCtx, stopSampler := context.WithCancel(context.Background())
	defer stopSampler()
	sampler := metrics.NewSampler(svc.Metrics, log, cfg.MetricsConfig.HostSampleInterval, time.Duration(cfg.MetricsConfig.RetentionDays)*24*time.Hour)
	go sampler.Run(samplerCtx)

	router := api.NewRouter(svc, log, rdb, cfg.RedisConfig.RateLimitMaxRequests, cfg.RedisConfig.RateLimitWindow, cfg.AuthConfig.CookieSecure)

	server := &http.Server{
		Addr:         cfg.ServerConfig.ServerAddr,
		Handler:      router,
		ReadTimeout:  cfg.ServerConfig.ServerReadTimeout,
		WriteTimeout: cfg.ServerConfig.ServerWriteTimeout,
		IdleTimeout:  cfg.ServerConfig.ServerIdleTimeout,
	}

	log.Info("Starting server", "address", cfg.ServerConfig.ServerAddr)
	if err := server.ListenAndServe(); err != nil {
		log.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
