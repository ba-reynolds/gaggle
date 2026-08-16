package cache

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps a Redis client with the app's caching helpers.
type Client struct {
	rdb     *redis.Client
	logger  *slog.Logger
	feedTTL time.Duration
}

func New(addr, password string, db int, feedTTL time.Duration, logger *slog.Logger) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Client{rdb: rdb, logger: logger, feedTTL: feedTTL}
}

// Ping verifies connectivity. It returns false (without failing the app) when
// Redis is unavailable so the server can degrade gracefully to the database.
func (c *Client) Ping(ctx context.Context) bool {
	if c == nil || c.rdb == nil {
		return false
	}
	if err := c.rdb.Ping(ctx).Err(); err != nil {
		c.logger.Warn("redis unavailable, running without cache", "error", err)
		return false
	}
	return true
}

// Close closes the underlying connection pool.
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

func (c *Client) feedKey(userID int, cursor string) string {
	return "feed:home:" + strconv.Itoa(userID) + ":" + cursor
}

// GetHomeFeed returns the cached feed payload for a user/cursor, if present.
func (c *Client) GetHomeFeed(ctx context.Context, userID int, cursor string) ([]byte, bool) {
	if c == nil || c.rdb == nil {
		return nil, false
	}
	data, err := c.rdb.Get(ctx, c.feedKey(userID, cursor)).Bytes()
	if err != nil {
		return nil, false
	}
	return data, true
}

// SetHomeFeed caches the feed payload for a user/cursor with the TTL.
func (c *Client) SetHomeFeed(ctx context.Context, userID int, cursor string, data []byte) {
	if c == nil || c.rdb == nil {
		return
	}
	if err := c.rdb.Set(ctx, c.feedKey(userID, cursor), data, c.feedTTL).Err(); err != nil {
		c.logger.Warn("failed to cache home feed", "userID", userID, "error", err)
	}
}

// InvalidateHomeFeed removes all cached home feed pages for a user. It is
// called after any write that changes what the user's feed would contain
// (new post, follow/unfollow, like/repost/etc).
func (c *Client) InvalidateHomeFeed(ctx context.Context, userID int) {
	if c == nil || c.rdb == nil {
		return
	}
	iter := c.rdb.Scan(ctx, 0, "feed:home:"+strconv.Itoa(userID)+":*", 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if len(keys) > 0 {
		if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
			c.logger.Warn("failed to invalidate home feed cache", "userID", userID, "error", err)
		}
	}
}

// Allow checks whether a request is within the rate limit for the given key.
// It uses a fixed-window counter: INCR on the key, set expiry on first hit.
func (c *Client) Allow(ctx context.Context, key string, maxRequests int, window time.Duration) bool {
	if c == nil || c.rdb == nil {
		// When Redis is unavailable, do not rate limit (degrade open).
		return true
	}
	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	_, err := pipe.Exec(ctx)
	if err != nil {
		c.logger.Warn("rate limit check failed, allowing request", "key", key, "error", err)
		return true
	}
	return incr.Val() <= int64(maxRequests)
}
