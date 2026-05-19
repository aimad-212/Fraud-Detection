package usecase

import (
	"fmt"
	"strings"

	"fraud-detection/internal/domain"
)

// RulesEngineImpl implements the RuleEngine interface
type RulesEngineImpl struct {
	rules []*domain.Rule
}

// NewRulesEngine creates a new rules engine
func NewRulesEngine(rules []*domain.Rule) domain.RuleEngine {
	return &RulesEngineImpl{
		rules: rules,
	}
}

// EvaluateTransaction evaluates all enabled rules against a transaction
func (re *RulesEngineImpl) EvaluateTransaction(transaction *domain.Transaction, context *domain.EvaluationContext) []domain.RuleViolation {
	var violations []domain.RuleViolation

	for _, rule := range re.rules {
		if !rule.Enabled {
			continue
		}

		if violation := re.evaluateRule(rule, transaction, context); violation != nil {
			violations = append(violations, *violation)
		}
	}

	return violations
}

// evaluateRule evaluates a single rule
func (re *RulesEngineImpl) evaluateRule(rule *domain.Rule, transaction *domain.Transaction, context *domain.EvaluationContext) *domain.RuleViolation {
	switch rule.Type {
	case domain.RuleTypeAmount:
		return re.evaluateAmountRule(rule, transaction)
	case domain.RuleTypeVelocity:
		return re.evaluateVelocityRule(rule, transaction, context)
	case domain.RuleTypeGeographic:
		return re.evaluateGeographicRule(rule, transaction)
	case domain.RuleTypeMerchant:
		return re.evaluateMerchantRule(rule, transaction)
	case domain.RuleTypeTime:
		return re.evaluateTimeRule(rule, transaction)
	default:
		return &domain.RuleViolation{
			RuleID:      rule.ID,
			RuleName:    rule.Name,
			Severity:    domain.SeverityMedium,
			Description: "Unknown rule type",
			Violation:   fmt.Sprintf("Rule type '%s' is not supported", rule.Type),
		}
	}
}

// evaluateAmountRule evaluates amount-based rules
func (re *RulesEngineImpl) evaluateAmountRule(rule *domain.Rule, transaction *domain.Transaction) *domain.RuleViolation {
	params := rule.Parameters

	// Check maximum amount
	if params.MaxAmount != nil && transaction.Amount > *params.MaxAmount {
		return &domain.RuleViolation{
			RuleID:      rule.ID,
			RuleName:    rule.Name,
			Severity:    rule.Severity,
			Description: rule.Description,
			Violation:   fmt.Sprintf("Transaction amount %.2f exceeds maximum allowed %.2f", transaction.Amount, *params.MaxAmount),
		}
	}

	// Check minimum amount (unusually low amounts might be suspicious)
	if params.MinAmount != nil && transaction.Amount < *params.MinAmount {
		return &domain.RuleViolation{
			RuleID:      rule.ID,
			RuleName:    rule.Name,
			Severity:    rule.Severity,
			Description: rule.Description,
			Violation:   fmt.Sprintf("Transaction amount %.2f is below minimum threshold %.2f", transaction.Amount, *params.MinAmount),
		}
	}

	return nil
}

// evaluateVelocityRule evaluates velocity-based rules (transactions per time window)
func (re *RulesEngineImpl) evaluateVelocityRule(rule *domain.Rule, transaction *domain.Transaction, context *domain.EvaluationContext) *domain.RuleViolation {
	if context == nil || context.UserTransactions == nil {
		return nil // Cannot evaluate without transaction history
	}

	params := rule.Parameters
	if params.MaxTransactions == nil || params.TimeWindow == nil {
		return nil
	}

	// Count transactions in the time window
	cutoffTime := transaction.CreatedAt.Add(-*params.TimeWindow)
	count := 0

	for _, tx := range context.UserTransactions {
		if tx.CreatedAt.After(cutoffTime) && tx.ID != transaction.ID {
			count++
		}
	}

	if count >= *params.MaxTransactions {
		return &domain.RuleViolation{
			RuleID:      rule.ID,
			RuleName:    rule.Name,
			Severity:    rule.Severity,
			Description: rule.Description,
			Violation:   fmt.Sprintf("%d transactions in the last %v (limit: %d)", count+1, *params.TimeWindow, *params.MaxTransactions),
		}
	}

	return nil
}

// evaluateGeographicRule evaluates geographic-based rules
func (re *RulesEngineImpl) evaluateGeographicRule(rule *domain.Rule, transaction *domain.Transaction) *domain.RuleViolation {
	params := rule.Parameters

	// Check blocked countries
	for _, blocked := range params.BlockedCountries {
		if strings.EqualFold(transaction.Country, blocked) {
			return &domain.RuleViolation{
				RuleID:      rule.ID,
				RuleName:    rule.Name,
				Severity:    rule.Severity,
				Description: rule.Description,
				Violation:   fmt.Sprintf("Transaction from blocked country: %s", transaction.Country),
			}
		}
	}

	// Check allowed countries (if specified)
	if len(params.AllowedCountries) > 0 {
		allowed := false
		for _, allowedCountry := range params.AllowedCountries {
			if strings.EqualFold(transaction.Country, allowedCountry) {
				allowed = true
				break
			}
		}
		if !allowed {
			return &domain.RuleViolation{
				RuleID:      rule.ID,
				RuleName:    rule.Name,
				Severity:    rule.Severity,
				Description: rule.Description,
				Violation:   fmt.Sprintf("Transaction from non-allowed country: %s", transaction.Country),
			}
		}
	}

	return nil
}

// evaluateMerchantRule evaluates merchant-based rules
func (re *RulesEngineImpl) evaluateMerchantRule(rule *domain.Rule, transaction *domain.Transaction) *domain.RuleViolation {
	params := rule.Parameters

	for _, suspicious := range params.SuspiciousMerchants {
		if strings.Contains(strings.ToLower(transaction.Merchant), strings.ToLower(suspicious)) {
			return &domain.RuleViolation{
				RuleID:      rule.ID,
				RuleName:    rule.Name,
				Severity:    rule.Severity,
				Description: rule.Description,
				Violation:   fmt.Sprintf("Transaction with suspicious merchant: %s", transaction.Merchant),
			}
		}
	}

	return nil
}

// evaluateTimeRule evaluates time-based rules
func (re *RulesEngineImpl) evaluateTimeRule(rule *domain.Rule, transaction *domain.Transaction) *domain.RuleViolation {
	params := rule.Parameters

	// Get hour of transaction
	hour := transaction.CreatedAt.Hour()

	for _, blockedHour := range params.BlockedHours {
		if hour == blockedHour {
			return &domain.RuleViolation{
				RuleID:      rule.ID,
				RuleName:    rule.Name,
				Severity:    rule.Severity,
				Description: rule.Description,
				Violation:   fmt.Sprintf("Transaction at blocked hour: %d", hour),
			}
		}
	}

	return nil
}

// GetRules returns all rules
func (re *RulesEngineImpl) GetRules() []*domain.Rule {
	return re.rules
}

// GetRuleByID returns a rule by ID
func (re *RulesEngineImpl) GetRuleByID(id string) (*domain.Rule, error) {
	for _, rule := range re.rules {
		if rule.ID == id {
			return rule, nil
		}
	}
	return nil, fmt.Errorf("rule with ID '%s' not found", id)
}

// EnableRule enables a rule by ID
func (re *RulesEngineImpl) EnableRule(id string) error {
	rule, err := re.GetRuleByID(id)
	if err != nil {
		return err
	}
	rule.Enabled = true
	return nil
}

// DisableRule disables a rule by ID
func (re *RulesEngineImpl) DisableRule(id string) error {
	rule, err := re.GetRuleByID(id)
	if err != nil {
		return err
	}
	rule.Enabled = false
	return nil
}

// CalculateRiskScore calculates overall risk score from violations
func (re *RulesEngineImpl) CalculateRiskScore(violations []domain.RuleViolation) float64 {
	if len(violations) == 0 {
		return 0.0
	}

	maxScore := 0.0
	totalScore := 0.0

	for _, violation := range violations {
		score := violation.Severity.GetSeverityScore()
		totalScore += score
		if score > maxScore {
			maxScore = score
		}
	}

	// Use a weighted combination of average and maximum severity
	averageScore := totalScore / float64(len(violations))
	riskScore := (averageScore * 0.7) + (maxScore * 0.3)

	// Cap at 1.0
	if riskScore > 1.0 {
		riskScore = 1.0
	}

	return riskScore
}
