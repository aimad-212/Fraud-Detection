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

func TestSecurityMiddleware(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name            string
		config          config.SecurityConfig
		expectedHeaders map[string]string
		description     string
	}{
		{
			name: "Security disabled",
			config: config.SecurityConfig{
				Enabled: false,
			},
			expectedHeaders: map[string]string{},
			description:     "When security is disabled, no security headers should be set",
		},
		{
			name: "Security enabled with defaults",
			config: config.SecurityConfig{
				Enabled:               true,
				HSTSEnabled:           true,
				HSTSMaxAge:            31536000,
				HSTSIncludeSubdomains: false,
				HSTSPreload:           false,
				CSPEnabled:            true,
				CSPPolicy:             "default-src 'self'; script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; sandbox",
				XFrameOptions:         "DENY",
				XContentTypeOptions:   "nosniff",
				XXSSProtection:        "1; mode=block",
				ReferrerPolicy:        "strict-origin-when-cross-origin",
				PermissionsPolicy:     "geolocation=(), microphone=(), camera=()",
				COEPEnabled:           false,
				COEPReportOnly:        false,
				CORPEnabled:           false,
				CORPReportOnly:        false,
				COOPEnabled:           false,
				COOPReportOnly:        false,
				RemoveServerHeader:    true,
				CacheControlEnabled:   true,
				CacheControlValue:     "no-store, no-cache, must-revalidate, proxy-revalidate",
			},
			expectedHeaders: map[string]string{
				"Strict-Transport-Security": "max-age=31536000",
				"Content-Security-Policy":   "default-src 'self'; script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; sandbox",
				"X-Frame-Options":           "DENY",
				"X-Content-Type-Options":    "nosniff",
				"X-XSS-Protection":          "1; mode=block",
				"Referrer-Policy":           "strict-origin-when-cross-origin",
				"Permissions-Policy":        "geolocation=(), microphone=(), camera=()",
				"Cache-Control":             "no-store, no-cache, must-revalidate, proxy-revalidate",
				"Pragma":                    "no-cache",
				"Expires":                   "0",
			},
			description: "When security is enabled, default security headers should be set",
		},
		{
			name: "Custom security headers",
			config: config.SecurityConfig{
				Enabled:               true,
				HSTSEnabled:           true,
				HSTSMaxAge:            2592000, // 30 days
				HSTSIncludeSubdomains: true,
				HSTSPreload:           true,
				CSPEnabled:            true,
				CSPPolicy:             "default-src 'self'; script-src 'self' https://cdn.example.com",
				XFrameOptions:         "SAMEORIGIN",
				XContentTypeOptions:   "nosniff",
				XXSSProtection:        "1; mode=block",
				ReferrerPolicy:        "no-referrer",
				RemoveServerHeader:    true,
				CacheControlEnabled:   true,
				CacheControlValue:     "public, max-age=3600",
			},
			expectedHeaders: map[string]string{
				"Strict-Transport-Security": "max-age=2592000; includeSubDomains; preload",
				"Content-Security-Policy":   "default-src 'self'; script-src 'self' https://cdn.example.com",
				"X-Frame-Options":           "SAMEORIGIN",
				"X-Content-Type-Options":    "nosniff",
				"X-XSS-Protection":          "1; mode=block",
				"Referrer-Policy":           "no-referrer",
				"Cache-Control":             "public, max-age=3600",
				"Pragma":                    "no-cache",
				"Expires":                   "0",
			},
			description: "Custom security header values should be respected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create Gin router with security middleware
			router := gin.New()
			securityMiddleware := httpHandler.NewSecurityMiddleware(tt.config)
			router.Use(securityMiddleware.Middleware())

			// Add a test endpoint
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "test"})
			})

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			// Perform request
			router.ServeHTTP(w, req)

			// Check status
			assert.Equal(t, http.StatusOK, w.Code, tt.description)

			// Check security headers
			for header, expectedValue := range tt.expectedHeaders {
				actualValue := w.Header().Get(header)
				assert.Equal(t, expectedValue, actualValue,
					"Header %s should match expected value. %s", header, tt.description)
			}

			// Check that server header is removed when configured
			if tt.config.RemoveServerHeader {
				assert.Empty(t, w.Header().Get("Server"), tt.description)
			}
		})
	}
}
