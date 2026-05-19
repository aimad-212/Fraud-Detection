package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"fraud-detection/internal/infrastructure/ratelimit"

	"github.com/gin-gonic/gin"
)

// RateLimitMiddleware provides Gin middleware for rate limiting
type RateLimitMiddleware struct {
	rateLimiter ratelimit.RateLimiterInterface
	configs     map[string]ratelimit.RateLimitConfig
}

// RateLimitConfig defines rate limit configuration for different endpoints
type EndpointConfig struct {
	Global   ratelimit.RateLimitConfig // Global rate limit
	PerIP    ratelimit.RateLimitConfig // Per IP rate limit
	PerUser  ratelimit.RateLimitConfig // Per user rate limit (if authenticated)
	Endpoint string                    // Endpoint identifier
}

// NewRateLimitMiddleware creates a new rate limiting middleware
func NewRateLimitMiddleware(rateLimiter ratelimit.RateLimiterInterface) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		rateLimiter: rateLimiter,
		configs:     make(map[string]ratelimit.RateLimitConfig),
	}
}

// AddEndpointConfig adds rate limit configuration for an endpoint
func (rlm *RateLimitMiddleware) AddEndpointConfig(path string, config EndpointConfig) {
	// Store different configurations for the endpoint
	rlm.configs[path+"_global"] = config.Global
	rlm.configs[path+"_ip"] = config.PerIP
	rlm.configs[path+"_user"] = config.PerUser
}

// SetDefaultConfig sets default rate limit configuration
func (rlm *RateLimitMiddleware) SetDefaultConfig(config EndpointConfig) {
	rlm.AddEndpointConfig("default", config)
}

// RateLimit creates a middleware function for rate limiting
func (rlm *RateLimitMiddleware) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// Get configuration for this endpoint
		config := rlm.getEndpointConfig(path)

		// Get client IP
		clientIP := rlm.getClientIP(c)

		// Get user ID if authenticated
		userID := rlm.getUserID(c)

		// Check multiple rate limits
		limits := []struct {
			name   string
			key    string
			config ratelimit.RateLimitConfig
		}{
			{"global", "global", config.Global},
			{"ip", "ip:" + clientIP, config.PerIP},
		}

		// Add user-based limit if authenticated
		if userID != "" {
			limits = append(limits, struct {
				name   string
				key    string
				config ratelimit.RateLimitConfig
			}{"user", "user:" + userID, config.PerUser})
		}

		// Check all rate limits
		for _, limit := range limits {
			if limit.config.Requests <= 0 {
				continue // Skip if not configured
			}

			result, err := rlm.rateLimiter.CheckRateLimit(c.Request.Context(), limit.key, limit.config)
			if err != nil {
				// Log error but allow request (fail-open)
				c.Header("X-Rate-Limit-Error", "rate_limit_service_error")
				continue
			}

			// Set rate limit headers
			rlm.setRateLimitHeaders(c, result, limit.name)

			// Check if request is allowed
			if !result.Allowed {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":       "Rate limit exceeded",
					"limit":       result.Limit,
					"remaining":   result.Remaining,
					"reset_time":  result.ResetTime.Unix(),
					"retry_after": result.RetryAfter.Seconds(),
					"limit_type":  limit.name,
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// getEndpointConfig gets rate limit configuration for an endpoint
func (rlm *RateLimitMiddleware) getEndpointConfig(path string) EndpointConfig {
	// Try to find exact match
	if global, exists := rlm.configs[path+"_global"]; exists {
		return EndpointConfig{
			Global:  global,
			PerIP:   rlm.configs[path+"_ip"],
			PerUser: rlm.configs[path+"_user"],
		}
	}

	// Try to find pattern match
	for configPath := range rlm.configs {
		if strings.HasSuffix(configPath, "_global") {
			pattern := strings.TrimSuffix(configPath, "_global")
			if rlm.pathMatches(path, pattern) {
				return EndpointConfig{
					Global:  rlm.configs[configPath],
					PerIP:   rlm.configs[pattern+"_ip"],
					PerUser: rlm.configs[pattern+"_user"],
				}
			}
		}
	}

	// Return default configuration
	return rlm.getDefaultConfig()
}

// getDefaultConfig returns default rate limit configuration
func (rlm *RateLimitMiddleware) getDefaultConfig() EndpointConfig {
	defaultConfig := EndpointConfig{
		Global: ratelimit.RateLimitConfig{
			Requests: 1000, // 1000 requests per hour globally
			Window:   time.Hour,
		},
		PerIP: ratelimit.RateLimitConfig{
			Requests: 100, // 100 requests per minute per IP
			Window:   time.Minute,
		},
		PerUser: ratelimit.RateLimitConfig{
			Requests: 500, // 500 requests per minute per user
			Window:   time.Minute,
		},
	}

	// Override with configured defaults if available
	if global, exists := rlm.configs["default_global"]; exists {
		defaultConfig.Global = global
	}
	if perIP, exists := rlm.configs["default_ip"]; exists {
		defaultConfig.PerIP = perIP
	}
	if perUser, exists := rlm.configs["default_user"]; exists {
		defaultConfig.PerUser = perUser
	}

	return defaultConfig
}

// pathMatches checks if a path matches a pattern (supports wildcards)
func (rlm *RateLimitMiddleware) pathMatches(path, pattern string) bool {
	// Simple exact match
	if path == pattern {
		return true
	}

	// Wildcard matching (basic implementation)
	if strings.Contains(pattern, "*") {
		// Convert pattern to regex-like pattern
		regexPattern := strings.ReplaceAll(pattern, "*", ".*")
		return strings.HasPrefix(path, strings.TrimSuffix(regexPattern, ".*"))
	}

	return false
}

// getClientIP extracts the real client IP address
func (rlm *RateLimitMiddleware) getClientIP(c *gin.Context) string {
	// Check X-Forwarded-For header
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Check X-Real-IP header
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return c.ClientIP()
}

// getUserID extracts user ID from JWT token context
func (rlm *RateLimitMiddleware) getUserID(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(string); ok {
			return id
		}
	}
	return ""
}

// setRateLimitHeaders sets rate limit related headers
func (rlm *RateLimitMiddleware) setRateLimitHeaders(c *gin.Context, result *ratelimit.RateLimitResult, limitType string) {
	prefix := "X-Rate-Limit"
	if limitType != "global" {
		prefix = "X-Rate-Limit-" + strings.Title(limitType)
	}

	c.Header(prefix+"-Limit", strconv.Itoa(result.Limit))
	c.Header(prefix+"-Remaining", strconv.Itoa(result.Remaining))
	c.Header(prefix+"-Reset", strconv.FormatInt(result.ResetTime.Unix(), 10))

	if !result.Allowed {
		c.Header("Retry-After", strconv.FormatInt(int64(result.RetryAfter.Seconds()), 10))
	}
}

// Predefined rate limit configurations
var (
	// AuthEndpoints - stricter limits for authentication endpoints
	AuthEndpoints = EndpointConfig{
		Global: ratelimit.RateLimitConfig{
			Requests: 100, // 100 requests per minute globally
			Window:   time.Minute,
		},
		PerIP: ratelimit.RateLimitConfig{
			Requests: 10, // 10 requests per minute per IP
			Window:   time.Minute,
		},
		PerUser: ratelimit.RateLimitConfig{
			Requests: 20, // 20 requests per minute per user
			Window:   time.Minute,
		},
	}

	// TransactionEndpoints - moderate limits for transaction processing
	TransactionEndpoints = EndpointConfig{
		Global: ratelimit.RateLimitConfig{
			Requests: 1000, // 1000 requests per minute globally
			Window:   time.Minute,
		},
		PerIP: ratelimit.RateLimitConfig{
			Requests: 100, // 100 requests per minute per IP
			Window:   time.Minute,
		},
		PerUser: ratelimit.RateLimitConfig{
			Requests: 200, // 200 requests per minute per user
			Window:   time.Minute,
		},
	}

	// ReadOnlyEndpoints - lenient limits for read-only operations
	ReadOnlyEndpoints = EndpointConfig{
		Global: ratelimit.RateLimitConfig{
			Requests: 5000, // 5000 requests per minute globally
			Window:   time.Minute,
		},
		PerIP: ratelimit.RateLimitConfig{
			Requests: 500, // 500 requests per minute per IP
			Window:   time.Minute,
		},
		PerUser: ratelimit.RateLimitConfig{
			Requests: 1000, // 1000 requests per minute per user
			Window:   time.Minute,
		},
	}

	// AdminEndpoints - strict limits for admin operations
	AdminEndpoints = EndpointConfig{
		Global: ratelimit.RateLimitConfig{
			Requests: 50, // 50 requests per minute globally
			Window:   time.Minute,
		},
		PerIP: ratelimit.RateLimitConfig{
			Requests: 20, // 20 requests per minute per IP
			Window:   time.Minute,
		},
		PerUser: ratelimit.RateLimitConfig{
			Requests: 30, // 30 requests per minute per user
			Window:   time.Minute,
		},
	}
)
