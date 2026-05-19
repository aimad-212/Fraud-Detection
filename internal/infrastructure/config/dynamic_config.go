package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fraud-detection/internal/config"
	"fraud-detection/internal/infrastructure/database"
	"fraud-detection/internal/infrastructure/messaging"

	"github.com/fsnotify/fsnotify"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// DynamicConfigManager provides dynamic configuration updates
type DynamicConfigManager struct {
	configPath      string
	logger          *zap.Logger
	mu              sync.RWMutex
	config          *config.Config
	watcher         *fsnotify.Watcher
	updateHandlers  map[string]ConfigUpdateHandler
	stopChan        chan struct{}
	wg              sync.WaitGroup
	natsConn        *nats.Conn
	messagingConfig *messaging.Config
}

// ConfigUpdateHandler defines the interface for handling configuration updates
type ConfigUpdateHandler interface {
	OnConfigUpdate(oldConfig, newConfig *config.Config) error
	GetConfigSection() string
}

// ConfigChangeEvent represents a configuration change event
type ConfigChangeEvent struct {
	Section     string                 `json:"section"`
	Changes     map[string]interface{} `json:"changes"`
	Timestamp   time.Time              `json:"timestamp"`
	TriggeredBy string                 `json:"triggered_by"` // file, api, system
}

// NewDynamicConfigManager creates a new dynamic configuration manager
func NewDynamicConfigManager(configPath string, logger *zap.Logger) (*DynamicConfigManager, error) {
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute config path: %w", err)
	}

	dcm := &DynamicConfigManager{
		configPath:     absPath,
		logger:         logger,
		updateHandlers: make(map[string]ConfigUpdateHandler),
		stopChan:       make(chan struct{}),
	}

	// Load initial configuration
	if err := dcm.loadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load initial config: %w", err)
	}

	return dcm, nil
}

// NewDynamicConfigManagerWithNATS creates a new dynamic configuration manager with NATS support
func NewDynamicConfigManagerWithNATS(configPath string, logger *zap.Logger, natsConn *nats.Conn, messagingConfig *messaging.Config) (*DynamicConfigManager, error) {
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute config path: %w", err)
	}

	dcm := &DynamicConfigManager{
		configPath:      absPath,
		logger:          logger,
		updateHandlers:  make(map[string]ConfigUpdateHandler),
		stopChan:        make(chan struct{}),
		natsConn:        natsConn,
		messagingConfig: messagingConfig,
	}

	// Load initial configuration
	if err := dcm.loadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load initial config: %w", err)
	}

	return dcm, nil
}

// RegisterUpdateHandler registers a handler for configuration updates
func (dcm *DynamicConfigManager) RegisterUpdateHandler(handler ConfigUpdateHandler) {
	dcm.mu.Lock()
	defer dcm.mu.Unlock()

	section := handler.GetConfigSection()
	dcm.updateHandlers[section] = handler
	dcm.logger.Info("Registered config update handler",
		zap.String("section", section))
}

// Start starts the dynamic configuration manager
func (dcm *DynamicConfigManager) Start(ctx context.Context) error {
	dcm.logger.Info("Starting dynamic configuration manager",
		zap.String("config_path", dcm.configPath))

	// Create file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	dcm.watcher = watcher

	// Watch the config file
	configDir := filepath.Dir(dcm.configPath)
	if err := watcher.Add(configDir); err != nil {
		return fmt.Errorf("failed to watch config directory: %w", err)
	}

	// Start the watch loop
	dcm.wg.Add(1)
	go dcm.watchLoop(ctx)

	dcm.logger.Info("Dynamic configuration manager started",
		zap.String("watching_directory", configDir))

	return nil
}

// Stop stops the dynamic configuration manager
func (dcm *DynamicConfigManager) Stop() {
	dcm.logger.Info("Stopping dynamic configuration manager")

	if dcm.watcher != nil {
		dcm.watcher.Close()
	}

	close(dcm.stopChan)
	dcm.wg.Wait()
}

// GetConfig returns the current configuration
func (dcm *DynamicConfigManager) GetConfig() *config.Config {
	dcm.mu.RLock()
	defer dcm.mu.RUnlock()

	// Return a copy to prevent external modifications
	configCopy := *dcm.config
	return &configCopy
}

// UpdateConfig updates the configuration programmatically
func (dcm *DynamicConfigManager) UpdateConfig(newConfig *config.Config, triggeredBy string) error {
	dcm.mu.Lock()
	defer dcm.mu.Unlock()

	oldConfig := dcm.config

	// Validate new configuration
	if err := dcm.validateConfig(newConfig); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Detect changes
	changes := dcm.detectChanges(oldConfig, newConfig)
	if len(changes) == 0 {
		dcm.logger.Info("No configuration changes detected")
		return nil
	}

	// Update configuration
	dcm.config = newConfig

	// Notify handlers
	if err := dcm.notifyHandlers(oldConfig, newConfig); err != nil {
		// Rollback on handler error
		dcm.config = oldConfig
		return fmt.Errorf("handler notification failed, rolled back: %w", err)
	}

	// Save to file
	if err := dcm.saveConfig(); err != nil {
		dcm.logger.Error("Failed to save updated config to file",
			zap.Error(err))
		// Don't rollback here as handlers were already notified
	}

	dcm.logger.Info("Configuration updated successfully",
		zap.String("triggered_by", triggeredBy),
		zap.Int("changes_count", len(changes)))

	// Publish change event
	dcm.publishChangeEvent(&ConfigChangeEvent{
		Changes:     changes,
		Timestamp:   time.Now(),
		TriggeredBy: triggeredBy,
	})

	return nil
}

// UpdateMLConfig updates only the ML configuration section
func (dcm *DynamicConfigManager) UpdateMLConfig(mlConfig *config.MLConfig, triggeredBy string) error {
	dcm.mu.Lock()
	defer dcm.mu.Unlock()

	oldConfig := dcm.config

	// Create new config with updated ML section
	newConfig := *oldConfig
	newConfig.FraudDetection.MLConfig = *mlConfig

	// Validate ML configuration
	if err := dcm.validateMLConfig(mlConfig); err != nil {
		return fmt.Errorf("ML config validation failed: %w", err)
	}

	// Detect ML changes
	changes := dcm.detectMLChanges(&oldConfig.FraudDetection.MLConfig, mlConfig)
	if len(changes) == 0 {
		dcm.logger.Info("No ML configuration changes detected")
		return nil
	}

	// Update configuration
	dcm.config = &newConfig

	// Notify ML handlers
	if err := dcm.notifyHandlers(oldConfig, &newConfig); err != nil {
		// Rollback on handler error
		dcm.config = oldConfig
		return fmt.Errorf("ML handler notification failed, rolled back: %w", err)
	}

	// Save to file
	if err := dcm.saveConfig(); err != nil {
		dcm.logger.Error("Failed to save updated ML config to file",
			zap.Error(err))
	}

	dcm.logger.Info("ML configuration updated successfully",
		zap.String("triggered_by", triggeredBy),
		zap.Int("changes_count", len(changes)))

	// Publish change event
	dcm.publishChangeEvent(&ConfigChangeEvent{
		Section:     "ml",
		Changes:     changes,
		Timestamp:   time.Now(),
		TriggeredBy: triggeredBy,
	})

	return nil
}

// loadConfig loads configuration from file
func (dcm *DynamicConfigManager) loadConfig() error {
	cfg, err := config.LoadFromFile(dcm.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	dcm.mu.Lock()
	dcm.config = cfg
	dcm.mu.Unlock()

	return nil
}

// saveConfig saves configuration to file
func (dcm *DynamicConfigManager) saveConfig() error {
	data, err := yaml.Marshal(dcm.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(dcm.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// watchLoop watches for file system changes
func (dcm *DynamicConfigManager) watchLoop(ctx context.Context) {
	defer dcm.wg.Done()

	for {
		select {
		case <-ctx.Done():
			dcm.logger.Info("Dynamic config manager context cancelled")
			return
		case <-dcm.stopChan:
			dcm.logger.Info("Dynamic config manager stopped")
			return
		case event, ok := <-dcm.watcher.Events:
			if !ok {
				return
			}

			if event.Name == dcm.configPath && event.Op&fsnotify.Write == fsnotify.Write {
				dcm.logger.Info("Config file modified, reloading",
					zap.String("file", event.Name))

				if err := dcm.reloadConfigFromFile(); err != nil {
					dcm.logger.Error("Failed to reload config from file",
						zap.Error(err))
				}
			}
		case err, ok := <-dcm.watcher.Errors:
			if !ok {
				return
			}
			dcm.logger.Error("File watcher error",
				zap.Error(err))
		}
	}
}

// SubscribeToConfigChanges subscribes to configuration change events via NATS
func (dcm *DynamicConfigManager) SubscribeToConfigChanges(section string, handler func(*ConfigChangeEvent)) error {
	if dcm.natsConn == nil || dcm.messagingConfig == nil || !dcm.messagingConfig.Enabled {
		return fmt.Errorf("NATS is not enabled or configured")
	}

	subject := fmt.Sprintf("%s.config.%s", dcm.messagingConfig.Subject, section)

	_, err := dcm.natsConn.Subscribe(subject, func(msg *nats.Msg) {
		var event ConfigChangeEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			dcm.logger.Error("Failed to deserialize configuration change event",
				zap.Error(err),
				zap.String("subject", subject))
			return
		}

		// Call the handler
		handler(&event)
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to configuration changes for section %s: %w", section, err)
	}

	dcm.logger.Info("Subscribed to configuration changes",
		zap.String("section", section),
		zap.String("subject", subject))

	return nil
}

// reloadConfigFromFile reloads configuration from file
func (dcm *DynamicConfigManager) reloadConfigFromFile() error {
	// Load new configuration
	newConfig, err := config.LoadFromFile(dcm.configPath)
	if err != nil {
		return fmt.Errorf("failed to load new config: %w", err)
	}

	dcm.mu.Lock()
	defer dcm.mu.Unlock()

	oldConfig := dcm.config

	// Validate new configuration
	if err := dcm.validateConfig(newConfig); err != nil {
		return fmt.Errorf("new config validation failed: %w", err)
	}

	// Detect changes
	changes := dcm.detectChanges(oldConfig, newConfig)
	if len(changes) == 0 {
		dcm.logger.Debug("No configuration changes detected in file reload")
		return nil
	}

	// Update configuration
	dcm.config = newConfig

	// Notify handlers
	if err := dcm.notifyHandlers(oldConfig, newConfig); err != nil {
		// Rollback on handler error
		dcm.config = oldConfig
		return fmt.Errorf("handler notification failed during file reload, rolled back: %w", err)
	}

	dcm.logger.Info("Configuration reloaded from file successfully",
		zap.Int("changes_count", len(changes)))

	// Publish change event
	dcm.publishChangeEvent(&ConfigChangeEvent{
		Changes:     changes,
		Timestamp:   time.Now(),
		TriggeredBy: "file",
	})

	return nil
}

// validateConfig validates the entire configuration
func (dcm *DynamicConfigManager) validateConfig(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate ML configuration
	if err := dcm.validateMLConfig(&cfg.FraudDetection.MLConfig); err != nil {
		return fmt.Errorf("ML config validation failed: %w", err)
	}

	// Add other validation rules as needed
	return nil
}

// validateMLConfig validates ML configuration
func (dcm *DynamicConfigManager) validateMLConfig(mlConfig *config.MLConfig) error {
	if mlConfig == nil {
		return fmt.Errorf("ML config cannot be nil")
	}

	// Validate weights sum to 1.0
	totalWeight := mlConfig.RuleWeight + mlConfig.MLWeight
	if totalWeight < 0.99 || totalWeight > 1.01 { // Allow small floating point errors
		return fmt.Errorf("rule weight (%f) + ML weight (%f) must sum to 1.0", mlConfig.RuleWeight, mlConfig.MLWeight)
	}

	// Validate thresholds
	if mlConfig.ApproveThreshold < 0 || mlConfig.ApproveThreshold > 1 {
		return fmt.Errorf("approve_threshold must be between 0 and 1")
	}
	if mlConfig.FlagThreshold < 0 || mlConfig.FlagThreshold > 1 {
		return fmt.Errorf("flag_threshold must be between 0 and 1")
	}
	if mlConfig.RejectThreshold < 0 || mlConfig.RejectThreshold > 1 {
		return fmt.Errorf("reject_threshold must be between 0 and 1")
	}
	if mlConfig.ApproveThreshold >= mlConfig.FlagThreshold {
		return fmt.Errorf("approve_threshold must be less than flag_threshold")
	}
	if mlConfig.FlagThreshold >= mlConfig.RejectThreshold {
		return fmt.Errorf("flag_threshold must be less than reject_threshold")
	}

	// Validate model parameters
	if mlConfig.MaxConcurrentPredictions < 0 {
		return fmt.Errorf("max_concurrent_predictions cannot be negative")
	}
	if mlConfig.PredictionTimeout == "" {
		return fmt.Errorf("prediction_timeout cannot be empty")
	}

	// Validate feature vector size
	if mlConfig.FeatureVectorSize < 0 {
		return fmt.Errorf("feature_vector_size cannot be negative")
	}
	if mlConfig.FeatureVectorSize > 1000 {
		return fmt.Errorf("feature_vector_size cannot exceed 1000")
	}

	// Validate batch settings
	if mlConfig.EnableBatchPrediction {
		if mlConfig.BatchSize <= 0 {
			return fmt.Errorf("batch_size must be positive when batch prediction is enabled")
		}
		if mlConfig.BatchSize > 1000 {
			return fmt.Errorf("batch_size cannot exceed 1000")
		}
	}

	// Validate drift detection
	if mlConfig.EnableDriftDetection {
		if mlConfig.DriftThreshold <= 0 || mlConfig.DriftThreshold > 1 {
			return fmt.Errorf("drift_threshold must be between 0 and 1 when drift detection is enabled")
		}
	}

	// Validate A/B testing settings
	if mlConfig.EnableABTesting {
		totalTraffic := mlConfig.ABTestControlWeight + mlConfig.ABTestTreatmentWeight
		if totalTraffic < 0.99 || totalTraffic > 1.01 {
			return fmt.Errorf("AB test control weight (%f) + treatment weight (%f) must sum to 1.0",
				mlConfig.ABTestControlWeight, mlConfig.ABTestTreatmentWeight)
		}
		if mlConfig.ABTestTrafficSplit <= 0 || mlConfig.ABTestTrafficSplit > 1 {
			return fmt.Errorf("AB test traffic split must be between 0 and 1")
		}
	}

	return nil
}

// validateServerConfig validates server configuration
func (dcm *DynamicConfigManager) validateServerConfig(serverConfig *config.ServerConfig) error {
	if serverConfig == nil {
		return fmt.Errorf("server config cannot be nil")
	}

	if serverConfig.Port <= 0 || serverConfig.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535")
	}

	if serverConfig.Mode == "" {
		return fmt.Errorf("server mode cannot be empty")
	}

	if serverConfig.Mode != "debug" && serverConfig.Mode != "release" && serverConfig.Mode != "test" {
		return fmt.Errorf("server mode must be one of: debug, release, test")
	}

	return nil
}

// validateDatabaseConfig validates database configuration
func (dcm *DynamicConfigManager) validateDatabaseConfig(dbConfig *database.Config) error {
	if dbConfig == nil {
		return fmt.Errorf("database config cannot be nil")
	}

	if dbConfig.Host == "" {
		return fmt.Errorf("database host cannot be empty")
	}

	if dbConfig.Port <= 0 || dbConfig.Port > 65535 {
		return fmt.Errorf("database port must be between 1 and 65535")
	}

	if dbConfig.Database == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	if dbConfig.MaxOpenConns <= 0 {
		return fmt.Errorf("max_open_conns must be positive")
	}

	if dbConfig.MaxIdleConns < 0 {
		return fmt.Errorf("max_idle_conns cannot be negative")
	}

	return nil
}

// detectChanges detects changes between old and new configuration
func (dcm *DynamicConfigManager) detectChanges(oldConfig, newConfig *config.Config) map[string]interface{} {
	changes := make(map[string]interface{})

	// Compare ML configuration
	oldMLBytes, _ := json.Marshal(oldConfig.FraudDetection.MLConfig)
	newMLBytes, _ := json.Marshal(newConfig.FraudDetection.MLConfig)

	if string(oldMLBytes) != string(newMLBytes) {
		changes["ml"] = map[string]interface{}{
			"old": string(oldMLBytes),
			"new": string(newMLBytes),
		}
	}

	// Add other configuration comparisons as needed
	return changes
}

// detectMLChanges detects changes in ML configuration
func (dcm *DynamicConfigManager) detectMLChanges(oldMLConfig, newMLConfig *config.MLConfig) map[string]interface{} {
	changes := make(map[string]interface{})

	oldBytes, _ := json.Marshal(oldMLConfig)
	newBytes, _ := json.Marshal(newMLConfig)

	if string(oldBytes) != string(newBytes) {
		changes["ml_config"] = map[string]interface{}{
			"old": string(oldBytes),
			"new": string(newBytes),
		}
	}

	return changes
}

// notifyHandlers notifies all registered handlers about configuration changes
func (dcm *DynamicConfigManager) notifyHandlers(oldConfig, newConfig *config.Config) error {
	for section, handler := range dcm.updateHandlers {
		if err := handler.OnConfigUpdate(oldConfig, newConfig); err != nil {
			dcm.logger.Error("Config update handler failed",
				zap.Error(err),
				zap.String("section", section))
			return fmt.Errorf("handler for section %s failed: %w", section, err)
		}

		dcm.logger.Info("Config update handler notified successfully",
			zap.String("section", section))
	}

	return nil
}

// publishChangeEvent publishes a configuration change event
func (dcm *DynamicConfigManager) publishChangeEvent(event *ConfigChangeEvent) {
	// Always log the configuration change
	dcm.logger.Info("Configuration change event",
		zap.String("section", event.Section),
		zap.String("triggered_by", event.TriggeredBy),
		zap.Time("timestamp", event.Timestamp),
		zap.Any("changes", event.Changes))

	// Publish to NATS if available
	if dcm.natsConn != nil && dcm.messagingConfig != nil && dcm.messagingConfig.Enabled {
		subject := fmt.Sprintf("%s.config.%s", dcm.messagingConfig.Subject, event.Section)

		// Serialize event to JSON
		eventData, err := json.Marshal(event)
		if err != nil {
			dcm.logger.Error("Failed to serialize configuration change event",
				zap.Error(err),
				zap.String("section", event.Section))
			return
		}

		// Publish to NATS
		if err := dcm.natsConn.Publish(subject, eventData); err != nil {
			dcm.logger.Error("Failed to publish configuration change event to NATS",
				zap.Error(err),
				zap.String("subject", subject),
				zap.String("section", event.Section))
		} else {
			dcm.logger.Debug("Configuration change event published to NATS",
				zap.String("subject", subject),
				zap.String("section", event.Section))
		}
	}
}

// MLConfigUpdateHandler implements ConfigUpdateHandler for ML configuration
type MLConfigUpdateHandler struct {
	logger   *zap.Logger
	onUpdate func(oldConfig, newConfig *config.Config) error
}

// NewMLConfigUpdateHandler creates a new ML configuration update handler
func NewMLConfigUpdateHandler(logger *zap.Logger, onUpdate func(oldConfig, newConfig *config.Config) error) *MLConfigUpdateHandler {
	return &MLConfigUpdateHandler{
		logger:   logger,
		onUpdate: onUpdate,
	}
}

// OnConfigUpdate handles ML configuration updates
func (h *MLConfigUpdateHandler) OnConfigUpdate(oldConfig, newConfig *config.Config) error {
	h.logger.Info("ML configuration updated",
		zap.Float64("old_rule_weight", oldConfig.FraudDetection.MLConfig.RuleWeight),
		zap.Float64("new_rule_weight", newConfig.FraudDetection.MLConfig.RuleWeight),
		zap.Float64("old_ml_weight", oldConfig.FraudDetection.MLConfig.MLWeight),
		zap.Float64("new_ml_weight", newConfig.FraudDetection.MLConfig.MLWeight))

	if h.onUpdate != nil {
		return h.onUpdate(oldConfig, newConfig)
	}

	return nil
}

// GetConfigSection returns the configuration section this handler handles
func (h *MLConfigUpdateHandler) GetConfigSection() string {
	return "ml"
}
