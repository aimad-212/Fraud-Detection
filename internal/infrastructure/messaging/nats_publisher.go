package messaging

import (
	"encoding/json"
	"fmt"
	"time"

	"fraud-detection/internal/domain"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// TransactionEvent represents a transaction event for messaging
type TransactionEvent struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	Merchant  string    `json:"merchant"`
	Country   string    `json:"country"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	DeviceID  string    `json:"device_id"`
	Status    string    `json:"status"`
	RiskScore float64   `json:"risk_score"`
	CreatedAt time.Time `json:"created_at"`
	EventType string    `json:"event_type"` // "created", "updated", "processed"
	Timestamp time.Time `json:"timestamp"`
}

// NATSPublisher handles publishing messages to NATS
type NATSPublisher struct {
	nc      *nats.Conn
	subject string
	logger  *zap.Logger
}

// NewNATSPublisher creates a new NATS publisher
func NewNATSPublisher(url, subject string, logger *zap.Logger) (*NATSPublisher, error) {
	nc, err := nats.Connect(url,
		nats.MaxReconnects(5),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			logger.Error("NATS disconnected", zap.Error(err))
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", zap.String("url", nc.ConnectedUrl()))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	logger.Info("Connected to NATS", zap.String("url", url), zap.String("subject", subject))

	return &NATSPublisher{
		nc:      nc,
		subject: subject,
		logger:  logger,
	}, nil
}

// PublishTransactionCreated publishes a transaction created event
func (p *NATSPublisher) PublishTransactionCreated(transaction *domain.Transaction) error {
	event := TransactionEvent{
		ID:        transaction.ID.String(),
		UserID:    transaction.UserID.String(),
		Amount:    transaction.Amount,
		Currency:  transaction.Currency,
		Merchant:  transaction.Merchant,
		Country:   transaction.Country,
		IPAddress: transaction.IPAddress,
		UserAgent: transaction.UserAgent,
		DeviceID:  transaction.DeviceID,
		Status:    string(transaction.Status),
		RiskScore: transaction.RiskScore,
		CreatedAt: transaction.CreatedAt,
		EventType: "created",
		Timestamp: time.Now(),
	}

	return p.publishEvent(event)
}

// PublishTransactionProcessed publishes a transaction processed event
func (p *NATSPublisher) PublishTransactionProcessed(transaction *domain.Transaction) error {
	event := TransactionEvent{
		ID:        transaction.ID.String(),
		UserID:    transaction.UserID.String(),
		Amount:    transaction.Amount,
		Currency:  transaction.Currency,
		Merchant:  transaction.Merchant,
		Country:   transaction.Country,
		IPAddress: transaction.IPAddress,
		UserAgent: transaction.UserAgent,
		DeviceID:  transaction.DeviceID,
		Status:    string(transaction.Status),
		RiskScore: transaction.RiskScore,
		CreatedAt: transaction.CreatedAt,
		EventType: "processed",
		Timestamp: time.Now(),
	}

	return p.publishEvent(event)
}

// PublishTransactionUpdated publishes a transaction updated event
func (p *NATSPublisher) PublishTransactionUpdated(transaction *domain.Transaction) error {
	event := TransactionEvent{
		ID:        transaction.ID.String(),
		UserID:    transaction.UserID.String(),
		Amount:    transaction.Amount,
		Currency:  transaction.Currency,
		Merchant:  transaction.Merchant,
		Country:   transaction.Country,
		IPAddress: transaction.IPAddress,
		UserAgent: transaction.UserAgent,
		DeviceID:  transaction.DeviceID,
		Status:    string(transaction.Status),
		RiskScore: transaction.RiskScore,
		CreatedAt: transaction.CreatedAt,
		EventType: "updated",
		Timestamp: time.Now(),
	}

	return p.publishEvent(event)
}

// publishEvent publishes an event to NATS
func (p *NATSPublisher) publishEvent(event TransactionEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = p.nc.Publish(p.subject, data)
	if err != nil {
		p.logger.Error("Failed to publish NATS message",
			zap.Error(err),
			zap.String("subject", p.subject),
			zap.String("event_type", event.EventType),
			zap.String("transaction_id", event.ID))
		return fmt.Errorf("failed to publish message: %w", err)
	}

	p.logger.Info("Published NATS message",
		zap.String("subject", p.subject),
		zap.String("event_type", event.EventType),
		zap.String("transaction_id", event.ID),
		zap.Float64("amount", event.Amount))

	return nil
}

// Close closes the NATS connection
func (p *NATSPublisher) Close() {
	if p.nc != nil {
		p.nc.Close()
		p.logger.Info("NATS publisher connection closed")
	}
}

// IsConnected returns true if connected to NATS
func (p *NATSPublisher) IsConnected() bool {
	return p.nc != nil && p.nc.IsConnected()
}
