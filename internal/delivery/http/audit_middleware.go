package http

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"fraud-detection/internal/infrastructure/audit"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuditMiddleware provides audit logging for HTTP requests
type AuditMiddleware struct {
	auditLogger audit.AuditLogger
	config      AuditMiddlewareConfig
}

// AuditMiddlewareConfig holds configuration for audit middleware
type AuditMiddlewareConfig struct {
	// Skip paths that don't need audit logging
	SkipPaths []string
	// Request body size limit for logging (in bytes)
	MaxRequestBodySize int64
	// Response body size limit for logging (in bytes)
	MaxResponseBodySize int64
	// Log request headers
	LogRequestHeaders bool
	// Log response headers
	LogResponseHeaders bool
	// Sensitive headers to mask
	SensitiveHeaders []string
	// Sensitive body fields to mask
	SensitiveFields []string
}

// NewAuditMiddleware creates a new audit middleware
func NewAuditMiddleware(auditLogger audit.AuditLogger, config AuditMiddlewareConfig) *AuditMiddleware {
	// Set defaults
	if config.MaxRequestBodySize == 0 {
		config.MaxRequestBodySize = 1024 // 1KB
	}
	if config.MaxResponseBodySize == 0 {
		config.MaxResponseBodySize = 1024 // 1KB
	}
	if len(config.SensitiveHeaders) == 0 {
		config.SensitiveHeaders = []string{
			"authorization", "cookie", "set-cookie", "x-api-key",
			"x-auth-token", "x-session-id", "password", "token",
		}
	}
	if len(config.SensitiveFields) == 0 {
		config.SensitiveFields = []string{
			"password", "token", "secret", "key", "credit_card",
			"ssn", "social_security", "bank_account", "cvv",
		}
	}

	return &AuditMiddleware{
		auditLogger: auditLogger,
		config:      config,
	}
}

// Middleware returns the Gin middleware function
func (m *AuditMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip audit logging for specified paths
		if m.shouldSkipPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		startTime := time.Now()
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)

		// Read request body for logging
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(io.LimitReader(c.Request.Body, m.config.MaxRequestBodySize))
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// Get client info
		clientIP := m.getClientIP(c)
		userAgent := c.GetHeader("User-Agent")

		// Create response writer wrapper to capture response
		responseWriter := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = responseWriter

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(startTime)

		// Get response body for logging
		var responseBody []byte
		if responseWriter.body.Len() > 0 {
			responseBody, _ = io.ReadAll(io.LimitReader(responseWriter.body, m.config.MaxResponseBodySize))
		}

		// Get user info from context (set by auth middleware) - extract after c.Next()
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")

		// Determine event type and action
		eventType, action := m.determineEventType(c.Request.Method, c.Request.URL.Path)

		// Create audit details
		details := m.createAuditDetails(c, requestBody, responseBody, duration)

		// Log the audit event
		m.logAuditEvent(c, eventType, action, requestID, clientIP, userAgent, userID, username, c.Writer.Status(), details)
	}
}

// shouldSkipPath checks if the path should be skipped from audit logging
func (m *AuditMiddleware) shouldSkipPath(path string) bool {
	for _, skipPath := range m.config.SkipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}
	return false
}

// getClientIP extracts the real client IP address
func (m *AuditMiddleware) getClientIP(c *gin.Context) string {
	// Check X-Forwarded-For header
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	return c.ClientIP()
}

// determineEventType determines the audit event type and action based on HTTP method and path
func (m *AuditMiddleware) determineEventType(method, path string) (audit.EventType, string) {
	// Authentication events
	if strings.Contains(path, "/auth/") || strings.Contains(path, "/login") || strings.Contains(path, "/register") {
		return audit.EventTypeAuth, method + " " + path
	}

	// Transaction events
	if strings.Contains(path, "/transactions") {
		return audit.EventTypeTransaction, method + " " + path
	}

	// Admin events
	if strings.Contains(path, "/admin") || strings.Contains(path, "/users") {
		return audit.EventTypeAdmin, method + " " + path
	}

	// Security events
	if strings.Contains(path, "/security") || method == "DELETE" {
		return audit.EventTypeSecurity, method + " " + path
	}

	// Default to data access
	return audit.EventTypeDataAccess, method + " " + path
}

// createAuditDetails creates the details map for the audit event
func (m *AuditMiddleware) createAuditDetails(c *gin.Context, requestBody, responseBody []byte, duration time.Duration) map[string]interface{} {
	details := make(map[string]interface{})

	// Request details
	details["method"] = c.Request.Method
	details["path"] = c.Request.URL.Path
	details["query"] = c.Request.URL.RawQuery
	details["duration_ms"] = duration.Milliseconds()

	// Add request headers if enabled
	if m.config.LogRequestHeaders {
		headers := make(map[string]string)
		for key, values := range c.Request.Header {
			if !m.isSensitiveHeader(key) {
				headers[key] = strings.Join(values, ", ")
			} else {
				headers[key] = "[MASKED]"
			}
		}
		details["request_headers"] = headers
	}

	// Add request body if present
	if len(requestBody) > 0 {
		details["request_body"] = m.maskSensitiveData(string(requestBody))
	}

	// Add response headers if enabled
	if m.config.LogResponseHeaders {
		headers := make(map[string]string)
		for key, values := range c.Writer.Header() {
			if !m.isSensitiveHeader(key) {
				headers[key] = strings.Join(values, ", ")
			} else {
				headers[key] = "[MASKED]"
			}
		}
		details["response_headers"] = headers
	}

	// Add response body if present
	if len(responseBody) > 0 {
		details["response_body"] = m.maskSensitiveData(string(responseBody))
	}

	return details
}

// isSensitiveHeader checks if a header contains sensitive information
func (m *AuditMiddleware) isSensitiveHeader(header string) bool {
	headerLower := strings.ToLower(header)
	for _, sensitive := range m.config.SensitiveHeaders {
		if strings.Contains(headerLower, strings.ToLower(sensitive)) {
			return true
		}
	}
	return false
}

// maskSensitiveData masks sensitive data in the request/response body
func (m *AuditMiddleware) maskSensitiveData(data string) string {
	for _, field := range m.config.SensitiveFields {
		// Simple masking - look for patterns like "password":"value"
		// Use case-insensitive matching
		pattern := fmt.Sprintf(`(?i)"%s":\s*"[^"]*"`, regexp.QuoteMeta(field))
		re := regexp.MustCompile(pattern)
		if re.MatchString(data) {
			// Replace the value with [MASKED]
			data = re.ReplaceAllString(data, fmt.Sprintf(`"%s":"[MASKED]"`, field))
		}
	}
	return data
}

// logAuditEvent logs the appropriate audit event based on event type
func (m *AuditMiddleware) logAuditEvent(c *gin.Context, eventType audit.EventType, action, requestID, clientIP, userAgent string, userID, username any, statusCode int, details map[string]interface{}) {
	// Convert user info to strings
	var userIDStr, usernameStr string
	if uid, ok := userID.(string); ok {
		userIDStr = uid
	}
	if uname, ok := username.(string); ok {
		usernameStr = uname
	}

	// Determine status based on HTTP status code
	status := "success"
	if statusCode >= 400 && statusCode < 500 {
		status = "client_error"
	} else if statusCode >= 500 {
		status = "server_error"
	}

	// Add status code to details
	details["status_code"] = statusCode
	details["request_id"] = requestID

	// Log based on event type
	switch eventType {
	case audit.EventTypeAuth:
		m.auditLogger.LogAuthEvent(action, userIDStr, usernameStr, clientIP, userAgent, status, details)
	case audit.EventTypeTransaction:
		// Extract transaction ID and amount if available
		var transactionID string
		var amount float64
		if tid, exists := details["transaction_id"]; exists {
			if tidStr, ok := tid.(string); ok {
				transactionID = tidStr
			}
		}
		if amt, exists := details["amount"]; exists {
			if amtFloat, ok := amt.(float64); ok {
				amount = amtFloat
			}
		}
		m.auditLogger.LogTransactionEvent(action, userIDStr, transactionID, clientIP, userAgent, status, amount, details)
	case audit.EventTypeAdmin:
		m.auditLogger.LogAdminEvent(action, userIDStr, usernameStr, clientIP, userAgent, status, c.Request.URL.Path, details)
	case audit.EventTypeSecurity:
		m.auditLogger.LogSecurityEvent(action, userIDStr, clientIP, userAgent, status, details)
	case audit.EventTypeDataAccess:
		m.auditLogger.LogDataAccessEvent(action, userIDStr, c.Request.URL.Path, clientIP, userAgent, status, details)
	}
}

// responseBodyWriter is a wrapper around gin.ResponseWriter to capture response body
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r *responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}
