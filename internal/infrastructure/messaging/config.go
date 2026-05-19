package messaging

import (
	"fmt"
	"time"
)

// Config holds NATS messaging configuration
type Config struct {
	Enabled       bool          `yaml:"enabled"`
	URL           string        `yaml:"url"`
	Subject       string        `yaml:"subject"`
	QueueGroup    string        `yaml:"queue_group"`
	MaxReconnect  int           `yaml:"max_reconnect"`
	ReconnectWait time.Duration `yaml:"reconnect_wait"`
}

// NewConfig creates a new NATS config with defaults
func NewConfig() *Config {
	return &Config{
		Enabled:       true,
		URL:           "nats://localhost:4222",
		Subject:       "transactions",
		QueueGroup:    "fraud-processor",
		MaxReconnect:  5,
		ReconnectWait: 2 * time.Second,
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Enabled {
		if c.URL == "" {
			return fmt.Errorf("NATS URL is required when enabled")
		}
		if c.Subject == "" {
			return fmt.Errorf("NATS subject is required when enabled")
		}
		if c.QueueGroup == "" {
			return fmt.Errorf("NATS queue group is required when enabled")
		}
		if c.MaxReconnect < 0 {
			return fmt.Errorf("max reconnect must be non-negative")
		}
		if c.ReconnectWait <= 0 {
			return fmt.Errorf("reconnect wait must be positive")
		}
	}
	return nil
}
