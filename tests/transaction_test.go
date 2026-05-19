package tests

import (
	"testing"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/repository"
	"fraud-detection/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestTransactionDomain(t *testing.T) {
	// Test NewTransaction
	userID := uuid.New()
	transaction := domain.NewTransaction(userID, 100.0, "USD")

	assert.NotNil(t, transaction)
	assert.Equal(t, userID, transaction.UserID)
	assert.Equal(t, 100.0, transaction.Amount)
	assert.Equal(t, "USD", transaction.Currency)
	assert.Equal(t, domain.StatusPending, transaction.Status)
	assert.Equal(t, 0.0, transaction.RiskScore)
	assert.NotZero(t, transaction.CreatedAt)
	assert.NotZero(t, transaction.UpdatedAt)

	// Test UpdateStatus
	transaction.UpdateStatus(domain.StatusApproved)
	assert.Equal(t, domain.StatusApproved, transaction.Status)

	// Test UpdateRiskScore
	transaction.UpdateRiskScore(0.75)
	assert.Equal(t, 0.75, transaction.RiskScore)

	// Test IsHighRisk
	assert.True(t, transaction.IsHighRisk(0.5))
	assert.False(t, transaction.IsHighRisk(0.9))
}

func TestInMemoryTransactionRepository(t *testing.T) {
	repo := repository.NewInMemoryTransactionRepository()
	logger := zaptest.NewLogger(t)
	uc := usecase.NewTransactionUseCase(repo, nil, logger)

	// Test CreateTransaction
	userID := uuid.New()
	transaction, err := uc.CreateTransaction(userID, 250.0, "USD", "Test Merchant")

	assert.NoError(t, err)
	assert.NotNil(t, transaction)
	assert.Equal(t, userID, transaction.UserID)
	assert.Equal(t, 250.0, transaction.Amount)
	assert.Equal(t, "USD", transaction.Currency)
	assert.Equal(t, "Test Merchant", transaction.Merchant)

	// Test GetTransaction
	retrieved, err := uc.GetTransaction(transaction.ID)
	assert.NoError(t, err)
	assert.Equal(t, transaction.ID, retrieved.ID)
	assert.Equal(t, transaction.Amount, retrieved.Amount)

	// Test GetUserTransactions
	userTransactions, err := uc.GetUserTransactions(userID)
	assert.NoError(t, err)
	assert.Len(t, userTransactions, 1)
	assert.Equal(t, transaction.ID, userTransactions[0].ID)

	// Test UpdateTransactionStatus
	err = uc.UpdateTransactionStatus(transaction.ID, domain.StatusApproved)
	assert.NoError(t, err)

	updated, err := uc.GetTransaction(transaction.ID)
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusApproved, updated.Status)

	// Test ListTransactions
	transactions, err := uc.ListTransactions(10, 0)
	assert.NoError(t, err)
	assert.Len(t, transactions, 1)

	// Test DeleteTransaction
	err = uc.DeleteTransaction(transaction.ID)
	assert.NoError(t, err)

	// Verify deletion
	_, err = uc.GetTransaction(transaction.ID)
	assert.Error(t, err)
}

func TestTransactionUseCaseValidation(t *testing.T) {
	repo := repository.NewInMemoryTransactionRepository()
	logger := zaptest.NewLogger(t)
	uc := usecase.NewTransactionUseCase(repo, nil, logger)

	userID := uuid.New()

	// Test negative amount
	_, err := uc.CreateTransaction(userID, -100.0, "USD", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")

	// Test invalid currency length
	_, err = uc.CreateTransaction(userID, 100.0, "US", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "3-letter code")

	// Test valid transaction
	transaction, err := uc.CreateTransaction(userID, 100.0, "USD", "")
	assert.NoError(t, err)
	assert.NotNil(t, transaction)
}

func TestFraudDetectionProcessing(t *testing.T) {
	repo := repository.NewInMemoryTransactionRepository()
	logger := zaptest.NewLogger(t)
	uc := usecase.NewTransactionUseCase(repo, nil, logger)

	userID := uuid.New()

	// Test low-risk transaction
	lowRiskTx, err := uc.CreateTransaction(userID, 50.0, "USD", "normal-merchant")
	assert.NoError(t, err)

	err = uc.ProcessTransaction(lowRiskTx)
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusApproved, lowRiskTx.Status)
	assert.True(t, lowRiskTx.RiskScore < 0.5)

	// Test high-risk transaction
	highRiskTx, err := uc.CreateTransaction(userID, 6000.0, "USD", "suspicious-merchant")
	assert.NoError(t, err)

	err = uc.ProcessTransaction(highRiskTx)
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusRejected, highRiskTx.Status)
	assert.True(t, highRiskTx.RiskScore >= 0.8)

	// Test medium-risk transaction
	mediumRiskTx, err := uc.CreateTransaction(userID, 1500.0, "USD", "normal-merchant")
	assert.NoError(t, err)

	err = uc.ProcessTransaction(mediumRiskTx)
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusFlagged, mediumRiskTx.Status)
	assert.True(t, mediumRiskTx.RiskScore >= 0.3 && mediumRiskTx.RiskScore < 0.8)
}

func BenchmarkTransactionCreation(b *testing.B) {
	repo := repository.NewInMemoryTransactionRepository()
	logger := zaptest.NewLogger(b)
	uc := usecase.NewTransactionUseCase(repo, nil, logger)

	userID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = uc.CreateTransaction(userID, 100.0, "USD", "Benchmark Merchant")
	}
}

func BenchmarkTransactionProcessing(b *testing.B) {
	repo := repository.NewInMemoryTransactionRepository()
	logger := zaptest.NewLogger(b)
	uc := usecase.NewTransactionUseCase(repo, nil, logger)

	userID := uuid.New()
	transaction, _ := uc.CreateTransaction(userID, 100.0, "USD", "Benchmark Merchant")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = uc.ProcessTransaction(transaction)
	}
}
