// Package cache handles Redis cache operations.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache provides cache operations using Redis.
type RedisCache struct {
	client    *redis.Client
	enabled   bool
	defaultTTL time.Duration
}

// Config holds Redis configuration.
type Config struct {
	URL             string
	Enabled         bool
	TTL             time.Duration
	PoolSize        int
	MinIdleConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
}

// New creates a new RedisCache with the given configuration.
func New(cfg Config) (*RedisCache, error) {
	if !cfg.Enabled {
		return &RedisCache{enabled: false}, nil
	}

	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	// Apply pool configuration if provided
	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}
	if cfg.MinIdleConns > 0 {
		opts.MinIdleConns = cfg.MinIdleConns
	}
	if cfg.MaxIdleConns > 0 {
		opts.MaxIdleConns = cfg.MaxIdleConns
	}
	if cfg.ConnMaxIdleTime > 0 {
		opts.ConnMaxIdleTime = cfg.ConnMaxIdleTime
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisCache{
		client:    client,
		enabled:   true,
		defaultTTL: cfg.TTL,
	}, nil
}

// Get retrieves a value from the cache by key.
// It unmarshals the JSON value into the provided interface.
func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	if !c.enabled {
		return fmt.Errorf("cache disabled")
	}

	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("key not found")
		}
		return fmt.Errorf("failed to get from cache: %w", err)
	}

	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return fmt.Errorf("failed to unmarshal cached value: %w", err)
	}

	return nil
}

// Set stores a value in the cache with the default TTL.
// The value is marshaled to JSON before storage.
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}) error {
	return c.SetWithTTL(ctx, key, value, c.defaultTTL)
}

// SetWithTTL stores a value in the cache with a specific TTL.
// The value is marshaled to JSON before storage.
func (c *RedisCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if !c.enabled {
		return nil // Silently fail if cache is disabled
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set in cache: %w", err)
	}

	return nil
}

// Delete removes a key from the cache.
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	if !c.enabled {
		return nil
	}

	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete from cache: %w", err)
	}

	return nil
}

// FlushAll clears all keys from the cache.
// Use with caution in production.
func (c *RedisCache) FlushAll(ctx context.Context) error {
	if !c.enabled {
		return nil
	}

	if err := c.client.FlushAll(ctx).Err(); err != nil {
		return fmt.Errorf("failed to flush cache: %w", err)
	}

	return nil
}

// Close closes the Redis client connection.
func (c *RedisCache) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// IsEnabled returns whether the cache is enabled.
func (c *RedisCache) IsEnabled() bool {
	return c.enabled
}

// Ping checks if the Redis connection is alive.
func (c *RedisCache) Ping(ctx context.Context) error {
	if !c.enabled {
		return nil
	}

	return c.client.Ping(ctx).Err()
}
