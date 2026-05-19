package ml

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yalue/onnxruntime_go"
)

// ModelScorer defines the interface for ML model scoring
type ModelScorer interface {
	Score(features interface{}) (float64, error)
	Close() error
	IsLoaded() bool
}

// MockModelScorer implements ModelScorer for testing and development
type MockModelScorer struct {
	mockScore float64
	isLoaded  bool
}

// NewMockModelScorer creates a mock model scorer for testing
func NewMockModelScorer(score float64) *MockModelScorer {
	return &MockModelScorer{
		mockScore: score,
		isLoaded:  true,
	}
}

// Score returns the mock score
func (m *MockModelScorer) Score(features interface{}) (float64, error) {
	if !m.isLoaded {
		return 0, fmt.Errorf("mock model is not loaded")
	}

	return m.mockScore, nil
}

// Close is a no-op for mock scorer
func (m *MockModelScorer) Close() error {
	m.isLoaded = false
	return nil
}

// IsLoaded returns the loaded status
func (m *MockModelScorer) IsLoaded() bool {
	return m.isLoaded
}

// ONNXModelScorer implements ModelScorer using ONNX runtime
type ONNXModelScorer struct {
	modelPath string
	session   *onnxruntime_go.DynamicSession[float32, float32]
	isLoaded  bool
	mu        sync.RWMutex
}

// NewONNXModelScorer creates a new ONNX model scorer
func NewONNXModelScorer(modelPath string) (*ONNXModelScorer, error) {
	// Initialize ONNX runtime environment
	if !onnxruntime_go.IsInitialized() {
		err := onnxruntime_go.InitializeEnvironment()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize ONNX runtime: %w", err)
		}
	}

	scorer := &ONNXModelScorer{
		modelPath: modelPath,
		isLoaded:  false,
	}

	// Load the ONNX model
	if err := scorer.loadModel(); err != nil {
		return nil, fmt.Errorf("failed to load ONNX model: %w", err)
	}

	return scorer, nil
}

// loadModel loads the ONNX model from file
func (s *ONNXModelScorer) loadModel() error {
	// Get input and output names from model
	inputInfo, outputInfo, err := onnxruntime_go.GetInputOutputInfo(s.modelPath)
	if err != nil {
		return fmt.Errorf("failed to get model IO info: %w", err)
	}

	inputNames := make([]string, len(inputInfo))
	for i, info := range inputInfo {
		inputNames[i] = info.Name
	}

	outputNames := make([]string, len(outputInfo))
	for i, info := range outputInfo {
		outputNames[i] = info.Name
	}

	// Create ONNX session
	// Using DynamicSession to handle different input shapes
	session, err := onnxruntime_go.NewDynamicSession[float32, float32](
		s.modelPath, inputNames, outputNames)
	if err != nil {
		return fmt.Errorf("failed to create ONNX session: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.session = session
	s.isLoaded = true
	return nil
}

// Score runs inference on the feature vector
func (s *ONNXModelScorer) Score(features interface{}) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.isLoaded || s.session == nil {
		return 0, fmt.Errorf("ONNX model is not loaded")
	}

	// Convert features to float32 slice
	var featureSlice []float32
	switch v := features.(type) {
	case []float32:
		featureSlice = v
	case []float64:
		// Convert []float64 to []float32
		featureSlice = make([]float32, len(v))
		for i, val := range v {
			featureSlice[i] = float32(val)
		}
	case []interface{}:
		// Convert []interface{} to []float32
		featureSlice = make([]float32, len(v))
		for i, val := range v {
			if f, ok := val.(float32); ok {
				featureSlice[i] = f
			} else if f, ok := val.(float64); ok {
				featureSlice[i] = float32(f)
			} else if f, ok := val.(int); ok {
				featureSlice[i] = float32(f)
			} else {
				return 0, fmt.Errorf("unsupported feature type: %T", val)
			}
		}
	default:
		return 0, fmt.Errorf("unsupported features type: %T", features)
	}

	// Create input tensor shape (batch_size=1, feature_count=len)
	shape := onnxruntime_go.NewShape(1, int64(len(featureSlice)))

	// Create input tensor
	inputTensor, err := onnxruntime_go.NewTensor[float32](shape, featureSlice)
	if err != nil {
		return 0, fmt.Errorf("failed to create input tensor: %w", err)
	}

	// Prepare output tensor (assuming single output)
	// The output shape will be determined by the model
	outputTensor, err := onnxruntime_go.NewEmptyTensor[float32](shape)
	if err != nil {
		return 0, fmt.Errorf("failed to create output tensor: %w", err)
	}

	// Run inference
	err = s.session.Run([]*onnxruntime_go.Tensor[float32]{inputTensor},
		[]*onnxruntime_go.Tensor[float32]{outputTensor})
	if err != nil {
		return 0, fmt.Errorf("ONNX inference failed: %w", err)
	}

	// Extract score from output
	// Assuming the model returns fraud probability in output[0][0]
	outputData := outputTensor.GetData()
	if len(outputData) == 0 {
		return 0, fmt.Errorf("no output data from ONNX model")
	}
	score := outputData[0]

	return float64(score), nil
}

// Close cleans up the ONNX session
func (s *ONNXModelScorer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session != nil {
		s.session.Destroy()
		s.session = nil
	}

	s.isLoaded = false
	return nil
}

// IsLoaded returns whether the model is loaded and ready
func (s *ONNXModelScorer) IsLoaded() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isLoaded && s.session != nil
}

// ModelScorerFactory creates model scorers based on model type
func ModelScorerFactory(modelPath, modelType string) (ModelScorer, error) {
	switch strings.ToLower(modelType) {
	case "mock":
		return NewMockModelScorer(0.1), nil
	case "onnx":
		return NewONNXModelScorer(modelPath)
	case "lgbm":
		return NewLGBMModelScorer(modelPath)
	case "xgboost", "xgb":
		return NewXGBoostModelScorer(modelPath)
	default:
		// Auto-detect based on file extension
		ext := strings.ToLower(filepath.Ext(modelPath))
		switch ext {
		case ".onnx":
			return NewONNXModelScorer(modelPath)
		case ".pkl":
			return NewXGBoostModelScorer(modelPath)
		default:
			return NewMockModelScorer(0.1), nil
		}
	}
}
