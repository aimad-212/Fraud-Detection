package ml

import (
	"fmt"
	"sync"
)

// LGBMModelScorer implements ModelScorer using LightGBM exported ONNX model
// This is a specialized wrapper around ONNXModelScorer for LightGBM models
type LGBMModelScorer struct {
	baseScorer *ONNXModelScorer
	modelPath  string
	isLoaded   bool
	mu         sync.RWMutex
}

// NewLGBMModelScorer creates a new LightGBM ONNX model scorer
func NewLGBMModelScorer(modelPath string) (*LGBMModelScorer, error) {
	// Create base ONNX scorer
	baseScorer, err := NewONNXModelScorer(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create base ONNX scorer: %w", err)
	}

	scorer := &LGBMModelScorer{
		baseScorer: baseScorer,
		modelPath:  modelPath,
		isLoaded:   true,
	}

	return scorer, nil
}

// Score runs inference on the feature vector using the LightGBM ONNX model
func (s *LGBMModelScorer) Score(features interface{}) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.isLoaded {
		return 0, fmt.Errorf("LGBM ONNX model is not loaded")
	}

	// Convert features to float32 array
	featureArray, ok := features.([]float32)
	if !ok {
		return 0, fmt.Errorf("expected []float32 features, got %T", features)
	}

	// Validate features
	if err := s.ValidateFeatures(featureArray); err != nil {
		return 0, fmt.Errorf("feature validation failed: %w", err)
	}

	// Use base ONNX scorer for inference
	score, err := s.baseScorer.Score(featureArray)
	if err != nil {
		return 0, fmt.Errorf("LightGBM inference failed: %w", err)
	}

	// Ensure score is in valid probability range [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score, nil
}

// Close cleans up the model resources
func (s *LGBMModelScorer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.isLoaded = false
	return s.baseScorer.Close()
}

// IsLoaded returns whether the model is loaded and ready
func (s *LGBMModelScorer) IsLoaded() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isLoaded && s.baseScorer.IsLoaded()
}

// GetModelInfo returns information about the loaded LightGBM model
func (s *LGBMModelScorer) GetModelInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info := map[string]interface{}{
		"model_type":    "lightgbm_onnx",
		"model_path":    s.modelPath,
		"is_loaded":     s.isLoaded,
		"algorithm":     "LightGBM",
		"export_format": "ONNX",
		"framework":     "scikit-learn + onnxmltools",
		"use_case":      "fraud_detection",
		"feature_count": 38,
		"output_range":  "0.0-1.0 (probability)",
		"training_data": "Kaggle Credit Card Fraud Detection",
		"dataset_size":  "284,807 transactions",
		"fraud_ratio":   "0.173% (492 fraud cases)",
	}

	return info
}

// ValidateFeatures validates that features are suitable for LightGBM model
func (s *LGBMModelScorer) ValidateFeatures(features []float32) error {
	if len(features) == 0 {
		return fmt.Errorf("empty feature vector")
	}

	// LightGBM models expect specific feature count
	expectedFeatures := 38
	if len(features) != expectedFeatures {
		return fmt.Errorf("expected %d features, got %d", expectedFeatures, len(features))
	}

	return nil
}
