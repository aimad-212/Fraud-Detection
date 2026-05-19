package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fraud-detection/internal/infrastructure/audit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditLogger_NewAuditLogger(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test_audit.log")

	config := audit.Config{
		Enabled:                true,
		LogLevel:               "info",
		OutputFile:             logFile,
		MaxFileSize:            10, // 10MB
		MaxBackups:             3,
		MaxAge:                 7, // 7 days
		RotateOnStartup:        false,
		AuditAuthEvents:        true,
		AuditTransactionEvents: true,
		AuditAdminEvents:       true,
		AuditSecurityEvents:    true,
		AuditDataAccess:        false,
	}

	auditLogger, err := audit.NewAuditLogger(config)
	require.NoError(t, err)
	require.NotNil(t, auditLogger)
	defer auditLogger.Close()

	// Log a test event to ensure file creation
	err = auditLogger.LogAuthEvent("test", "user123", "testuser", "127.0.0.1", "test-agent", "success", nil)
	require.NoError(t, err)

	// Give some time for file creation and logging
	time.Sleep(100 * time.Millisecond)

	// Verify log file was created
	_, err = os.Stat(logFile)
	assert.NoError(t, err)
}

func TestAuditLogger_Disabled(t *testing.T) {
	config := audit.Config{
		Enabled: false,
	}

	auditLogger, err := audit.NewAuditLogger(config)
	require.NoError(t, err)
	require.NotNil(t, auditLogger)

	// Should be a noop logger
	err = auditLogger.LogAuthEvent("login", "user123", "testuser", "127.0.0.1", "test-agent", "success", nil)
	assert.NoError(t, err)

	err = auditLogger.Close()
	assert.NoError(t, err)
}

func TestAuditLogger_LogAuthEvent(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test_auth_audit.log")

	config := audit.Config{
		Enabled:         true,
		LogLevel:        "info",
		OutputFile:      logFile,
		AuditAuthEvents: true,
	}

	auditLogger, err := audit.NewAuditLogger(config)
	require.NoError(t, err)
	defer auditLogger.Close()

	details := map[string]interface{}{
		"method": "POST",
		"path":   "/api/v1/auth/login",
	}

	err = auditLogger.LogAuthEvent("login", "user123", "testuser", "127.0.0.1", "test-agent", "success", details)
	require.NoError(t, err)

	// Give some time for async logging
	time.Sleep(100 * time.Millisecond)

	// Verify log file contains the event
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var logEntry map[string]interface{}
	err = json.Unmarshal(data, &logEntry)
	require.NoError(t, err)

	// Extract the event JSON from the log entry
	eventJSON, ok := logEntry["event"].(string)
	require.True(t, ok)

	var event audit.AuditEvent
	err = json.Unmarshal([]byte(eventJSON), &event)
	require.NoError(t, err)

	assert.Equal(t, audit.EventTypeAuth, event.EventType)
	assert.Equal(t, "login", event.Action)
	assert.Equal(t, "user123", event.UserID)
	assert.Equal(t, "testuser", event.Username)
	assert.Equal(t, "127.0.0.1", event.IPAddress)
	assert.Equal(t, "test-agent", event.UserAgent)
	assert.Equal(t, "success", event.Status)
	assert.Equal(t, "authentication", event.Resource)
	assert.NotZero(t, event.Timestamp)
	assert.NotEmpty(t, event.EventID)
}

func TestAuditLogger_LogTransactionEvent(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test_transaction_audit.log")

	config := audit.Config{
		Enabled:                true,
		LogLevel:               "info",
		OutputFile:             logFile,
		AuditTransactionEvents: true,
	}

	auditLogger, err := audit.NewAuditLogger(config)
	require.NoError(t, err)
	defer auditLogger.Close()

	details := map[string]interface{}{
		"currency": "USD",
		"merchant": "Test Store",
	}

	err = auditLogger.LogTransactionEvent("create", "user123", "txn456", "127.0.0.1", "test-agent", "success", 150.50, details)
	require.NoError(t, err)

	// Give some time for async logging
	time.Sleep(100 * time.Millisecond)

	// Verify log file contains the event
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var logEntry map[string]interface{}
	err = json.Unmarshal(data, &logEntry)
	require.NoError(t, err)

	// Extract the event JSON from the log entry
	eventJSON, ok := logEntry["event"].(string)
	require.True(t, ok)

	var event audit.AuditEvent
	err = json.Unmarshal([]byte(eventJSON), &event)
	require.NoError(t, err)

	assert.Equal(t, audit.EventTypeTransaction, event.EventType)
	assert.Equal(t, "create", event.Action)
	assert.Equal(t, "user123", event.UserID)
	assert.Equal(t, "127.0.0.1", event.IPAddress)
	assert.Equal(t, "test-agent", event.UserAgent)
	assert.Equal(t, "success", event.Status)
	assert.Equal(t, "transaction", event.Resource)
	assert.Equal(t, 150.50, event.Details["amount"])
	assert.Equal(t, "txn456", event.Details["transaction_id"])
}

func TestAuditLogger_LogAdminEvent(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test_admin_audit.log")

	config := audit.Config{
		Enabled:          true,
		LogLevel:         "info",
		OutputFile:       logFile,
		AuditAdminEvents: true,
	}

	auditLogger, err := audit.NewAuditLogger(config)
	require.NoError(t, err)
	defer auditLogger.Close()

	details := map[string]interface{}{
		"target_user": "user456",
		"role":        "admin",
	}

	err = auditLogger.LogAdminEvent("user_update", "admin123", "adminuser", "127.0.0.1", "test-agent", "success", "users", details)
	require.NoError(t, err)

	// Give some time for async logging
	time.Sleep(100 * time.Millisecond)

	// Verify log file contains the event
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var logEntry map[string]interface{}
	err = json.Unmarshal(data, &logEntry)
	require.NoError(t, err)

	// Extract the event JSON from the log entry
	eventJSON, ok := logEntry["event"].(string)
	require.True(t, ok)

	var event audit.AuditEvent
	err = json.Unmarshal([]byte(eventJSON), &event)
	require.NoError(t, err)

	assert.Equal(t, audit.EventTypeAdmin, event.EventType)
	assert.Equal(t, "user_update", event.Action)
	assert.Equal(t, "admin123", event.UserID)
	assert.Equal(t, "adminuser", event.Username)
	assert.Equal(t, "127.0.0.1", event.IPAddress)
	assert.Equal(t, "test-agent", event.UserAgent)
	assert.Equal(t, "success", event.Status)
	assert.Equal(t, "users", event.Resource)
}

func TestAuditLogger_LogSecurityEvent(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test_security_audit.log")

	config := audit.Config{
		Enabled:             true,
		LogLevel:            "info",
		OutputFile:          logFile,
		AuditSecurityEvents: true,
	}

	auditLogger, err := audit.NewAuditLogger(config)
	require.NoError(t, err)
	defer auditLogger.Close()

	details := map[string]interface{}{
		"reason":     "invalid_token",
		"attempt":    3,
		"locked_out": true,
	}

	err = auditLogger.LogSecurityEvent("auth_failure", "user123", "192.168.1.100", "malicious-agent", "failed", details)
	require.NoError(t, err)

	// Give some time for async logging
	time.Sleep(100 * time.Millisecond)

	// Verify log file contains the event
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var logEntry map[string]interface{}
	err = json.Unmarshal(data, &logEntry)
	require.NoError(t, err)

	// Extract the event JSON from the log entry
	eventJSON, ok := logEntry["event"].(string)
	require.True(t, ok)

	var event audit.AuditEvent
	err = json.Unmarshal([]byte(eventJSON), &event)
	require.NoError(t, err)

	assert.Equal(t, audit.EventTypeSecurity, event.EventType)
	assert.Equal(t, "auth_failure", event.Action)
	assert.Equal(t, "user123", event.UserID)
	assert.Equal(t, "192.168.1.100", event.IPAddress)
	assert.Equal(t, "malicious-agent", event.UserAgent)
	assert.Equal(t, "failed", event.Status)
	assert.Equal(t, "security", event.Resource)
	assert.Equal(t, "invalid_token", event.Details["reason"])
}

func TestAuditLogger_LogDataAccessEvent(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test_data_access_audit.log")

	config := audit.Config{
		Enabled:         true,
		LogLevel:        "info",
		OutputFile:      logFile,
		AuditDataAccess: true,
	}

	auditLogger, err := audit.NewAuditLogger(config)
	require.NoError(t, err)
	defer auditLogger.Close()

	details := map[string]interface{}{
		"query":        "SELECT * FROM transactions WHERE user_id = ?",
		"record_count": 25,
	}

	err = auditLogger.LogDataAccessEvent("read", "user123", "transactions", "127.0.0.1", "test-agent", "success", details)
	require.NoError(t, err)

	// Give some time for async logging
	time.Sleep(100 * time.Millisecond)

	// Verify log file contains the event
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var logEntry map[string]interface{}
	err = json.Unmarshal(data, &logEntry)
	require.NoError(t, err)

	// Extract the event JSON from the log entry
	eventJSON, ok := logEntry["event"].(string)
	require.True(t, ok)

	var event audit.AuditEvent
	err = json.Unmarshal([]byte(eventJSON), &event)
	require.NoError(t, err)

	assert.Equal(t, audit.EventTypeDataAccess, event.EventType)
	assert.Equal(t, "read", event.Action)
	assert.Equal(t, "user123", event.UserID)
	assert.Equal(t, "127.0.0.1", event.IPAddress)
	assert.Equal(t, "test-agent", event.UserAgent)
	assert.Equal(t, "success", event.Status)
	assert.Equal(t, "transactions", event.Resource)
}

func TestAuditLogger_EventTypeFiltering(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test_filter_audit.log")

	config := audit.Config{
		Enabled:                true,
		LogLevel:               "info",
		OutputFile:             logFile,
		AuditAuthEvents:        true,
		AuditTransactionEvents: false, // Disabled
		AuditAdminEvents:       false, // Disabled
		AuditSecurityEvents:    true,
		AuditDataAccess:        false, // Disabled
	}

	auditLogger, err := audit.NewAuditLogger(config)
	require.NoError(t, err)
	defer auditLogger.Close()

	// These should be logged
	err = auditLogger.LogAuthEvent("login", "user123", "testuser", "127.0.0.1", "test-agent", "success", nil)
	require.NoError(t, err)

	err = auditLogger.LogSecurityEvent("auth_failure", "user123", "127.0.0.1", "test-agent", "failed", nil)
	require.NoError(t, err)

	// These should be filtered out (not logged)
	err = auditLogger.LogTransactionEvent("create", "user123", "txn456", "127.0.0.1", "test-agent", "success", 100.0, nil)
	require.NoError(t, err)

	err = auditLogger.LogAdminEvent("user_update", "admin123", "adminuser", "127.0.0.1", "test-agent", "success", "users", nil)
	require.NoError(t, err)

	err = auditLogger.LogDataAccessEvent("read", "user123", "transactions", "127.0.0.1", "test-agent", "success", nil)
	require.NoError(t, err)

	// Give some time for async logging
	time.Sleep(100 * time.Millisecond)

	// Verify log file contains only the expected events
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// Should contain auth and security events, but not transaction, admin, or data access events
	logContent := string(data)
	assert.Contains(t, logContent, "auth")
	assert.Contains(t, logContent, "security")
	assert.NotContains(t, logContent, "transaction")
	assert.NotContains(t, logContent, "admin")
	assert.NotContains(t, logContent, "data_access")
}

func TestAuditLogger_LogEvent_Generic(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test_generic_audit.log")

	config := audit.Config{
		Enabled:    true,
		LogLevel:   "info",
		OutputFile: logFile,
	}

	auditLogger, err := audit.NewAuditLogger(config)
	require.NoError(t, err)
	defer auditLogger.Close()

	event := audit.AuditEvent{
		EventType: audit.EventTypeAuth,
		Action:    "custom_action",
		Resource:  "custom_resource",
		UserID:    "user123",
		Status:    "success",
		Details: map[string]interface{}{
			"custom_field": "custom_value",
		},
	}

	err = auditLogger.LogEvent(event)
	require.NoError(t, err)

	// Give some time for async logging
	time.Sleep(100 * time.Millisecond)

	// Verify log file contains the event
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var logEntry map[string]interface{}
	err = json.Unmarshal(data, &logEntry)
	require.NoError(t, err)

	// Extract the event JSON from the log entry
	eventJSON, ok := logEntry["event"].(string)
	require.True(t, ok)

	var loggedEvent audit.AuditEvent
	err = json.Unmarshal([]byte(eventJSON), &loggedEvent)
	require.NoError(t, err)

	assert.Equal(t, event.EventType, loggedEvent.EventType)
	assert.Equal(t, event.Action, loggedEvent.Action)
	assert.Equal(t, event.Resource, loggedEvent.Resource)
	assert.Equal(t, event.UserID, loggedEvent.UserID)
	assert.Equal(t, event.Status, loggedEvent.Status)
	assert.Equal(t, event.Details["custom_field"], loggedEvent.Details["custom_field"])
	assert.NotZero(t, loggedEvent.Timestamp)
	assert.NotEmpty(t, loggedEvent.EventID)
}

func TestAuditLogger_ErrorStatus(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test_error_audit.log")

	config := audit.Config{
		Enabled:         true,
		LogLevel:        "info",
		OutputFile:      logFile,
		AuditAuthEvents: true,
	}

	auditLogger, err := audit.NewAuditLogger(config)
	require.NoError(t, err)
	defer auditLogger.Close()

	// Test error status
	err = auditLogger.LogAuthEvent("login", "user123", "testuser", "127.0.0.1", "test-agent", "error", nil)
	require.NoError(t, err)

	// Test failed status
	err = auditLogger.LogAuthEvent("login", "user123", "testuser", "127.0.0.1", "test-agent", "failed", nil)
	require.NoError(t, err)

	// Test denied status
	err = auditLogger.LogAuthEvent("login", "user123", "testuser", "127.0.0.1", "test-agent", "denied", nil)
	require.NoError(t, err)

	// Give some time for async logging
	time.Sleep(100 * time.Millisecond)

	// Verify log file contains the events with appropriate log levels
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	logContent := string(data)
	// Error/failed/denied events should be logged at error level
	assert.Contains(t, logContent, "\"level\":\"error\"")
}
