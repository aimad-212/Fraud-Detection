package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpHandler "fraud-detection/internal/delivery/http"
	"fraud-detection/internal/domain"
	"fraud-detection/internal/infrastructure/auth"
	"fraud-detection/internal/repository"
	"fraud-detection/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogger implements Logger interface for testing
type TestLogger struct{}

func (l *TestLogger) Error(msg string, args ...interface{}) {}
func (l *TestLogger) Warn(msg string, args ...interface{})  {}
func (l *TestLogger) Info(msg string, args ...interface{})  {}

func TestJWTAuthentication(t *testing.T) {
	// Initialize JWT service
	jwtConfig := auth.JWTConfig{
		SecretKey:          "test-secret-key-that-is-long-enough-for-testing",
		Issuer:             "test-service",
		AccessTokenExpiry:  15 * time.Minute,   // 15 minutes
		RefreshTokenExpiry: 7 * 24 * time.Hour, // 7 days
	}

	jwtService, err := auth.NewJWTService(jwtConfig)
	require.NoError(t, err)

	// Initialize user repository and service
	userRepo := repository.NewInMemoryUserRepository()
	logger := &TestLogger{}
	userService := usecase.NewUserService(userRepo, jwtService, logger)

	// Create default admin user
	err = userRepo.CreateDefaultAdmin()
	require.NoError(t, err)

	// Initialize handlers
	userHandler := httpHandler.NewUserHandler(userService, logger)
	authMiddleware := httpHandler.NewAuthMiddleware(jwtService)

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Test login
	t.Run("Login with valid credentials", func(t *testing.T) {
		router := gin.New()
		router.POST("/auth/login", userHandler.Login)

		loginReq := domain.LoginRequest{
			Username: "admin",
			Password: "Admin123!",
		}

		jsonData, _ := json.Marshal(loginReq)
		req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response domain.LoginResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.AccessToken)
		assert.NotEmpty(t, response.RefreshToken)
		assert.Equal(t, "Bearer", response.TokenType)
		assert.Equal(t, "admin", response.User.Username)
	})

	t.Run("Login with invalid credentials", func(t *testing.T) {
		router := gin.New()
		router.POST("/auth/login", userHandler.Login)

		loginReq := domain.LoginRequest{
			Username: "admin",
			Password: "wrongpassword",
		}

		jsonData, _ := json.Marshal(loginReq)
		req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Access protected endpoint with valid token", func(t *testing.T) {
		// Test JWT validation directly first
		// First, login to get a token
		router := gin.New()
		router.POST("/auth/login", userHandler.Login)

		loginReq := domain.LoginRequest{
			Username: "admin",
			Password: "Admin123!",
		}

		jsonData, _ := json.Marshal(loginReq)
		req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Login should succeed")

		var loginResponse domain.LoginResponse
		err := json.Unmarshal(w.Body.Bytes(), &loginResponse)
		require.NoError(t, err)

		t.Logf("Access token: %s", loginResponse.AccessToken)
		t.Logf("Token type: %s", loginResponse.TokenType)
		t.Logf("User: %s", loginResponse.User.Username)

		// Test direct JWT validation
		claims, err := jwtService.ValidateToken(loginResponse.AccessToken)
		t.Logf("Direct validation result: %v, error: %v", claims, err)
		if err != nil {
			t.Logf("Validation error: %s", err.Error())
		}
		if claims != nil {
			t.Logf("Claims UserID: %s, Username: %s", claims.UserID, claims.Username)
		}

		// Now access protected endpoint
		protectedRouter := gin.New()
		protectedRouter.GET("/profile", authMiddleware.RequireAuth(), userHandler.GetProfile)

		protectedReq, _ := http.NewRequest("GET", "/profile", nil)
		protectedReq.Header.Set("Authorization", "Bearer "+loginResponse.AccessToken)

		protectedW := httptest.NewRecorder()
		protectedRouter.ServeHTTP(protectedW, protectedReq)

		t.Logf("Protected endpoint response code: %d", protectedW.Code)
		t.Logf("Protected endpoint response body: %s", protectedW.Body.String())

		assert.Equal(t, http.StatusOK, protectedW.Code)

		var profileResponse domain.UserInfo
		err = json.Unmarshal(protectedW.Body.Bytes(), &profileResponse)
		require.NoError(t, err)

		assert.Equal(t, "admin", profileResponse.Username)
	})

	t.Run("Access protected endpoint without token", func(t *testing.T) {
		router := gin.New()
		router.GET("/profile", authMiddleware.RequireAuth(), userHandler.GetProfile)

		req, _ := http.NewRequest("GET", "/profile", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Access protected endpoint with invalid token", func(t *testing.T) {
		router := gin.New()
		router.GET("/profile", authMiddleware.RequireAuth(), userHandler.GetProfile)

		req, _ := http.NewRequest("GET", "/profile", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Role-based access control", func(t *testing.T) {
		// First, login as admin
		router := gin.New()
		router.POST("/auth/login", userHandler.Login)

		loginReq := domain.LoginRequest{
			Username: "admin",
			Password: "Admin123!",
		}

		jsonData, _ := json.Marshal(loginReq)
		req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var loginResponse domain.LoginResponse
		err := json.Unmarshal(w.Body.Bytes(), &loginResponse)
		require.NoError(t, err)

		// Test admin-only endpoint
		adminRouter := gin.New()
		adminRouter.GET("/admin/users", authMiddleware.RequireAuth(), authMiddleware.RequireRole(string(domain.RoleAdmin)), userHandler.ListUsers)

		adminReq, _ := http.NewRequest("GET", "/admin/users", nil)
		adminReq.Header.Set("Authorization", "Bearer "+loginResponse.AccessToken)

		adminW := httptest.NewRecorder()
		adminRouter.ServeHTTP(adminW, adminReq)

		assert.Equal(t, http.StatusOK, adminW.Code)
	})
}
