package ml

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// XGBoostModelScorer implements ModelScorer using XGBoost pickle model
type XGBoostModelScorer struct {
	modelPath string
	isLoaded  bool
	mu        sync.RWMutex
}

// NewXGBoostModelScorer creates a new XGBoost pickle model scorer
func NewXGBoostModelScorer(modelPath string) (*XGBoostModelScorer, error) {
	scorer := &XGBoostModelScorer{
		modelPath: modelPath,
		isLoaded:  false,
	}

	// Try to load the model
	if err := scorer.loadModel(); err != nil {
		return nil, fmt.Errorf("failed to load XGBoost model: %w", err)
	}

	return scorer, nil
}

// loadModel loads the XGBoost model using Python
func (s *XGBoostModelScorer) loadModel() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if model file exists
	if _, err := exec.LookPath("python3"); err != nil {
		return fmt.Errorf("python3 not found: %w", err)
	}

	// Test model loading with Python
	cmd := exec.Command("python3", "-c", fmt.Sprintf(`
import pickle
import sys
import warnings
import io

# Suppress warnings
warnings.filterwarnings("ignore")

try:
    with open('%s', 'rb') as f:
        model = pickle.load(f)
    print("MODEL_LOADED")
except Exception as e:
    print(f"ERROR: {e}", file=sys.stderr)
    sys.exit(1)
`, s.modelPath))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to load model: %s, output: %s", err, string(output))
	}

	// Check if output contains "MODEL_LOADED" (ignoring warnings)
	if !strings.Contains(string(output), "MODEL_LOADED") {
		return fmt.Errorf("model loading failed: %s", string(output))
	}

	s.isLoaded = true
	return nil
}

// Score predicts fraud probability using XGBoost model
func (s *XGBoostModelScorer) Score(features interface{}) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.isLoaded {
		return 0.0, fmt.Errorf("model not loaded")
	}

	// Convert features to map[string]interface{}
	featureMap, ok := features.(map[string]interface{})
	if !ok {
		return 0.0, fmt.Errorf("expected map[string]interface{} features, got %T", features)
	}

	// Convert features to JSON
	featuresJSON, err := json.Marshal(featureMap)
	if err != nil {
		return 0.0, fmt.Errorf("failed to marshal features: %w", err)
	}

	// Use Python to predict
	cmd := exec.Command("python3", "-c", fmt.Sprintf(`
import pickle
import json
import numpy as np
import sys
import warnings

# Suppress warnings
warnings.filterwarnings("ignore")

try:
    # Load model
    with open('%s', 'rb') as f:
        model = pickle.load(f)
    
    # Parse features
    features = json.loads('''%s''')
    
    # Convert to numpy array in the right order
    # Expected feature order: Time, V1-V28, Amount, Amount_log, Hour, Day, Amount_bin, V_mean, V_std, V_max, V_min
    feature_order = ['Time', 'V1', 'V2', 'V3', 'V4', 'V5', 'V6', 'V7', 'V8', 'V9', 'V10', 
                    'V11', 'V12', 'V13', 'V14', 'V15', 'V16', 'V17', 'V18', 'V19', 'V20',
                    'V21', 'V22', 'V23', 'V24', 'V25', 'V26', 'V27', 'V28', 'Amount',
                    'Amount_log', 'Hour', 'Day', 'Amount_bin', 'V_mean', 'V_std', 'V_max', 'V_min']
    
    # Create feature array
    feature_array = []
    for feature_name in feature_order:
        if feature_name in features:
            feature_array.append(float(features[feature_name]))
        else:
            feature_array.append(0.0)  # Default value for missing features
    
    # Reshape for prediction
    X = np.array(feature_array).reshape(1, -1)
    
    # Predict probability
    proba = model.predict_proba(X)[0]
    fraud_prob = float(proba[1])  # Probability of class 1 (fraud)
    
    print(fraud_prob)
    
except Exception as e:
    print(f"ERROR: {e}", file=sys.stderr)
    sys.exit(1)
`, s.modelPath, string(featuresJSON)))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0.0, fmt.Errorf("prediction failed: %s, output: %s", err, string(output))
	}

	// Parse probability
	probStr := string(output)
	prob, err := strconv.ParseFloat(probStr, 64)
	if err != nil {
		return 0.0, fmt.Errorf("failed to parse probability: %s, output: %s", err, probStr)
	}

	return prob, nil
}

// IsLoaded returns whether the model is loaded
func (s *XGBoostModelScorer) IsLoaded() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isLoaded
}

// Reload reloads the model
func (s *XGBoostModelScorer) Reload() error {
	return s.loadModel()
}

// Close cleans up resources
func (s *XGBoostModelScorer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isLoaded = false
	return nil
}
