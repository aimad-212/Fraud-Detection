package http

import (
	"strconv"

	"fraud-detection/internal/config"
	"github.com/gin-gonic/gin"
)

// SecurityMiddleware provides security headers middleware
type SecurityMiddleware struct {
	config config.SecurityConfig
}

// NewSecurityMiddleware creates a new security middleware
func NewSecurityMiddleware(config config.SecurityConfig) *SecurityMiddleware {
	return &SecurityMiddleware{
		config: config,
	}
}

// Middleware returns the Gin middleware function
func (m *SecurityMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.Enabled {
			c.Next()
			return
		}

		// HTTP Strict Transport Security (HSTS)
		if m.config.HSTSEnabled {
			maxAge := m.config.HSTSMaxAge
			if maxAge == 0 {
				maxAge = 31536000 // 1 year default
			}

			hstsValue := "max-age=" + strconv.Itoa(maxAge)
			if m.config.HSTSIncludeSubdomains {
				hstsValue += "; includeSubDomains"
			}
			if m.config.HSTSPreload {
				hstsValue += "; preload"
			}
			c.Header("Strict-Transport-Security", hstsValue)
		}

		// Content Security Policy (CSP)
		if m.config.CSPEnabled {
			csp := m.config.CSPPolicy
			if csp == "" {
				// Default CSP policy for REST API
				csp = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; sandbox"
			}
			c.Header("Content-Security-Policy", csp)
		}

		// X-Frame-Options
		if m.config.XFrameOptions != "" {
			c.Header("X-Frame-Options", m.config.XFrameOptions)
		} else {
			c.Header("X-Frame-Options", "DENY")
		}

		// X-Content-Type-Options
		if m.config.XContentTypeOptions != "" {
			c.Header("X-Content-Type-Options", m.config.XContentTypeOptions)
		} else {
			c.Header("X-Content-Type-Options", "nosniff")
		}

		// X-XSS-Protection
		if m.config.XXSSProtection != "" {
			c.Header("X-XSS-Protection", m.config.XXSSProtection)
		} else {
			c.Header("X-XSS-Protection", "1; mode=block")
		}

		// Referrer Policy
		if m.config.ReferrerPolicy != "" {
			c.Header("Referrer-Policy", m.config.ReferrerPolicy)
		} else {
			c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		}

		// Permissions Policy
		if m.config.PermissionsPolicy != "" {
			c.Header("Permissions-Policy", m.config.PermissionsPolicy)
		}

		// Cross-Origin Embedder Policy
		if m.config.COEPEnabled {
			value := "require-corp"
			if m.config.COEPReportOnly {
				value = "report-to"
			}
			c.Header("Cross-Origin-Embedder-Policy", value)
		}

		// Cross-Origin Resource Policy
		if m.config.CORPEnabled {
			value := "same-origin"
			if m.config.CORPReportOnly {
				value = "report-to"
			}
			c.Header("Cross-Origin-Resource-Policy", value)
		}

		// Cross-Origin Opener Policy
		if m.config.COOPEnabled {
			value := "same-origin"
			if m.config.COOPReportOnly {
				value = "report-to"
			}
			c.Header("Cross-Origin-Opener-Policy", value)
		}

		// Remove server information
		if m.config.RemoveServerHeader {
			c.Header("Server", "")
		}

		// Cache control for API endpoints
		if m.config.CacheControlEnabled {
			cacheControl := m.config.CacheControlValue
			if cacheControl == "" {
				cacheControl = "no-store, no-cache, must-revalidate, proxy-revalidate"
			}
			c.Header("Cache-Control", cacheControl)
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}

		c.Next()
	}
}
