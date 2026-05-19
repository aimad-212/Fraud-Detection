package database

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config holds database configuration
type Config struct {
	Type            string        `yaml:"type"`
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Database        string        `yaml:"database"`
	Username        string        `yaml:"username"`
	Password        string        `yaml:"password"`
	SSLMode         string        `yaml:"sslmode"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// NewConfig creates a new database config with defaults
func NewConfig() *Config {
	return &Config{
		Type:            "memory",
		Host:            "localhost",
		Port:            5432,
		Database:        "fraud_detection",
		Username:        "fraud_user",
		Password:        "fraud_password",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    25,
		ConnMaxLifetime: 5 * time.Minute,
	}
}

// DSN returns the database connection string
func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.Username, c.Password, c.Database, c.SSLMode)
}

// Connect establishes a database connection
func (c *Config) Connect() (*gorm.DB, error) {
	if c.Type == "memory" {
		// For in-memory, we'll still use a simple setup
		// In production, you might want to use sqlite for testing
		return nil, fmt.Errorf("memory database not implemented for GORM, use postgres")
	}

	// GORM configuration
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	// Connect to PostgreSQL
	db, err := gorm.Open(postgres.Open(c.DSN()), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(c.MaxOpenConns)
	sqlDB.SetMaxIdleConns(c.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(c.ConnMaxLifetime)

	return db, nil
}

// IsMemory returns true if using in-memory database
func (c *Config) IsMemory() bool {
	return c.Type == "memory"
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Type != "memory" && c.Type != "postgres" && c.Type != "mysql" {
		return fmt.Errorf("unsupported database type: %s", c.Type)
	}

	if c.Type != "memory" {
		if c.Host == "" {
			return fmt.Errorf("database host is required")
		}
		if c.Port <= 0 || c.Port > 65535 {
			return fmt.Errorf("invalid database port: %d", c.Port)
		}
		if c.Database == "" {
			return fmt.Errorf("database name is required")
		}
		if c.Username == "" {
			return fmt.Errorf("database username is required")
		}
	}

	return nil
}
