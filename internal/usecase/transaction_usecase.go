package usecase

import (
	"errors"
	"fmt"
	"time"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/infrastructure/monitoring"
	"fraud-detection/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TransactionPublisher defines the interface for publishing transaction events
type TransactionPublisher interface {
	PublishTransactionCreated(transaction *domain.Transaction) error
	PublishTransactionProcessed(transaction *domain.Transaction) error
	PublishTransactionUpdated(transaction *domain.Transaction) error
}

// TransactionUseCase defines the business logic for transaction operations
type TransactionUseCase interface {
	CreateTransaction(userID uuid.UUID, amount float64, currency string, merchant string) (*domain.Transaction, error)
	GetTransaction(id uuid.UUID) (*domain.Transaction, error)
	GetUserTransactions(userID uuid.UUID) ([]*domain.Transaction, error)
	UpdateTransactionStatus(id uuid.UUID, status domain.TransactionStatus) error
	DeleteTransaction(id uuid.UUID) error
	ListTransactions(limit, offset int) ([]*domain.Transaction, error)
	ProcessTransaction(transaction *domain.Transaction) error
}

// TransactionUseCaseImpl implements TransactionUseCase
type TransactionUseCaseImpl struct {
	repo            repository.TransactionRepository
	publisher       TransactionPublisher
	logger          *zap.Logger
	metrics         *monitoring.Metrics
	decisionService *DecisionService
}

// NewTransactionUseCase creates a new transaction use case
func NewTransactionUseCase(repo repository.TransactionRepository, publisher TransactionPublisher, logger *zap.Logger) TransactionUseCase {
	return &TransactionUseCaseImpl{
		repo:            repo,
		publisher:       publisher,
		logger:          logger,
		metrics:         nil, // Will be set later if monitoring is enabled
		decisionService: nil, // Will be set later if advanced decision making is enabled
	}
}

// SetDecisionService sets the decision service for advanced fraud detection
func (uc *TransactionUseCaseImpl) SetDecisionService(decisionService *DecisionService) {
	uc.decisionService = decisionService
}

// SetMetrics sets the metrics collector for the use case
func (uc *TransactionUseCaseImpl) SetMetrics(metrics *monitoring.Metrics) {
	uc.metrics = metrics
}

// CreateTransaction creates a new transaction with basic validation
func (uc *TransactionUseCaseImpl) CreateTransaction(userID uuid.UUID, amount float64, currency string, merchant string) (*domain.Transaction, error) {
	start := time.Now()

	// Basic validation
	if amount <= 0 {
		if uc.metrics != nil {
			uc.metrics.RecordProcessingError()
		}
		return nil, errors.New("transaction amount must be positive")
	}

	if currency == "" {
		currency = "USD" // Default currency
	}

	if len(currency) != 3 {
		if uc.metrics != nil {
			uc.metrics.RecordProcessingError()
		}
		return nil, errors.New("currency must be a 3-letter code")
	}

	// Create transaction
	transaction := domain.NewTransaction(userID, amount, currency)
	if merchant != "" {
		transaction.Merchant = merchant
	}

	// Save to repository
	err := uc.repo.Create(transaction)
	if err != nil {
		if uc.metrics != nil {
			uc.metrics.RecordProcessingError()
		}
		uc.logger.Error("Failed to create transaction",
			zap.Error(err),
			zap.String("user_id", userID.String()),
			zap.Float64("amount", amount))
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// Record metrics
	if uc.metrics != nil {
		uc.metrics.RecordTransactionCreated()
		duration := time.Since(start)
		uc.metrics.RecordTransactionProcessed(duration, 0.0, "created") // Risk score will be calculated later
	}

	uc.logger.Info("Transaction created successfully",
		zap.String("transaction_id", transaction.ID.String()),
		zap.String("user_id", userID.String()),
		zap.Float64("amount", amount))

	// Publish transaction created event
	if uc.publisher != nil {
		if err := uc.publisher.PublishTransactionCreated(transaction); err != nil {
			uc.logger.Error("Failed to publish transaction created event",
				zap.Error(err),
				zap.String("transaction_id", transaction.ID.String()))
			// Don't fail the transaction creation if publishing fails
		}
	}

	return transaction, nil
}

// GetTransaction retrieves a transaction by ID
func (uc *TransactionUseCaseImpl) GetTransaction(id uuid.UUID) (*domain.Transaction, error) {
	transaction, err := uc.repo.GetByID(id)
	if err != nil {
		uc.logger.Error("Failed to get transaction",
			zap.Error(err),
			zap.String("transaction_id", id.String()))
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	return transaction, nil
}

// GetUserTransactions retrieves all transactions for a user
func (uc *TransactionUseCaseImpl) GetUserTransactions(userID uuid.UUID) ([]*domain.Transaction, error) {
	transactions, err := uc.repo.GetByUserID(userID.String())
	if err != nil {
		uc.logger.Error("Failed to get user transactions",
			zap.Error(err),
			zap.String("user_id", userID.String()))
		return nil, fmt.Errorf("failed to get user transactions: %w", err)
	}

	return transactions, nil
}

// UpdateTransactionStatus updates the status of a transaction
func (uc *TransactionUseCaseImpl) UpdateTransactionStatus(id uuid.UUID, status domain.TransactionStatus) error {
	start := time.Now()

	// Get existing transaction
	transaction, err := uc.repo.GetByID(id)
	if err != nil {
		if uc.metrics != nil {
			uc.metrics.RecordProcessingError()
		}
		return fmt.Errorf("failed to get transaction for update: %w", err)
	}

	// Update status
	transaction.UpdateStatus(status)

	// Save changes
	err = uc.repo.Update(transaction)
	if err != nil {
		if uc.metrics != nil {
			uc.metrics.RecordProcessingError()
		}
		uc.logger.Error("Failed to update transaction status",
			zap.Error(err),
			zap.String("transaction_id", id.String()),
			zap.String("new_status", string(status)))
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	// Record metrics
	if uc.metrics != nil {
		duration := time.Since(start)
		uc.metrics.RecordTransactionProcessed(duration, transaction.RiskScore, string(status))
	}

	uc.logger.Info("Transaction status updated",
		zap.String("transaction_id", id.String()),
		zap.String("old_status", string(transaction.Status)),
		zap.String("new_status", string(status)))

	// Publish transaction updated event
	if uc.publisher != nil {
		if err := uc.publisher.PublishTransactionUpdated(transaction); err != nil {
			uc.logger.Error("Failed to publish transaction updated event",
				zap.Error(err),
				zap.String("transaction_id", id.String()))
			// Don't fail the transaction update if publishing fails
		}
	}

	return nil
}

// DeleteTransaction removes a transaction
func (uc *TransactionUseCaseImpl) DeleteTransaction(id uuid.UUID) error {
	err := uc.repo.Delete(id)
	if err != nil {
		if uc.metrics != nil {
			uc.metrics.RecordProcessingError()
		}
		uc.logger.Error("Failed to delete transaction",
			zap.Error(err),
			zap.String("transaction_id", id.String()))
		return fmt.Errorf("failed to delete transaction: %w", err)
	}

	// Record metrics
	if uc.metrics != nil {
		uc.metrics.RecordTransactionDeleted()
	}

	uc.logger.Info("Transaction deleted",
		zap.String("transaction_id", id.String()))

	return nil
}

// ListTransactions returns a paginated list of transactions
func (uc *TransactionUseCaseImpl) ListTransactions(limit, offset int) ([]*domain.Transaction, error) {
	if limit <= 0 {
		limit = 50 // Default limit
	}
	if limit > 100 {
		limit = 100 // Maximum limit to prevent memory issues
	}
	if offset < 0 {
		offset = 0
	}

	transactions, err := uc.repo.ListTransactions(limit, offset)
	if err != nil {
		uc.logger.Error("Failed to list transactions",
			zap.Error(err),
			zap.Int("limit", limit),
			zap.Int("offset", offset))
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	return transactions, nil
}

// ProcessTransaction applies fraud detection logic using decision service if available
func (uc *TransactionUseCaseImpl) ProcessTransaction(transaction *domain.Transaction) error {
	start := time.Now()

	var riskScore float64
	var newStatus domain.TransactionStatus

	// Use advanced decision service if available, otherwise fall back to basic rules
	if uc.decisionService != nil {
		// Use advanced decision service
		decisionResult, err := uc.decisionService.EvaluateTransaction(transaction)
		if err != nil {
			uc.logger.Error("Failed to evaluate transaction with decision service",
				zap.Error(err),
				zap.String("transaction_id", transaction.ID.String()))
			// Fall back to basic processing
			return uc.processTransactionBasic(transaction, start)
		}

		riskScore = decisionResult.RiskScore
		// Convert decision to status
		switch decisionResult.Decision {
		case domain.DecisionApprove:
			newStatus = domain.StatusApproved
		case domain.DecisionReject:
			newStatus = domain.StatusRejected
		case domain.DecisionFlag:
			newStatus = domain.StatusFlagged
		default:
			newStatus = domain.StatusApproved
		}

		uc.logger.Info("Transaction processed with decision service",
			zap.String("transaction_id", transaction.ID.String()),
			zap.String("decision", string(decisionResult.Decision)),
			zap.Float64("risk_score", riskScore),
			zap.String("explanation", decisionResult.Explanation))
	} else {
		// Use basic processing
		return uc.processTransactionBasic(transaction, start)
	}

	// Update transaction
	transaction.UpdateRiskScore(riskScore)
	transaction.UpdateStatus(newStatus)

	// Save updated transaction
	err := uc.repo.Update(transaction)
	if err != nil {
		if uc.metrics != nil {
			uc.metrics.RecordProcessingError()
		}
		uc.logger.Error("Failed to process transaction",
			zap.Error(err),
			zap.String("transaction_id", transaction.ID.String()),
			zap.Float64("risk_score", riskScore))
		return fmt.Errorf("failed to process transaction: %w", err)
	}

	// Record metrics
	if uc.metrics != nil {
		duration := time.Since(start)
		uc.metrics.RecordTransactionProcessed(duration, riskScore, string(newStatus))
	}

	uc.logger.Info("Transaction processed",
		zap.String("transaction_id", transaction.ID.String()),
		zap.Float64("amount", transaction.Amount),
		zap.Float64("risk_score", riskScore),
		zap.String("status", string(newStatus)))

	// Publish transaction processed event
	if uc.publisher != nil {
		if err := uc.publisher.PublishTransactionProcessed(transaction); err != nil {
			uc.logger.Error("Failed to publish transaction processed event",
				zap.Error(err),
				zap.String("transaction_id", transaction.ID.String()))
			// Don't fail the transaction processing if publishing fails
		}
	}

	return nil
}

// processTransactionBasic applies basic fraud detection logic (fallback)
func (uc *TransactionUseCaseImpl) processTransactionBasic(transaction *domain.Transaction, start time.Time) error {
	// Basic rule-based fraud detection
	riskScore := 0.0

	// Rule 1: High amount (> $1000)
	if transaction.Amount > 1000 {
		riskScore += 0.3
	}

	// Rule 2: Very high amount (> $5000)
	if transaction.Amount > 5000 {
		riskScore += 0.4
	}

	// Rule 3: Suspicious merchant (placeholder logic)
	if transaction.Merchant == "suspicious-merchant" {
		riskScore += 0.5
	}

	// Rule 4: International transaction (placeholder)
	if transaction.Country != "" && transaction.Country != "US" {
		riskScore += 0.1
	}

	// Cap risk score at 1.0
	if riskScore > 1.0 {
		riskScore = 1.0
	}

	// Update risk score
	transaction.UpdateRiskScore(riskScore)

	// Determine status based on risk score
	var newStatus domain.TransactionStatus
	if riskScore >= 0.8 {
		newStatus = domain.StatusRejected
	} else if riskScore >= 0.3 {
		newStatus = domain.StatusFlagged
	} else {
		newStatus = domain.StatusApproved
	}

	transaction.UpdateStatus(newStatus)

	// Save updated transaction
	err := uc.repo.Update(transaction)
	if err != nil {
		if uc.metrics != nil {
			uc.metrics.RecordProcessingError()
		}
		uc.logger.Error("Failed to process transaction",
			zap.Error(err),
			zap.String("transaction_id", transaction.ID.String()),
			zap.Float64("risk_score", riskScore))
		return fmt.Errorf("failed to process transaction: %w", err)
	}

	// Record metrics
	if uc.metrics != nil {
		duration := time.Since(start)
		uc.metrics.RecordTransactionProcessed(duration, riskScore, string(newStatus))
	}

	uc.logger.Info("Transaction processed with basic rules",
		zap.String("transaction_id", transaction.ID.String()),
		zap.Float64("amount", transaction.Amount),
		zap.Float64("risk_score", riskScore),
		zap.String("status", string(newStatus)))

	// Publish transaction processed event
	if uc.publisher != nil {
		if err := uc.publisher.PublishTransactionProcessed(transaction); err != nil {
			uc.logger.Error("Failed to publish transaction processed event",
				zap.Error(err),
				zap.String("transaction_id", transaction.ID.String()))
			// Don't fail the transaction processing if publishing fails
		}
	}

	return nil
}
