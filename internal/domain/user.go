package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Username     string     `json:"username" db:"username"`
	Email        string     `json:"email" db:"email"`
	PasswordHash string     `json:"-" db:"password_hash"`
	FirstName    string     `json:"first_name" db:"first_name"`
	LastName     string     `json:"last_name" db:"last_name"`
	IsActive     bool       `json:"is_active" db:"is_active"`
	Roles        []string   `json:"roles" db:"roles"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	LastLoginAt  *time.Time `json:"last_login_at" db:"last_login_at"`
}

// UserRole represents different user roles
type UserRole string

const (
	RoleAdmin    UserRole = "admin"
	RoleAnalyst  UserRole = "analyst"
	RoleViewer   UserRole = "viewer"
	RoleOperator UserRole = "operator"
	RoleAuditor  UserRole = "auditor"
)

// UserStatus represents user account status
type UserStatus string

const (
	StatusActive    UserStatus = "active"
	StatusInactive  UserStatus = "inactive"
	StatusSuspended UserStatus = "suspended"
	StatusBlocked   UserStatus = "blocked"
)

// LoginRequest represents login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents login response
type LoginResponse struct {
	User         *UserInfo `json:"user"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// RefreshTokenRequest represents refresh token request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// UserInfo represents user information for API responses
type UserInfo struct {
	ID        uuid.UUID  `json:"id"`
	Username  string     `json:"username"`
	Email     string     `json:"email"`
	FirstName string     `json:"first_name"`
	LastName  string     `json:"last_name"`
	IsActive  bool       `json:"is_active"`
	Roles     []string   `json:"roles"`
	CreatedAt time.Time  `json:"created_at"`
	LastLogin *time.Time `json:"last_login_at"`
}

// CreateUserRequest represents user creation request
type CreateUserRequest struct {
	Username  string   `json:"username" binding:"required,min=3,max=50"`
	Email     string   `json:"email" binding:"required,email"`
	Password  string   `json:"password" binding:"required,min=8"`
	FirstName string   `json:"first_name" binding:"required,min=1,max=50"`
	LastName  string   `json:"last_name" binding:"required,min=1,max=50"`
	Roles     []string `json:"roles" binding:"required,min=1"`
}

// UpdateUserRequest represents user update request
type UpdateUserRequest struct {
	Email     *string  `json:"email,omitempty" binding:"omitempty,email"`
	FirstName *string  `json:"first_name,omitempty" binding:"omitempty,min=1,max=50"`
	LastName  *string  `json:"last_name,omitempty" binding:"omitempty,min=1,max=50"`
	IsActive  *bool    `json:"is_active,omitempty"`
	Roles     []string `json:"roles,omitempty"`
}

// ChangePasswordRequest represents password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

// UserRepository defines the interface for user data operations
type UserRepository interface {
	Create(user *User) error
	GetByID(id uuid.UUID) (*User, error)
	GetByUsername(username string) (*User, error)
	GetByEmail(email string) (*User, error)
	Update(user *User) error
	Delete(id uuid.UUID) error
	List(offset, limit int) ([]*User, error)
	UpdateLastLogin(id uuid.UUID) error
}

// UserService defines the interface for user business logic
type UserService interface {
	Register(req *CreateUserRequest) (*User, error)
	Login(req *LoginRequest) (*LoginResponse, error)
	RefreshToken(req *RefreshTokenRequest) (*LoginResponse, error)
	GetProfile(userID uuid.UUID) (*UserInfo, error)
	UpdateProfile(userID uuid.UUID, req *UpdateUserRequest) (*User, error)
	ChangePassword(userID uuid.UUID, req *ChangePasswordRequest) error
	Logout(userID uuid.UUID) error
	ListUsers(offset, limit int) ([]*User, error)
	DeleteUser(userID uuid.UUID) error
}

// IsValidRole checks if a role is valid
func IsValidRole(role string) bool {
	validRoles := []string{
		string(RoleAdmin),
		string(RoleAnalyst),
		string(RoleViewer),
		string(RoleOperator),
		string(RoleAuditor),
	}

	for _, validRole := range validRoles {
		if role == validRole {
			return true
		}
	}

	return false
}

// ValidateRoles checks if all roles in the slice are valid
func ValidateRoles(roles []string) error {
	for _, role := range roles {
		if !IsValidRole(role) {
			return &ValidationError{
				Field:   "roles",
				Message: "invalid role: " + role,
			}
		}
	}

	return nil
}

// HasRole checks if user has a specific role
func (u *User) HasRole(role string) bool {
	for _, userRole := range u.Roles {
		if userRole == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if user has any of the specified roles
func (u *User) HasAnyRole(roles ...string) bool {
	roleMap := make(map[string]bool)
	for _, role := range roles {
		roleMap[role] = true
	}

	for _, userRole := range u.Roles {
		if roleMap[userRole] {
			return true
		}
	}

	return false
}

// ToUserInfo converts User to UserInfo (excluding sensitive data)
func (u *User) ToUserInfo() *UserInfo {
	return &UserInfo{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		IsActive:  u.IsActive,
		Roles:     u.Roles,
		CreatedAt: u.CreatedAt,
		LastLogin: u.LastLoginAt,
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// NewUserID creates a new user ID
func NewUserID() uuid.UUID {
	return uuid.New()
}

// ParseUserID parses a user ID from string
func ParseUserID(idStr string) (uuid.UUID, error) {
	return uuid.Parse(idStr)
}

// UserID type alias for clarity
type UserID = uuid.UUID
