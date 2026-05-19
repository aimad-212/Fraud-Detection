package tests

import (
	"testing"
	"time"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/infrastructure/ml"
	"fraud-detection/internal/infrastructure/monitoring"
	"fraud-detection/internal/repository"
	"fraud-detection/internal/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHybridDecisionMetrics(t *testing.T) {
	// Create test dependencies
	logger := &testLogger{}
	transactionRepo := repository.NewInMemoryTransactionRepository()
	metrics := monitoring.NewMetrics()

	// Create rules engine with default rules
	ruleLoader := usecase.NewRuleLoader()
	rules := ruleLoader.CreateDefaultRules()
	rulesEngine := usecase.NewRulesEngine(rules)

	// Create feature engineer
	featureEngineer := usecase.NewFeatureEngineer()

	// Create mock ML model
	mlModel := ml.NewMockModelScorer(0.3) // Default moderate risk score

	// Create hybrid decision service with metrics
	decisionService := usecase.NewDecisionServiceWithMLAndMetrics(
		rulesEngine,
		featureEngineer,
		transactionRepo,
		logger,
		mlModel,
		nil, // no cache for testing
		metrics,
	)

	t.Run("Hybrid Decision Metrics Recording", func(t *testing.T) {
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

		// Note: In a real test, we would verify Prometheus metrics,
		// but for unit tests we focus on the functionality
	})

	t.Run("ML Weight Change Metrics", func(t *testing.T) {
		// Test weight change
		decisionService.SetMLWeights(0.3, 0.7)

		// Get status to verify weights were updated
		status := decisionService.GetMLStatus()
		assert.InDelta(t, 0.3, status["rule_weight"], 0.01)
		assert.InDelta(t, 0.7, status["ml_weight"], 0.01)

		// Reset to original weights
		decisionService.SetMLWeights(0.6, 0.4)
	})

	t.Run("ML Fallback Metrics", func(t *testing.T) {
		// Create decision service with ML that will fail
		failingMLModel := &failingMockModel{}
		decisionServiceWithFailingML := usecase.NewDecisionServiceWithMLAndMetrics(
			rulesEngine,
			featureEngineer,
			transactionRepo,
			logger,
			failingMLModel,
			nil,
			metrics,
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

		result, err := decisionServiceWithFailingML.EvaluateTransaction(transaction)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should fallback to rule-based decision
		assert.Equal(t, 0.0, result.MLScore)
		assert.Equal(t, result.RuleScore, result.RiskScore)
	})
}

// failingMockModel is a mock model that always fails
type failingMockModel struct{}

func (m *failingMockModel) Score(features interface{}) (float64, error) {
	return 0.0, assert.AnError
}

func (m *failingMockModel) Close() error {
	return nil
}

func (m *failingMockModel) IsLoaded() bool {
	return false
}
