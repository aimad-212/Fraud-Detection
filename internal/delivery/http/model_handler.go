package http

import (
	"net/http"
	"strconv"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/usecase"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ModelHandler handles HTTP requests for model management
type ModelHandler struct {
	modelManager usecase.ModelManager
	logger       *zap.Logger
}

// NewModelHandler creates a new model handler
func NewModelHandler(modelManager usecase.ModelManager, logger *zap.Logger) *ModelHandler {
	return &ModelHandler{
		modelManager: modelManager,
		logger:       logger,
	}
}

// UploadModelRequest represents the request to upload a model
type UploadModelRequest struct {
	Name        string `form:"name" binding:"required"`
	Version     string `form:"version" binding:"required"`
	Description string `form:"description"`
}

// ModelResponse represents the response for model operations
type ModelResponse struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Version     string                   `json:"version"`
	Type        domain.ModelType         `json:"type"`
	Status      domain.ModelStatus       `json:"status"`
	FilePath    string                   `json:"file_path"`
	Checksum    string                   `json:"checksum"`
	Description string                   `json:"description"`
	Metadata    *domain.ModelMetadata    `json:"metadata,omitempty"`
	Performance *domain.ModelPerformance `json:"performance,omitempty"`
	IsActive    bool                     `json:"is_active"`
	CreatedAt   string                   `json:"created_at"`
	UpdatedAt   string                   `json:"updated_at"`
	ActivatedAt *string                  `json:"activated_at,omitempty"`
}

// ModelListResponse represents the response for model list
type ModelListResponse struct {
	Models []ModelResponse `json:"models"`
	Total  int             `json:"total"`
}

// ModelMetricsResponse represents the response for model metrics
type ModelMetricsResponse struct {
	ModelName         string                   `json:"model_name"`
	TotalVersions     int                      `json:"total_versions"`
	ActiveVersion     string                   `json:"active_version"`
	RecentPerformance *domain.ModelPerformance `json:"recent_performance,omitempty"`
	VersionHistory    []usecase.VersionSummary `json:"version_history"`
	LastUpdated       string                   `json:"last_updated"`
}

// UploadModel handles model upload
func (h *ModelHandler) UploadModel(c *gin.Context) {
	var req UploadModelRequest
	if err := c.ShouldBind(&req); err != nil {
		h.logger.Error("Invalid upload request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("model")
	if err != nil {
		h.logger.Error("Failed to get uploaded file", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model file is required"})
		return
	}
	defer file.Close()

	// Upload model
	model, err := h.modelManager.UploadModel(file, header, req.Name, req.Version, req.Description)
	if err != nil {
		h.logger.Error("Failed to upload model",
			zap.Error(err),
			zap.String("name", req.Name),
			zap.String("version", req.Version))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload model: " + err.Error()})
		return
	}

	response := h.modelToResponse(model)
	c.JSON(http.StatusCreated, gin.H{
		"message": "Model uploaded successfully",
		"model":   response,
	})
}

// ActivateModel handles model activation
func (h *ModelHandler) ActivateModel(c *gin.Context) {
	modelID := c.Param("modelId")
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model ID is required"})
		return
	}

	if err := h.modelManager.ActivateModel(modelID); err != nil {
		h.logger.Error("Failed to activate model",
			zap.Error(err),
			zap.String("model_id", modelID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to activate model: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Model activated successfully"})
}

// DeactivateModel handles model deactivation
func (h *ModelHandler) DeactivateModel(c *gin.Context) {
	modelID := c.Param("modelId")
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model ID is required"})
		return
	}

	if err := h.modelManager.DeactivateModel(modelID); err != nil {
		h.logger.Error("Failed to deactivate model",
			zap.Error(err),
			zap.String("model_id", modelID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate model: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Model deactivated successfully"})
}

// GetModel handles getting a specific model
func (h *ModelHandler) GetModel(c *gin.Context) {
	modelID := c.Param("modelId")
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model ID is required"})
		return
	}

	model, err := h.modelManager.GetModel(modelID)
	if err != nil {
		h.logger.Error("Failed to get model",
			zap.Error(err),
			zap.String("model_id", modelID))
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found: " + err.Error()})
		return
	}

	response := h.modelToResponse(model)
	c.JSON(http.StatusOK, response)
}

// GetActiveModel handles getting the active model for a given name
func (h *ModelHandler) GetActiveModel(c *gin.Context) {
	modelName := c.Param("modelName")
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model name is required"})
		return
	}

	model, err := h.modelManager.GetActiveModel(modelName)
	if err != nil {
		h.logger.Error("Failed to get active model",
			zap.Error(err),
			zap.String("model_name", modelName))
		c.JSON(http.StatusNotFound, gin.H{"error": "Active model not found: " + err.Error()})
		return
	}

	response := h.modelToResponse(model)
	c.JSON(http.StatusOK, response)
}

// ListModels handles listing models
func (h *ModelHandler) ListModels(c *gin.Context) {
	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	models, err := h.modelManager.ListModels(limit, offset)
	if err != nil {
		h.logger.Error("Failed to list models", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list models: " + err.Error()})
		return
	}

	response := ModelListResponse{
		Models: make([]ModelResponse, len(models)),
		Total:  len(models),
	}

	for i, model := range models {
		response.Models[i] = h.modelToResponse(model)
	}

	c.JSON(http.StatusOK, response)
}

// GetModelHistory handles getting model version history
func (h *ModelHandler) GetModelHistory(c *gin.Context) {
	modelName := c.Param("modelName")
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model name is required"})
		return
	}

	models, err := h.modelManager.GetModelHistory(modelName)
	if err != nil {
		h.logger.Error("Failed to get model history",
			zap.Error(err),
			zap.String("model_name", modelName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get model history: " + err.Error()})
		return
	}

	response := make([]ModelResponse, len(models))
	for i, model := range models {
		response[i] = h.modelToResponse(model)
	}

	c.JSON(http.StatusOK, gin.H{
		"model_name": modelName,
		"versions":   response,
		"total":      len(response),
	})
}

// DeleteModel handles model deletion
func (h *ModelHandler) DeleteModel(c *gin.Context) {
	modelID := c.Param("modelId")
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model ID is required"})
		return
	}

	if err := h.modelManager.DeleteModel(modelID); err != nil {
		h.logger.Error("Failed to delete model",
			zap.Error(err),
			zap.String("model_id", modelID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete model: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Model deleted successfully"})
}

// RollbackModel handles model rollback
func (h *ModelHandler) RollbackModel(c *gin.Context) {
	modelName := c.Param("modelName")
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model name is required"})
		return
	}

	targetVersion := c.Param("version")
	if targetVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Target version is required"})
		return
	}

	if err := h.modelManager.RollbackModel(modelName, targetVersion); err != nil {
		h.logger.Error("Failed to rollback model",
			zap.Error(err),
			zap.String("model_name", modelName),
			zap.String("target_version", targetVersion))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rollback model: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Model rolled back successfully"})
}

// GetModelMetrics handles getting model metrics
func (h *ModelHandler) GetModelMetrics(c *gin.Context) {
	modelName := c.Param("modelName")
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model name is required"})
		return
	}

	metrics, err := h.modelManager.GetModelMetrics(modelName)
	if err != nil {
		h.logger.Error("Failed to get model metrics",
			zap.Error(err),
			zap.String("model_name", modelName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get model metrics: " + err.Error()})
		return
	}

	response := ModelMetricsResponse{
		ModelName:         metrics.ModelName,
		TotalVersions:     metrics.TotalVersions,
		ActiveVersion:     metrics.ActiveVersion,
		RecentPerformance: metrics.RecentPerformance,
		VersionHistory:    make([]usecase.VersionSummary, len(metrics.VersionHistory)),
		LastUpdated:       metrics.LastUpdated.Format("2006-01-02T15:04:05Z"),
	}

	for i, version := range metrics.VersionHistory {
		response.VersionHistory[i] = *version
	}

	c.JSON(http.StatusOK, response)
}

// Helper functions

func (h *ModelHandler) modelToResponse(model *domain.ModelVersion) ModelResponse {
	response := ModelResponse{
		ID:          model.ID,
		Name:        model.Name,
		Version:     model.Version,
		Type:        model.Type,
		Status:      model.Status,
		FilePath:    model.FilePath,
		Checksum:    model.Checksum,
		Description: model.Description,
		IsActive:    model.IsActive,
		CreatedAt:   model.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   model.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if model.ActivatedAt != nil {
		activatedAt := model.ActivatedAt.Format("2006-01-02T15:04:05Z")
		response.ActivatedAt = &activatedAt
	}

	// Parse metadata if available
	if metadata, err := model.GetMetadata(); err == nil {
		response.Metadata = metadata
	}

	// Parse performance if available
	if performance, err := model.GetPerformance(); err == nil {
		response.Performance = performance
	}

	return response
}

// RegisterRoutes registers the model management routes
func (h *ModelHandler) RegisterRoutes(router *gin.RouterGroup) {
	// Model CRUD operations - use UUID-based routes
	models := router.Group("/models")
	{
		models.POST("/upload", h.UploadModel)                  // Upload new model
		models.GET("", h.ListModels)                           // List all models
		models.GET("/:modelId", h.GetModel)                    // Get specific model by ID
		models.DELETE("/:modelId", h.DeleteModel)              // Delete model by ID
		models.POST("/:modelId/activate", h.ActivateModel)     // Activate model by ID
		models.POST("/:modelId/deactivate", h.DeactivateModel) // Deactivate model by ID
	}

	// Model operations by name - use separate path segments to avoid conflicts
	router.GET("/models/by-name/:modelName/active", h.GetActiveModel)            // Get active model by name
	router.GET("/models/by-name/:modelName/history", h.GetModelHistory)          // Get model version history
	router.POST("/models/by-name/:modelName/rollback/:version", h.RollbackModel) // Rollback to specific version
	router.GET("/models/by-name/:modelName/metrics", h.GetModelMetrics)          // Get model metrics
}
