package usecase

import (
	"fmt"

	"fraud-detection/internal/infrastructure/messaging"
	"fraud-detection/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TransactionEventProcessor processes transaction events from NATS
type TransactionEventProcessor struct {
	repo   repository.TransactionRepository
	logger *zap.Logger
}

// NewTransactionEventProcessor creates a new transaction event processor
func NewTransactionEventProcessor(repo repository.TransactionRepository, logger *zap.Logger) *TransactionEventProcessor {
	return &TransactionEventProcessor{
		repo:   repo,
		logger: logger,
	}
}

// ProcessTransactionEvent processes a transaction event received from NATS
func (p *TransactionEventProcessor) ProcessTransactionEvent(event *messaging.TransactionEvent) error {
	p.logger.Info("Processing transaction event",
		zap.String("event_type", event.EventType),
		zap.String("transaction_id", event.ID),
		zap.Float64("amount", event.Amount))

	switch event.EventType {
	case "created":
		return p.processCreatedEvent(event)
	case "processed":
		return p.processProcessedEvent(event)
	case "updated":
		return p.processUpdatedEvent(event)
	default:
		p.logger.Warn("Unknown event type received",
			zap.String("event_type", event.EventType),
			zap.String("transaction_id", event.ID))
		return nil // Don't fail on unknown event types
	}
}

// processCreatedEvent handles transaction created events
func (p *TransactionEventProcessor) processCreatedEvent(event *messaging.TransactionEvent) error {
	// For created events, we might want to do additional processing
	// like updating user profiles, sending notifications, etc.
	// For now, just log the event
	p.logger.Info("Transaction created event processed",
		zap.String("transaction_id", event.ID),
		zap.String("user_id", event.UserID),
		zap.Float64("amount", event.Amount))

	// Could add logic here for:
	// - Updating user transaction counts
	// - Triggering real-time alerts
	// - Updating caches

	return nil
}

// processProcessedEvent handles transaction processed events
func (p *TransactionEventProcessor) processProcessedEvent(event *messaging.TransactionEvent) error {
	// For processed events, we might want to:
	// - Update decision logs
	// - Trigger downstream processing
	// - Send notifications based on risk score

	p.logger.Info("Transaction processed event received",
		zap.String("transaction_id", event.ID),
		zap.Float64("risk_score", event.RiskScore),
		zap.String("status", event.Status))

	// Get the transaction to verify it exists and get full details
	transactionID, err := uuid.Parse(event.ID)
	if err != nil {
		return fmt.Errorf("invalid transaction ID in event: %w", err)
	}

	transaction, err := p.repo.GetByID(transactionID)
	if err != nil {
		p.logger.Error("Failed to get transaction for processed event",
			zap.Error(err),
			zap.String("transaction_id", event.ID))
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	// Verify the event data matches our transaction
	if transaction.RiskScore != event.RiskScore {
		p.logger.Warn("Risk score mismatch in processed event",
			zap.String("transaction_id", event.ID),
			zap.Float64("event_risk_score", event.RiskScore),
			zap.Float64("stored_risk_score", transaction.RiskScore))
	}

	if string(transaction.Status) != event.Status {
		p.logger.Warn("Status mismatch in processed event",
			zap.String("transaction_id", event.ID),
			zap.String("event_status", event.Status),
			zap.String("stored_status", string(transaction.Status)))
	}

	// Could add logic here for:
	// - Logging decisions to audit trail
	// - Triggering compliance checks
	// - Sending alerts for high-risk transactions

	return nil
}

// processUpdatedEvent handles transaction updated events
func (p *TransactionEventProcessor) processUpdatedEvent(event *messaging.TransactionEvent) error {
	// For updated events, we might want to:
	// - Update external systems
	// - Trigger re-processing if needed
	// - Send status change notifications

	p.logger.Info("Transaction updated event received",
		zap.String("transaction_id", event.ID),
		zap.String("status", event.Status))

	// Get the transaction to verify the update
	transactionID, err := uuid.Parse(event.ID)
	if err != nil {
		return fmt.Errorf("invalid transaction ID in event: %w", err)
	}

	transaction, err := p.repo.GetByID(transactionID)
	if err != nil {
		p.logger.Error("Failed to get transaction for updated event",
			zap.Error(err),
			zap.String("transaction_id", event.ID))
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	// Verify the event data matches our transaction
	if string(transaction.Status) != event.Status {
		p.logger.Warn("Status mismatch in updated event",
			zap.String("transaction_id", event.ID),
			zap.String("event_status", event.Status),
			zap.String("stored_status", string(transaction.Status)))
	}

	// Could add logic here for:
	// - Notifying payment processors
	// - Updating user dashboards
	// - Triggering refund processes

	return nil
}
