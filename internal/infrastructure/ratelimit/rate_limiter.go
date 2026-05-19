package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

// RateLimiterInterface defines the interface for rate limiting implementations
type RateLimiterInterface interface {
	CheckRateLimit(ctx context.Context, key string, config RateLimitConfig) (*RateLimitResult, error)
	GetRateLimitInfo(ctx context.Context, key string, config RateLimitConfig) (*RateLimitResult, error)
}

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	redisClient *redis.Client
	prefix      string
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Requests int           // Number of requests allowed
	Window   time.Duration // Time window for the limit
}

// RateLimitResult represents the result of a rate limit check
type RateLimitResult struct {
	Allowed    bool          // Whether the request is allowed
	Remaining  int           // Number of requests remaining
	ResetTime  time.Time     // When the rate limit window resets
	RetryAfter time.Duration // How long to wait before retrying
	Limit      int           // The total limit for this window
	Window     time.Duration // The time window duration
}

// NewRateLimiter creates a new rate limiter with Redis backend
func NewRateLimiter(redisClient *redis.Client, prefix string) *RateLimiter {
	if prefix == "" {
		prefix = "ratelimit"
	}

	return &RateLimiter{
		redisClient: redisClient,
		prefix:      prefix,
	}
}

// CheckRateLimit checks if a request should be allowed based on rate limits
func (rl *RateLimiter) CheckRateLimit(ctx context.Context, key string, config RateLimitConfig) (*RateLimitResult, error) {
	if rl.redisClient == nil {
		// If Redis is not available, allow all requests (fail-open)
		return &RateLimitResult{
			Allowed:   true,
			Remaining: config.Requests,
			Limit:     config.Requests,
			Window:    config.Window,
		}, nil
	}

	// Use sliding window algorithm with Redis
	now := time.Now()
	windowStart := now.Add(-config.Window)

	redisKey := fmt.Sprintf("%s:%s", rl.prefix, key)

	// Use Redis pipeline for atomic operations
	pipe := rl.redisClient.Pipeline()

	// Remove old entries outside the window
	pipe.ZRemRangeByScore(ctx, redisKey, "0", strconv.FormatInt(windowStart.UnixNano(), 10))

	// Count current requests in window
	currentCmd := pipe.ZCard(ctx, redisKey)

	// Add current request
	pipe.ZAdd(ctx, redisKey, &redis.Z{
		Score:  float64(now.UnixNano()),
		Member: now.UnixNano(),
	})

	// Set expiration for the key
	pipe.Expire(ctx, redisKey, config.Window)

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute rate limit pipeline: %w", err)
	}

	current, err := currentCmd.Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get current request count: %w", err)
	}

	allowed := current <= int64(config.Requests)
	remaining := config.Requests - int(current)

	// Calculate reset time (when the oldest request will expire)
	oldestCmd := rl.redisClient.ZRangeWithScores(ctx, redisKey, 0, 0)
	oldest, err := oldestCmd.Result()
	if err != nil || len(oldest) == 0 {
		// If no requests or error, reset time is now + window
		return &RateLimitResult{
			Allowed:   allowed,
			Remaining: remaining,
			ResetTime: now.Add(config.Window),
			Limit:     config.Requests,
			Window:    config.Window,
		}, nil
	}

	oldestTime := time.Unix(0, int64(oldest[0].Score))
	resetTime := oldestTime.Add(config.Window)

	var retryAfter time.Duration
	if !allowed {
		retryAfter = time.Until(resetTime)
		if retryAfter < 0 {
			retryAfter = 0
		}
	}

	return &RateLimitResult{
		Allowed:    allowed,
		Remaining:  remaining,
		ResetTime:  resetTime,
		RetryAfter: retryAfter,
		Limit:      config.Requests,
		Window:     config.Window,
	}, nil
}

// CheckIPRateLimit checks rate limit for an IP address
func (rl *RateLimiter) CheckIPRateLimit(ctx context.Context, ip string, config RateLimitConfig) (*RateLimitResult, error) {
	key := fmt.Sprintf("ip:%s", ip)
	return rl.CheckRateLimit(ctx, key, config)
}

// CheckUserRateLimit checks rate limit for a user ID
func (rl *RateLimiter) CheckUserRateLimit(ctx context.Context, userID string, config RateLimitConfig) (*RateLimitResult, error) {
	key := fmt.Sprintf("user:%s", userID)
	return rl.CheckRateLimit(ctx, key, config)
}

// CheckEndpointRateLimit checks rate limit for a specific endpoint
func (rl *RateLimiter) CheckEndpointRateLimit(ctx context.Context, endpoint string, config RateLimitConfig) (*RateLimitResult, error) {
	key := fmt.Sprintf("endpoint:%s", endpoint)
	return rl.CheckRateLimit(ctx, key, config)
}

// CheckCombinedRateLimit checks rate limit for combined IP + User + Endpoint
func (rl *RateLimiter) CheckCombinedRateLimit(ctx context.Context, ip, userID, endpoint string, config RateLimitConfig) (*RateLimitResult, error) {
	key := fmt.Sprintf("combined:%s:%s:%s", ip, userID, endpoint)
	return rl.CheckRateLimit(ctx, key, config)
}

// ResetRateLimit resets the rate limit for a specific key
func (rl *RateLimiter) ResetRateLimit(ctx context.Context, key string) error {
	if rl.redisClient == nil {
		return nil // No-op if Redis is not available
	}

	redisKey := fmt.Sprintf("%s:%s", rl.prefix, key)
	return rl.redisClient.Del(ctx, redisKey).Err()
}

// GetRateLimitInfo gets current rate limit information without incrementing
func (rl *RateLimiter) GetRateLimitInfo(ctx context.Context, key string, config RateLimitConfig) (*RateLimitResult, error) {
	if rl.redisClient == nil {
		return &RateLimitResult{
			Allowed:   true,
			Remaining: config.Requests,
			Limit:     config.Requests,
			Window:    config.Window,
		}, nil
	}

	redisKey := fmt.Sprintf("%s:%s", rl.prefix, key)
	now := time.Now()
	windowStart := now.Add(-config.Window)

	// Remove old entries outside the window
	_, err := rl.redisClient.ZRemRangeByScore(ctx, redisKey, "0", strconv.FormatInt(windowStart.UnixNano(), 10)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to clean old entries: %w", err)
	}

	// Count current requests in window
	current, err := rl.redisClient.ZCard(ctx, redisKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get current request count: %w", err)
	}

	allowed := current < int64(config.Requests)
	remaining := config.Requests - int(current)

	// Calculate reset time
	oldest, err := rl.redisClient.ZRangeWithScores(ctx, redisKey, 0, 0).Result()
	if err != nil || len(oldest) == 0 {
		return &RateLimitResult{
			Allowed:   allowed,
			Remaining: remaining,
			ResetTime: now.Add(config.Window),
			Limit:     config.Requests,
			Window:    config.Window,
		}, nil
	}

	oldestTime := time.Unix(0, int64(oldest[0].Score))
	resetTime := oldestTime.Add(config.Window)

	return &RateLimitResult{
		Allowed:   allowed,
		Remaining: remaining,
		ResetTime: resetTime,
		Limit:     config.Requests,
		Window:    config.Window,
	}, nil
}
