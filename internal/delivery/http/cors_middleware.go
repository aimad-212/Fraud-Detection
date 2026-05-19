package http

import (
	"net/http"
	"strconv"

	"fraud-detection/internal/config"
	"github.com/gin-gonic/gin"
)

// CorsMiddleware provides CORS middleware
type CorsMiddleware struct {
	config config.CORSConfig
}

// CorsConfig holds CORS configuration
type CorsConfig struct {
	Enabled          bool     `json:"enabled"`
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	ExposedHeaders   []string `json:"exposed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           int      `json:"max_age_seconds"`
}

// NewCorsMiddleware creates a new CORS middleware
func NewCorsMiddleware(config config.CORSConfig) *CorsMiddleware {
	// Set defaults
	if len(config.AllowedOrigins) == 0 {
		config.AllowedOrigins = []string{"*"}
	}
	if len(config.AllowedMethods) == 0 {
		config.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	}
	if len(config.AllowedHeaders) == 0 {
		config.AllowedHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	}
	if config.MaxAge == 0 {
		config.MaxAge = 86400 // 24 hours
	}

	return &CorsMiddleware{
		config: config,
	}
}

// Middleware returns the Gin middleware function
func (m *CorsMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.Enabled {
			c.Next()
			return
		}

		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		if m.isOriginAllowed(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
		} else if len(m.config.AllowedOrigins) == 1 && m.config.AllowedOrigins[0] == "*" {
			c.Header("Access-Control-Allow-Origin", "*")
		}

		// Set other CORS headers
		c.Header("Access-Control-Allow-Methods", m.joinMethods())
		c.Header("Access-Control-Allow-Headers", m.joinHeaders())

		if len(m.config.ExposedHeaders) > 0 {
			c.Header("Access-Control-Expose-Headers", m.joinExposedHeaders())
		}

		if m.config.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		c.Header("Access-Control-Max-Age", strconv.Itoa(m.config.MaxAge))

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// isOriginAllowed checks if the origin is in the allowed list
func (m *CorsMiddleware) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}

	for _, allowedOrigin := range m.config.AllowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			return true
		}
	}
	return false
}

// joinMethods joins allowed methods into a comma-separated string
func (m *CorsMiddleware) joinMethods() string {
	return m.joinStringSlice(m.config.AllowedMethods)
}

// joinHeaders joins allowed headers into a comma-separated string
func (m *CorsMiddleware) joinHeaders() string {
	return m.joinStringSlice(m.config.AllowedHeaders)
}

// joinExposedHeaders joins exposed headers into a comma-separated string
func (m *CorsMiddleware) joinExposedHeaders() string {
	return m.joinStringSlice(m.config.ExposedHeaders)
}

// joinStringSlice is a helper to join string slices
func (m *CorsMiddleware) joinStringSlice(slice []string) string {
	if len(slice) == 0 {
		return ""
	}

	result := slice[0]
	for i := 1; i < len(slice); i++ {
		result += ", " + slice[i]
	}
	return result
}
