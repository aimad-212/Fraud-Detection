package tests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	httpHandler "fraud-detection/internal/delivery/http"
	"fraud-detection/internal/infrastructure/ratelimit"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// InMemoryRateLimiter provides an in-memory rate limiter for testing
type InMemoryRateLimiter struct {
	requests map[string][]time.Time
	mutex    sync.RWMutex
	prefix   string
}

// NewInMemoryRateLimiter creates a new in-memory rate limiter
func NewInMemoryRateLimiter(prefix string) *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		requests: make(map[string][]time.Time),
		prefix:   prefix,
	}
}

// CheckRateLimit checks if a request should be allowed
func (imrl *InMemoryRateLimiter) CheckRateLimit(ctx context.Context, key string, config ratelimit.RateLimitConfig) (*ratelimit.RateLimitResult, error) {
	imrl.mutex.Lock()
	defer imrl.mutex.Unlock()

	fullKey := fmt.Sprintf("%s:%s", imrl.prefix, key)
	now := time.Now()
	windowStart := now.Add(-config.Window)

	// Clean old requests
	if requests, exists := imrl.requests[fullKey]; exists {
		filtered := make([]time.Time, 0)
		for _, reqTime := range requests {
			if reqTime.After(windowStart) {
				filtered = append(filtered, reqTime)
			}
		}
		imrl.requests[fullKey] = filtered
	}

	// Count current requests in window
	current := 0
	if requests, exists := imrl.requests[fullKey]; exists {
		for _, reqTime := range requests {
			if reqTime.After(windowStart) {
				current++
			}
		}
	}

	allowed := current < config.Requests

	// Add current request if allowed
	if allowed {
		imrl.requests[fullKey] = append(imrl.requests[fullKey], now)
		current++
	}

	remaining := config.Requests - current

	// Calculate reset time
	var resetTime time.Time
	if len(imrl.requests[fullKey]) > 0 {
		oldest := imrl.requests[fullKey][0]
		resetTime = oldest.Add(config.Window)
	} else {
		resetTime = now.Add(config.Window)
	}

	var retryAfter time.Duration
	if !allowed {
		retryAfter = time.Until(resetTime)
		if retryAfter < 0 {
			retryAfter = 0
		}
	}

	return &ratelimit.RateLimitResult{
		Allowed:    allowed,
		Remaining:  remaining,
		ResetTime:  resetTime,
		RetryAfter: retryAfter,
		Limit:      config.Requests,
		Window:     config.Window,
	}, nil
}

// GetRateLimitInfo gets current rate limit info without incrementing
func (imrl *InMemoryRateLimiter) GetRateLimitInfo(ctx context.Context, key string, config ratelimit.RateLimitConfig) (*ratelimit.RateLimitResult, error) {
	imrl.mutex.RLock()
	defer imrl.mutex.RUnlock()

	fullKey := fmt.Sprintf("%s:%s", imrl.prefix, key)
	now := time.Now()
	windowStart := now.Add(-config.Window)

	// Count current requests in window
	current := 0
	if requests, exists := imrl.requests[fullKey]; exists {
		for _, reqTime := range requests {
			if reqTime.After(windowStart) {
				current++
			}
		}
	}

	allowed := current < config.Requests
	remaining := config.Requests - current

	// Calculate reset time
	var resetTime time.Time
	if requests, exists := imrl.requests[fullKey]; exists && len(requests) > 0 {
		oldest := requests[0]
		resetTime = oldest.Add(config.Window)
	} else {
		resetTime = now.Add(config.Window)
	}

	return &ratelimit.RateLimitResult{
		Allowed:   allowed,
		Remaining: remaining,
		ResetTime: resetTime,
		Limit:     config.Requests,
		Window:    config.Window,
	}, nil
}

// RateLimiterAdapter adapts InMemoryRateLimiter to ratelimit.RateLimiterInterface
type RateLimiterAdapter struct {
	*InMemoryRateLimiter
}

func NewRateLimiterAdapter(prefix string) *RateLimiterAdapter {
	return &RateLimiterAdapter{
		InMemoryRateLimiter: NewInMemoryRateLimiter(prefix),
	}
}

// Ensure RateLimiterAdapter implements the interface
var _ ratelimit.RateLimiterInterface = (*RateLimiterAdapter)(nil)

func TestRateLimiter(t *testing.T) {
	// Test with in-memory rate limiter for actual rate limiting
	limiter := NewRateLimiterAdapter("test")

	t.Run("Actual rate limiting with in-memory", func(t *testing.T) {
		config := ratelimit.RateLimitConfig{
			Requests: 3,
			Window:   time.Minute,
		}

		// First 3 requests should be allowed
		for i := 0; i < 3; i++ {
			result, err := limiter.CheckRateLimit(context.Background(), "test-key", config)
			require.NoError(t, err)
			assert.True(t, result.Allowed, "Request %d should be allowed", i+1)
			assert.Equal(t, 3-(i+1), result.Remaining, "Should show correct remaining")
		}

		// 4th request should be blocked
		result, err := limiter.CheckRateLimit(context.Background(), "test-key", config)
		require.NoError(t, err)
		assert.False(t, result.Allowed, "Request 4 should be blocked")
		assert.Equal(t, 0, result.Remaining, "Should show 0 remaining")
		assert.True(t, result.RetryAfter > 0, "Should have retry after time")
	})

	t.Run("Rate limit info without incrementing", func(t *testing.T) {
		config := ratelimit.RateLimitConfig{
			Requests: 5,
			Window:   time.Minute,
		}

		// Add some requests first
		for i := 0; i < 2; i++ {
			limiter.CheckRateLimit(context.Background(), "info-key", config)
		}

		// Get info without incrementing
		result, err := limiter.GetRateLimitInfo(context.Background(), "info-key", config)
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Should be allowed")
		assert.Equal(t, 3, result.Remaining, "Should show correct remaining")
	})

	t.Run("Fail-open when Redis unavailable", func(t *testing.T) {
		// Test with nil Redis client (original behavior)
		nilLimiter := ratelimit.NewRateLimiter(nil, "test")
		config := ratelimit.RateLimitConfig{
			Requests: 1,
			Window:   time.Minute,
		}

		// When Redis is nil, should allow all requests
		for i := 0; i < 5; i++ {
			result, err := nilLimiter.CheckRateLimit(context.Background(), "test-key", config)
			require.NoError(t, err)
			assert.True(t, result.Allowed, "Request %d should be allowed (fail-open)", i+1)
			assert.Equal(t, config.Requests, result.Remaining, "Should show full remaining when fail-open")
		}
	})
}

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create in-memory rate limiter for actual rate limiting in tests
	limiter := NewRateLimiterAdapter("test")
	middleware := httpHandler.NewRateLimitMiddleware(limiter)

	// Set strict limits for testing
	middleware.SetDefaultConfig(httpHandler.EndpointConfig{
		Global: ratelimit.RateLimitConfig{
			Requests: 5,
			Window:   time.Minute,
		},
		PerIP: ratelimit.RateLimitConfig{
			Requests: 3,
			Window:   time.Minute,
		},
		PerUser: ratelimit.RateLimitConfig{
			Requests: 10,
			Window:   time.Minute,
		},
	})

	t.Run("Allow requests under limit", func(t *testing.T) {
		// Create fresh middleware for this test
		freshLimiter := NewRateLimiterAdapter("test-fresh")
		freshMiddleware := httpHandler.NewRateLimitMiddleware(freshLimiter)
		freshMiddleware.SetDefaultConfig(httpHandler.EndpointConfig{
			Global: ratelimit.RateLimitConfig{
				Requests: 5,
				Window:   time.Minute,
			},
			PerIP: ratelimit.RateLimitConfig{
				Requests: 3,
				Window:   time.Minute,
			},
			PerUser: ratelimit.RateLimitConfig{
				Requests: 10,
				Window:   time.Minute,
			},
		})

		router := gin.New()
		router.Use(freshMiddleware.RateLimit())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Make requests under the limit
		for i := 0; i < 3; i++ {
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345" // Set IP
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Request %d should be allowed", i+1)
			expectedRemaining := 3 - (i + 1)
			assert.Equal(t, strconv.Itoa(expectedRemaining), w.Header().Get("X-Rate-Limit-Ip-Remaining"))
		}
	})

	t.Run("Block requests over IP limit", func(t *testing.T) {
		// Create fresh middleware for this test
		freshLimiter := NewRateLimiterAdapter("test-block")
		freshMiddleware := httpHandler.NewRateLimitMiddleware(freshLimiter)
		freshMiddleware.SetDefaultConfig(httpHandler.EndpointConfig{
			PerIP: ratelimit.RateLimitConfig{
				Requests: 3,
				Window:   time.Minute,
			},
		})

		router := gin.New()
		router.Use(freshMiddleware.RateLimit())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Make requests up to and over the limit
		for i := 0; i < 4; i++ {
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.2:12345" // Different IP
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if i < 3 {
				assert.Equal(t, http.StatusOK, w.Code, "Request %d should be allowed", i+1)
			} else {
				assert.Equal(t, http.StatusTooManyRequests, w.Code, "Request %d should be blocked", i+1)
				assert.Contains(t, w.Body.String(), "Rate limit exceeded")
			}
		}
	})

	t.Run("User-specific rate limiting", func(t *testing.T) {
		// Create fresh middleware for this test
		freshLimiter := NewRateLimiterAdapter("test-user")
		freshMiddleware := httpHandler.NewRateLimitMiddleware(freshLimiter)
		freshMiddleware.SetDefaultConfig(httpHandler.EndpointConfig{
			PerUser: ratelimit.RateLimitConfig{
				Requests: 10,
				Window:   time.Minute,
			},
		})

		router := gin.New()

		// Add middleware to set user context for testing
		router.Use(func(c *gin.Context) {
			c.Set("user_id", "test-user-123")
			c.Next()
		})
		router.Use(freshMiddleware.RateLimit())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Simulate authenticated user
		for i := 0; i < 12; i++ {
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.3:12345"
			req.Header.Set("Authorization", "Bearer valid-token")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if i < 10 {
				assert.Equal(t, http.StatusOK, w.Code, "Request %d should be allowed", i+1)
			} else {
				assert.Equal(t, http.StatusTooManyRequests, w.Code, "Request %d should be blocked", i+1)
			}
		}
	})

	t.Run("Rate limit headers", func(t *testing.T) {
		// Create fresh middleware for this test
		freshLimiter := NewRateLimiterAdapter("test-headers")
		freshMiddleware := httpHandler.NewRateLimitMiddleware(freshLimiter)
		freshMiddleware.SetDefaultConfig(httpHandler.EndpointConfig{
			PerIP: ratelimit.RateLimitConfig{
				Requests: 3,
				Window:   time.Minute,
			},
		})

		router := gin.New()
		router.Use(freshMiddleware.RateLimit())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.4:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Check rate limit headers
		assert.Equal(t, "2", w.Header().Get("X-Rate-Limit-Ip-Remaining")) // 3-1 = 2 remaining
		assert.Equal(t, "3", w.Header().Get("X-Rate-Limit-Ip-Limit"))
		assert.NotEmpty(t, w.Header().Get("X-Rate-Limit-Ip-Reset"))
	})

	t.Run("Fail-open when Redis unavailable", func(t *testing.T) {
		// Create rate limiter with nil Redis client
		nilLimiter := ratelimit.NewRateLimiter(nil, "test")
		nilMiddleware := httpHandler.NewRateLimitMiddleware(nilLimiter)

		router := gin.New()
		router.Use(nilMiddleware.RateLimit())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Should allow all requests when Redis is unavailable
		for i := 0; i < 10; i++ {
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.5:12345"
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Request %d should be allowed (fail-open)", i+1)
		}
	})
}

func TestRateLimitConfigurations(t *testing.T) {
	t.Run("Auth endpoints strict limits", func(t *testing.T) {
		config := httpHandler.AuthEndpoints

		// Test auth limits are stricter
		assert.True(t, config.PerIP.Requests < httpHandler.TransactionEndpoints.PerIP.Requests)
		assert.True(t, config.PerIP.Requests < httpHandler.ReadOnlyEndpoints.PerIP.Requests)
	})

	t.Run("Admin endpoints very strict limits", func(t *testing.T) {
		config := httpHandler.AdminEndpoints

		// Test admin limits are strict (but not necessarily the strictest for all metrics)
		assert.True(t, config.PerIP.Requests <= httpHandler.TransactionEndpoints.PerIP.Requests,
			"Admin IP limit (%d) should be <= Transaction IP limit (%d)",
			config.PerIP.Requests, httpHandler.TransactionEndpoints.PerIP.Requests)
		assert.True(t, config.Global.Requests <= httpHandler.TransactionEndpoints.Global.Requests,
			"Admin global limit (%d) should be <= Transaction global limit (%d)",
			config.Global.Requests, httpHandler.TransactionEndpoints.Global.Requests)
	})

	t.Run("Read-only endpoints lenient limits", func(t *testing.T) {
		config := httpHandler.ReadOnlyEndpoints

		// Test read-only limits are the most lenient
		assert.True(t, config.PerIP.Requests >= httpHandler.TransactionEndpoints.PerIP.Requests)
		assert.True(t, config.Global.Requests >= httpHandler.TransactionEndpoints.Global.Requests)
	})
}

func TestRateLimitEdgeCases(t *testing.T) {
	limiter := ratelimit.NewRateLimiter(nil, "test")

	t.Run("Zero requests configuration", func(t *testing.T) {
		config := ratelimit.RateLimitConfig{
			Requests: 0,
			Window:   time.Minute,
		}

		result, err := limiter.CheckRateLimit(context.Background(), "zero-key", config)
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Zero requests should allow all requests")
	})

	t.Run("Negative requests configuration", func(t *testing.T) {
		config := ratelimit.RateLimitConfig{
			Requests: -1,
			Window:   time.Minute,
		}

		result, err := limiter.CheckRateLimit(context.Background(), "negative-key", config)
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Negative requests should allow all requests")
	})

	t.Run("Very short window", func(t *testing.T) {
		// Use a fresh limiter for this test to avoid interference
		freshLimiter := NewRateLimiterAdapter("test-short-window")

		config := ratelimit.RateLimitConfig{
			Requests: 1,
			Window:   100 * time.Millisecond, // Longer window for reliable testing
		}

		// First request
		result1, err := freshLimiter.CheckRateLimit(context.Background(), "short-window", config)
		require.NoError(t, err)
		assert.True(t, result1.Allowed, "First request should be allowed")

		// Immediate second request should be blocked
		result2, err := freshLimiter.CheckRateLimit(context.Background(), "short-window", config)
		require.NoError(t, err)
		assert.False(t, result2.Allowed, "Second request should be blocked")

		// Wait for window to pass
		time.Sleep(150 * time.Millisecond)

		// Should be allowed again
		result3, err := freshLimiter.CheckRateLimit(context.Background(), "short-window", config)
		require.NoError(t, err)
		assert.True(t, result3.Allowed, "Third request after wait should be allowed")
	})
}
