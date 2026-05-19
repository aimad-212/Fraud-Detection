package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	httpHandler "fraud-detection/internal/delivery/http"
	"fraud-detection/internal/infrastructure/audit"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockAuditLogger is a mock implementation of audit.AuditLogger
type MockAuditLogger struct {
	mock.Mock
}

func (m *MockAuditLogger) LogEvent(event audit.AuditEvent) error {
	args := m.Called(event)
	return args.Error(0)
}

func (m *MockAuditLogger) LogAuthEvent(action, userID, username, ipAddress, userAgent, status string, details map[string]interface{}) error {
	args := m.Called(action, userID, username, ipAddress, userAgent, status, details)
	return args.Error(0)
}

func (m *MockAuditLogger) LogTransactionEvent(action, userID, transactionID, ipAddress, userAgent, status string, amount float64, details map[string]interface{}) error {
	args := m.Called(action, userID, transactionID, ipAddress, userAgent, status, amount, details)
	return args.Error(0)
}

func (m *MockAuditLogger) LogAdminEvent(action, userID, username, ipAddress, userAgent, status string, resource string, details map[string]interface{}) error {
	args := m.Called(action, userID, username, ipAddress, userAgent, status, resource, details)
	return args.Error(0)
}

func (m *MockAuditLogger) LogSecurityEvent(action, userID, ipAddress, userAgent, status string, details map[string]interface{}) error {
	args := m.Called(action, userID, ipAddress, userAgent, status, details)
	return args.Error(0)
}

func (m *MockAuditLogger) LogDataAccessEvent(action, userID, resource, ipAddress, userAgent, status string, details map[string]interface{}) error {
	args := m.Called(action, userID, resource, ipAddress, userAgent, status, details)
	return args.Error(0)
}

func (m *MockAuditLogger) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestAuditMiddleware_NewAuditMiddleware(t *testing.T) {
	mockAuditLogger := &MockAuditLogger{}

	config := httpHandler.AuditMiddlewareConfig{
		SkipPaths:           []string{"/health", "/metrics"},
		MaxRequestBodySize:  512,
		MaxResponseBodySize: 512,
		LogRequestHeaders:   true,
		LogResponseHeaders:  false,
		SensitiveHeaders:    []string{"authorization"},
		SensitiveFields:     []string{"password"},
	}

	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)
	assert.NotNil(t, middleware)
}

func TestAuditMiddleware_SkipPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuditLogger := &MockAuditLogger{}
	mockAuditLogger.On("LogTransactionEvent", "GET /api/v1/transactions", "", "", mock.Anything, mock.Anything, "success", float64(0), mock.Anything).Return(nil)

	config := httpHandler.AuditMiddlewareConfig{
		SkipPaths: []string{"/health", "/metrics"},
	}

	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	router.GET("/api/v1/transactions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"transactions": []string{}})
	})

	// Test skipped path
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Test non-skipped path
	req = httptest.NewRequest("GET", "/api/v1/transactions", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Should have called audit method for non-skipped path
	mockAuditLogger.AssertExpectations(t)
}

func TestAuditMiddleware_AuthEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuditLogger := &MockAuditLogger{}
	mockAuditLogger.On("LogAuthEvent", "POST /api/v1/auth/login", "", "", mock.Anything, mock.Anything, "success", mock.Anything).Return(nil)

	config := httpHandler.AuditMiddlewareConfig{}
	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"token": "test-token"})
	})

	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(`{"username":"test","password":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockAuditLogger.AssertExpectations(t)
}

func TestAuditMiddleware_TransactionEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuditLogger := &MockAuditLogger{}
	mockAuditLogger.On("LogTransactionEvent", "POST /api/v1/transactions", "", "", mock.Anything, mock.Anything, "success", float64(0), mock.Anything).Return(nil)

	config := httpHandler.AuditMiddlewareConfig{}
	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.POST("/api/v1/transactions", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"id": "txn123", "amount": 100.50})
	})

	req := httptest.NewRequest("POST", "/api/v1/transactions", bytes.NewBufferString(`{"amount":100.50,"currency":"USD"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	mockAuditLogger.AssertExpectations(t)
}

func TestAuditMiddleware_AdminEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuditLogger := &MockAuditLogger{}
	mockAuditLogger.On("LogAdminEvent", "GET /api/v1/users", "", "", mock.Anything, mock.Anything, "success", "/api/v1/users", mock.Anything).Return(nil)

	config := httpHandler.AuditMiddlewareConfig{}
	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.GET("/api/v1/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"users": []string{}})
	})

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockAuditLogger.AssertExpectations(t)
}

func TestAuditMiddleware_SecurityEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuditLogger := &MockAuditLogger{}
	// Test DELETE on a non-transaction endpoint to trigger security event
	mockAuditLogger.On("LogSecurityEvent", "DELETE /api/v1/data/123", "", mock.Anything, mock.Anything, "success", mock.Anything).Return(nil)

	config := httpHandler.AuditMiddlewareConfig{}
	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.DELETE("/api/v1/data/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deleted": true})
	})

	req := httptest.NewRequest("DELETE", "/api/v1/data/123", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockAuditLogger.AssertExpectations(t)
}

func TestAuditMiddleware_ErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuditLogger := &MockAuditLogger{}
	mockAuditLogger.On("LogAuthEvent", "POST /api/v1/auth/login", "", "", mock.Anything, mock.Anything, "client_error", mock.Anything).Return(nil)

	config := httpHandler.AuditMiddlewareConfig{}
	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credentials"})
	})

	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(`{"username":"test","password":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockAuditLogger.AssertExpectations(t)
}

func TestAuditMiddleware_ServerErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuditLogger := &MockAuditLogger{}
	mockAuditLogger.On("LogTransactionEvent", "POST /api/v1/transactions", "", "", mock.Anything, mock.Anything, "server_error", float64(0), mock.Anything).Return(nil)

	config := httpHandler.AuditMiddlewareConfig{}
	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.POST("/api/v1/transactions", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	})

	req := httptest.NewRequest("POST", "/api/v1/transactions", bytes.NewBufferString(`{"amount":100.50}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockAuditLogger.AssertExpectations(t)
}

func TestAuditMiddleware_WithUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuditLogger := &MockAuditLogger{}
	// /api/v1/profile doesn't match any specific patterns, so it's a data access event
	mockAuditLogger.On("LogDataAccessEvent", "GET /api/v1/profile", "user123", "/api/v1/profile", mock.Anything, mock.Anything, "success", mock.Anything).Return(nil)

	config := httpHandler.AuditMiddlewareConfig{}
	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.Use(func(c *gin.Context) {
		// Simulate auth middleware setting user context
		c.Set("user_id", "user123")
		c.Set("username", "testuser")
		c.Next()
	})
	router.GET("/api/v1/profile", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user": "testuser"})
	})

	req := httptest.NewRequest("GET", "/api/v1/profile", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockAuditLogger.AssertExpectations(t)
}

func TestAuditMiddleware_RequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuditLogger := &MockAuditLogger{}
	// /api/v1/transactions is a transaction event
	mockAuditLogger.On("LogTransactionEvent", "GET /api/v1/transactions", "", "", mock.Anything, mock.Anything, "success", float64(0), mock.Anything).Return(nil)

	config := httpHandler.AuditMiddlewareConfig{}
	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.GET("/api/v1/transactions", func(c *gin.Context) {
		requestID, exists := c.Get("request_id")
		require.True(t, exists)
		assert.NotEmpty(t, requestID)
		c.JSON(http.StatusOK, gin.H{"request_id": requestID})
	})

	req := httptest.NewRequest("GET", "/api/v1/transactions", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotEmpty(t, response["request_id"])

	mockAuditLogger.AssertExpectations(t)
}

func TestAuditMiddleware_ClientIPExtraction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuditLogger := &MockAuditLogger{}

	// Test X-Forwarded-For header
	mockAuditLogger.On("LogDataAccessEvent", "GET /api/v1/test1", "", "/api/v1/test1", "192.168.1.100", mock.Anything, "success", mock.Anything).Return(nil)

	// Test X-Real-IP header
	mockAuditLogger.On("LogDataAccessEvent", "GET /api/v1/test2", "", "/api/v1/test2", "10.0.0.1", mock.Anything, "success", mock.Anything).Return(nil)

	// Test RemoteAddr fallback
	mockAuditLogger.On("LogDataAccessEvent", "GET /api/v1/test3", "", "/api/v1/test3", "127.0.0.1", mock.Anything, "success", mock.Anything).Return(nil)

	config := httpHandler.AuditMiddlewareConfig{}
	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.GET("/api/v1/test1", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{}) })
	router.GET("/api/v1/test2", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{}) })
	router.GET("/api/v1/test3", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{}) })

	// Test X-Forwarded-For
	req := httptest.NewRequest("GET", "/api/v1/test1", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1")
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Test X-Real-IP
	req = httptest.NewRequest("GET", "/api/v1/test2", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.Header.Set("User-Agent", "test-agent")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Test RemoteAddr fallback
	req = httptest.NewRequest("GET", "/api/v1/test3", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "127.0.0.1:12345"
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	mockAuditLogger.AssertExpectations(t)
}

func TestAuditMiddleware_BodyLogging(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuditLogger := &MockAuditLogger{}

	// Capture the details to verify body logging
	mockAuditLogger.On("LogTransactionEvent", "POST /api/v1/transactions", "", "", mock.Anything, mock.Anything, "success", float64(0), mock.MatchedBy(func(details map[string]interface{}) bool {
		// Check that request body is logged
		_, hasRequestBody := details["request_body"]
		return hasRequestBody
	})).Return(nil)

	config := httpHandler.AuditMiddlewareConfig{
		MaxRequestBodySize:  1024,
		MaxResponseBodySize: 1024,
	}
	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.POST("/api/v1/transactions", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"id": "txn123", "status": "created"})
	})

	requestBody := `{"amount":100.50,"currency":"USD","merchant":"Test Store"}`
	req := httptest.NewRequest("POST", "/api/v1/transactions", bytes.NewBufferString(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	mockAuditLogger.AssertExpectations(t)
}

func TestAuditMiddleware_SensitiveDataMasking(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuditLogger := &MockAuditLogger{}

	// Use a more flexible expectation - we'll check the actual call details after
	mockAuditLogger.On("LogAuthEvent", "POST /api/v1/auth/login", "", "", mock.Anything, mock.Anything, "success", mock.Anything).Return(nil)

	config := httpHandler.AuditMiddlewareConfig{
		SensitiveFields: []string{"password"},
	}
	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"token": "test-token"})
	})

	requestBody := `{"username":"testuser","password":"secret123"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify that the mock was called and check the arguments
	calls := mockAuditLogger.Calls
	assert.Len(t, calls, 1)
	assert.Equal(t, "LogAuthEvent", calls[0].Method)

	// Check the details argument for masked password
	details := calls[0].Arguments[6].(map[string]interface{})
	requestBodyLogged, ok := details["request_body"].(string)
	assert.True(t, ok)
	assert.NotContains(t, requestBodyLogged, "secret123")
	assert.Contains(t, requestBodyLogged, "[MASKED]")
}

func TestAuditMiddleware_ResponseCapture(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuditLogger := &MockAuditLogger{}

	// Capture the details to verify response body logging
	mockAuditLogger.On("LogDataAccessEvent", "GET /api/v1/test", "", "/api/v1/test", mock.Anything, mock.Anything, "success", mock.MatchedBy(func(details map[string]interface{}) bool {
		// Check that response body is logged
		if responseBody, ok := details["response_body"].(string); ok {
			return contains(responseBody, "test data")
		}
		return false
	})).Return(nil)

	config := httpHandler.AuditMiddlewareConfig{
		MaxResponseBodySize: 1024,
	}
	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.GET("/api/v1/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "test data"})
	})

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockAuditLogger.AssertExpectations(t)
}

func TestAuditMiddleware_DefaultConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAuditLogger := &MockAuditLogger{}
	mockAuditLogger.On("LogDataAccessEvent", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Test with empty config (should use defaults)
	config := httpHandler.AuditMiddlewareConfig{}
	middleware := httpHandler.NewAuditMiddleware(mockAuditLogger, config)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.GET("/api/v1/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockAuditLogger.AssertExpectations(t)
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())))
}
