package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache interface for caching operations
type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}) error
	Delete(ctx context.Context, key string) error
	Close() error
}

// Config holds cache configuration
type Config struct {
	RedisURL string
	TTL      time.Duration
}

// DefaultConfig returns default cache configuration from environment
func DefaultConfig() Config {
	ttl := 600 // 10 minutes default
	if ttlStr := os.Getenv("CACHE_TTL"); ttlStr != "" {
		if parsed, err := strconv.Atoi(ttlStr); err == nil {
			ttl = parsed
		}
	}

	return Config{
		RedisURL: os.Getenv("REDIS_URL"),
		TTL:      time.Duration(ttl) * time.Second,
	}
}

// RedisCache implements Cache using Redis
type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(cfg Config) (*RedisCache, error) {
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parsing redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connecting to redis: %w", err)
	}

	return &RedisCache{
		client: client,
		ttl:    cfg.TTL,
	}, nil
}

func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return ErrCacheMiss
		}
		return fmt.Errorf("redis get: %w", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("unmarshaling cached data: %w", err)
	}

	return nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshaling data: %w", err)
	}

	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}

func (c *RedisCache) Delete(ctx context.Context, key string) error {
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis delete: %w", err)
	}
	return nil
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}

// MemoryCache implements Cache using in-memory storage (fallback)
type MemoryCache struct {
	data map[string]cacheEntry
	ttl  time.Duration
	mu   sync.RWMutex
}

type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(ttl time.Duration) *MemoryCache {
	return &MemoryCache{
		data: make(map[string]cacheEntry),
		ttl:  ttl,
	}
}

func (c *MemoryCache) Get(_ context.Context, key string, dest interface{}) error {
	c.mu.RLock()
	entry, ok := c.data[key]
	c.mu.RUnlock()

	if !ok {
		return ErrCacheMiss
	}

	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.data, key)
		c.mu.Unlock()
		return ErrCacheMiss
	}

	if err := json.Unmarshal(entry.data, dest); err != nil {
		return fmt.Errorf("unmarshaling cached data: %w", err)
	}

	return nil
}

func (c *MemoryCache) Set(_ context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshaling data: %w", err)
	}

	c.mu.Lock()
	c.data[key] = cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return nil
}

func (c *MemoryCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	delete(c.data, key)
	c.mu.Unlock()
	return nil
}

func (c *MemoryCache) Close() error {
	return nil
}

// ErrCacheMiss is returned when a key is not found in cache
var ErrCacheMiss = fmt.Errorf("cache miss")

// New creates a new cache based on configuration
// Returns Redis if configured, otherwise falls back to memory cache
func New(cfg Config) (Cache, error) {
	if cfg.RedisURL != "" {
		cache, err := NewRedisCache(cfg)
		if err != nil {
			fmt.Printf("Warning: Failed to connect to Redis (%v), using memory cache\n", err)
			return NewMemoryCache(cfg.TTL), nil
		}
		fmt.Println("Using Redis cache")
		return cache, nil
	}

	fmt.Println("Using memory cache")
	return NewMemoryCache(cfg.TTL), nil
}
