package tests

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/infrastructure/messaging"
	"fraud-detection/internal/repository"
	"fraud-detection/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// MockNATSServer simulates a NATS server for testing
type MockNATSServer struct {
	mu       sync.RWMutex
	messages map[string][]messaging.TransactionEvent
}

func NewMockNATSServer() *MockNATSServer {
	return &MockNATSServer{
		messages: make(map[string][]messaging.TransactionEvent),
	}
}

func (m *MockNATSServer) Publish(subject string, event messaging.TransactionEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages[subject] = append(m.messages[subject], event)
}

func (m *MockNATSServer) GetMessages(subject string) []messaging.TransactionEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]messaging.TransactionEvent{}, m.messages[subject]...)
}

func (m *MockNATSServer) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make(map[string][]messaging.TransactionEvent)
}

// MockNATSPublisher implements the TransactionPublisher interface for testing
type MockNATSPublisher struct {
	server  *MockNATSServer
	subject string
}

func NewMockNATSPublisher(server *MockNATSServer, subject string) *MockNATSPublisher {
	return &MockNATSPublisher{
		server:  server,
		subject: subject,
	}
}

func (m *MockNATSPublisher) PublishTransactionCreated(transaction *domain.Transaction) error {
	event := messaging.TransactionEvent{
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
	m.server.Publish(m.subject, event)
	return nil
}

func (m *MockNATSPublisher) PublishTransactionProcessed(transaction *domain.Transaction) error {
	event := messaging.TransactionEvent{
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
	m.server.Publish(m.subject, event)
	return nil
}

func (m *MockNATSPublisher) PublishTransactionUpdated(transaction *domain.Transaction) error {
	event := messaging.TransactionEvent{
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
	m.server.Publish(m.subject, event)
	return nil
}

func TestNATSEventPublishing(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockServer := NewMockNATSServer()
	publisher := NewMockNATSPublisher(mockServer, "transactions")

	repo := repository.NewInMemoryTransactionRepository()
	uc := usecase.NewTransactionUseCase(repo, publisher, logger)

	// Create a test transaction
	userID := uuid.New()
	transaction, err := uc.CreateTransaction(userID, 150.50, "USD", "Test Store")
	require.NoError(t, err)
	require.NotNil(t, transaction)

	// Check that creation event was published
	messages := mockServer.GetMessages("transactions")
	require.Len(t, messages, 1)

	event := messages[0]
	assert.Equal(t, "created", event.EventType)
	assert.Equal(t, transaction.ID.String(), event.ID)
	assert.Equal(t, userID.String(), event.UserID)
	assert.Equal(t, 150.50, event.Amount)
	assert.Equal(t, "USD", event.Currency)
	assert.Equal(t, "Test Store", event.Merchant)
	assert.Equal(t, string(domain.StatusPending), event.Status)
	assert.Equal(t, 0.0, event.RiskScore)

	// Process the transaction (should publish processed event)
	err = uc.ProcessTransaction(transaction)
	require.NoError(t, err)

	// Check that processed event was published
	messages = mockServer.GetMessages("transactions")
	require.Len(t, messages, 2)

	processedEvent := messages[1]
	assert.Equal(t, "processed", processedEvent.EventType)
	assert.Equal(t, transaction.ID.String(), processedEvent.ID)
	assert.True(t, processedEvent.RiskScore >= 0) // Should have been calculated

	// Update transaction status (should publish updated event)
	err = uc.UpdateTransactionStatus(transaction.ID, domain.StatusFlagged)
	require.NoError(t, err)

	// Check that updated event was published
	messages = mockServer.GetMessages("transactions")
	require.Len(t, messages, 3)

	updatedEvent := messages[2]
	assert.Equal(t, "updated", updatedEvent.EventType)
	assert.Equal(t, transaction.ID.String(), updatedEvent.ID)
	assert.Equal(t, string(domain.StatusFlagged), updatedEvent.Status)
}

func TestNATSEventProcessing(t *testing.T) {
	logger := zaptest.NewLogger(t)
	repo := repository.NewInMemoryTransactionRepository()
	processor := usecase.NewTransactionEventProcessor(repo, logger)

	// Create a transaction directly in repo for testing
	userID := uuid.New()
	transaction := domain.NewTransaction(userID, 250.75, "USD")
	transaction.Merchant = "Test Merchant"
	transaction.RiskScore = 0.3
	transaction.UpdateStatus(domain.StatusFlagged)
	err := repo.Create(transaction)
	require.NoError(t, err)

	// Create a processed event
	event := messaging.TransactionEvent{
		ID:        transaction.ID.String(),
		UserID:    transaction.UserID.String(),
		Amount:    transaction.Amount,
		Currency:  transaction.Currency,
		Merchant:  transaction.Merchant,
		Status:    string(transaction.Status),
		RiskScore: transaction.RiskScore,
		EventType: "processed",
		Timestamp: time.Now(),
	}

	// Process the event
	err = processor.ProcessTransactionEvent(&event)
	require.NoError(t, err)

	// Verify the transaction exists and has correct data
	retrieved, err := repo.GetByID(transaction.ID)
	require.NoError(t, err)
	assert.Equal(t, transaction.Amount, retrieved.Amount)
	assert.Equal(t, transaction.RiskScore, retrieved.RiskScore)
	assert.Equal(t, transaction.Status, retrieved.Status)
}

func TestNATSEventJSONSerialization(t *testing.T) {
	// Test that events can be properly serialized to/from JSON
	userID := uuid.New()
	transaction := domain.NewTransaction(userID, 100.00, "USD")
	transaction.Merchant = "Test Store"
	transaction.RiskScore = 0.5
	transaction.UpdateStatus(domain.StatusApproved)

	event := messaging.TransactionEvent{
		ID:        transaction.ID.String(),
		UserID:    transaction.UserID.String(),
		Amount:    transaction.Amount,
		Currency:  transaction.Currency,
		Merchant:  transaction.Merchant,
		Status:    string(transaction.Status),
		RiskScore: transaction.RiskScore,
		EventType: "created",
		Timestamp: time.Now(),
	}

	// Serialize to JSON
	data, err := json.Marshal(event)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Deserialize from JSON
	var deserialized messaging.TransactionEvent
	err = json.Unmarshal(data, &deserialized)
	require.NoError(t, err)

	// Verify all fields match
	assert.Equal(t, event.ID, deserialized.ID)
	assert.Equal(t, event.UserID, deserialized.UserID)
	assert.Equal(t, event.Amount, deserialized.Amount)
	assert.Equal(t, event.Currency, deserialized.Currency)
	assert.Equal(t, event.Merchant, deserialized.Merchant)
	assert.Equal(t, event.Status, deserialized.Status)
	assert.Equal(t, event.RiskScore, deserialized.RiskScore)
	assert.Equal(t, event.EventType, deserialized.EventType)
}

func TestNATSEventProcessingUnknownEventType(t *testing.T) {
	logger := zaptest.NewLogger(t)
	repo := repository.NewInMemoryTransactionRepository()
	processor := usecase.NewTransactionEventProcessor(repo, logger)

	event := messaging.TransactionEvent{
		ID:        uuid.New().String(),
		EventType: "unknown",
		Timestamp: time.Now(),
	}

	// Processing unknown event type should not error
	err := processor.ProcessTransactionEvent(&event)
	assert.NoError(t, err)
}

func BenchmarkNATSEventPublishing(b *testing.B) {
	logger := zaptest.NewLogger(b)
	mockServer := NewMockNATSServer()
	publisher := NewMockNATSPublisher(mockServer, "transactions")

	repo := repository.NewInMemoryTransactionRepository()
	uc := usecase.NewTransactionUseCase(repo, publisher, logger)

	userID := uuid.New()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := uc.CreateTransaction(userID, 100.00, "USD", "Benchmark Store")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkNATSEventProcessing(b *testing.B) {
	logger := zaptest.NewLogger(b)
	repo := repository.NewInMemoryTransactionRepository()
	processor := usecase.NewTransactionEventProcessor(repo, logger)

	// Create a transaction for processing
	userID := uuid.New()
	transaction := domain.NewTransaction(userID, 100.00, "USD")
	transaction.Merchant = "Benchmark Store"
	err := repo.Create(transaction)
	require.NoError(b, err)

	event := messaging.TransactionEvent{
		ID:        transaction.ID.String(),
		UserID:    transaction.UserID.String(),
		Amount:    transaction.Amount,
		Currency:  transaction.Currency,
		Merchant:  transaction.Merchant,
		Status:    string(domain.StatusApproved),
		RiskScore: 0.0,
		EventType: "processed",
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := processor.ProcessTransactionEvent(&event)
		if err != nil {
			b.Fatal(err)
		}
	}
}
