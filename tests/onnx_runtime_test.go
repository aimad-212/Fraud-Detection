package tests

import (
	"testing"

	"fraud-detection/internal/infrastructure/ml"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestONNXRuntimeAvailability checks if ONNX runtime is available
func TestONNXRuntimeAvailability(t *testing.T) {
	t.Log("Checking ONNX Runtime availability...")

	// Try to create ONNX scorer - this will initialize runtime
	scorer, err := ml.NewONNXModelScorer("models/fraud_model.onnx")

	if err != nil {
		// ONNX runtime not available - this is OK in CI/CD
		t.Logf("ONNX Runtime not available: %v", err)
		t.Log("This is expected in environments without ONNX runtime library")
		return
	}

	// If we got here, ONNX runtime is available
	require.NotNil(t, scorer)
	t.Log("ONNX Runtime is available!")

	// Test IsLoaded
	t.Logf("Model loaded: %v", scorer.IsLoaded())

	// Clean up
	err = scorer.Close()
	require.NoError(t, err)
}

// TestONNXModelScorerFeatureConversion tests feature type conversion
func TestONNXModelScorerFeatureConversion(t *testing.T) {
	// Skip if ONNX runtime not available
	scorer, err := ml.NewONNXModelScorer("models/fraud_model.onnx")
	if err != nil {
		t.Skip("ONNX runtime not available, skipping feature conversion test")
	}
	defer scorer.Close()

	t.Run("Float32 Features", func(t *testing.T) {
		features := []float32{1.0, 2.0, 3.0, 4.0, 5.0}
		_, err := scorer.Score(features)
		if scorer.IsLoaded() {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
	})

	t.Run("Float64 Features", func(t *testing.T) {
		features := []float64{1.5, 2.5, 3.5, 4.5, 5.5}
		_, err := scorer.Score(features)
		if scorer.IsLoaded() {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
	})

	t.Run("Interface Features", func(t *testing.T) {
		features := []interface{}{1.0, 2.0, 3.0, 4.0, 5.0}
		_, err := scorer.Score(features)
		if scorer.IsLoaded() {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
	})

	t.Run("Invalid Feature Type", func(t *testing.T) {
		features := []string{"invalid", "data"}
		_, err := scorer.Score(features)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported")
	})
}

// TestONNXModelScorerEdgeCases tests edge cases
func TestONNXModelScorerEdgeCases(t *testing.T) {
	// Skip if ONNX runtime not available
	scorer, err := ml.NewONNXModelScorer("models/fraud_model.onnx")
	if err != nil {
		t.Skip("ONNX runtime not available, skipping edge case tests")
	}
	defer scorer.Close()

	t.Run("Empty Features", func(t *testing.T) {
		features := []float32{}
		_, err := scorer.Score(features)
		// Should handle empty features gracefully
		assert.Error(t, err)
	})

	t.Run("Score Before Load", func(t *testing.T) {
		// Create a new scorer and try to score before checking IsLoaded
		tempScorer, err := ml.NewONNXModelScorer("models/fraud_model.onnx")
		if err != nil {
			t.Skip("ONNX runtime not available")
		}

		// Don't wait for load, just try to score
		_, err = tempScorer.Score([]float32{1.0, 2.0, 3.0})

		if tempScorer.IsLoaded() {
			// If loaded, should not error
			assert.NoError(t, err)
		} else {
			// If not loaded, should error
			assert.Error(t, err)
		}

		tempScorer.Close()
	})

	t.Run("Multiple Score Calls", func(t *testing.T) {
		if !scorer.IsLoaded() {
			t.Skip("Model not loaded")
		}

		for i := 0; i < 5; i++ {
			features := []float32{float32(i), float32(i + 1), float32(i + 2)}
			score, err := scorer.Score(features)
			assert.NoError(t, err)
			assert.GreaterOrEqual(t, score, 0.0)
			assert.LessOrEqual(t, score, 1.0)
		}
	})

	t.Run("Close Multiple Times", func(t *testing.T) {
		// First close should succeed
		err := scorer.Close()
		assert.NoError(t, err)

		// Second close should also succeed (no-op)
		err = scorer.Close()
		assert.NoError(t, err)
	})
}

// TestONNXFactoryIntegration tests the factory pattern
func TestONNXFactoryIntegration(t *testing.T) {
	t.Run("Mock Model", func(t *testing.T) {
		scorer, err := ml.ModelScorerFactory("models/fraud_model.pkl", "mock")
		require.NoError(t, err)
		assert.NotNil(t, scorer)
		assert.True(t, scorer.IsLoaded())

		score, err := scorer.Score([]float32{1.0, 2.0, 3.0})
		require.NoError(t, err)
		assert.Equal(t, 0.1, score) // Mock default score

		scorer.Close()
	})

	t.Run("ONNX Model", func(t *testing.T) {
		scorer, err := ml.ModelScorerFactory("models/fraud_model.onnx", "onnx")

		if err != nil {
			// ONNX runtime not available
			t.Logf("ONNX factory test skipped: %v", err)
			return
		}

		assert.NotNil(t, scorer)
		t.Logf("Factory created ONNX scorer, loaded: %v", scorer.IsLoaded())

		// If loaded, test scoring
		if scorer.IsLoaded() {
			score, err := scorer.Score([]float32{1.0, 2.0, 3.0})
			assert.NoError(t, err)
			assert.GreaterOrEqual(t, score, 0.0)
			assert.LessOrEqual(t, score, 1.0)
		}

		scorer.Close()
	})

	t.Run("Invalid Model Type", func(t *testing.T) {
		scorer, err := ml.ModelScorerFactory("models/fraud_model.onnx", "invalid_type")
		assert.Error(t, err)
		assert.Nil(t, scorer)
	})
}
