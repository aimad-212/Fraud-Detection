package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/repository"

	"go.uber.org/zap"
)

// ModelManager defines the interface for model management operations
type ModelManager interface {
	UploadModel(file multipart.File, header *multipart.FileHeader, name, version, description string) (*domain.ModelVersion, error)
	ActivateModel(modelID string) error
	DeactivateModel(modelID string) error
	GetModel(modelID string) (*domain.ModelVersion, error)
	GetActiveModel(name string) (*domain.ModelVersion, error)
	ListModels(limit, offset int) ([]*domain.ModelVersion, error)
	GetModelHistory(name string) ([]*domain.ModelVersion, error)
	DeleteModel(modelID string) error
	RollbackModel(name, targetVersion string) error
	UpdateModelPerformance(modelID string, performance *domain.ModelPerformance) error
	ValidateModelFile(filePath string) error
	GetModelMetrics(name string) (*ModelMetrics, error)
}

// ModelMetrics represents aggregated model metrics
type ModelMetrics struct {
	ModelName         string                   `json:"model_name"`
	TotalVersions     int                      `json:"total_versions"`
	ActiveVersion     string                   `json:"active_version"`
	RecentPerformance *domain.ModelPerformance `json:"recent_performance"`
	VersionHistory    []*VersionSummary        `json:"version_history"`
	LastUpdated       time.Time                `json:"last_updated"`
}

// VersionSummary represents a summary of a model version
type VersionSummary struct {
	Version     string                   `json:"version"`
	Status      domain.ModelStatus       `json:"status"`
	IsActive    bool                     `json:"is_active"`
	CreatedAt   time.Time                `json:"created_at"`
	ActivatedAt *time.Time               `json:"activated_at,omitempty"`
	Performance *domain.ModelPerformance `json:"performance,omitempty"`
}

// ModelManagerImpl implements ModelManager
type ModelManagerImpl struct {
	modelRepo      repository.ModelRepository
	logger         *zap.Logger
	modelsDir      string
	eventPublisher ModelEventPublisher
}

// ModelEventPublisher defines the interface for publishing model events
type ModelEventPublisher interface {
	PublishModelUpdate(event *domain.ModelUpdateEvent) error
}

// NewModelManager creates a new model manager
func NewModelManager(modelRepo repository.ModelRepository, logger *zap.Logger, modelsDir string) ModelManager {
	return &ModelManagerImpl{
		modelRepo: modelRepo,
		logger:    logger,
		modelsDir: modelsDir,
	}
}

// SetEventPublisher sets the event publisher
func (mm *ModelManagerImpl) SetEventPublisher(publisher ModelEventPublisher) {
	mm.eventPublisher = publisher
}

// UploadModel uploads and registers a new model version
func (mm *ModelManagerImpl) UploadModel(file multipart.File, header *multipart.FileHeader, name, version, description string) (*domain.ModelVersion, error) {
	// Validate inputs
	if name == "" {
		return nil, fmt.Errorf("model name is required")
	}
	if version == "" {
		return nil, fmt.Errorf("model version is required")
	}
	if header == nil {
		return nil, fmt.Errorf("model file is required")
	}

	// Create models directory if it doesn't exist
	if err := os.MkdirAll(mm.modelsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create models directory: %w", err)
	}

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%s-%s%s", name, version, ext)
	filePath := filepath.Join(mm.modelsDir, filename)

	// Save file to disk
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create model file: %w", err)
	}
	defer dst.Close()

	// Calculate checksum while copying
	hasher := sha256.New()
	multiWriter := io.MultiWriter(dst, hasher)

	if _, err := io.Copy(multiWriter, file); err != nil {
		os.Remove(filePath) // Clean up on error
		return nil, fmt.Errorf("failed to save model file: %w", err)
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))

	// Validate model file
	if err := mm.ValidateModelFile(filePath); err != nil {
		os.Remove(filePath) // Clean up on error
		return nil, fmt.Errorf("model validation failed: %w", err)
	}

	// Determine model type from file extension
	modelType := mm.determineModelType(ext)

	// Create model version
	model := domain.NewModelVersion(name, version, string(modelType), filePath, checksum, description)

	// Save to repository
	if err := mm.modelRepo.Create(model); err != nil {
		os.Remove(filePath) // Clean up on error
		return nil, fmt.Errorf("failed to save model version: %w", err)
	}

	mm.logger.Info("Model uploaded successfully",
		zap.String("model_name", name),
		zap.String("version", version),
		zap.String("file_path", filePath),
		zap.String("checksum", checksum))

	// Publish model update event
	if mm.eventPublisher != nil {
		event := domain.NewModelUpdateEvent(model.ID, name, version, "upload", "user", "Model uploaded via API")
		if err := mm.eventPublisher.PublishModelUpdate(event); err != nil {
			mm.logger.Error("Failed to publish model update event",
				zap.Error(err),
				zap.String("model_id", model.ID))
		}
	}

	return model, nil
}

// ActivateModel activates a model version
func (mm *ModelManagerImpl) ActivateModel(modelID string) error {
	// Get current active model for rollback info
	model, err := mm.modelRepo.GetByID(modelID)
	if err != nil {
		return fmt.Errorf("failed to get model: %w", err)
	}

	var oldVersion *string
	if activeModel, err := mm.modelRepo.GetActiveModel(model.Name); err == nil {
		oldVersion = &activeModel.Version
	}

	// Activate the model
	if err := mm.modelRepo.ActivateModel(modelID); err != nil {
		return fmt.Errorf("failed to activate model: %w", err)
	}

	mm.logger.Info("Model activated successfully",
		zap.String("model_id", modelID),
		zap.String("model_name", model.Name),
		zap.String("version", model.Version))

	// Publish model update event
	if mm.eventPublisher != nil {
		event := domain.NewModelUpdateEvent(modelID, model.Name, model.Version, "activate", "user", "Model activated via API")
		event.OldVersion = oldVersion
		event.NewVersion = &model.Version
		if err := mm.eventPublisher.PublishModelUpdate(event); err != nil {
			mm.logger.Error("Failed to publish model update event",
				zap.Error(err),
				zap.String("model_id", modelID))
		}
	}

	return nil
}

// DeactivateModel deactivates a model version
func (mm *ModelManagerImpl) DeactivateModel(modelID string) error {
	model, err := mm.modelRepo.GetByID(modelID)
	if err != nil {
		return fmt.Errorf("failed to get model: %w", err)
	}

	if err := mm.modelRepo.DeactivateModel(modelID); err != nil {
		return fmt.Errorf("failed to deactivate model: %w", err)
	}

	mm.logger.Info("Model deactivated successfully",
		zap.String("model_id", modelID),
		zap.String("model_name", model.Name),
		zap.String("version", model.Version))

	// Publish model update event
	if mm.eventPublisher != nil {
		event := domain.NewModelUpdateEvent(modelID, model.Name, model.Version, "deactivate", "user", "Model deactivated via API")
		if err := mm.eventPublisher.PublishModelUpdate(event); err != nil {
			mm.logger.Error("Failed to publish model update event",
				zap.Error(err),
				zap.String("model_id", modelID))
		}
	}

	return nil
}

// GetModel retrieves a model version by ID
func (mm *ModelManagerImpl) GetModel(modelID string) (*domain.ModelVersion, error) {
	return mm.modelRepo.GetByID(modelID)
}

// GetActiveModel retrieves the active model version for a given name
func (mm *ModelManagerImpl) GetActiveModel(name string) (*domain.ModelVersion, error) {
	return mm.modelRepo.GetActiveModel(name)
}

// ListModels returns a paginated list of model versions
func (mm *ModelManagerImpl) ListModels(limit, offset int) ([]*domain.ModelVersion, error) {
	return mm.modelRepo.List(limit, offset)
}

// GetModelHistory retrieves the version history for a model
func (mm *ModelManagerImpl) GetModelHistory(name string) ([]*domain.ModelVersion, error) {
	return mm.modelRepo.GetModelHistory(name)
}

// DeleteModel deletes a model version
func (mm *ModelManagerImpl) DeleteModel(modelID string) error {
	model, err := mm.modelRepo.GetByID(modelID)
	if err != nil {
		return fmt.Errorf("failed to get model: %w", err)
	}

	// Don't allow deletion of active models
	if model.IsActive {
		return fmt.Errorf("cannot delete active model version")
	}

	// Delete from repository
	if err := mm.modelRepo.Delete(modelID); err != nil {
		return fmt.Errorf("failed to delete model: %w", err)
	}

	// Delete file from disk
	if err := os.Remove(model.FilePath); err != nil && !os.IsNotExist(err) {
		mm.logger.Warn("Failed to delete model file from disk",
			zap.Error(err),
			zap.String("file_path", model.FilePath))
	}

	mm.logger.Info("Model deleted successfully",
		zap.String("model_id", modelID),
		zap.String("model_name", model.Name),
		zap.String("version", model.Version))

	// Publish model update event
	if mm.eventPublisher != nil {
		event := domain.NewModelUpdateEvent(modelID, model.Name, model.Version, "delete", "user", "Model deleted via API")
		if err := mm.eventPublisher.PublishModelUpdate(event); err != nil {
			mm.logger.Error("Failed to publish model update event",
				zap.Error(err),
				zap.String("model_id", modelID))
		}
	}

	return nil
}

// RollbackModel rolls back to a specific version
func (mm *ModelManagerImpl) RollbackModel(name, targetVersion string) error {
	// Get current active model
	currentActive, err := mm.modelRepo.GetActiveModel(name)
	if err == nil {
		// Deactivate current model
		if err := mm.modelRepo.DeactivateModel(currentActive.ID); err != nil {
			return fmt.Errorf("failed to deactivate current model: %w", err)
		}
	}

	// Find target version
	models, err := mm.modelRepo.GetByName(name)
	if err != nil {
		return fmt.Errorf("failed to get model versions: %w", err)
	}

	var targetModel *domain.ModelVersion
	for _, model := range models {
		if model.Version == targetVersion {
			targetModel = model
			break
		}
	}

	if targetModel == nil {
		return fmt.Errorf("model version %s not found", targetVersion)
	}

	// Activate target version
	if err := mm.modelRepo.ActivateModel(targetModel.ID); err != nil {
		return fmt.Errorf("failed to activate target model: %w", err)
	}

	mm.logger.Info("Model rolled back successfully",
		zap.String("model_name", name),
		zap.String("target_version", targetVersion))

	// Publish model update event
	if mm.eventPublisher != nil {
		event := domain.NewModelUpdateEvent(targetModel.ID, name, targetVersion, "rollback", "user", fmt.Sprintf("Rolled back to version %s", targetVersion))
		if currentActive != nil {
			event.OldVersion = &currentActive.Version
		}
		event.NewVersion = &targetVersion
		if err := mm.eventPublisher.PublishModelUpdate(event); err != nil {
			mm.logger.Error("Failed to publish model update event",
				zap.Error(err),
				zap.String("model_id", targetModel.ID))
		}
	}

	return nil
}

// UpdateModelPerformance updates the performance metrics for a model
func (mm *ModelManagerImpl) UpdateModelPerformance(modelID string, performance *domain.ModelPerformance) error {
	model, err := mm.modelRepo.GetByID(modelID)
	if err != nil {
		return fmt.Errorf("failed to get model: %w", err)
	}

	model.UpdatePerformance(performance)

	if err := mm.modelRepo.Update(model); err != nil {
		return fmt.Errorf("failed to update model performance: %w", err)
	}

	mm.logger.Info("Model performance updated",
		zap.String("model_id", modelID),
		zap.Float64("accuracy", performance.Accuracy),
		zap.Float64("average_latency_ms", performance.AverageLatency))

	return nil
}

// ValidateModelFile validates a model file
func (mm *ModelManagerImpl) ValidateModelFile(filePath string) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("model file does not exist: %s", filePath)
	}

	// Check file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if fileInfo.Size() == 0 {
		return fmt.Errorf("model file is empty")
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	validExts := []string{".onnx", ".pkl", ".joblib", ".h5", ".pb", ".tflite"}
	valid := false
	for _, validExt := range validExts {
		if ext == validExt {
			valid = true
			break
		}
	}

	if !valid {
		return fmt.Errorf("unsupported model file extension: %s", ext)
	}

	// Additional validation can be added here based on model type
	// For example, checking ONNX model structure, etc.

	return nil
}

// GetModelMetrics retrieves aggregated metrics for a model
func (mm *ModelManagerImpl) GetModelMetrics(name string) (*ModelMetrics, error) {
	// Get all versions of the model
	versions, err := mm.modelRepo.GetModelHistory(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get model history: %w", err)
	}

	metrics := &ModelMetrics{
		ModelName:      name,
		TotalVersions:  len(versions),
		VersionHistory: make([]*VersionSummary, 0, len(versions)),
		LastUpdated:    time.Now(),
	}

	// Process versions
	for _, version := range versions {
		summary := &VersionSummary{
			Version:     version.Version,
			Status:      version.Status,
			IsActive:    version.IsActive,
			CreatedAt:   version.CreatedAt,
			ActivatedAt: version.ActivatedAt,
		}

		// Get performance if available
		if perf, err := version.GetPerformance(); err == nil {
			summary.Performance = perf
			if version.IsActive {
				metrics.RecentPerformance = perf
			}
		}

		metrics.VersionHistory = append(metrics.VersionHistory, summary)

		if version.IsActive {
			metrics.ActiveVersion = version.Version
		}
	}

	return metrics, nil
}

// Helper functions

func (mm *ModelManagerImpl) determineModelType(ext string) domain.ModelType {
	switch strings.ToLower(ext) {
	case ".onnx":
		return domain.ModelTypeONNX
	case ".pkl", ".joblib":
		return domain.ModelTypeCustom
	default:
		return domain.ModelTypeCustom
	}
}
