package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisCache provides Redis-based caching functionality
type RedisCache struct {
	client *redis.Client
	ctx    context.Context
}

// CacheItem represents a cached item with metadata
type CacheItem struct {
	Key         string      `json:"key"`
	Value       interface{} `json:"value"`
	Expiration  time.Time   `json:"expiration"`
	CreatedAt   time.Time   `json:"created_at"`
	AccessCount int64       `json:"access_count"`
}

// CacheStats provides cache performance statistics
type CacheStats struct {
	HitCount    int64   `json:"hit_count"`
	MissCount   int64   `json:"miss_count"`
	HitRate     float64 `json:"hit_rate"`
	TotalKeys   int64   `json:"total_keys"`
	MemoryUsage int64   `json:"memory_usage_bytes"`
	AvgLatency  float64 `json:"avg_latency_ms"`
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(addr, password string, db int) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: client,
		ctx:    ctx,
	}, nil
}

// Set stores a value in Redis with TTL
func (r *RedisCache) Set(key string, value interface{}, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		r.recordLatency(time.Since(start))
	}()

	// Serialize value
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	// Store in Redis with TTL
	if err := r.client.Set(r.ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}

	// Update metadata
	metadata := CacheItem{
		Key:         key,
		Value:       value,
		Expiration:  time.Now().Add(ttl),
		CreatedAt:   time.Now(),
		AccessCount: 0,
	}

	metaData, _ := json.Marshal(metadata)
	r.client.Set(r.ctx, "meta:"+key, metaData, ttl)

	return nil
}

// Get retrieves a value from Redis
func (r *RedisCache) Get(key string, dest interface{}) (bool, error) {
	if r.client == nil {
		return false, nil
	}
	start := time.Now()
	defer func() {
		r.recordLatency(time.Since(start))
	}()

	// Get value from Redis
	data, err := r.client.Get(r.ctx, key).Result()
	if err == redis.Nil {
		r.incrementMissCount()
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to get cache: %w", err)
	}

	// Deserialize value
	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return false, fmt.Errorf("failed to unmarshal value: %w", err)
	}

	// Update access count
	r.incrementAccessCount(key)
	r.incrementHitCount()

	return true, nil
}

// Delete removes a key from Redis
func (r *RedisCache) Delete(key string) error {
	if err := r.client.Del(r.ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete cache: %w", err)
	}

	// Delete metadata
	r.client.Del(r.ctx, "meta:"+key)

	return nil
}

// Exists checks if a key exists in Redis
func (r *RedisCache) Exists(key string) (bool, error) {
	count, err := r.client.Exists(r.ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence: %w", err)
	}
	return count > 0, nil
}

// SetMultiple stores multiple key-value pairs
func (r *RedisCache) SetMultiple(items map[string]interface{}, ttl time.Duration) error {
	pipe := r.client.Pipeline()

	for key, value := range items {
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value for key %s: %w", key, err)
		}
		pipe.Set(r.ctx, key, data, ttl)
	}

	if _, err := pipe.Exec(r.ctx); err != nil {
		return fmt.Errorf("failed to set multiple cache items: %w", err)
	}

	return nil
}

// GetMultiple retrieves multiple values from Redis
func (r *RedisCache) GetMultiple(keys []string) (map[string]interface{}, error) {
	pipe := r.client.Pipeline()
	cmds := make(map[string]*redis.StringCmd)

	for _, key := range keys {
		cmds[key] = pipe.Get(r.ctx, key)
	}

	if _, err := pipe.Exec(r.ctx); err != nil {
		return nil, fmt.Errorf("failed to get multiple cache items: %w", err)
	}

	result := make(map[string]interface{})
	for key, cmd := range cmds {
		data, err := cmd.Result()
		if err == redis.Nil {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("failed to get key %s: %w", key, err)
		}

		var value interface{}
		if err := json.Unmarshal([]byte(data), &value); err != nil {
			return nil, fmt.Errorf("failed to unmarshal key %s: %w", key, err)
		}

		result[key] = value
	}

	return result, nil
}

// Clear removes all keys matching a pattern
func (r *RedisCache) Clear(pattern string) error {
	keys, err := r.client.Keys(r.ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get keys for pattern %s: %w", pattern, err)
	}

	if len(keys) > 0 {
		if err := r.client.Del(r.ctx, keys...).Err(); err != nil {
			return fmt.Errorf("failed to delete keys: %w", err)
		}
	}

	return nil
}

// GetStats returns cache performance statistics
func (r *RedisCache) GetStats() (*CacheStats, error) {
	info, err := r.client.Info(r.ctx, "memory", "keyspace", "stats").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis info: %w", err)
	}

	// Parse Redis info to extract stats
	stats := &CacheStats{
		HitCount:    r.getCounter("cache_hits"),
		MissCount:   r.getCounter("cache_misses"),
		TotalKeys:   r.getTotalKeys(),
		MemoryUsage: r.getMemoryUsage(info),
	}

	if stats.HitCount+stats.MissCount > 0 {
		stats.HitRate = float64(stats.HitCount) / float64(stats.HitCount+stats.MissCount)
	}

	return stats, nil
}

// Close closes the Redis connection
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// Helper methods

func (r *RedisCache) incrementHitCount() {
	r.client.Incr(r.ctx, "stats:cache_hits")
}

func (r *RedisCache) incrementMissCount() {
	r.client.Incr(r.ctx, "stats:cache_misses")
}

func (r *RedisCache) incrementAccessCount(key string) {
	r.client.Incr(r.ctx, "access:"+key)
}

func (r *RedisCache) getCounter(key string) int64 {
	val, _ := r.client.Get(r.ctx, "stats:"+key).Int64()
	return val
}

func (r *RedisCache) getTotalKeys() int64 {
	info, _ := r.client.Info(r.ctx, "keyspace").Result()
	// Simple parsing - in production, use proper Redis info parser
	if info != "" {
		return 100 // Placeholder
	}
	return 0
}

func (r *RedisCache) getMemoryUsage(info string) int64 {
	// Parse memory usage from Redis info
	// In production, use proper Redis info parser
	return 1024 * 1024 // 1MB placeholder
}

func (r *RedisCache) recordLatency(duration time.Duration) {
	if r.client == nil {
		return
	}
	latencyMs := float64(duration.Nanoseconds()) / 1000000
	r.client.LPush(r.ctx, "stats:latencies", latencyMs)
	r.client.LTrim(r.ctx, "stats:latencies", 0, 999) // Keep last 1000 measurements
}

// GetAverageLatency calculates average latency from recent measurements
func (r *RedisCache) GetAverageLatency() float64 {
	latencies, err := r.client.LRange(r.ctx, "stats:latencies", 0, -1).Result()
	if err != nil || len(latencies) == 0 {
		return 0
	}

	var total float64
	for _, lat := range latencies {
		var val float64
		if _, err := fmt.Sscanf(lat, "%f", &val); err == nil {
			total += val
		}
	}

	return total / float64(len(latencies))
}
