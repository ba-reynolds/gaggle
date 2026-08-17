package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	_ "github.com/ba-reynolds/gophersocial/docs" // Imports Swagger docs
	"github.com/ba-reynolds/gophersocial/internal/api"
	"github.com/ba-reynolds/gophersocial/internal/auth"
	"github.com/ba-reynolds/gophersocial/internal/cache"
	"github.com/ba-reynolds/gophersocial/internal/service"
	"github.com/ba-reynolds/gophersocial/internal/store"
	"github.com/ba-reynolds/gophersocial/pkg/config"
	"github.com/ba-reynolds/gophersocial/pkg/logger"
)

// @title         GopherSocial API
// @version       1.0
// @description   API for GopherSocial, a social network for gophers
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
