package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RuleType represents different types of fraud detection rules
type RuleType string

const (
	RuleTypeAmount     RuleType = "amount"
	RuleTypeVelocity   RuleType = "velocity"
	RuleTypeGeographic RuleType = "geographic"
	RuleTypeMerchant   RuleType = "merchant"
	RuleTypeTime       RuleType = "time"
)

// RuleSeverity represents the severity level of a rule violation
type RuleSeverity string

const (
	SeverityLow      RuleSeverity = "low"
	SeverityMedium   RuleSeverity = "medium"
	SeverityHigh     RuleSeverity = "high"
	SeverityCritical RuleSeverity = "critical"
)

// Rule represents a fraud detection rule
type Rule struct {
	ID          string       `json:"id" yaml:"id"`
	Name        string       `json:"name" yaml:"name"`
	Description string       `json:"description" yaml:"description"`
	Type        RuleType     `json:"type" yaml:"type"`
	Severity    RuleSeverity `json:"severity" yaml:"severity"`
	Enabled     bool         `json:"enabled" yaml:"enabled"`
	Parameters  RuleParams   `json:"parameters" yaml:"parameters"`
}

// RuleParams contains rule-specific parameters
type RuleParams struct {
	// Amount rules
	MaxAmount *float64 `json:"max_amount,omitempty" yaml:"max_amount,omitempty"`
	MinAmount *float64 `json:"min_amount,omitempty" yaml:"min_amount,omitempty"`

	// Velocity rules
	MaxTransactions *int           `json:"max_transactions,omitempty" yaml:"max_transactions,omitempty"`
	TimeWindow      *time.Duration `json:"time_window,omitempty" yaml:"time_window,omitempty"`

	// Geographic rules
	AllowedCountries []string `json:"allowed_countries,omitempty" yaml:"allowed_countries,omitempty"`
	BlockedCountries []string `json:"blocked_countries,omitempty" yaml:"blocked_countries,omitempty"`

	// Merchant rules
	SuspiciousMerchants []string `json:"suspicious_merchants,omitempty" yaml:"suspicious_merchants,omitempty"`

	// Time rules
	BlockedHours []int `json:"blocked_hours,omitempty" yaml:"blocked_hours,omitempty"`
}

// RuleViolation represents a rule violation
type RuleViolation struct {
	RuleID      string       `json:"rule_id"`
	RuleName    string       `json:"rule_name"`
	Severity    RuleSeverity `json:"severity"`
	Description string       `json:"description"`
	Violation   string       `json:"violation"`
}

// RuleEngine defines the interface for rule evaluation
type RuleEngine interface {
	EvaluateTransaction(transaction *Transaction, context *EvaluationContext) []RuleViolation
	CalculateRiskScore(violations []RuleViolation) float64
	GetRules() []*Rule
	GetRuleByID(id string) (*Rule, error)
	EnableRule(id string) error
	DisableRule(id string) error
}

// EvaluationContext provides additional context for rule evaluation
type EvaluationContext struct {
	UserTransactions []*Transaction `json:"user_transactions"`
	UserProfile      *UserProfile   `json:"user_profile"`
	IPAddress        string         `json:"ip_address"`
	UserAgent        string         `json:"user_agent"`
}

// UserProfile represents user behavior profile
type UserProfile struct {
	UserID                   string    `json:"user_id"`
	AverageTransactionAmount float64   `json:"average_transaction_amount"`
	TransactionCount         int       `json:"transaction_count"`
	LastTransactionTime      time.Time `json:"last_transaction_time"`
	PreferredCountries       []string  `json:"preferred_countries"`
	PreferredMerchants       []string  `json:"preferred_merchants"`
	RiskScore                float64   `json:"risk_score"`
}

// NewRule creates a new rule with default values
func NewRule(id, name, description string, ruleType RuleType, severity RuleSeverity) *Rule {
	return &Rule{
		ID:          id,
		Name:        name,
		Description: description,
		Type:        ruleType,
		Severity:    severity,
		Enabled:     true,
		Parameters:  RuleParams{},
	}
}

// Validate checks if the rule is valid
func (r *Rule) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("rule ID cannot be empty")
	}
	if r.Name == "" {
		return fmt.Errorf("rule name cannot be empty")
	}
	if r.Type == "" {
		return fmt.Errorf("rule type cannot be empty")
	}
	if r.Severity == "" {
		return fmt.Errorf("rule severity cannot be empty")
	}

	// Type-specific validation
	switch r.Type {
	case RuleTypeAmount:
		if r.Parameters.MaxAmount == nil && r.Parameters.MinAmount == nil {
			return fmt.Errorf("amount rule must have max_amount or min_amount")
		}
	case RuleTypeVelocity:
		if r.Parameters.MaxTransactions == nil || r.Parameters.TimeWindow == nil {
			return fmt.Errorf("velocity rule must have max_transactions and time_window")
		}
	case RuleTypeGeographic:
		if len(r.Parameters.AllowedCountries) == 0 && len(r.Parameters.BlockedCountries) == 0 {
			return fmt.Errorf("geographic rule must have allowed_countries or blocked_countries")
		}
	}

	return nil
}

// GetSeverityScore returns a numeric score for severity
func (s RuleSeverity) GetSeverityScore() float64 {
	switch s {
	case SeverityLow:
		return 0.2
	case SeverityMedium:
		return 0.5
	case SeverityHigh:
		return 0.8
	case SeverityCritical:
		return 1.0
	default:
		return 0.0
	}
}

// String returns string representation of RuleType
func (rt RuleType) String() string {
	return string(rt)
}

// String returns string representation of RuleSeverity
func (rs RuleSeverity) String() string {
	return string(rs)
}

// Decision represents the final decision for a transaction
type Decision string

const (
	DecisionApprove Decision = "approve"
	DecisionReject  Decision = "reject"
	DecisionFlag    Decision = "flag"
)

// DecisionResult represents the result of a fraud evaluation
type DecisionResult struct {
	TransactionID uuid.UUID       `json:"transaction_id"`
	Decision      Decision        `json:"decision"`
	RiskScore     float64         `json:"risk_score"`
	RuleScore     float64         `json:"rule_score"`
	MLScore       float64         `json:"ml_score"`
	Violations    []RuleViolation `json:"violations"`
	Features      map[string]any  `json:"features"`
	Explanation   string          `json:"explanation"`
	ProcessedAt   time.Time       `json:"processed_at"`
}

// NewDecisionResult creates a new decision result
func NewDecisionResult(transactionID uuid.UUID, decision Decision, riskScore float64) *DecisionResult {
	return &DecisionResult{
		TransactionID: transactionID,
		Decision:      decision,
		RiskScore:     riskScore,
		Violations:    []RuleViolation{},
		Features:      make(map[string]any),
		ProcessedAt:   time.Now(),
	}
}
