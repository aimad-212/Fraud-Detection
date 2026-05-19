package messaging

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// NATSSubscriber handles subscribing to NATS messages
type NATSSubscriber struct {
	nc         *nats.Conn
	subject    string
	queueGroup string
	logger     *zap.Logger
	processor  TransactionProcessor
	wg         sync.WaitGroup
	stopChan   chan struct{}
}

// TransactionProcessor defines the interface for processing transaction events
type TransactionProcessor interface {
	ProcessTransactionEvent(event *TransactionEvent) error
}

// NewNATSSubscriber creates a new NATS subscriber
func NewNATSSubscriber(url, subject, queueGroup string, processor TransactionProcessor, logger *zap.Logger) (*NATSSubscriber, error) {
	nc, err := nats.Connect(url,
		nats.MaxReconnects(5),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			logger.Error("NATS subscriber disconnected", zap.Error(err))
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS subscriber reconnected", zap.String("url", nc.ConnectedUrl()))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	logger.Info("Connected to NATS subscriber", zap.String("url", url), zap.String("subject", subject), zap.String("queue_group", queueGroup))

	return &NATSSubscriber{
		nc:         nc,
		subject:    subject,
		queueGroup: queueGroup,
		logger:     logger,
		processor:  processor,
		stopChan:   make(chan struct{}),
	}, nil
}

// Start begins subscribing to messages
func (s *NATSSubscriber) Start() error {
	s.wg.Add(1)
	go s.subscribe()

	s.logger.Info("NATS subscriber started", zap.String("subject", s.subject), zap.String("queue_group", s.queueGroup))
	return nil
}

// subscribe handles the subscription logic
func (s *NATSSubscriber) subscribe() {
	defer s.wg.Done()

	// Subscribe with queue group for load balancing
	sub, err := s.nc.QueueSubscribe(s.subject, s.queueGroup, s.messageHandler)
	if err != nil {
		s.logger.Fatal("Failed to subscribe to NATS", zap.Error(err))
	}
	defer sub.Unsubscribe()

	// Wait for stop signal
	<-s.stopChan
	s.logger.Info("NATS subscriber stopping")
}

// messageHandler processes incoming messages
func (s *NATSSubscriber) messageHandler(msg *nats.Msg) {
	startTime := time.Now()

	// Parse the message
	var event TransactionEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		s.logger.Error("Failed to unmarshal NATS message",
			zap.Error(err),
			zap.String("subject", msg.Subject),
			zap.ByteString("data", msg.Data))
		return
	}

	s.logger.Info("Received NATS message",
		zap.String("subject", msg.Subject),
		zap.String("event_type", event.EventType),
		zap.String("transaction_id", event.ID),
		zap.Float64("amount", event.Amount))

	// Process the transaction event
	err := s.processor.ProcessTransactionEvent(&event)
	processingTime := time.Since(startTime)

	if err != nil {
		s.logger.Error("Failed to process transaction event",
			zap.Error(err),
			zap.String("transaction_id", event.ID),
			zap.String("event_type", event.EventType),
			zap.Duration("processing_time", processingTime))

		// In a production system, you might want to implement retry logic
		// or send to a dead letter queue
		return
	}

	s.logger.Info("Successfully processed transaction event",
		zap.String("transaction_id", event.ID),
		zap.String("event_type", event.EventType),
		zap.Duration("processing_time", processingTime))

	// Acknowledge the message (if using NATS streaming or JetStream)
	// msg.Ack()
}

// Stop gracefully stops the subscriber
func (s *NATSSubscriber) Stop() {
	close(s.stopChan)
	s.wg.Wait()

	if s.nc != nil {
		s.nc.Close()
		s.logger.Info("NATS subscriber connection closed")
	}
}

// IsConnected returns true if connected to NATS
func (s *NATSSubscriber) IsConnected() bool {
	return s.nc != nil && s.nc.IsConnected()
}

// GetStats returns subscriber statistics
func (s *NATSSubscriber) GetStats() map[string]interface{} {
	if s.nc == nil {
		return map[string]interface{}{
			"connected": false,
		}
	}

	stats := s.nc.Stats()
	return map[string]interface{}{
		"connected":    s.nc.IsConnected(),
		"server_url":   s.nc.ConnectedUrl(),
		"messages_in":  stats.InMsgs,
		"messages_out": stats.OutMsgs,
		"bytes_in":     stats.InBytes,
		"bytes_out":    stats.OutBytes,
		"reconnects":   stats.Reconnects,
		"subject":      s.subject,
		"queue_group":  s.queueGroup,
	}
}
