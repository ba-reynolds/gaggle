package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/ba-reynolds/gaggle/internal/seedgen"
	"github.com/ba-reynolds/gaggle/internal/store"
	"github.com/ba-reynolds/gaggle/pkg/config"
	"github.com/brianvoe/gofakeit/v7"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(log)

	db, err := store.NewPostgresStorage(cfg.DBConfig)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to database: %v", err))
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		panic(fmt.Sprintf("failed to ping database: %v", err))
	}

	st := store.NewStore(db, log, cfg.AppConfig.MediaDir)
	ctx := context.Background()

	// Posts per tick: configurable via SIMULATE_POSTS (default 10). Keep the
	// per-run activity modest so a frequent cron grows the site without
	// becoming spammy.
	postsPerTick := 10
	if raw := os.Getenv("SIMULATE_POSTS"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			postsPerTick = n
		}
	}

	if err := seedgen.Tick(ctx, st, log, gofakeit.New(uint64(time.Now().UnixNano())), time.Now().UTC(), postsPerTick); err != nil {
		panic(fmt.Sprintf("simulate tick failed: %v", err))
	}

	fmt.Println("✅ Simulation tick complete")
}