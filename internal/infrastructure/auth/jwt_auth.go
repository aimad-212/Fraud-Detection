package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTService provides JWT token generation and validation
type JWTService struct {
	secretKey     []byte
	issuer        string
	expiryTime    time.Duration
	refreshExpiry time.Duration
}

// Claims represents the JWT claims structure
type Claims struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// UserInfo represents user information for token generation
type UserInfo struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
}

// JWTConfig represents JWT configuration
type JWTConfig struct {
	SecretKey          string        `yaml:"secret_key"`
	Issuer             string        `yaml:"issuer"`
	AccessTokenExpiry  time.Duration `yaml:"access_token_expiry"`
	RefreshTokenExpiry time.Duration `yaml:"refresh_token_expiry"`
}

// NewJWTService creates a new JWT service
func NewJWTService(config JWTConfig) (*JWTService, error) {
	if config.SecretKey == "" {
		return nil, fmt.Errorf("secret key is required")
	}

	if len(config.SecretKey) < 32 {
		return nil, fmt.Errorf("secret key must be at least 32 characters long")
	}

	// Set defaults
	if config.Issuer == "" {
		config.Issuer = "fraud-detection-service"
	}
	if config.AccessTokenExpiry == 0 {
		config.AccessTokenExpiry = 15 * time.Minute
	}
	if config.RefreshTokenExpiry == 0 {
		config.RefreshTokenExpiry = 7 * 24 * time.Hour // 7 days
	}

	return &JWTService{
		secretKey:     []byte(config.SecretKey),
		issuer:        config.Issuer,
		expiryTime:    config.AccessTokenExpiry,
		refreshExpiry: config.RefreshTokenExpiry,
	}, nil
}

// GenerateTokenPair generates both access and refresh tokens
func (j *JWTService) GenerateTokenPair(userInfo UserInfo) (*TokenPair, error) {
	// Generate access token
	accessToken, err := j.generateToken(userInfo, j.expiryTime)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := j.generateToken(userInfo, j.refreshExpiry)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(j.expiryTime),
		TokenType:    "Bearer",
	}, nil
}

// generateToken generates a JWT token with specified expiry
func (j *JWTService) generateToken(userInfo UserInfo, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userInfo.UserID,
		Username: userInfo.Username,
		Email:    userInfo.Email,
		Roles:    userInfo.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   userInfo.UserID,
			Audience:  []string{"fraud-detection-api"},
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        fmt.Sprintf("%d", now.Unix()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secretKey)
}

// ValidateToken validates a JWT token and returns claims
func (j *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// RefreshToken generates a new access token from a refresh token
func (j *JWTService) RefreshToken(refreshTokenString string) (*TokenPair, error) {
	// Validate refresh token
	claims, err := j.ValidateToken(refreshTokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Create user info from claims
	userInfo := UserInfo{
		UserID:   claims.UserID,
		Username: claims.Username,
		Email:    claims.Email,
		Roles:    claims.Roles,
	}

	// Generate new token pair
	return j.GenerateTokenPair(userInfo)
}

// ExtractTokenFromHeader extracts JWT token from Authorization header
func ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", fmt.Errorf("authorization header is required")
	}

	const bearerPrefix = "Bearer "
	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		return "", fmt.Errorf("authorization header must start with 'Bearer '")
	}

	return authHeader[len(bearerPrefix):], nil
}

// IsTokenExpired checks if a token is expired
func (j *JWTService) IsTokenExpired(tokenString string) bool {
	claims, err := j.ValidateToken(tokenString)
	if err != nil {
		return true
	}

	return claims.ExpiresAt.Time.Before(time.Now())
}

// GetUserIDFromToken extracts user ID from token
func (j *JWTService) GetUserIDFromToken(tokenString string) (string, error) {
	claims, err := j.ValidateToken(tokenString)
	if err != nil {
		return "", fmt.Errorf("failed to validate token: %w", err)
	}

	return claims.UserID, nil
}

// GetUserRolesFromToken extracts user roles from token
func (j *JWTService) GetUserRolesFromToken(tokenString string) ([]string, error) {
	claims, err := j.ValidateToken(tokenString)
	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}

	return claims.Roles, nil
}

// HasRole checks if user has specific role
func (j *JWTService) HasRole(tokenString, role string) bool {
	roles, err := j.GetUserRolesFromToken(tokenString)
	if err != nil {
		return false
	}

	for _, userRole := range roles {
		if userRole == role {
			return true
		}
	}

	return false
}

// HasAnyRole checks if user has any of the specified roles
func (j *JWTService) HasAnyRole(tokenString string, roles ...string) bool {
	userRoles, err := j.GetUserRolesFromToken(tokenString)
	if err != nil {
		return false
	}

	roleMap := make(map[string]bool)
	for _, role := range roles {
		roleMap[role] = true
	}

	for _, userRole := range userRoles {
		if roleMap[userRole] {
			return true
		}
	}

	return false
}

// DefaultJWTConfig returns default JWT configuration
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		SecretKey:          "your-super-secret-key-change-this-in-production",
		Issuer:             "fraud-detection-service",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour, // 7 days
	}
}
