package tests

import (
	"os"
	"testing"

	"fraud-detection/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMLConfigurationLoading(t *testing.T) {
	t.Run("Default ML Configuration", func(t *testing.T) {
		cfg := config.NewConfig()

		// Test default ML configuration values
		assert.True(t, cfg.FraudDetection.EnableML)
		assert.Equal(t, "./ml/models/fraud_model.onnx", cfg.FraudDetection.ModelPath)
		assert.Equal(t, "onnx", cfg.FraudDetection.MLConfig.ModelType)
		assert.Equal(t, "./ml/models/fraud_model.onnx", cfg.FraudDetection.MLConfig.ModelPath)
		assert.Equal(t, "1.0.0", cfg.FraudDetection.MLConfig.ModelVersion)
		assert.True(t, cfg.FraudDetection.MLConfig.FallbackToRules)
		assert.Equal(t, 0.6, cfg.FraudDetection.MLConfig.RuleWeight)
		assert.Equal(t, 0.4, cfg.FraudDetection.MLConfig.MLWeight)
		assert.False(t, cfg.FraudDetection.MLConfig.EnableDynamicWeights)
		assert.Equal(t, 0.3, cfg.FraudDetection.MLConfig.ApproveThreshold)
		assert.Equal(t, 0.7, cfg.FraudDetection.MLConfig.FlagThreshold)
		assert.Equal(t, 0.9, cfg.FraudDetection.MLConfig.RejectThreshold)
		assert.Equal(t, 20, cfg.FraudDetection.MLConfig.FeatureVectorSize)
		assert.True(t, cfg.FraudDetection.MLConfig.EnableFeatureCache)
		assert.Equal(t, "300s", cfg.FraudDetection.MLConfig.FeatureCacheTTL)
		assert.Equal(t, 100, cfg.FraudDetection.MLConfig.MaxConcurrentPredictions)
		assert.Equal(t, "1000ms", cfg.FraudDetection.MLConfig.PredictionTimeout)
		assert.False(t, cfg.FraudDetection.MLConfig.EnableBatchPrediction)
		assert.Equal(t, 10, cfg.FraudDetection.MLConfig.BatchSize)
		assert.False(t, cfg.FraudDetection.MLConfig.EnableDriftDetection)
		assert.Equal(t, 0.1, cfg.FraudDetection.MLConfig.DriftThreshold)
		assert.True(t, cfg.FraudDetection.MLConfig.EnableModelAccuracy)
		assert.Equal(t, "24h", cfg.FraudDetection.MLConfig.AccuracyWindow)
		assert.False(t, cfg.FraudDetection.MLConfig.EnableABTesting)
		assert.Equal(t, 0.1, cfg.FraudDetection.MLConfig.ABTestTrafficSplit)
		assert.Equal(t, 0.5, cfg.FraudDetection.MLConfig.ABTestControlWeight)
		assert.Equal(t, 0.5, cfg.FraudDetection.MLConfig.ABTestTreatmentWeight)
	})

	t.Run("Environment Variables Override", func(t *testing.T) {
		// Set environment variables
		os.Setenv("ML_ENABLED", "false")
		os.Setenv("ML_MODEL_TYPE", "mock")
		os.Setenv("ML_MODEL_VERSION", "2.0.0")
		os.Setenv("ML_FALLBACK_TO_RULES", "false")
		os.Setenv("ML_RULE_WEIGHT", "0.7")
		os.Setenv("ML_WEIGHT", "0.3")
		os.Setenv("ML_ENABLE_DYNAMIC_WEIGHTS", "true")
		os.Setenv("ML_APPROVE_THRESHOLD", "0.2")
		os.Setenv("ML_FLAG_THRESHOLD", "0.8")
		os.Setenv("ML_REJECT_THRESHOLD", "0.95")
		os.Setenv("ML_FEATURE_VECTOR_SIZE", "30")
		os.Setenv("ML_ENABLE_FEATURE_CACHE", "false")
		os.Setenv("ML_FEATURE_CACHE_TTL", "600s")
		os.Setenv("ML_MAX_CONCURRENT_PREDICTIONS", "200")
		os.Setenv("ML_PREDICTION_TIMEOUT", "2000ms")
		os.Setenv("ML_ENABLE_BATCH_PREDICTION", "true")
		os.Setenv("ML_BATCH_SIZE", "20")
		os.Setenv("ML_ENABLE_DRIFT_DETECTION", "true")
		os.Setenv("ML_DRIFT_THRESHOLD", "0.2")
		os.Setenv("ML_ENABLE_MODEL_ACCURACY", "false")
		os.Setenv("ML_ACCURACY_WINDOW", "48h")
		os.Setenv("ML_ENABLE_AB_TESTING", "true")
		os.Setenv("ML_AB_TEST_TRAFFIC_SPLIT", "0.2")
		os.Setenv("ML_AB_TEST_CONTROL_WEIGHT", "0.6")
		os.Setenv("ML_AB_TEST_TREATMENT_WEIGHT", "0.4")

		defer func() {
			// Clean up environment variables
			envVars := []string{
				"ML_ENABLED", "ML_MODEL_TYPE", "ML_MODEL_VERSION", "ML_FALLBACK_TO_RULES",
				"ML_RULE_WEIGHT", "ML_WEIGHT", "ML_ENABLE_DYNAMIC_WEIGHTS", "ML_APPROVE_THRESHOLD",
				"ML_FLAG_THRESHOLD", "ML_REJECT_THRESHOLD", "ML_FEATURE_VECTOR_SIZE",
				"ML_ENABLE_FEATURE_CACHE", "ML_FEATURE_CACHE_TTL", "ML_MAX_CONCURRENT_PREDICTIONS",
				"ML_PREDICTION_TIMEOUT", "ML_ENABLE_BATCH_PREDICTION", "ML_BATCH_SIZE",
				"ML_ENABLE_DRIFT_DETECTION", "ML_DRIFT_THRESHOLD", "ML_ENABLE_MODEL_ACCURACY",
				"ML_ACCURACY_WINDOW", "ML_ENABLE_AB_TESTING", "ML_AB_TEST_TRAFFIC_SPLIT",
				"ML_AB_TEST_CONTROL_WEIGHT", "ML_AB_TEST_TREATMENT_WEIGHT",
			}
			for _, env := range envVars {
				os.Unsetenv(env)
			}
		}()

		cfg := config.NewConfig()
		cfg.LoadFromEnv()

		// Test that environment variables override defaults
		assert.False(t, cfg.FraudDetection.EnableML)
		assert.Equal(t, "mock", cfg.FraudDetection.MLConfig.ModelType)
		assert.Equal(t, "2.0.0", cfg.FraudDetection.MLConfig.ModelVersion)
		assert.False(t, cfg.FraudDetection.MLConfig.FallbackToRules)
		assert.Equal(t, 0.7, cfg.FraudDetection.MLConfig.RuleWeight)
		assert.Equal(t, 0.3, cfg.FraudDetection.MLConfig.MLWeight)
		assert.True(t, cfg.FraudDetection.MLConfig.EnableDynamicWeights)
		assert.Equal(t, 0.2, cfg.FraudDetection.MLConfig.ApproveThreshold)
		assert.Equal(t, 0.8, cfg.FraudDetection.MLConfig.FlagThreshold)
		assert.Equal(t, 0.95, cfg.FraudDetection.MLConfig.RejectThreshold)
		assert.Equal(t, 30, cfg.FraudDetection.MLConfig.FeatureVectorSize)
		assert.False(t, cfg.FraudDetection.MLConfig.EnableFeatureCache)
		assert.Equal(t, "600s", cfg.FraudDetection.MLConfig.FeatureCacheTTL)
		assert.Equal(t, 200, cfg.FraudDetection.MLConfig.MaxConcurrentPredictions)
		assert.Equal(t, "2000ms", cfg.FraudDetection.MLConfig.PredictionTimeout)
		assert.True(t, cfg.FraudDetection.MLConfig.EnableBatchPrediction)
		assert.Equal(t, 20, cfg.FraudDetection.MLConfig.BatchSize)
		assert.True(t, cfg.FraudDetection.MLConfig.EnableDriftDetection)
		assert.Equal(t, 0.2, cfg.FraudDetection.MLConfig.DriftThreshold)
		assert.False(t, cfg.FraudDetection.MLConfig.EnableModelAccuracy)
		assert.Equal(t, "48h", cfg.FraudDetection.MLConfig.AccuracyWindow)
		assert.True(t, cfg.FraudDetection.MLConfig.EnableABTesting)
		assert.Equal(t, 0.2, cfg.FraudDetection.MLConfig.ABTestTrafficSplit)
		assert.Equal(t, 0.6, cfg.FraudDetection.MLConfig.ABTestControlWeight)
		assert.Equal(t, 0.4, cfg.FraudDetection.MLConfig.ABTestTreatmentWeight)
	})

	t.Run("YAML Configuration Loading", func(t *testing.T) {
		cfg, err := config.LoadFromFile("../configs/config.yaml")
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Test that YAML values are loaded correctly
		assert.True(t, cfg.FraudDetection.EnableML)
		assert.Equal(t, "mock", cfg.FraudDetection.MLConfig.ModelType)
		assert.Equal(t, "./models/fraud-detection-xgb-1.0.0.pkl", cfg.FraudDetection.MLConfig.ModelPath)
		assert.Equal(t, "1.0.0", cfg.FraudDetection.MLConfig.ModelVersion)
		assert.True(t, cfg.FraudDetection.MLConfig.FallbackToRules)
		assert.Equal(t, 0.6, cfg.FraudDetection.MLConfig.RuleWeight)
		assert.Equal(t, 0.4, cfg.FraudDetection.MLConfig.MLWeight)
		assert.False(t, cfg.FraudDetection.MLConfig.EnableDynamicWeights)
		assert.Equal(t, 0.3, cfg.FraudDetection.MLConfig.ApproveThreshold)
		assert.Equal(t, 0.7, cfg.FraudDetection.MLConfig.FlagThreshold)
		assert.Equal(t, 0.9, cfg.FraudDetection.MLConfig.RejectThreshold)
		assert.Equal(t, 20, cfg.FraudDetection.MLConfig.FeatureVectorSize)
		assert.True(t, cfg.FraudDetection.MLConfig.EnableFeatureCache)
		assert.Equal(t, "300s", cfg.FraudDetection.MLConfig.FeatureCacheTTL)
		assert.Equal(t, 100, cfg.FraudDetection.MLConfig.MaxConcurrentPredictions)
		assert.Equal(t, "5s", cfg.FraudDetection.MLConfig.PredictionTimeout)
		assert.False(t, cfg.FraudDetection.MLConfig.EnableBatchPrediction)
		assert.Equal(t, 10, cfg.FraudDetection.MLConfig.BatchSize)
		assert.False(t, cfg.FraudDetection.MLConfig.EnableDriftDetection)
		assert.Equal(t, 0.1, cfg.FraudDetection.MLConfig.DriftThreshold)
		assert.True(t, cfg.FraudDetection.MLConfig.EnableModelAccuracy)
		assert.Equal(t, "24h", cfg.FraudDetection.MLConfig.AccuracyWindow)
		assert.False(t, cfg.FraudDetection.MLConfig.EnableABTesting)
		assert.Equal(t, 0.1, cfg.FraudDetection.MLConfig.ABTestTrafficSplit)
		assert.Equal(t, 0.5, cfg.FraudDetection.MLConfig.ABTestControlWeight)
		assert.Equal(t, 0.5, cfg.FraudDetection.MLConfig.ABTestTreatmentWeight)
	})

	t.Run("Invalid Environment Variables", func(t *testing.T) {
		// Set invalid environment variables
		os.Setenv("ML_RULE_WEIGHT", "invalid")
		os.Setenv("ML_FEATURE_VECTOR_SIZE", "not_a_number")
		os.Setenv("ML_ENABLED", "maybe")

		defer func() {
			os.Unsetenv("ML_RULE_WEIGHT")
			os.Unsetenv("ML_FEATURE_VECTOR_SIZE")
			os.Unsetenv("ML_ENABLED")
		}()

		cfg := config.NewConfig()
		cfg.LoadFromEnv()

		// Test that invalid values don't crash and keep defaults
		assert.Equal(t, 0.6, cfg.FraudDetection.MLConfig.RuleWeight)       // Default value
		assert.Equal(t, 20, cfg.FraudDetection.MLConfig.FeatureVectorSize) // Default value
		assert.True(t, cfg.FraudDetection.EnableML)                        // Default value (invalid "maybe" should be ignored)
	})
}
