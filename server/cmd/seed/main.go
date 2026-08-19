package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"time"

	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/store"
	"github.com/ba-reynolds/gaggle/pkg/config"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	// Initialize logger
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(log)

	// Initialize database connection
	db, err := store.NewPostgresStorage(cfg.DBConfig)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to database: %v", err))
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		panic(fmt.Sprintf("failed to ping database: %v", err))
	}

	// Initialize store
	store := store.NewStore(db, log, cfg.AppConfig.MediaDir)

	// Create context
	ctx := context.Background()

	// Idempotency guard: if the seed anchor user already exists, the database
	// is already seeded. Exit quietly so `make seed` / container re-runs are safe.
	if _, err := store.Users.GetByEmail(ctx, "alice@example.com"); err == nil {
		fmt.Println("✅ Database already seeded (alice@example.com exists); skipping.")
		return
	}

	fmt.Println("🌱 Starting database seeding...")

	// Seed users
	users := seedUsers(ctx, store, log)
	fmt.Printf("✅ Created %d users\n", len(users))

	// Seed user profiles
	seedUserProfiles(ctx, store, log, users)
	fmt.Println("✅ Created user profiles")

	// Seed posts
	posts := seedPosts(ctx, store, log, users)
	fmt.Printf("✅ Created %d posts\n", len(posts))

	// Seed user relationships
	seedUserRelationships(ctx, store, log, users)
	fmt.Println("✅ Created user relationships")

	// Seed media (dummy entries)
	seedMedia(ctx, store, log)
	fmt.Println("✅ Created dummy media entries")

	fmt.Println("🎉 Database seeding completed successfully!")
}

func seedUsers(ctx context.Context, store *store.Store, log *slog.Logger) []*models.User {
	users := []*models.User{
		{
			Username: "alice",
			Email:    "alice@example.com",
			Password: hashPassword("password123"),
		},
		{
			Username: "bob",
			Email:    "bob@example.com",
			Password: hashPassword("password123"),
		},
		{
			Username: "charlie",
			Email:    "charlie@example.com",
			Password: hashPassword("password123"),
		},
		{
			Username: "diana",
			Email:    "diana@example.com",
			Password: hashPassword("password123"),
		},
		{
			Username: "eve",
			Email:    "eve@example.com",
			Password: hashPassword("password123"),
		},
		{
			Username: "frank",
			Email:    "frank@example.com",
			Password: hashPassword("password123"),
		},
		{
			Username: "grace",
			Email:    "grace@example.com",
			Password: hashPassword("password123"),
		},
		{
			Username: "henry",
			Email:    "henry@example.com",
			Password: hashPassword("password123"),
		},
	}

	createdUsers := make([]*models.User, 0, len(users))

	for _, user := range users {
		// Start transaction
		tx, err := store.DB.BeginTx(ctx, nil)
		if err != nil {
			log.Error("failed to begin transaction for user creation", "error", err)
			continue
		}

		// Create user
		err = store.Users.Create(ctx, tx, user)
		if err != nil {
			log.Error("failed to create user", "username", user.Username, "error", err)
			tx.Rollback()
			continue
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			log.Error("failed to commit transaction for user creation", "error", err)
			continue
		}

		createdUsers = append(createdUsers, user)
		fmt.Printf("  👤 Created user: %s (ID: %d)\n", user.Username, user.ID)
	}

	return createdUsers
}

func seedUserProfiles(ctx context.Context, store *store.Store, log *slog.Logger, users []*models.User) {
	displayNames := []string{
		"Alice Johnson", "Bob Smith", "Charlie Brown", "Diana Prince",
		"Eve Wilson", "Frank Miller", "Grace Kelly", "Henry Ford",
	}

	bios := []string{
		"Software engineer and coffee enthusiast ☕",
		"Digital artist creating amazing visuals 🎨",
		"Travel blogger sharing adventures around the world 🌍",
		"Fitness coach helping people reach their goals 💪",
		"Food lover and amateur chef 👨‍🍳",
		"Music producer and DJ 🎵",
		"Photographer capturing life's beautiful moments 📸",
		"Bookworm and literary critic 📚",
	}

	locations := []string{
		"San Francisco, CA", "New York, NY", "London, UK", "Tokyo, Japan",
		"Paris, France", "Sydney, Australia", "Toronto, Canada", "Berlin, Germany",
	}

	websites := []string{
		"https://alice.dev", "https://bobart.com", "https://charlie-travels.com",
		"https://diana-fitness.com", "https://eve-cooks.com", "https://frank-music.com",
		"https://grace-photos.com", "https://henry-books.com",
	}

	for i, user := range users {
		if i >= len(displayNames) {
			break
		}

		// Generate random birth date (18-65 years old)
		years := rand.Intn(47) + 18
		months := rand.Intn(12)
		days := rand.Intn(28) + 1
		birthDate := time.Now().AddDate(-years, -months, -days)

		userWithProfile := &models.UserWithProfile{
			User: *user,
			Profile: models.UserProfile{
				DisplayName:        displayNames[i],
				Bio:                bios[i],
				ProfilePictureUUID: uuid.Nil,
				BannerUUID:         uuid.Nil,
				BirthDate:          models.Date{Time: birthDate},
				Location:           locations[i],
				Website:            websites[i],
			},
		}

		// Start transaction
		tx, err := store.DB.BeginTx(ctx, nil)
		if err != nil {
			log.Error("failed to begin transaction for profile creation", "error", err)
			continue
		}

		// Update user profile
		err = store.Users.UpdateUserProfile(ctx, tx, userWithProfile)
		if err != nil {
			log.Error("failed to update user profile", "username", user.Username, "error", err)
			tx.Rollback()
			continue
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			log.Error("failed to commit transaction for profile creation", "error", err)
			continue
		}

		fmt.Printf("  📝 Created profile for: %s\n", user.Username)
	}
}

func seedPosts(ctx context.Context, store *store.Store, log *slog.Logger, users []*models.User) []*models.Post {
	postContents := []string{
		"Just finished a great coding session! 🚀 #programming #coding",
		"Beautiful sunset today! Nature is amazing 🌅 #sunset #nature",
		"Working on some new artwork. Can't wait to share the final result! 🎨 #art",
		"Had an amazing workout today. Feeling energized! 💪 #fitness",
		"Cooking up something delicious in the kitchen 👨‍🍳 #food",
		"New track dropping soon! Stay tuned 🎵 #music",
		"Captured this amazing moment today 📸 #photography",
		"Reading an incredible book right now. Highly recommend! 📚",
		"Great meeting with the team today. Excited about our new project! 👥",
		"Travel plans are coming together nicely ✈️",
		"Learning something new every day! 📖",
		"Perfect weather for a walk in the park 🌳",
		"Music studio session was productive today 🎤",
		"Photography workshop was amazing! 📷",
		"Book club discussion was fascinating tonight 📖",
		"Tech conference was mind-blowing! 🤯",
	}

	posts := make([]*models.Post, 0)

	// Create some original posts
	for i, content := range postContents {
		if i >= len(users) {
			break
		}

		post := &models.Post{
			Content:    content,
			AuthorID:   users[i].ID,
			Visibility: "public",
		}

		// Start transaction
		tx, err := store.DB.BeginTx(ctx, nil)
		if err != nil {
			log.Error("failed to begin transaction for post creation", "error", err)
			continue
		}

		// Create post
		err = store.Posts.Create(ctx, tx, post)
		if err != nil {
			log.Error("failed to create post", "author", users[i].Username, "error", err)
			tx.Rollback()
			continue
		}

		// Sync hashtags so seeded posts actually power /trends (the service
		// layer does this, but the seed calls the store directly).
		if err := store.Hashtags.SyncPost(ctx, tx, post.ID, post.Content); err != nil {
			log.Error("failed to sync hashtags for seeded post", "postID", post.ID, "error", err)
			tx.Rollback()
			continue
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			log.Error("failed to commit transaction for post creation", "error", err)
			continue
		}

		posts = append(posts, post)
		fmt.Printf("  📝 Created post by %s: %s\n", users[i].Username, truncateString(content, 50))
	}

	// Create some reply posts
	replyContents := []string{
		"Great post! 👍",
		"Thanks for sharing! 🙏",
		"Love this! ❤️",
		"Amazing work! 👏",
		"Keep it up! 💪",
		"Interesting perspective! 🤔",
		"Can't wait to see more! 👀",
		"Fantastic! 🌟",
	}

	for i, replyContent := range replyContents {
		if i >= len(posts) {
			break
		}

		// Randomly select a user to reply (different from the original author)
		authorIndex := (i + 1) % len(users)
		parentPost := posts[i]

		reply := &models.Post{
			Content:    replyContent,
			AuthorID:   users[authorIndex].ID,
			ParentID:   &parentPost.ID,
			Visibility: "public",
		}

		// Start transaction
		tx, err := store.DB.BeginTx(ctx, nil)
		if err != nil {
			log.Error("failed to begin transaction for reply creation", "error", err)
			continue
		}

		// Create reply
		err = store.Posts.Create(ctx, tx, reply)
		if err != nil {
			log.Error("failed to create reply", "author", users[authorIndex].Username, "error", err)
			tx.Rollback()
			continue
		}

		// Sync hashtags for parity with top-level posts (replies rarely carry
		// any, but keeps behavior identical to the service layer).
		if err := store.Hashtags.SyncPost(ctx, tx, reply.ID, reply.Content); err != nil {
			log.Error("failed to sync hashtags for seeded reply", "postID", reply.ID, "error", err)
			tx.Rollback()
			continue
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			log.Error("failed to commit transaction for reply creation", "error", err)
			continue
		}

		posts = append(posts, reply)
		fmt.Printf("  💬 Created reply by %s to post %d: %s\n", users[authorIndex].Username, parentPost.ID, truncateString(replyContent, 30))
	}

	return posts
}

func seedUserRelationships(ctx context.Context, store *store.Store, log *slog.Logger, users []*models.User) {
	// Create some follow relationships
	followPairs := [][]int{
		{1, 2}, {1, 3}, {2, 1}, {2, 4}, {3, 1}, {3, 5},
		{4, 2}, {4, 6}, {5, 3}, {5, 7}, {6, 4}, {6, 8},
		{7, 5}, {7, 1}, {8, 6}, {8, 2},
	}

	for _, pair := range followPairs {
		if pair[0] > len(users) || pair[1] > len(users) {
			continue
		}

		// Start transaction
		tx, err := store.DB.BeginTx(ctx, nil)
		if err != nil {
			log.Error("failed to begin transaction for relationship creation", "error", err)
			continue
		}

		// Create relationship
		relationship := &models.UserRelationship{
			FollowerID:       pair[0],
			FollowingID:      pair[1],
			RelationshipType: "follow",
		}

		err = store.UserRelationships.Create(ctx, tx, relationship)
		if err != nil {
			log.Error("failed to create relationship", "follower", pair[0], "following", pair[1], "error", err)
			tx.Rollback()
			continue
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			log.Error("failed to commit transaction for relationship creation", "error", err)
			continue
		}

		fmt.Printf("  👥 Created follow relationship: User %d → User %d\n", pair[0], pair[1])
	}

	// Create one block relationship for testing
	if len(users) >= 2 {
		tx, err := store.DB.BeginTx(ctx, nil)
		if err != nil {
			log.Error("failed to begin transaction for block creation", "error", err)
			return
		}

		blockRelationship := &models.UserRelationship{
			FollowerID:       1, // Alice
			FollowingID:      5, // Eve
			RelationshipType: "block",
		}

		err = store.UserRelationships.Create(ctx, tx, blockRelationship)
		if err != nil {
			log.Error("failed to create block relationship", "error", err)
			tx.Rollback()
			return
		}

		if err := tx.Commit(); err != nil {
			log.Error("failed to commit transaction for block creation", "error", err)
			return
		}

		fmt.Printf("  🚫 Created block relationship: User 1 → User 5\n")
	}
}

func seedMedia(ctx context.Context, store *store.Store, log *slog.Logger) {
	// Create some dummy media entries for testing
	mediaEntries := []models.Media{
		{
			UUID:     uuid.New(),
			MimeType: "image/jpeg",
			Filename: "profile_picture_1.jpg",
		},
		{
			UUID:     uuid.New(),
			MimeType: "image/png",
			Filename: "banner_image_1.png",
		},
		{
			UUID:     uuid.New(),
			MimeType: "image/jpeg",
			Filename: "post_image_1.jpg",
		},
		{
			UUID:     uuid.New(),
			MimeType: "image/gif",
			Filename: "animated_avatar.gif",
		},
	}

	for _, media := range mediaEntries {
		// Start transaction
		tx, err := store.DB.BeginTx(ctx, nil)
		if err != nil {
			log.Error("failed to begin transaction for media creation", "error", err)
			continue
		}

		// Create media entry
		err = store.Media.Create(ctx, tx, &media)
		if err != nil {
			log.Error("failed to create media entry", "filename", media.Filename, "error", err)
			tx.Rollback()
			continue
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			log.Error("failed to commit transaction for media creation", "error", err)
			continue
		}

		fmt.Printf("  📁 Created media entry: %s (%s)\n", media.Filename, media.MimeType)
	}
}

func hashPassword(password string) string {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("failed to hash password: %v", err))
	}
	return string(hashedPassword)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
