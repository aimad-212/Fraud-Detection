package ml

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/repository"

	"go.uber.org/zap"
)

// ModelHotReloadService provides hot-reload capabilities for ML models
type ModelHotReloadService struct {
	modelRepo      repository.ModelRepository
	logger         *zap.Logger
	modelsDir      string
	watchInterval  time.Duration
	stopChan       chan struct{}
	wg             sync.WaitGroup
	mu             sync.RWMutex
	activeModels   map[string]*domain.ModelVersion // name -> model
	reloadHandlers map[string]ModelReloadHandler   // modelType -> handler
}

// ModelReloadHandler defines the interface for handling model reloads
type ModelReloadHandler interface {
	ReloadModel(model *domain.ModelVersion) error
	ValidateModel(model *domain.ModelVersion) error
	GetModelType() domain.ModelType
}

// ModelReloadEvent represents a model reload event
type ModelReloadEvent struct {
	Model     *domain.ModelVersion `json:"model"`
	Action    string               `json:"action"` // loaded, reloaded, failed
	Timestamp time.Time            `json:"timestamp"`
	Error     string               `json:"error,omitempty"`
	Duration  time.Duration        `json:"duration_ms"`
}

// NewModelHotReloadService creates a new hot-reload service
func NewModelHotReloadService(modelRepo repository.ModelRepository, logger *zap.Logger, modelsDir string) *ModelHotReloadService {
	return &ModelHotReloadService{
		modelRepo:      modelRepo,
		logger:         logger,
		modelsDir:      modelsDir,
		watchInterval:  30 * time.Second, // Default check interval
		stopChan:       make(chan struct{}),
		activeModels:   make(map[string]*domain.ModelVersion),
		reloadHandlers: make(map[string]ModelReloadHandler),
	}
}

// SetWatchInterval sets the watch interval for checking model updates
func (s *ModelHotReloadService) SetWatchInterval(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watchInterval = interval
}

// RegisterReloadHandler registers a reload handler for a specific model type
func (s *ModelHotReloadService) RegisterReloadHandler(handler ModelReloadHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadHandlers[string(handler.GetModelType())] = handler
	s.logger.Info("Registered model reload handler",
		zap.String("model_type", string(handler.GetModelType())))
}

// Start starts the hot-reload service
func (s *ModelHotReloadService) Start(ctx context.Context) error {
	s.logger.Info("Starting model hot-reload service",
		zap.Duration("watch_interval", s.watchInterval))

	// Load initial active models
	if err := s.loadActiveModels(); err != nil {
		return fmt.Errorf("failed to load initial active models: %w", err)
	}

	// Start the watch loop
	s.wg.Add(1)
	go s.watchLoop(ctx)

	return nil
}

// Stop stops the hot-reload service
func (s *ModelHotReloadService) Stop() {
	s.logger.Info("Stopping model hot-reload service")
	close(s.stopChan)
	s.wg.Wait()
}

// ReloadModel manually reloads a specific model
func (s *ModelHotReloadService) ReloadModel(modelName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	model, err := s.modelRepo.GetActiveModel(modelName)
	if err != nil {
		return fmt.Errorf("failed to get active model: %w", err)
	}

	return s.reloadModelInternal(model)
}

// GetActiveModel returns the currently active model for a given name
func (s *ModelHotReloadService) GetActiveModel(modelName string) (*domain.ModelVersion, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	model, exists := s.activeModels[modelName]
	return model, exists
}

// GetAllActiveModels returns all currently active models
func (s *ModelHotReloadService) GetAllActiveModels() map[string]*domain.ModelVersion {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*domain.ModelVersion)
	for name, model := range s.activeModels {
		result[name] = model
	}
	return result
}

// loadActiveModels loads all currently active models
func (s *ModelHotReloadService) loadActiveModels() error {
	s.logger.Info("Loading active models")

	// Get all models and find active ones
	models, err := s.modelRepo.List(1000, 0) // Get all models
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	loadedCount := 0
	for _, model := range models {
		if model.IsActive {
			if err := s.loadModel(model); err != nil {
				s.logger.Error("Failed to load active model",
					zap.Error(err),
					zap.String("model_name", model.Name),
					zap.String("version", model.Version))
				continue
			}
			loadedCount++
		}
	}

	s.logger.Info("Loaded active models",
		zap.Int("total_loaded", loadedCount))

	return nil
}

// watchLoop continuously watches for model updates
func (s *ModelHotReloadService) watchLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.watchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Model hot-reload service context cancelled")
			return
		case <-s.stopChan:
			s.logger.Info("Model hot-reload service stopped")
			return
		case <-ticker.C:
			s.checkForUpdates()
		}
	}
}

// checkForUpdates checks for model updates and reloads if necessary
func (s *ModelHotReloadService) checkForUpdates() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get all models
	models, err := s.modelRepo.List(1000, 0)
	if err != nil {
		s.logger.Error("Failed to list models during update check", zap.Error(err))
		return
	}

	for _, model := range models {
		if !model.IsActive {
			continue
		}

		// Check if model needs reload
		currentModel, exists := s.activeModels[model.Name]
		if !exists || currentModel.Version != model.Version || currentModel.UpdatedAt.Before(model.UpdatedAt) {
			s.logger.Info("Detected model update, reloading",
				zap.String("model_name", model.Name),
				zap.String("old_version", func() string {
					if !exists {
						return "none"
					}
					return currentModel.Version
				}()),
				zap.String("new_version", model.Version))

			if err := s.reloadModelInternal(model); err != nil {
				s.logger.Error("Failed to reload model",
					zap.Error(err),
					zap.String("model_name", model.Name),
					zap.String("version", model.Version))
			}
		}
	}
}

// loadModel loads a model into memory
func (s *ModelHotReloadService) loadModel(model *domain.ModelVersion) error {
	start := time.Now()

	// Get the appropriate handler
	handler, exists := s.reloadHandlers[string(model.Type)]
	if !exists {
		return fmt.Errorf("no reload handler for model type: %s", model.Type)
	}

	// Validate the model
	if err := handler.ValidateModel(model); err != nil {
		return fmt.Errorf("model validation failed: %w", err)
	}

	// Reload the model
	if err := handler.ReloadModel(model); err != nil {
		return fmt.Errorf("model reload failed: %w", err)
	}

	// Update active models cache
	s.activeModels[model.Name] = model

	duration := time.Since(start)
	s.logger.Info("Model loaded successfully",
		zap.String("model_name", model.Name),
		zap.String("version", model.Version),
		zap.String("model_type", string(model.Type)),
		zap.Duration("load_duration", duration))

	// Publish reload event
	s.publishReloadEvent(&ModelReloadEvent{
		Model:     model,
		Action:    "loaded",
		Timestamp: time.Now(),
		Duration:  duration,
	})

	return nil
}

// reloadModelInternal reloads a model (internal method, assumes lock is held)
func (s *ModelHotReloadService) reloadModelInternal(model *domain.ModelVersion) error {
	start := time.Now()

	// Get the appropriate handler
	handler, exists := s.reloadHandlers[string(model.Type)]
	if !exists {
		return fmt.Errorf("no reload handler for model type: %s", model.Type)
	}

	// Validate the model
	if err := handler.ValidateModel(model); err != nil {
		s.publishReloadEvent(&ModelReloadEvent{
			Model:     model,
			Action:    "failed",
			Timestamp: time.Now(),
			Error:     fmt.Sprintf("validation failed: %v", err),
			Duration:  time.Since(start),
		})
		return fmt.Errorf("model validation failed: %w", err)
	}

	// Reload the model
	if err := handler.ReloadModel(model); err != nil {
		s.publishReloadEvent(&ModelReloadEvent{
			Model:     model,
			Action:    "failed",
			Timestamp: time.Now(),
			Error:     fmt.Sprintf("reload failed: %v", err),
			Duration:  time.Since(start),
		})
		return fmt.Errorf("model reload failed: %w", err)
	}

	// Update active models cache
	s.activeModels[model.Name] = model

	duration := time.Since(start)
	s.logger.Info("Model reloaded successfully",
		zap.String("model_name", model.Name),
		zap.String("version", model.Version),
		zap.String("model_type", string(model.Type)),
		zap.Duration("reload_duration", duration))

	// Publish reload event
	s.publishReloadEvent(&ModelReloadEvent{
		Model:     model,
		Action:    "reloaded",
		Timestamp: time.Now(),
		Duration:  duration,
	})

	return nil
}

// publishReloadEvent publishes a reload event (placeholder for event publishing)
func (s *ModelHotReloadService) publishReloadEvent(event *ModelReloadEvent) {
	// In a real implementation, this would publish to an event bus or message queue
	s.logger.Debug("Model reload event",
		zap.String("model_name", event.Model.Name),
		zap.String("version", event.Model.Version),
		zap.String("action", event.Action),
		zap.Duration("duration", event.Duration),
		zap.String("error", event.Error))
}

// ONNXModelReloadHandler implements ModelReloadHandler for ONNX models
type ONNXModelReloadHandler struct {
	logger *zap.Logger
}

// NewONNXModelReloadHandler creates a new ONNX model reload handler
func NewONNXModelReloadHandler(logger *zap.Logger) *ONNXModelReloadHandler {
	return &ONNXModelReloadHandler{logger: logger}
}

// GetModelType returns the model type this handler supports
func (h *ONNXModelReloadHandler) GetModelType() domain.ModelType {
	return domain.ModelTypeONNX
}

// ValidateModel validates an ONNX model
func (h *ONNXModelReloadHandler) ValidateModel(model *domain.ModelVersion) error {
	// Check if file exists and has correct extension
	if filepath.Ext(model.FilePath) != ".onnx" {
		return fmt.Errorf("invalid file extension for ONNX model: %s", filepath.Ext(model.FilePath))
	}

	// Additional ONNX-specific validation can be added here
	// For example, checking the model structure, inputs/outputs, etc.

	return nil
}

// ReloadModel reloads an ONNX model
func (h *ONNXModelReloadHandler) ReloadModel(model *domain.ModelVersion) error {
	// In a real implementation, this would:
	// 1. Load the ONNX model from disk
	// 2. Validate the model structure
	// 3. Update the in-memory model instance
	// 4. Handle any necessary cleanup of the old model

	h.logger.Info("Reloading ONNX model",
		zap.String("model_name", model.Name),
		zap.String("version", model.Version),
		zap.String("file_path", model.FilePath))

	// Placeholder implementation
	// In reality, this would interface with the ONNX runtime
	return nil
}

// MockModelReloadHandler implements ModelReloadHandler for mock models
type MockModelReloadHandler struct {
	logger *zap.Logger
}

// NewMockModelReloadHandler creates a new mock model reload handler
func NewMockModelReloadHandler(logger *zap.Logger) *MockModelReloadHandler {
	return &MockModelReloadHandler{logger: logger}
}

// GetModelType returns the model type this handler supports
func (h *MockModelReloadHandler) GetModelType() domain.ModelType {
	return domain.ModelTypeMock
}

// ValidateModel validates a mock model
func (h *MockModelReloadHandler) ValidateModel(model *domain.ModelVersion) error {
	// Mock models don't need file validation
	return nil
}

// ReloadModel reloads a mock model
func (h *MockModelReloadHandler) ReloadModel(model *domain.ModelVersion) error {
	h.logger.Info("Reloading mock model",
		zap.String("model_name", model.Name),
		zap.String("version", model.Version))

	// Mock models don't need actual reloading
	return nil
}
