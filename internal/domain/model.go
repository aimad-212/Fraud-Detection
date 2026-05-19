package domain

import (
	"encoding/json"
	"time"
)

// ModelType represents the type of ML model
type ModelType string

const (
	ModelTypeONNX   ModelType = "onnx"
	ModelTypeMock   ModelType = "mock"
	ModelTypeCustom ModelType = "custom"
)

// ModelStatus represents the status of a model
type ModelStatus string

const (
	ModelStatusActive   ModelStatus = "active"
	ModelStatusInactive ModelStatus = "inactive"
	ModelStatusTesting  ModelStatus = "testing"
	ModelStatusFailed   ModelStatus = "failed"
)

// ModelVersion represents a versioned ML model
type ModelVersion struct {
	ID          string      `json:"id" gorm:"primaryKey;type:varchar(255)"`
	Name        string      `json:"name" gorm:"not null;index"`
	Version     string      `json:"version" gorm:"not null"`
	Type        ModelType   `json:"type" gorm:"not null"`
	Status      ModelStatus `json:"status" gorm:"not null;default:'inactive'"`
	FilePath    string      `json:"file_path" gorm:"not null"`
	Checksum    string      `json:"checksum" gorm:"not null"`
	Description string      `json:"description"`
	Metadata    string      `json:"metadata" gorm:"type:text"` // JSON string
	IsActive    bool        `json:"is_active" gorm:"default:false"`
	Performance string      `json:"performance" gorm:"type:text"` // JSON string with metrics
	CreatedAt   time.Time   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time   `json:"updated_at" gorm:"autoUpdateTime"`
	ActivatedAt *time.Time  `json:"activated_at"`
}

// ModelMetadata represents additional model metadata
type ModelMetadata struct {
	Features        []string               `json:"features"`
	InputShape      []int                  `json:"input_shape"`
	OutputShape     []int                  `json:"output_shape"`
	TrainingDate    *time.Time             `json:"training_date"`
	TrainingData    string                 `json:"training_data"`
	Accuracy        float64                `json:"accuracy"`
	Precision       float64                `json:"precision"`
	Recall          float64                `json:"recall"`
	F1Score         float64                `json:"f1_score"`
	Threshold       float64                `json:"threshold"`
	Hyperparameters map[string]interface{} `json:"hyperparameters"`
	Tags            []string               `json:"tags"`
}

// ModelPerformance represents model performance metrics
type ModelPerformance struct {
	TotalPredictions   int64     `json:"total_predictions"`
	CorrectPredictions int64     `json:"correct_predictions"`
	Accuracy           float64   `json:"accuracy"`
	AverageLatency     float64   `json:"average_latency_ms"`
	LastUpdated        time.Time `json:"last_updated"`
	TPR                float64   `json:"true_positive_rate"`  // True Positive Rate
	FPR                float64   `json:"false_positive_rate"` // False Positive Rate
	TNR                float64   `json:"true_negative_rate"`  // True Negative Rate
	FNR                float64   `json:"false_negative_rate"` // False Negative Rate
}

// NewModelVersion creates a new model version
func NewModelVersion(name, version, modelType, filePath, checksum, description string) *ModelVersion {
	return &ModelVersion{
		ID:          generateModelID(name, version),
		Name:        name,
		Version:     version,
		Type:        ModelType(modelType),
		Status:      ModelStatusInactive,
		FilePath:    filePath,
		Checksum:    checksum,
		Description: description,
		Metadata:    "{}",
		Performance: "{}",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// Activate activates the model version
func (m *ModelVersion) Activate() {
	now := time.Now()
	m.Status = ModelStatusActive
	m.IsActive = true
	m.ActivatedAt = &now
	m.UpdatedAt = now
}

// Deactivate deactivates the model version
func (m *ModelVersion) Deactivate() {
	m.Status = ModelStatusInactive
	m.IsActive = false
	m.UpdatedAt = time.Now()
}

// SetTesting sets the model status to testing
func (m *ModelVersion) SetTesting() {
	m.Status = ModelStatusTesting
	m.UpdatedAt = time.Now()
}

// SetFailed sets the model status to failed
func (m *ModelVersion) SetFailed() {
	m.Status = ModelStatusFailed
	m.IsActive = false
	m.UpdatedAt = time.Now()
}

// UpdatePerformance updates the performance metrics
func (m *ModelVersion) UpdatePerformance(performance *ModelPerformance) {
	m.Performance = performanceToJSON(performance)
	m.UpdatedAt = time.Now()
}

// GetMetadata returns the parsed metadata
func (m *ModelVersion) GetMetadata() (*ModelMetadata, error) {
	return jsonToMetadata(m.Metadata)
}

// SetMetadata sets the metadata
func (m *ModelVersion) SetMetadata(metadata *ModelMetadata) {
	m.Metadata = metadataToJSON(metadata)
	m.UpdatedAt = time.Now()
}

// GetPerformance returns the parsed performance metrics
func (m *ModelVersion) GetPerformance() (*ModelPerformance, error) {
	return jsonToPerformance(m.Performance)
}

// ModelUpdateEvent represents a model update event
type ModelUpdateEvent struct {
	ModelID     string    `json:"model_id"`
	ModelName   string    `json:"model_name"`
	Version     string    `json:"version"`
	Action      string    `json:"action"` // activate, deactivate, update, delete
	Timestamp   time.Time `json:"timestamp"`
	TriggeredBy string    `json:"triggered_by"` // user, system, scheduler
	OldVersion  *string   `json:"old_version,omitempty"`
	NewVersion  *string   `json:"new_version,omitempty"`
	Reason      string    `json:"reason"`
	Metadata    string    `json:"metadata"`
}

// NewModelUpdateEvent creates a new model update event
func NewModelUpdateEvent(modelID, modelName, version, action, triggeredBy, reason string) *ModelUpdateEvent {
	return &ModelUpdateEvent{
		ModelID:     modelID,
		ModelName:   modelName,
		Version:     version,
		Action:      action,
		Timestamp:   time.Now(),
		TriggeredBy: triggeredBy,
		Reason:      reason,
		Metadata:    "{}",
	}
}

// Helper functions for JSON serialization
func generateModelID(name, version string) string {
	return name + "-" + version
}

func metadataToJSON(metadata *ModelMetadata) string {
	data, _ := json.Marshal(metadata)
	return string(data)
}

func jsonToMetadata(jsonStr string) (*ModelMetadata, error) {
	var metadata ModelMetadata
	err := json.Unmarshal([]byte(jsonStr), &metadata)
	return &metadata, err
}

func performanceToJSON(performance *ModelPerformance) string {
	data, _ := json.Marshal(performance)
	return string(data)
}

func jsonToPerformance(jsonStr string) (*ModelPerformance, error) {
	var performance ModelPerformance
	err := json.Unmarshal([]byte(jsonStr), &performance)
	return &performance, err
}
