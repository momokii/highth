// Package cache handles Redis cache operations.
package cache

import (
	"context"
	"time"
)

// Cache defines the interface for cache operations.
// RedisCache implements this interface.
type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}) error
	SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	FlushAll(ctx context.Context) error
	IsEnabled() bool
	Ping(ctx context.Context) error
	Close() error
}
