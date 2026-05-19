package repository

import (
	"fmt"
	"sync"
	"time"

	"fraud-detection/internal/domain"

	"golang.org/x/crypto/bcrypt"
)

// InMemoryUserRepository implements UserRepository interface using in-memory storage
type InMemoryUserRepository struct {
	users map[domain.UserID]*domain.User
	mutex sync.RWMutex
}

// NewInMemoryUserRepository creates a new in-memory user repository
func NewInMemoryUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{
		users: make(map[domain.UserID]*domain.User),
		mutex: sync.RWMutex{},
	}
}

// Create creates a new user
func (r *InMemoryUserRepository) Create(user *domain.User) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if user already exists
	for _, existingUser := range r.users {
		if existingUser.Username == user.Username {
			return fmt.Errorf("username already exists")
		}
		if existingUser.Email == user.Email {
			return fmt.Errorf("email already exists")
		}
	}

	// Add user
	r.users[user.ID] = user
	return nil
}

// GetByID retrieves a user by ID
func (r *InMemoryUserRepository) GetByID(id domain.UserID) (*domain.User, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	user, exists := r.users[id]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	// Return a copy to avoid modifying the original
	userCopy := *user
	return &userCopy, nil
}

// GetByUsername retrieves a user by username
func (r *InMemoryUserRepository) GetByUsername(username string) (*domain.User, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for _, user := range r.users {
		if user.Username == username {
			// Return a copy to avoid modifying the original
			userCopy := *user
			return &userCopy, nil
		}
	}

	return nil, fmt.Errorf("user not found")
}

// GetByEmail retrieves a user by email
func (r *InMemoryUserRepository) GetByEmail(email string) (*domain.User, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for _, user := range r.users {
		if user.Email == email {
			// Return a copy to avoid modifying the original
			userCopy := *user
			return &userCopy, nil
		}
	}

	return nil, fmt.Errorf("user not found")
}

// Update updates an existing user
func (r *InMemoryUserRepository) Update(user *domain.User) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	existingUser, exists := r.users[user.ID]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// Check if username/email conflicts with other users
	for id, existing := range r.users {
		if id != user.ID {
			if existing.Username == user.Username {
				return fmt.Errorf("username already exists")
			}
			if existing.Email == user.Email {
				return fmt.Errorf("email already exists")
			}
		}
	}

	// Update user
	user.UpdatedAt = time.Now()
	r.users[user.ID] = user

	// Preserve last login time if it exists in the original
	if existingUser.LastLoginAt != nil {
		r.users[user.ID].LastLoginAt = existingUser.LastLoginAt
	}

	return nil
}

// Delete deletes a user by ID
func (r *InMemoryUserRepository) Delete(id domain.UserID) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.users[id]; !exists {
		return fmt.Errorf("user not found")
	}

	delete(r.users, id)
	return nil
}

// List retrieves a list of users with pagination
func (r *InMemoryUserRepository) List(offset, limit int) ([]*domain.User, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Convert map to slice
	users := make([]*domain.User, 0, len(r.users))
	for _, user := range r.users {
		userCopy := *user
		users = append(users, &userCopy)
	}

	// Apply pagination
	if offset >= len(users) {
		return []*domain.User{}, nil
	}

	end := offset + limit
	if end > len(users) {
		end = len(users)
	}

	return users[offset:end], nil
}

// UpdateLastLogin updates the last login time for a user
func (r *InMemoryUserRepository) UpdateLastLogin(id domain.UserID) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	user, exists := r.users[id]
	if !exists {
		return fmt.Errorf("user not found")
	}

	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = now

	return nil
}

// Count returns the total number of users
func (r *InMemoryUserRepository) Count() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.users)
}

// CreateDefaultAdmin creates a default admin user if no users exist
func (r *InMemoryUserRepository) CreateDefaultAdmin() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if any users exist
	if len(r.users) > 0 {
		return nil // Users already exist, don't create default admin
	}

	// Create default admin user with bcrypt hash
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("Admin123!"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	adminUser := &domain.User{
		ID:           domain.NewUserID(),
		Username:     "admin",
		Email:        "admin@fraud-detection.com",
		PasswordHash: string(hashedPassword),
		FirstName:    "System",
		LastName:     "Administrator",
		IsActive:     true,
		Roles:        []string{"admin"},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	r.users[adminUser.ID] = adminUser
	return nil
}

// SeedTestData creates sample users for testing
func (r *InMemoryUserRepository) SeedTestData() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Create sample users
	sampleUsers := []*domain.User{
		{
			ID:           domain.NewUserID(),
			Username:     "analyst1",
			Email:        "analyst1@example.com",
			PasswordHash: "password123_hashed",
			FirstName:    "John",
			LastName:     "Doe",
			IsActive:     true,
			Roles:        []string{"analyst"},
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           domain.NewUserID(),
			Username:     "viewer1",
			Email:        "viewer1@example.com",
			PasswordHash: "password123_hashed",
			FirstName:    "Jane",
			LastName:     "Smith",
			IsActive:     true,
			Roles:        []string{"viewer"},
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           domain.NewUserID(),
			Username:     "operator1",
			Email:        "operator1@example.com",
			PasswordHash: "password123_hashed",
			FirstName:    "Bob",
			LastName:     "Johnson",
			IsActive:     true,
			Roles:        []string{"operator"},
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	for _, user := range sampleUsers {
		r.users[user.ID] = user
	}

	return nil
}
