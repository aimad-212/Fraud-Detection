package tests

import (
	"testing"
	"time"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/infrastructure/ml"
	"fraud-detection/internal/repository"
	"fraud-detection/internal/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHybridDecisionService(t *testing.T) {
	// Create test dependencies
	logger := &testLogger{}
	transactionRepo := repository.NewInMemoryTransactionRepository()

	// Create rules engine with default rules
	ruleLoader := usecase.NewRuleLoader()
	rules := ruleLoader.CreateDefaultRules()
	rulesEngine := usecase.NewRulesEngine(rules)

	// Create feature engineer
	featureEngineer := usecase.NewFeatureEngineer()

	// Create mock ML model
	mlModel := ml.NewMockModelScorer(0.3) // Default moderate risk score

	// Create hybrid decision service
	decisionService := usecase.NewDecisionServiceWithML(
		rulesEngine,
		featureEngineer,
		transactionRepo,
		logger,
		mlModel,
		nil, // no cache for testing
	)

	t.Run("Hybrid Decision with ML", func(t *testing.T) {
		// Create test transaction
		transaction := &domain.Transaction{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			Amount:    1500.0, // High amount
			Currency:  "USD",
			Merchant:  "Test Store",
			Country:   "US",
			CreatedAt: time.Now(),
		}

		// Evaluate transaction
		result, err := decisionService.EvaluateTransaction(transaction)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify hybrid scoring
		assert.GreaterOrEqual(t, result.RuleScore, 0.0)
		assert.LessOrEqual(t, result.RuleScore, 1.0)
		assert.GreaterOrEqual(t, result.MLScore, 0.0)
		assert.LessOrEqual(t, result.MLScore, 1.0)
		assert.GreaterOrEqual(t, result.RiskScore, 0.0)
		assert.LessOrEqual(t, result.RiskScore, 1.0)

		// Verify decision is one of the valid options
		assert.Contains(t, []domain.Decision{
			domain.DecisionApprove,
			domain.DecisionReject,
			domain.DecisionFlag,
		}, result.Decision)

		// Verify explanation contains both rule and ML information
		assert.Contains(t, result.Explanation, "Scoring Breakdown")
		assert.Contains(t, result.Explanation, "Rule-based Score")
		assert.Contains(t, result.Explanation, "ML Model Score")
	})

	t.Run("Decision Service without ML", func(t *testing.T) {
		// Create decision service without ML
		decisionServiceNoML := usecase.NewDecisionServiceWithML(
			rulesEngine,
			featureEngineer,
			transactionRepo,
			logger,
			nil, // no ML model
			nil,
		)

		transaction := &domain.Transaction{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			Amount:    100.0,
			Currency:  "USD",
			Merchant:  "Safe Store",
			Country:   "US",
			CreatedAt: time.Now(),
		}

		result, err := decisionServiceNoML.EvaluateTransaction(transaction)
		require.NoError(t, err)
		require.NotNil(t, result)

		// ML score should be 0 when ML is disabled
		assert.Equal(t, 0.0, result.MLScore)
		assert.Equal(t, result.RuleScore, result.RiskScore) // Should be equal when ML is disabled
	})

	t.Run("ML Weight Adjustment", func(t *testing.T) {
		// Test dynamic weight adjustment
		originalRuleWeight := 0.6
		originalMLWeight := 0.4

		// Adjust weights
		decisionService.SetMLWeights(0.3, 0.7)

		// Get status to verify weights were updated
		status := decisionService.GetMLStatus()
		assert.InDelta(t, 0.3, status["rule_weight"], 0.01)
		assert.InDelta(t, 0.7, status["ml_weight"], 0.01)

		// Reset to original weights
		decisionService.SetMLWeights(originalRuleWeight, originalMLWeight)
	})

	t.Run("ML Enable/Disable", func(t *testing.T) {
		// Test ML enable/disable functionality
		status := decisionService.GetMLStatus()
		assert.True(t, status["ml_enabled"].(bool))
		assert.True(t, status["ml_available"].(bool))

		// Disable ML
		decisionService.EnableML(false)
		status = decisionService.GetMLStatus()
		assert.False(t, status["ml_enabled"].(bool))
		assert.True(t, status["ml_available"].(bool)) // Still available, just disabled

		// Re-enable ML
		decisionService.EnableML(true)
		status = decisionService.GetMLStatus()
		assert.True(t, status["ml_enabled"].(bool))
	})
}

func TestHybridScoringLogic(t *testing.T) {
	logger := &testLogger{}
	transactionRepo := repository.NewInMemoryTransactionRepository()

	ruleLoader := usecase.NewRuleLoader()
	rules := ruleLoader.CreateDefaultRules()
	rulesEngine := usecase.NewRulesEngine(rules)

	featureEngineer := usecase.NewFeatureEngineer()
	mlModel := ml.NewMockModelScorer(0.3) // Default moderate risk score

	decisionService := usecase.NewDecisionServiceWithML(
		rulesEngine,
		featureEngineer,
		transactionRepo,
		logger,
		mlModel,
		nil,
	)

	t.Run("High Risk Transaction", func(t *testing.T) {
		// Create a high-risk transaction
		transaction := &domain.Transaction{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			Amount:    10000.0, // Very high amount
			Currency:  "USD",
			Merchant:  "Unknown Store",
			Country:   "NG", // Nigeria (high risk)
			CreatedAt: time.Now(),
		}

		result, err := decisionService.EvaluateTransaction(transaction)
		require.NoError(t, err)

		// Should have elevated risk scores
		assert.Greater(t, result.RiskScore, 0.3)
		assert.Contains(t, []domain.Decision{
			domain.DecisionReject,
			domain.DecisionFlag,
		}, result.Decision)
	})

	t.Run("Low Risk Transaction", func(t *testing.T) {
		// Create a low-risk transaction
		transaction := &domain.Transaction{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			Amount:    25.0, // Low amount
			Currency:  "USD",
			Merchant:  "Amazon",
			Country:   "US", // Low risk country
			CreatedAt: time.Now(),
		}

		result, err := decisionService.EvaluateTransaction(transaction)
		require.NoError(t, err)

		// Should have low risk scores
		assert.Less(t, result.RiskScore, 0.4)
		assert.Equal(t, domain.DecisionApprove, result.Decision)
	})
}

// Test logger implementation
type testLogger struct{}

func (l *testLogger) Error(msg string, args ...interface{}) {}
func (l *testLogger) Warn(msg string, args ...interface{})  {}
func (l *testLogger) Info(msg string, args ...interface{})  {}
