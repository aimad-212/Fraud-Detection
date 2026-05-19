package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// EventType represents the type of audit event
type EventType string

const (
	EventTypeAuth        EventType = "auth"
	EventTypeTransaction EventType = "transaction"
	EventTypeAdmin       EventType = "admin"
	EventTypeSecurity    EventType = "security"
	EventTypeDataAccess  EventType = "data_access"
)

// AuditEvent represents an audit log entry
type AuditEvent struct {
	Timestamp    time.Time              `json:"timestamp"`
	EventID      string                 `json:"event_id"`
	EventType    EventType              `json:"event_type"`
	Action       string                 `json:"action"`
	Resource     string                 `json:"resource"`
	UserID       string                 `json:"user_id,omitempty"`
	Username     string                 `json:"username,omitempty"`
	IPAddress    string                 `json:"ip_address,omitempty"`
	UserAgent    string                 `json:"user_agent,omitempty"`
	RequestID    string                 `json:"request_id,omitempty"`
	SessionID    string                 `json:"session_id,omitempty"`
	Status       string                 `json:"status"`
	Details      map[string]interface{} `json:"details,omitempty"`
	RiskScore    float64                `json:"risk_score,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Duration     int64                  `json:"duration_ms,omitempty"`
}

// AuditLogger interface for audit logging
type AuditLogger interface {
	LogEvent(event AuditEvent) error
	LogAuthEvent(action, userID, username, ipAddress, userAgent, status string, details map[string]interface{}) error
	LogTransactionEvent(action, userID, transactionID, ipAddress, userAgent, status string, amount float64, details map[string]interface{}) error
	LogAdminEvent(action, userID, username, ipAddress, userAgent, status string, resource string, details map[string]interface{}) error
	LogSecurityEvent(action, userID, ipAddress, userAgent, status string, details map[string]interface{}) error
	LogDataAccessEvent(action, userID, resource, ipAddress, userAgent, status string, details map[string]interface{}) error
	Close() error
}

// Config holds audit logger configuration
type Config struct {
	Enabled                bool
	LogLevel               string
	OutputFile             string
	MaxFileSize            int
	MaxBackups             int
	MaxAge                 int
	RotateOnStartup        bool
	AuditAuthEvents        bool
	AuditTransactionEvents bool
	AuditAdminEvents       bool
	AuditSecurityEvents    bool
	AuditDataAccess        bool
}

// zapAuditLogger implements AuditLogger using zap
type zapAuditLogger struct {
	logger *zap.Logger
	config Config
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(config Config) (AuditLogger, error) {
	if !config.Enabled {
		return &noopAuditLogger{}, nil
	}

	// Ensure log directory exists
	logDir := filepath.Dir(config.OutputFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	// Configure lumberjack for log rotation
	lumberjackLogger := &lumberjack.Logger{
		Filename:   config.OutputFile,
		MaxSize:    config.MaxFileSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   true,
	}

	// Rotate on startup if requested
	if config.RotateOnStartup {
		if err := lumberjackLogger.Rotate(); err != nil {
			return nil, fmt.Errorf("failed to rotate audit log on startup: %w", err)
		}
	}

	// Create zap encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create core
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(lumberjackLogger),
		getLogLevel(config.LogLevel),
	)

	// Create logger
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return &zapAuditLogger{
		logger: logger,
		config: config,
	}, nil
}

// getLogLevel converts string log level to zapcore.Level
func getLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// generateEventID generates a unique event ID
func generateEventID() string {
	return fmt.Sprintf("audit_%d", time.Now().UnixNano())
}

// LogEvent logs a generic audit event
func (a *zapAuditLogger) LogEvent(event AuditEvent) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.EventID == "" {
		event.EventID = generateEventID()
	}

	// Convert to JSON for structured logging
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// Log with appropriate level based on status
	switch event.Status {
	case "error", "failed", "denied":
		a.logger.Error("audit_event", zap.String("event", string(eventJSON)))
	case "warning", "suspicious":
		a.logger.Warn("audit_event", zap.String("event", string(eventJSON)))
	default:
		a.logger.Info("audit_event", zap.String("event", string(eventJSON)))
	}

	return nil
}

// LogAuthEvent logs authentication-related events
func (a *zapAuditLogger) LogAuthEvent(action, userID, username, ipAddress, userAgent, status string, details map[string]interface{}) error {
	if !a.config.AuditAuthEvents {
		return nil
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventID:   generateEventID(),
		EventType: EventTypeAuth,
		Action:    action,
		Resource:  "authentication",
		UserID:    userID,
		Username:  username,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Status:    status,
		Details:   details,
	}

	return a.LogEvent(event)
}

// LogTransactionEvent logs transaction-related events
func (a *zapAuditLogger) LogTransactionEvent(action, userID, transactionID, ipAddress, userAgent, status string, amount float64, details map[string]interface{}) error {
	if !a.config.AuditTransactionEvents {
		return nil
	}

	if details == nil {
		details = make(map[string]interface{})
	}
	details["transaction_id"] = transactionID
	details["amount"] = amount

	event := AuditEvent{
		Timestamp: time.Now(),
		EventID:   generateEventID(),
		EventType: EventTypeTransaction,
		Action:    action,
		Resource:  "transaction",
		UserID:    userID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Status:    status,
		Details:   details,
	}

	return a.LogEvent(event)
}

// LogAdminEvent logs administration-related events
func (a *zapAuditLogger) LogAdminEvent(action, userID, username, ipAddress, userAgent, status string, resource string, details map[string]interface{}) error {
	if !a.config.AuditAdminEvents {
		return nil
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventID:   generateEventID(),
		EventType: EventTypeAdmin,
		Action:    action,
		Resource:  resource,
		UserID:    userID,
		Username:  username,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Status:    status,
		Details:   details,
	}

	return a.LogEvent(event)
}

// LogSecurityEvent logs security-related events
func (a *zapAuditLogger) LogSecurityEvent(action, userID, ipAddress, userAgent, status string, details map[string]interface{}) error {
	if !a.config.AuditSecurityEvents {
		return nil
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventID:   generateEventID(),
		EventType: EventTypeSecurity,
		Action:    action,
		Resource:  "security",
		UserID:    userID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Status:    status,
		Details:   details,
	}

	return a.LogEvent(event)
}

// LogDataAccessEvent logs data access events
func (a *zapAuditLogger) LogDataAccessEvent(action, userID, resource, ipAddress, userAgent, status string, details map[string]interface{}) error {
	if !a.config.AuditDataAccess {
		return nil
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventID:   generateEventID(),
		EventType: EventTypeDataAccess,
		Action:    action,
		Resource:  resource,
		UserID:    userID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Status:    status,
		Details:   details,
	}

	return a.LogEvent(event)
}

// Close closes the audit logger
func (a *zapAuditLogger) Close() error {
	return a.logger.Sync()
}

// noopAuditLogger is a no-op implementation for when audit logging is disabled
type noopAuditLogger struct{}

func (n *noopAuditLogger) LogEvent(event AuditEvent) error {
	return nil
}

func (n *noopAuditLogger) LogAuthEvent(action, userID, username, ipAddress, userAgent, status string, details map[string]interface{}) error {
	return nil
}

func (n *noopAuditLogger) LogTransactionEvent(action, userID, transactionID, ipAddress, userAgent, status string, amount float64, details map[string]interface{}) error {
	return nil
}

func (n *noopAuditLogger) LogAdminEvent(action, userID, username, ipAddress, userAgent, status string, resource string, details map[string]interface{}) error {
	return nil
}

func (n *noopAuditLogger) LogSecurityEvent(action, userID, ipAddress, userAgent, status string, details map[string]interface{}) error {
	return nil
}

func (n *noopAuditLogger) LogDataAccessEvent(action, userID, resource, ipAddress, userAgent, status string, details map[string]interface{}) error {
	return nil
}

func (n *noopAuditLogger) Close() error {
	return nil
}
