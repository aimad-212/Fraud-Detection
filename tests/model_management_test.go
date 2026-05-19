package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fraud-detection/internal/config"
	httpHandler "fraud-detection/internal/delivery/http"
	"fraud-detection/internal/domain"
	dynamicConfig "fraud-detection/internal/infrastructure/config"
	"fraud-detection/internal/infrastructure/ml"
	"fraud-detection/internal/repository"
	"fraud-detection/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestModelManager(t *testing.T) {
	logger := zaptest.NewLogger(t)
	modelRepo := repository.NewInMemoryModelRepository()
	tempDir := t.TempDir()

	modelManager := usecase.NewModelManager(modelRepo, logger, tempDir)

	t.Run("UploadModel", func(t *testing.T) {
		// Create a temporary model file
		modelFile := filepath.Join(tempDir, "test_model.onnx")
		content := []byte("fake onnx model content")
		require.NoError(t, os.WriteFile(modelFile, content, 0644))

		// Create multipart form data
		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)

		// Add model file
		part, err := writer.CreateFormFile("model", "test_model.onnx")
		require.NoError(t, err)
		_, err = part.Write(content)
		require.NoError(t, err)

		// Add form fields
		require.NoError(t, writer.WriteField("name", "fraud-model"))
		require.NoError(t, writer.WriteField("version", "1.0.0"))
		require.NoError(t, writer.WriteField("description", "Test model"))
		require.NoError(t, writer.Close())

		// Create HTTP request
		req := httptest.NewRequest(http.MethodPost, "/models/upload", &requestBody)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		// Create test file reader
		file, header, err := req.FormFile("model")
		require.NoError(t, err)
		defer file.Close()

		// Upload model
		model, err := modelManager.UploadModel(file, header, "fraud-model", "1.0.0", "Test model")
		require.NoError(t, err)
		assert.Equal(t, "fraud-model", model.Name)
		assert.Equal(t, "1.0.0", model.Version)
		assert.Equal(t, domain.ModelTypeONNX, model.Type)
		assert.Equal(t, domain.ModelStatusInactive, model.Status)
		assert.False(t, model.IsActive)
	})

	t.Run("ActivateModel", func(t *testing.T) {
		// First upload a model
		modelFile := filepath.Join(tempDir, "test_model2.onnx")
		content := []byte("fake onnx model content 2")
		require.NoError(t, os.WriteFile(modelFile, content, 0644))

		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)

		part, err := writer.CreateFormFile("model", "test_model2.onnx")
		require.NoError(t, err)
		_, err = part.Write(content)
		require.NoError(t, err)

		require.NoError(t, writer.WriteField("name", "fraud-model"))
		require.NoError(t, writer.WriteField("version", "2.0.0"))
		require.NoError(t, writer.WriteField("description", "Test model v2"))
		require.NoError(t, writer.Close())

		req := httptest.NewRequest(http.MethodPost, "/models/upload", &requestBody)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		file, header, err := req.FormFile("model")
		require.NoError(t, err)
		defer file.Close()

		model, err := modelManager.UploadModel(file, header, "fraud-model", "2.0.0", "Test model v2")
		require.NoError(t, err)

		// Activate the model
		err = modelManager.ActivateModel(model.ID)
		require.NoError(t, err)

		// Check that model is now active
		activeModel, err := modelManager.GetActiveModel("fraud-model")
		require.NoError(t, err)
		assert.Equal(t, model.ID, activeModel.ID)
		assert.Equal(t, "2.0.0", activeModel.Version)
		assert.True(t, activeModel.IsActive)
		assert.Equal(t, domain.ModelStatusActive, activeModel.Status)
	})

	t.Run("RollbackModel", func(t *testing.T) {
		// Upload another model version
		modelFile := filepath.Join(tempDir, "test_model3.onnx")
		content := []byte("fake onnx model content 3")
		require.NoError(t, os.WriteFile(modelFile, content, 0644))

		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)

		part, err := writer.CreateFormFile("model", "test_model3.onnx")
		require.NoError(t, err)
		_, err = part.Write(content)
		require.NoError(t, err)

		require.NoError(t, writer.WriteField("name", "fraud-model"))
		require.NoError(t, writer.WriteField("version", "3.0.0"))
		require.NoError(t, writer.WriteField("description", "Test model v3"))
		require.NoError(t, writer.Close())

		req := httptest.NewRequest(http.MethodPost, "/models/upload", &requestBody)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		file, header, err := req.FormFile("model")
		require.NoError(t, err)
		defer file.Close()

		model, err := modelManager.UploadModel(file, header, "fraud-model", "3.0.0", "Test model v3")
		require.NoError(t, err)

		// Activate the new model
		err = modelManager.ActivateModel(model.ID)
		require.NoError(t, err)

		// Rollback to version 2.0.0
		err = modelManager.RollbackModel("fraud-model", "2.0.0")
		require.NoError(t, err)

		// Check that version 2.0.0 is now active
		activeModel, err := modelManager.GetActiveModel("fraud-model")
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", activeModel.Version)
	})

	t.Run("GetModelHistory", func(t *testing.T) {
		history, err := modelManager.GetModelHistory("fraud-model")
		require.NoError(t, err)
		assert.Len(t, history, 3) // We have 3 versions: 1.0.0, 2.0.0, 3.0.0

		// Check that versions are in descending order
		assert.Equal(t, "3.0.0", history[0].Version)
		assert.Equal(t, "2.0.0", history[1].Version)
		assert.Equal(t, "1.0.0", history[2].Version)
	})

	t.Run("GetModelMetrics", func(t *testing.T) {
		metrics, err := modelManager.GetModelMetrics("fraud-model")
		require.NoError(t, err)
		assert.Equal(t, "fraud-model", metrics.ModelName)
		assert.Equal(t, 3, metrics.TotalVersions)
		assert.Equal(t, "2.0.0", metrics.ActiveVersion)
		assert.Len(t, metrics.VersionHistory, 3)
	})
}

func TestModelHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	modelRepo := repository.NewInMemoryModelRepository()
	tempDir := t.TempDir()

	modelManager := usecase.NewModelManager(modelRepo, logger, tempDir)
	handler := httpHandler.NewModelHandler(modelManager, logger)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	t.Run("UploadModel", func(t *testing.T) {
		// Create a temporary model file
		modelFile := filepath.Join(tempDir, "test_handler_model.onnx")
		content := []byte("fake onnx model content for handler")
		require.NoError(t, os.WriteFile(modelFile, content, 0644))

		// Create multipart form data
		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)

		part, err := writer.CreateFormFile("model", "test_handler_model.onnx")
		require.NoError(t, err)
		_, err = part.Write(content)
		require.NoError(t, err)

		require.NoError(t, writer.WriteField("name", "handler-test-model"))
		require.NoError(t, writer.WriteField("version", "1.0.0"))
		require.NoError(t, writer.WriteField("description", "Handler test model"))
		require.NoError(t, writer.Close())

		// Create HTTP request
		req := httptest.NewRequest(http.MethodPost, "/api/v1/models/upload", &requestBody)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		// Perform request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Model uploaded successfully", response["message"])
		assert.NotNil(t, response["model"])
	})

	t.Run("ListModels", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.NotNil(t, response["models"])
		assert.NotNil(t, response["total"])
	})

	t.Run("GetActiveModel", func(t *testing.T) {
		// First activate a model
		models, err := modelRepo.GetByName("handler-test-model")
		require.NoError(t, err)
		require.Len(t, models, 1)

		err = modelManager.ActivateModel(models[0].ID)
		require.NoError(t, err)

		// Get active model
		req := httptest.NewRequest(http.MethodGet, "/api/v1/models/by-name/handler-test-model/active", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response httpHandler.ModelResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "handler-test-model", response.Name)
		assert.Equal(t, "1.0.0", response.Version)
		assert.True(t, response.IsActive)
	})
}

func TestModelHotReloadService(t *testing.T) {
	logger := zaptest.NewLogger(t)
	modelRepo := repository.NewInMemoryModelRepository()
	tempDir := t.TempDir()

	hotReloadService := ml.NewModelHotReloadService(modelRepo, logger, tempDir)

	// Register handlers
	onnxHandler := ml.NewONNXModelReloadHandler(logger)
	mockHandler := ml.NewMockModelReloadHandler(logger)

	hotReloadService.RegisterReloadHandler(onnxHandler)
	hotReloadService.RegisterReloadHandler(mockHandler)

	t.Run("StartStop", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := hotReloadService.Start(ctx)
		require.NoError(t, err)

		// Let it run for a bit
		time.Sleep(100 * time.Millisecond)

		hotReloadService.Stop()
	})

	t.Run("GetActiveModel", func(t *testing.T) {
		// Create and activate a model
		model := domain.NewModelVersion("test-model", "1.0.0", "onnx", "/path/to/model.onnx", "checksum", "Test model")
		model.Activate()

		err := modelRepo.Create(model)
		require.NoError(t, err)

		// Check if hot reload service can find it
		activeModel, exists := hotReloadService.GetActiveModel("test-model")
		assert.False(t, exists) // Not loaded yet since we're not running the service
		assert.Nil(t, activeModel)
	})
}

func TestDynamicConfigManager(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Create initial config
	initialConfig := config.NewConfig()
	initialConfig.FraudDetection.MLConfig.RuleWeight = 0.6
	initialConfig.FraudDetection.MLConfig.MLWeight = 0.4

	// Save initial config
	data, err := json.Marshal(initialConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0644))

	t.Run("LoadConfig", func(t *testing.T) {
		dcm, err := dynamicConfig.NewDynamicConfigManager(configPath, logger)
		require.NoError(t, err)

		loadedConfig := dcm.GetConfig()
		assert.Equal(t, 0.6, loadedConfig.FraudDetection.MLConfig.RuleWeight)
		assert.Equal(t, 0.4, loadedConfig.FraudDetection.MLConfig.MLWeight)
	})

	t.Run("UpdateMLConfig", func(t *testing.T) {
		dcm, err := dynamicConfig.NewDynamicConfigManager(configPath, logger)
		require.NoError(t, err)

		// Update ML config
		newMLConfig := &config.MLConfig{
			RuleWeight:        0.7,
			MLWeight:          0.3,
			ApproveThreshold:  0.2,
			FlagThreshold:     0.8,
			RejectThreshold:   0.95,
			PredictionTimeout: "100ms",
		}

		err = dcm.UpdateMLConfig(newMLConfig, "test")
		require.NoError(t, err)

		// Check updated config
		updatedConfig := dcm.GetConfig()
		assert.Equal(t, 0.7, updatedConfig.FraudDetection.MLConfig.RuleWeight)
		assert.Equal(t, 0.3, updatedConfig.FraudDetection.MLConfig.MLWeight)
		assert.Equal(t, 0.2, updatedConfig.FraudDetection.MLConfig.ApproveThreshold)
	})

	t.Run("ConfigValidation", func(t *testing.T) {
		dcm, err := dynamicConfig.NewDynamicConfigManager(configPath, logger)
		require.NoError(t, err)

		// Test invalid ML config (weights don't sum to 1.0)
		invalidMLConfig := &config.MLConfig{
			RuleWeight: 0.8,
			MLWeight:   0.5, // Sum is 1.3, should fail
		}

		err = dcm.UpdateMLConfig(invalidMLConfig, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must sum to 1.0")

		// Test invalid thresholds
		invalidThresholdConfig := &config.MLConfig{
			RuleWeight:       0.6,
			MLWeight:         0.4,
			ApproveThreshold: 0.8, // Higher than flag threshold
			FlagThreshold:    0.7,
			RejectThreshold:  0.9,
		}

		err = dcm.UpdateMLConfig(invalidThresholdConfig, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "approve_threshold must be less than flag_threshold")
	})
}

func TestModelVersioning(t *testing.T) {
	logger := zaptest.NewLogger(t)
	modelRepo := repository.NewInMemoryModelRepository()
	tempDir := t.TempDir()

	modelManager := usecase.NewModelManager(modelRepo, logger, tempDir)

	t.Run("VersionSequence", func(t *testing.T) {
		// Upload multiple versions
		versions := []string{"1.0.0", "1.1.0", "2.0.0", "2.1.0"}

		for i, version := range versions {
			modelFile := filepath.Join(tempDir, fmt.Sprintf("model_%s.onnx", version))
			content := []byte(fmt.Sprintf("model content for version %s", version))
			require.NoError(t, os.WriteFile(modelFile, content, 0644))

			var requestBody bytes.Buffer
			writer := multipart.NewWriter(&requestBody)

			part, err := writer.CreateFormFile("model", fmt.Sprintf("model_%s.onnx", version))
			require.NoError(t, err)
			_, err = part.Write(content)
			require.NoError(t, err)

			require.NoError(t, writer.WriteField("name", "version-test-model"))
			require.NoError(t, writer.WriteField("version", version))
			require.NoError(t, writer.WriteField("description", fmt.Sprintf("Test model version %s", version)))
			require.NoError(t, writer.Close())

			req := httptest.NewRequest(http.MethodPost, "/models/upload", &requestBody)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			file, header, err := req.FormFile("model")
			require.NoError(t, err)
			defer file.Close()

			model, err := modelManager.UploadModel(file, header, "version-test-model", version, fmt.Sprintf("Test model version %s", version))
			require.NoError(t, err)
			assert.Equal(t, version, model.Version)

			// Activate the last version
			if i == len(versions)-1 {
				err = modelManager.ActivateModel(model.ID)
				require.NoError(t, err)
			}
		}

		// Check version history
		history, err := modelManager.GetModelHistory("version-test-model")
		require.NoError(t, err)
		assert.Len(t, history, 4)

		// Check that versions are in descending order
		assert.Equal(t, "2.1.0", history[0].Version)
		assert.Equal(t, "2.0.0", history[1].Version)
		assert.Equal(t, "1.1.0", history[2].Version)
		assert.Equal(t, "1.0.0", history[3].Version)

		// Check active version
		activeModel, err := modelManager.GetActiveModel("version-test-model")
		require.NoError(t, err)
		assert.Equal(t, "2.1.0", activeModel.Version)
	})

	t.Run("RollbackToSpecificVersion", func(t *testing.T) {
		// Rollback to version 1.1.0
		err := modelManager.RollbackModel("version-test-model", "1.1.0")
		require.NoError(t, err)

		// Check active version
		activeModel, err := modelManager.GetActiveModel("version-test-model")
		require.NoError(t, err)
		assert.Equal(t, "1.1.0", activeModel.Version)
	})
}

func TestModelPerformanceTracking(t *testing.T) {
	logger := zaptest.NewLogger(t)
	modelRepo := repository.NewInMemoryModelRepository()
	tempDir := t.TempDir()

	modelManager := usecase.NewModelManager(modelRepo, logger, tempDir)

	t.Run("UpdatePerformance", func(t *testing.T) {
		// Create a model
		model := domain.NewModelVersion("perf-test-model", "1.0.0", "onnx", "/path/to/model.onnx", "checksum", "Performance test model")
		model.Activate()

		err := modelRepo.Create(model)
		require.NoError(t, err)

		// Update performance metrics
		performance := &domain.ModelPerformance{
			TotalPredictions:   1000,
			CorrectPredictions: 950,
			Accuracy:           0.95,
			AverageLatency:     50.5,
			LastUpdated:        time.Now(),
			TPR:                0.92,
			FPR:                0.03,
			TNR:                0.97,
			FNR:                0.08,
		}

		err = modelManager.UpdateModelPerformance(model.ID, performance)
		require.NoError(t, err)

		// Check updated model
		updatedModel, err := modelManager.GetModel(model.ID)
		require.NoError(t, err)

		retrievedPerf, err := updatedModel.GetPerformance()
		require.NoError(t, err)
		assert.Equal(t, int64(1000), retrievedPerf.TotalPredictions)
		assert.Equal(t, int64(950), retrievedPerf.CorrectPredictions)
		assert.Equal(t, 0.95, retrievedPerf.Accuracy)
		assert.Equal(t, 50.5, retrievedPerf.AverageLatency)
	})

	t.Run("GetModelMetricsWithPerformance", func(t *testing.T) {
		metrics, err := modelManager.GetModelMetrics("perf-test-model")
		require.NoError(t, err)
		assert.NotNil(t, metrics.RecentPerformance)
		assert.Equal(t, 0.95, metrics.RecentPerformance.Accuracy)
		assert.Equal(t, 50.5, metrics.RecentPerformance.AverageLatency)
	})
}
