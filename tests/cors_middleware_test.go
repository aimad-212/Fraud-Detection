package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"fraud-detection/internal/config"
	httpHandler "fraud-detection/internal/delivery/http"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCorsMiddleware(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		config         config.CORSConfig
		origin         string
		method         string
		expectedStatus int
		expectedOrigin string
		description    string
	}{
		{
			name: "CORS disabled",
			config: config.CORSConfig{
				Enabled: false,
			},
			origin:         "https://example.com",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedOrigin: "",
			description:    "When CORS is disabled, no CORS headers should be set",
		},
		{
			name: "Allowed origin",
			config: config.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"GET", "POST"},
				AllowedHeaders: []string{"Content-Type"},
			},
			origin:         "https://example.com",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedOrigin: "https://example.com",
			description:    "Allowed origin should receive CORS headers",
		},
		{
			name: "Disallowed origin",
			config: config.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"GET", "POST"},
				AllowedHeaders: []string{"Content-Type"},
			},
			origin:         "https://malicious-site.com",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedOrigin: "",
			description:    "Disallowed origin should not receive Access-Control-Allow-Origin header",
		},
		{
			name: "Wildcard origin",
			config: config.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{"GET", "POST"},
				AllowedHeaders: []string{"Content-Type"},
			},
			origin:         "https://any-site.com",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedOrigin: "https://any-site.com",
			description:    "Wildcard origin should allow any origin and return the specific origin",
		},
		{
			name: "Preflight request",
			config: config.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"GET", "POST"},
				AllowedHeaders: []string{"Content-Type"},
			},
			origin:         "https://example.com",
			method:         "OPTIONS",
			expectedStatus: http.StatusNoContent,
			expectedOrigin: "https://example.com",
			description:    "Preflight requests should return 204 No Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create Gin router with CORS middleware
			router := gin.New()
			corsMiddleware := httpHandler.NewCorsMiddleware(tt.config)
			router.Use(corsMiddleware.Middleware())

			// Add a test endpoint
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "test"})
			})

			// Create request
			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.method == "OPTIONS" {
				req.Header.Set("Access-Control-Request-Method", "POST")
				req.Header.Set("Access-Control-Request-Headers", "Content-Type")
			}

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			router.ServeHTTP(w, req)

			// Check status
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Check CORS headers
			if tt.config.Enabled {
				if tt.expectedOrigin != "" {
					assert.Equal(t, tt.expectedOrigin, w.Header().Get("Access-Control-Allow-Origin"), tt.description)
				} else {
					assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"), tt.description)
				}

				// Check other CORS headers are present when enabled
				assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"), tt.description)
				assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"), tt.description)
				assert.NotEmpty(t, w.Header().Get("Access-Control-Max-Age"), tt.description)
			} else {
				// When disabled, no CORS headers should be present
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"), tt.description)
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Methods"), tt.description)
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Headers"), tt.description)
			}
		})
	}
}

func TestCorsMiddlewareDefaults(t *testing.T) {
	// Test that default values are set correctly
	config := config.CORSConfig{
		Enabled: true,
	}

	middleware := httpHandler.NewCorsMiddleware(config)

	// Create a test request to trigger defaults
	router := gin.New()
	router.Use(middleware.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check that defaults are applied (should return the specific origin when there's a match)
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
	assert.Equal(t, "86400", w.Header().Get("Access-Control-Max-Age"))
}
