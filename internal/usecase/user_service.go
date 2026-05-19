package usecase

import (
	"fmt"
	"time"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/infrastructure/auth"

	"golang.org/x/crypto/bcrypt"
)

// UserServiceImpl implements the UserService interface
type UserServiceImpl struct {
	userRepo   domain.UserRepository
	jwtService *auth.JWTService
	logger     Logger
}

// NewUserService creates a new user service
func NewUserService(userRepo domain.UserRepository, jwtService *auth.JWTService, logger Logger) domain.UserService {
	return &UserServiceImpl{
		userRepo:   userRepo,
		jwtService: jwtService,
		logger:     logger,
	}
}

// Register creates a new user account
func (us *UserServiceImpl) Register(req *domain.CreateUserRequest) (*domain.User, error) {
	// Validate roles
	if err := domain.ValidateRoles(req.Roles); err != nil {
		return nil, err
	}

	// Check if username already exists
	existingUser, err := us.userRepo.GetByUsername(req.Username)
	if err == nil && existingUser != nil {
		return nil, &domain.ValidationError{
			Field:   "username",
			Message: "username already exists",
		}
	}

	// Check if email already exists
	existingUser, err = us.userRepo.GetByEmail(req.Email)
	if err == nil && existingUser != nil {
		return nil, &domain.ValidationError{
			Field:   "email",
			Message: "email already exists",
		}
	}

	// Hash password
	passwordHash, err := us.hashPassword(req.Password)
	if err != nil {
		us.logger.Error("Failed to hash password", "error", err)
		return nil, fmt.Errorf("failed to process password")
	}

	// Create user
	user := &domain.User{
		ID:           domain.NewUserID(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		IsActive:     true,
		Roles:        req.Roles,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := us.userRepo.Create(user); err != nil {
		us.logger.Error("Failed to create user", "error", err, "username", req.Username)
		return nil, fmt.Errorf("failed to create user")
	}

	us.logger.Info("User registered successfully", "user_id", user.ID, "username", user.Username)
	return user, nil
}

// Login authenticates a user and returns tokens
func (us *UserServiceImpl) Login(req *domain.LoginRequest) (*domain.LoginResponse, error) {
	// Get user by username
	user, err := us.userRepo.GetByUsername(req.Username)
	if err != nil {
		us.logger.Warn("Login attempt with non-existent username", "username", req.Username)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if user is active
	if !user.IsActive {
		us.logger.Warn("Login attempt with inactive user", "user_id", user.ID, "username", req.Username)
		return nil, fmt.Errorf("account is inactive")
	}

	// Verify password
	if !us.verifyPassword(req.Password, user.PasswordHash) {
		us.logger.Warn("Login attempt with invalid password", "user_id", user.ID, "username", req.Username)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Update last login
	if err := us.userRepo.UpdateLastLogin(user.ID); err != nil {
		us.logger.Error("Failed to update last login", "error", err, "user_id", user.ID)
		// Continue anyway - this is not critical
	}

	// Generate tokens
	userInfo := auth.UserInfo{
		UserID:   user.ID.String(),
		Username: user.Username,
		Email:    user.Email,
		Roles:    user.Roles,
	}

	tokenPair, err := us.jwtService.GenerateTokenPair(userInfo)
	if err != nil {
		us.logger.Error("Failed to generate tokens", "error", err, "user_id", user.ID)
		return nil, fmt.Errorf("failed to generate authentication tokens")
	}

	us.logger.Info("User logged in successfully", "user_id", user.ID, "username", user.Username)

	return &domain.LoginResponse{
		User:         user.ToUserInfo(),
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
		TokenType:    tokenPair.TokenType,
	}, nil
}

// RefreshToken generates new tokens from a refresh token
func (us *UserServiceImpl) RefreshToken(req *domain.RefreshTokenRequest) (*domain.LoginResponse, error) {
	// Validate refresh token and get user info
	claims, err := us.jwtService.ValidateToken(req.RefreshToken)
	if err != nil {
		us.logger.Warn("Invalid refresh token", "error", err)
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Get user from database
	userID, err := domain.ParseUserID(claims.UserID)
	if err != nil {
		us.logger.Error("Invalid user ID in token", "user_id", claims.UserID)
		return nil, fmt.Errorf("invalid token")
	}

	user, err := us.userRepo.GetByID(userID)
	if err != nil {
		us.logger.Warn("User not found for refresh token", "user_id", userID)
		return nil, fmt.Errorf("user not found")
	}

	// Check if user is still active
	if !user.IsActive {
		us.logger.Warn("Refresh token for inactive user", "user_id", user.ID)
		return nil, fmt.Errorf("account is inactive")
	}

	// Generate new tokens
	userInfo := auth.UserInfo{
		UserID:   user.ID.String(),
		Username: user.Username,
		Email:    user.Email,
		Roles:    user.Roles,
	}

	tokenPair, err := us.jwtService.GenerateTokenPair(userInfo)
	if err != nil {
		us.logger.Error("Failed to generate new tokens", "error", err, "user_id", user.ID)
		return nil, fmt.Errorf("failed to generate authentication tokens")
	}

	us.logger.Info("Token refreshed successfully", "user_id", user.ID, "username", user.Username)

	return &domain.LoginResponse{
		User:         user.ToUserInfo(),
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
		TokenType:    tokenPair.TokenType,
	}, nil
}

// GetProfile retrieves user profile
func (us *UserServiceImpl) GetProfile(userID domain.UserID) (*domain.UserInfo, error) {
	user, err := us.userRepo.GetByID(userID)
	if err != nil {
		us.logger.Error("Failed to get user profile", "error", err, "user_id", userID)
		return nil, fmt.Errorf("user not found")
	}

	return user.ToUserInfo(), nil
}

// UpdateProfile updates user profile
func (us *UserServiceImpl) UpdateProfile(userID domain.UserID, req *domain.UpdateUserRequest) (*domain.User, error) {
	user, err := us.userRepo.GetByID(userID)
	if err != nil {
		us.logger.Error("Failed to get user for update", "error", err, "user_id", userID)
		return nil, fmt.Errorf("user not found")
	}

	// Update fields if provided
	if req.Email != nil {
		// Check if email is already taken by another user
		existingUser, err := us.userRepo.GetByEmail(*req.Email)
		if err == nil && existingUser != nil && existingUser.ID != userID {
			return nil, &domain.ValidationError{
				Field:   "email",
				Message: "email already exists",
			}
		}
		user.Email = *req.Email
	}

	if req.FirstName != nil {
		user.FirstName = *req.FirstName
	}

	if req.LastName != nil {
		user.LastName = *req.LastName
	}

	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	if req.Roles != nil {
		// Validate roles
		if err := domain.ValidateRoles(req.Roles); err != nil {
			return nil, err
		}
		user.Roles = req.Roles
	}

	user.UpdatedAt = time.Now()

	if err := us.userRepo.Update(user); err != nil {
		us.logger.Error("Failed to update user", "error", err, "user_id", userID)
		return nil, fmt.Errorf("failed to update user")
	}

	us.logger.Info("User profile updated", "user_id", userID, "username", user.Username)
	return user, nil
}

// ChangePassword changes user password
func (us *UserServiceImpl) ChangePassword(userID domain.UserID, req *domain.ChangePasswordRequest) error {
	user, err := us.userRepo.GetByID(userID)
	if err != nil {
		us.logger.Error("Failed to get user for password change", "error", err, "user_id", userID)
		return fmt.Errorf("user not found")
	}

	// Verify current password
	if !us.verifyPassword(req.CurrentPassword, user.PasswordHash) {
		us.logger.Warn("Invalid current password for password change", "user_id", userID)
		return fmt.Errorf("current password is incorrect")
	}

	// Hash new password
	newPasswordHash, err := us.hashPassword(req.NewPassword)
	if err != nil {
		us.logger.Error("Failed to hash new password", "error", err, "user_id", userID)
		return fmt.Errorf("failed to process new password")
	}

	// Update password
	user.PasswordHash = newPasswordHash
	user.UpdatedAt = time.Now()

	if err := us.userRepo.Update(user); err != nil {
		us.logger.Error("Failed to update password", "error", err, "user_id", userID)
		return fmt.Errorf("failed to update password")
	}

	us.logger.Info("Password changed successfully", "user_id", userID, "username", user.Username)
	return nil
}

// Logout handles user logout (invalidate tokens if needed)
func (us *UserServiceImpl) Logout(userID domain.UserID) error {
	// In a real implementation, you might want to:
	// 1. Add the token to a blacklist
	// 2. Remove refresh tokens from database
	// 3. Log the logout event

	us.logger.Info("User logged out", "user_id", userID)
	return nil
}

// ListUsers retrieves a list of users (admin only)
func (us *UserServiceImpl) ListUsers(offset, limit int) ([]*domain.User, error) {
	users, err := us.userRepo.List(offset, limit)
	if err != nil {
		us.logger.Error("Failed to list users", "error", err)
		return nil, fmt.Errorf("failed to retrieve users")
	}

	return users, nil
}

// DeleteUser deletes a user account (admin only)
func (us *UserServiceImpl) DeleteUser(userID domain.UserID) error {
	if err := us.userRepo.Delete(userID); err != nil {
		us.logger.Error("Failed to delete user", "error", err, "user_id", userID)
		return fmt.Errorf("failed to delete user")
	}

	us.logger.Info("User deleted", "user_id", userID)
	return nil
}

// Password hashing and verification (simplified implementation)
// In production, use bcrypt or argon2

func (us *UserServiceImpl) hashPassword(password string) (string, error) {
	// Generate bcrypt hash from password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hashedPassword), nil
}

func (us *UserServiceImpl) verifyPassword(password, hash string) bool {
	// Compare bcrypt hash with password
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
