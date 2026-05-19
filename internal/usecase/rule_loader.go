package usecase

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fraud-detection/internal/domain"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// RuleConfig represents the YAML configuration structure
type RuleConfig struct {
	Rules []*domain.Rule `yaml:"rules"`
}

// RuleLoader handles loading rules from configuration files with hot-reload support
type RuleLoader struct {
	mu           sync.RWMutex
	rules        []*domain.Rule
	rulesFile    string
	watcher      *fsnotify.Watcher
	onReload     func([]*domain.Rule)
	lastModified time.Time
}

// NewRuleLoader creates a new rule loader
func NewRuleLoader() *RuleLoader {
	return &RuleLoader{
		rules: make([]*domain.Rule, 0),
	}
}

// NewRuleLoaderWithFile creates a new rule loader with file watching
func NewRuleLoaderWithFile(rulesFile string, onReload func([]*domain.Rule)) (*RuleLoader, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	rl := &RuleLoader{
		rules:     make([]*domain.Rule, 0),
		rulesFile: rulesFile,
		watcher:   watcher,
		onReload:  onReload,
	}

	// Load initial rules
	if err := rl.loadFromFile(); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to load initial rules: %w", err)
	}

	// Start watching the file
	go rl.watchFile()

	return rl, nil
}

// loadFromFile loads rules from the configured file (internal method)
func (rl *RuleLoader) loadFromFile() error {
	data, err := os.ReadFile(rl.rulesFile)
	if err != nil {
		return fmt.Errorf("failed to read rules file: %w", err)
	}

	var config RuleConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse rules YAML: %w", err)
	}

	// Validate and process rules
	for _, rule := range config.Rules {
		if err := rl.processRule(rule); err != nil {
			return fmt.Errorf("invalid rule '%s': %w", rule.ID, err)
		}
	}

	rl.mu.Lock()
	rl.rules = config.Rules
	rl.mu.Unlock()

	return nil
}

// watchFile watches for file changes and reloads rules
func (rl *RuleLoader) watchFile() {
	defer rl.watcher.Close()

	// Watch the directory containing the rules file
	rulesDir := filepath.Dir(rl.rulesFile)
	if err := rl.watcher.Add(rulesDir); err != nil {
		fmt.Printf("Failed to watch directory %s: %v\n", rulesDir, err)
		return
	}

	for {
		select {
		case event, ok := <-rl.watcher.Events:
			if !ok {
				return
			}

			// Check if the event is for our rules file
			if event.Name == rl.rulesFile && (event.Op&fsnotify.Write == fsnotify.Write) {
				// Add a small delay to handle rapid file writes
				time.Sleep(100 * time.Millisecond)

				if err := rl.reloadRules(); err != nil {
					fmt.Printf("Failed to reload rules: %v\n", err)
				}
			}

		case err, ok := <-rl.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("File watcher error: %v\n", err)
		}
	}
}

// reloadRules reloads rules from file and triggers callback
func (rl *RuleLoader) reloadRules() error {
	if err := rl.loadFromFile(); err != nil {
		return err
	}

	// Trigger reload callback
	if rl.onReload != nil {
		rl.mu.RLock()
		rulesCopy := make([]*domain.Rule, len(rl.rules))
		copy(rulesCopy, rl.rules)
		rl.mu.RUnlock()

		rl.onReload(rulesCopy)
	}

	return nil
}

// GetRules returns the current rules (thread-safe)
func (rl *RuleLoader) GetRules() []*domain.Rule {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	rulesCopy := make([]*domain.Rule, len(rl.rules))
	copy(rulesCopy, rl.rules)
	return rulesCopy
}

// LoadRulesFromFile loads rules from a YAML file
func (rl *RuleLoader) LoadRulesFromFile(filename string) ([]*domain.Rule, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	var config RuleConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse rules YAML: %w", err)
	}

	// Validate and process rules
	for _, rule := range config.Rules {
		if err := rl.processRule(rule); err != nil {
			return nil, fmt.Errorf("invalid rule '%s': %w", rule.ID, err)
		}
	}

	return config.Rules, nil
}

// processRule processes and validates a rule after loading
func (rl *RuleLoader) processRule(rule *domain.Rule) error {
	// Validate the rule
	if err := rule.Validate(); err != nil {
		return err
	}

	// Process time durations for velocity rules
	if rule.Type == domain.RuleTypeVelocity && rule.Parameters.TimeWindow != nil {
		// TimeWindow is already parsed as duration by yaml.Unmarshal
		// But we can add additional validation here
		if *rule.Parameters.TimeWindow <= 0 {
			return fmt.Errorf("time_window must be positive")
		}
	}

	return nil
}

// SaveRulesToFile saves rules to a YAML file
func (rl *RuleLoader) SaveRulesToFile(filename string, rules []*domain.Rule) error {
	config := RuleConfig{Rules: rules}

	data, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to marshal rules to YAML: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write rules file: %w", err)
	}

	return nil
}

// CreateDefaultRules creates a set of default rules
func (rl *RuleLoader) CreateDefaultRules() []*domain.Rule {
	rules := []*domain.Rule{
		// Amount rules
		domain.NewRule("high_amount", "High Amount Transaction", "Flags transactions above $5000", domain.RuleTypeAmount, domain.SeverityHigh),
		domain.NewRule("very_high_amount", "Very High Amount Transaction", "Blocks transactions above $10000", domain.RuleTypeAmount, domain.SeverityCritical),

		// Velocity rules
		domain.NewRule("rapid_transactions", "Rapid Transactions", "Flags 5+ transactions per hour", domain.RuleTypeVelocity, domain.SeverityHigh),

		// Geographic rules
		domain.NewRule("blocked_countries", "Blocked Countries", "Blocks transactions from high-risk countries", domain.RuleTypeGeographic, domain.SeverityCritical),

		// Merchant rules
		domain.NewRule("suspicious_merchants", "Suspicious Merchants", "Flags transactions with suspicious merchants", domain.RuleTypeMerchant, domain.SeverityHigh),

		// Time rules
		domain.NewRule("unusual_hours", "Unusual Hours", "Flags transactions during unusual hours", domain.RuleTypeTime, domain.SeverityMedium),
	}

	// Configure rule parameters
	for _, rule := range rules {
		switch rule.ID {
		case "high_amount":
			maxAmount := 5000.0
			rule.Parameters.MaxAmount = &maxAmount
		case "very_high_amount":
			maxAmount := 10000.0
			rule.Parameters.MaxAmount = &maxAmount
		case "rapid_transactions":
			maxTx := 5
			timeWindow := 1 * time.Hour
			rule.Parameters.MaxTransactions = &maxTx
			rule.Parameters.TimeWindow = &timeWindow
		case "blocked_countries":
			rule.Parameters.BlockedCountries = []string{"KP", "IR", "SY"}
		case "suspicious_merchants":
			rule.Parameters.SuspiciousMerchants = []string{"fake-store", "test-merchant", "suspicious-vendor"}
		case "unusual_hours":
			rule.Parameters.BlockedHours = []int{2, 3, 4, 5}
		}
	}

	return rules
}

// ValidateRules validates a set of rules
func (rl *RuleLoader) ValidateRules(rules []*domain.Rule) error {
	seenIDs := make(map[string]bool)

	for _, rule := range rules {
		// Check for duplicate IDs
		if seenIDs[rule.ID] {
			return fmt.Errorf("duplicate rule ID: %s", rule.ID)
		}
		seenIDs[rule.ID] = true

		// Validate individual rule
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("rule '%s' validation failed: %w", rule.ID, err)
		}
	}

	return nil
}

// GetRulesByType returns rules filtered by type
func (rl *RuleLoader) GetRulesByType(rules []*domain.Rule, ruleType domain.RuleType) []*domain.Rule {
	var filtered []*domain.Rule
	for _, rule := range rules {
		if rule.Type == ruleType {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

// GetEnabledRules returns only enabled rules
func (rl *RuleLoader) GetEnabledRules(rules []*domain.Rule) []*domain.Rule {
	var enabled []*domain.Rule
	for _, rule := range rules {
		if rule.Enabled {
			enabled = append(enabled, rule)
		}
	}
	return enabled
}

// Close closes the file watcher
func (rl *RuleLoader) Close() error {
	if rl.watcher != nil {
		return rl.watcher.Close()
	}
	return nil
}
