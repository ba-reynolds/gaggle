package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ba-reynolds/gaggle/internal/seedgen"
	"github.com/ba-reynolds/gaggle/internal/store"
	"github.com/ba-reynolds/gaggle/pkg/config"
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

	force := false
	for _, a := range os.Args {
		if a == "--force" || a == "-force" {
			force = true
		}
	}
	if os.Getenv("SEED_FORCE") == "1" {
		force = true
	}
	if !force {
		// Idempotency guard: if the seed anchor user already exists, the database
		// is already seeded. Exit quietly so `make seed` / container re-runs are safe.
		if _, err := st.Users.GetByEmail(ctx, "alice@example.com"); err == nil {
			fmt.Println("✅ Database already seeded (alice@example.com exists); skipping. Use --force or SEED_FORCE=1 to re-seed.")
			return
		}
	} else {
		fmt.Println("🔄 Force re-seed: truncating existing data...")
		if _, err := st.DB.ExecContext(ctx, `TRUNCATE users CASCADE`); err != nil {
			fmt.Printf("warn: truncate users: %v\n", err)
		}
		if _, err := st.DB.ExecContext(ctx, `TRUNCATE badges CASCADE`); err != nil {
			// badges has no FK to users; ensure leftover assigned badges are cleared
		}
		if _, err := st.DB.ExecContext(ctx, `TRUNCATE media CASCADE`); err != nil {
		}
		if _, err := st.DB.ExecContext(ctx, `TRUNCATE lists CASCADE`); err != nil {
		}
		if _, err := st.DB.ExecContext(ctx, `TRUNCATE conversations CASCADE`); err != nil {
		}
		if err := os.RemoveAll(cfg.AppConfig.MediaDir); err != nil {
			fmt.Printf("warn: remove media dir: %v\n", err)
		}
		if err := os.MkdirAll(cfg.AppConfig.MediaDir, 0o755); err != nil {
			panic(fmt.Sprintf("failed to recreate media dir: %v", err))
		}
	}

	fmt.Println("🌱 Starting database seeding...")

	// Generate the deterministic dataset (fixed seed + now) and write it.
	ds := seedgen.GenerateFixed(time.Now().UTC())
	if err := seedgen.Apply(ctx, st, log, ds, seedgen.ApplyOptions{
		MediaDir: cfg.AppConfig.MediaDir,
	}); err != nil {
		panic(fmt.Sprintf("seed apply failed: %v", err))
	}

	fmt.Printf("✅ Created %d users\n", len(ds.Users))
	fmt.Printf("✅ Created %d posts (%d top-level, %d replies)\n", len(ds.Posts), seedgen.TopLevelPosts, seedgen.ReplyPosts)
	fmt.Printf("✅ Created %d likes, %d reposts, %d bookmarks\n", len(ds.Likes), len(ds.Reposts), len(ds.Bookmarks))
	fmt.Printf("✅ Created %d relationships, %d DMs, %d lists, %d badges, %d media files\n",
		len(ds.Relationships), len(ds.DMConversations), len(ds.Lists), len(ds.Badges), len(ds.Media))
	fmt.Println("🎉 Database seeding completed successfully!")

	fmt.Println()
	fmt.Println("  Demo accounts (password123):")
	for i := 0; i < seedgen.AnchorUsers; i++ {
		fmt.Printf("    %s / password123\n", ds.Users[i].Username)
	}
	fmt.Println("  Admin: alice / password123")
}