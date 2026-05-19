package tests

import (
	"testing"
	"time"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestRulesEngine_AmountRules(t *testing.T) {
	// Create rules
	highAmountRule := domain.NewRule("high_amount", "High Amount", "Amount exceeds threshold", domain.RuleTypeAmount, domain.SeverityHigh)
	highAmountRule.Parameters.MaxAmount = &[]float64{5000}[0]

	rules := []*domain.Rule{highAmountRule}
	engine := usecase.NewRulesEngine(rules)

	// Create test transaction
	userID := uuid.New()
	transaction := domain.NewTransaction(userID, 6000, "USD")

	// Test high amount violation
	context := &domain.EvaluationContext{}
	violations := engine.EvaluateTransaction(transaction, context)

	assert.Len(t, violations, 1)
	assert.Equal(t, "high_amount", violations[0].RuleID)
	assert.Equal(t, domain.SeverityHigh, violations[0].Severity)
	assert.Contains(t, violations[0].Violation, "6000.00 exceeds maximum")
}

func TestRulesEngine_VelocityRules(t *testing.T) {
	// Create velocity rule
	velocityRule := domain.NewRule("rapid_transactions", "Rapid Transactions", "Too many transactions in time window", domain.RuleTypeVelocity, domain.SeverityHigh)
	maxTx := 3
	timeWindow := 1 * time.Hour
	velocityRule.Parameters.MaxTransactions = &maxTx
	velocityRule.Parameters.TimeWindow = &timeWindow

	rules := []*domain.Rule{velocityRule}
	engine := usecase.NewRulesEngine(rules)

	// Create test transactions
	userID := uuid.New()
	now := time.Now()

	// Create 4 transactions in the last hour
	transactions := []*domain.Transaction{
		domain.NewTransaction(userID, 100, "USD"),
		domain.NewTransaction(userID, 200, "USD"),
		domain.NewTransaction(userID, 300, "USD"),
		domain.NewTransaction(userID, 400, "USD"),
	}

	// Set timestamps
	for i, tx := range transactions {
		tx.CreatedAt = now.Add(time.Duration(-i) * time.Minute)
	}

	// Current transaction
	currentTx := domain.NewTransaction(userID, 500, "USD")
	currentTx.CreatedAt = now

	context := &domain.EvaluationContext{
		UserTransactions: transactions,
	}

	violations := engine.EvaluateTransaction(currentTx, context)

	assert.Len(t, violations, 1)
	assert.Equal(t, "rapid_transactions", violations[0].RuleID)
	assert.Contains(t, violations[0].Violation, "5 transactions")
}

func TestRulesEngine_GeographicRules(t *testing.T) {
	// Create blocked countries rule
	geoRule := domain.NewRule("blocked_countries", "Blocked Countries", "Transaction from blocked country", domain.RuleTypeGeographic, domain.SeverityCritical)
	geoRule.Parameters.BlockedCountries = []string{"KP", "IR"}

	rules := []*domain.Rule{geoRule}
	engine := usecase.NewRulesEngine(rules)

	// Test blocked country
	userID := uuid.New()
	transaction := domain.NewTransaction(userID, 1000, "USD")
	transaction.Country = "KP"

	context := &domain.EvaluationContext{}
	violations := engine.EvaluateTransaction(transaction, context)

	assert.Len(t, violations, 1)
	assert.Equal(t, "blocked_countries", violations[0].RuleID)
	assert.Contains(t, violations[0].Violation, "blocked country: KP")
}

func TestRulesEngine_MerchantRules(t *testing.T) {
	// Create suspicious merchant rule
	merchantRule := domain.NewRule("suspicious_merchants", "Suspicious Merchants", "Transaction with suspicious merchant", domain.RuleTypeMerchant, domain.SeverityHigh)
	merchantRule.Parameters.SuspiciousMerchants = []string{"fake-store", "test-shop"}

	rules := []*domain.Rule{merchantRule}
	engine := usecase.NewRulesEngine(rules)

	// Test suspicious merchant
	userID := uuid.New()
	transaction := domain.NewTransaction(userID, 1000, "USD")
	transaction.Merchant = "fake-store"

	context := &domain.EvaluationContext{}
	violations := engine.EvaluateTransaction(transaction, context)

	assert.Len(t, violations, 1)
	assert.Equal(t, "suspicious_merchants", violations[0].RuleID)
	assert.Contains(t, violations[0].Violation, "suspicious merchant")
}

func TestRulesEngine_TimeRules(t *testing.T) {
	// Create blocked hours rule
	timeRule := domain.NewRule("blocked_hours", "Blocked Hours", "Transaction during blocked hours", domain.RuleTypeTime, domain.SeverityMedium)
	timeRule.Parameters.BlockedHours = []int{2, 3, 4}

	rules := []*domain.Rule{timeRule}
	engine := usecase.NewRulesEngine(rules)

	// Test blocked hour
	userID := uuid.New()
	transaction := domain.NewTransaction(userID, 1000, "USD")
	// Set time to 3 AM
	transaction.CreatedAt = time.Date(2024, 1, 1, 3, 0, 0, 0, time.UTC)

	context := &domain.EvaluationContext{}
	violations := engine.EvaluateTransaction(transaction, context)

	assert.Len(t, violations, 1)
	assert.Equal(t, "blocked_hours", violations[0].RuleID)
	assert.Contains(t, violations[0].Violation, "blocked hour: 3")
}

func TestRulesEngine_CalculateRiskScore(t *testing.T) {
	rules := []*domain.Rule{}
	engine := usecase.NewRulesEngine(rules)

	// Test empty violations
	score := engine.CalculateRiskScore([]domain.RuleViolation{})
	assert.Equal(t, 0.0, score)

	// Test with violations
	violations := []domain.RuleViolation{
		{Severity: domain.SeverityLow},
		{Severity: domain.SeverityHigh},
	}
	score = engine.CalculateRiskScore(violations)
	assert.True(t, score > 0.0)
	assert.True(t, score <= 1.0)
}

func TestFeatureEngineering(t *testing.T) {
	fe := usecase.NewFeatureEngineer()

	userID := uuid.New()
	transaction := domain.NewTransaction(userID, 1000, "USD")
	transaction.Merchant = "test-merchant"
	transaction.Country = "US"
	transaction.CreatedAt = time.Now()

	// Create user profile
	profile := &domain.UserProfile{
		UserID:                   userID.String(),
		AverageTransactionAmount: 500,
		TransactionCount:         10,
		PreferredCountries:       []string{"US", "CA"},
		PreferredMerchants:       []string{"trusted-merchant"},
		RiskScore:                0.1,
	}

	// Create transaction history
	history := []*domain.Transaction{
		domain.NewTransaction(userID, 400, "USD"),
		domain.NewTransaction(userID, 600, "USD"),
	}

	context := &domain.EvaluationContext{
		UserTransactions: history,
		UserProfile:      profile,
	}

	features := fe.ExtractFeatures(transaction, context)

	assert.NotNil(t, features)
	assert.Equal(t, 1000.0, features.Amount)
	assert.Equal(t, "test-merchant", features.Merchant)
	assert.Equal(t, "US", features.Country)
	assert.Equal(t, 2, features.UserTotalTransactions)
	assert.True(t, features.AmountToAverageRatio > 1.0)
	assert.True(t, features.IsNewMerchant)
	assert.False(t, features.IsNewCountry)
}

func BenchmarkRulesEngine_EvaluateTransaction(b *testing.B) {
	// Create multiple rules
	rules := []*domain.Rule{
		domain.NewRule("amount", "Amount Rule", "Test", domain.RuleTypeAmount, domain.SeverityMedium),
		domain.NewRule("velocity", "Velocity Rule", "Test", domain.RuleTypeVelocity, domain.SeverityMedium),
		domain.NewRule("geo", "Geo Rule", "Test", domain.RuleTypeGeographic, domain.SeverityMedium),
	}

	// Configure rules
	maxAmount := 1000.0
	rules[0].Parameters.MaxAmount = &maxAmount

	maxTx := 5
	timeWindow := 1 * time.Hour
	rules[1].Parameters.MaxTransactions = &maxTx
	rules[1].Parameters.TimeWindow = &timeWindow

	rules[2].Parameters.BlockedCountries = []string{"KP"}

	engine := usecase.NewRulesEngine(rules)

	userID := uuid.New()
	transaction := domain.NewTransaction(userID, 500, "USD")
	context := &domain.EvaluationContext{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.EvaluateTransaction(transaction, context)
	}
}
