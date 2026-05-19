package http

import (
	"net/http"
	"strconv"

	"fraud-detection/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	userService domain.UserService
	logger      Logger
}

// Logger interface for user handler
type Logger interface {
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Info(msg string, args ...interface{})
}

// NewUserHandler creates a new user handler
func NewUserHandler(userService domain.UserService, logger Logger) *UserHandler {
	return &UserHandler{
		userService: userService,
		logger:      logger,
	}
}

// Register handles user registration
func (uh *UserHandler) Register(c *gin.Context) {
	var req domain.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	user, err := uh.userService.Register(&req)
	if err != nil {
		if validationErr, ok := err.(*domain.ValidationError); ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": validationErr.Error(),
			})
			return
		}

		uh.logger.Error("Failed to register user", "error", err, "username", req.Username)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create user",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"user":    user.ToUserInfo(),
	})
}

// Login handles user authentication
func (uh *UserHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	response, err := uh.userService.Login(&req)
	if err != nil {
		uh.logger.Warn("Login failed", "error", err, "username", req.Username)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid credentials",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// RefreshToken handles token refresh
func (uh *UserHandler) RefreshToken(c *gin.Context) {
	var req domain.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	response, err := uh.userService.RefreshToken(&req)
	if err != nil {
		uh.logger.Warn("Token refresh failed", "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid refresh token",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetProfile handles getting user profile
func (uh *UserHandler) GetProfile(c *gin.Context) {
	userIDStr, exists := GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	profile, err := uh.userService.GetProfile(userID)
	if err != nil {
		uh.logger.Error("Failed to get user profile", "error", err, "user_id", userID)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// UpdateProfile handles updating user profile
func (uh *UserHandler) UpdateProfile(c *gin.Context) {
	userIDStr, exists := GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	var req domain.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	user, err := uh.userService.UpdateProfile(userID, &req)
	if err != nil {
		if validationErr, ok := err.(*domain.ValidationError); ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": validationErr.Error(),
			})
			return
		}

		uh.logger.Error("Failed to update user profile", "error", err, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update profile",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile updated successfully",
		"user":    user.ToUserInfo(),
	})
}

// ChangePassword handles password change
func (uh *UserHandler) ChangePassword(c *gin.Context) {
	userIDStr, exists := GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	var req domain.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	if err := uh.userService.ChangePassword(userID, &req); err != nil {
		uh.logger.Warn("Password change failed", "error", err, "user_id", userID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password changed successfully",
	})
}

// Logout handles user logout
func (uh *UserHandler) Logout(c *gin.Context) {
	userIDStr, exists := GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	if err := uh.userService.Logout(userID); err != nil {
		uh.logger.Error("Failed to logout user", "error", err, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to logout",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// ListUsers handles listing users (admin only)
func (uh *UserHandler) ListUsers(c *gin.Context) {
	// Parse pagination parameters
	offset := 0
	limit := 50

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	users, err := uh.userService.ListUsers(offset, limit)
	if err != nil {
		uh.logger.Error("Failed to list users", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve users",
		})
		return
	}

	// Convert to user info (excluding sensitive data)
	userInfos := make([]*domain.UserInfo, len(users))
	for i, user := range users {
		userInfos[i] = user.ToUserInfo()
	}

	c.JSON(http.StatusOK, gin.H{
		"users":  userInfos,
		"offset": offset,
		"limit":  limit,
		"total":  len(userInfos),
	})
}

// DeleteUser handles user deletion (admin only)
func (uh *UserHandler) DeleteUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	if err := uh.userService.DeleteUser(userID); err != nil {
		uh.logger.Error("Failed to delete user", "error", err, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete user",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User deleted successfully",
	})
}

// GetCurrentUser returns current authenticated user info
func (uh *UserHandler) GetCurrentUser(c *gin.Context) {
	userIDStr, exists := GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	profile, err := uh.userService.GetProfile(userID)
	if err != nil {
		uh.logger.Error("Failed to get current user", "error", err, "user_id", userID)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, profile)
}
