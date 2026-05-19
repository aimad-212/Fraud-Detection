package repository

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"fraud-detection/internal/domain"

	"gorm.io/gorm"
)

// ModelRepository defines the interface for model data operations
type ModelRepository interface {
	Create(model *domain.ModelVersion) error
	GetByID(id string) (*domain.ModelVersion, error)
	GetByName(name string) ([]*domain.ModelVersion, error)
	GetActiveModel(name string) (*domain.ModelVersion, error)
	Update(model *domain.ModelVersion) error
	Delete(id string) error
	List(limit, offset int) ([]*domain.ModelVersion, error)
	ActivateModel(id string) error
	DeactivateModel(id string) error
	GetModelHistory(name string) ([]*domain.ModelVersion, error)
}

// ModelRepositoryImpl implements ModelRepository using GORM
type ModelRepositoryImpl struct {
	db *gorm.DB
}

// NewModelRepository creates a new model repository
func NewModelRepository(db *gorm.DB) ModelRepository {
	return &ModelRepositoryImpl{db: db}
}

// Create creates a new model version
func (r *ModelRepositoryImpl) Create(model *domain.ModelVersion) error {
	if err := r.db.Create(model).Error; err != nil {
		return fmt.Errorf("failed to create model version: %w", err)
	}
	return nil
}

// GetByID retrieves a model version by ID
func (r *ModelRepositoryImpl) GetByID(id string) (*domain.ModelVersion, error) {
	var model domain.ModelVersion
	if err := r.db.Where("id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("model version with ID %s not found", id)
		}
		return nil, fmt.Errorf("failed to get model version: %w", err)
	}
	return &model, nil
}

// GetByName retrieves all model versions for a given model name
func (r *ModelRepositoryImpl) GetByName(name string) ([]*domain.ModelVersion, error) {
	var models []*domain.ModelVersion
	if err := r.db.Where("name = ?", name).Order("created_at DESC, version DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("failed to get model versions by name: %w", err)
	}
	return models, nil
}

// GetActiveModel retrieves the active model version for a given model name
func (r *ModelRepositoryImpl) GetActiveModel(name string) (*domain.ModelVersion, error) {
	var model domain.ModelVersion
	if err := r.db.Where("name = ? AND is_active = ?", name, true).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("no active model found for %s", name)
		}
		return nil, fmt.Errorf("failed to get active model: %w", err)
	}
	return &model, nil
}

// Update updates a model version
func (r *ModelRepositoryImpl) Update(model *domain.ModelVersion) error {
	if err := r.db.Save(model).Error; err != nil {
		return fmt.Errorf("failed to update model version: %w", err)
	}
	return nil
}

// Delete deletes a model version
func (r *ModelRepositoryImpl) Delete(id string) error {
	if err := r.db.Delete(&domain.ModelVersion{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete model version: %w", err)
	}
	return nil
}

// List returns a paginated list of model versions
func (r *ModelRepositoryImpl) List(limit, offset int) ([]*domain.ModelVersion, error) {
	var models []*domain.ModelVersion
	if err := r.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("failed to list model versions: %w", err)
	}
	return models, nil
}

// ActivateModel activates a model version and deactivates all other versions of the same model
func (r *ModelRepositoryImpl) ActivateModel(id string) error {
	// Start transaction
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get the model to activate
	var model domain.ModelVersion
	if err := tx.Where("id = ?", id).First(&model).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("model version with ID %s not found", id)
		}
		return fmt.Errorf("failed to get model version: %w", err)
	}

	// Deactivate all other versions of the same model
	if err := tx.Model(&domain.ModelVersion{}).
		Where("name = ? AND id != ?", model.Name, id).
		Updates(map[string]interface{}{
			"is_active":  false,
			"status":     domain.ModelStatusInactive,
			"updated_at": time.Now(),
		}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to deactivate other model versions: %w", err)
	}

	// Activate the selected model
	now := time.Now()
	if err := tx.Model(&model).Updates(map[string]interface{}{
		"is_active":    true,
		"status":       domain.ModelStatusActive,
		"activated_at": &now,
		"updated_at":   now,
	}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to activate model version: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit model activation transaction: %w", err)
	}

	return nil
}

// DeactivateModel deactivates a model version
func (r *ModelRepositoryImpl) DeactivateModel(id string) error {
	if err := r.db.Model(&domain.ModelVersion{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_active":  false,
			"status":     domain.ModelStatusInactive,
			"updated_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("failed to deactivate model version: %w", err)
	}
	return nil
}

// GetModelHistory retrieves the version history for a model
func (r *ModelRepositoryImpl) GetModelHistory(name string) ([]*domain.ModelVersion, error) {
	var models []*domain.ModelVersion
	if err := r.db.Where("name = ?", name).
		Order("created_at DESC, version DESC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("failed to get model history: %w", err)
	}
	return models, nil
}

// InMemoryModelRepository implements ModelRepository using in-memory storage
type InMemoryModelRepository struct {
	models map[string]*domain.ModelVersion
}

// NewInMemoryModelRepository creates a new in-memory model repository
func NewInMemoryModelRepository() ModelRepository {
	return &InMemoryModelRepository{
		models: make(map[string]*domain.ModelVersion),
	}
}

// Create creates a new model version
func (r *InMemoryModelRepository) Create(model *domain.ModelVersion) error {
	r.models[model.ID] = model
	return nil
}

// GetByID retrieves a model version by ID
func (r *InMemoryModelRepository) GetByID(id string) (*domain.ModelVersion, error) {
	model, exists := r.models[id]
	if !exists {
		return nil, fmt.Errorf("model version with ID %s not found", id)
	}
	return model, nil
}

// GetByName retrieves all model versions for a given model name
func (r *InMemoryModelRepository) GetByName(name string) ([]*domain.ModelVersion, error) {
	var models []*domain.ModelVersion
	for _, model := range r.models {
		if model.Name == name {
			models = append(models, model)
		}
	}
	return models, nil
}

// GetActiveModel retrieves the active model version for a given model name
func (r *InMemoryModelRepository) GetActiveModel(name string) (*domain.ModelVersion, error) {
	for _, model := range r.models {
		if model.Name == name && model.IsActive {
			return model, nil
		}
	}
	return nil, fmt.Errorf("no active model found for %s", name)
}

// Update updates a model version
func (r *InMemoryModelRepository) Update(model *domain.ModelVersion) error {
	r.models[model.ID] = model
	return nil
}

// Delete deletes a model version
func (r *InMemoryModelRepository) Delete(id string) error {
	delete(r.models, id)
	return nil
}

// List returns a paginated list of model versions
func (r *InMemoryModelRepository) List(limit, offset int) ([]*domain.ModelVersion, error) {
	var models []*domain.ModelVersion
	count := 0
	for _, model := range r.models {
		if count >= offset && len(models) < limit {
			models = append(models, model)
		}
		count++
	}
	return models, nil
}

// ActivateModel activates a model version and deactivates all other versions of the same model
func (r *InMemoryModelRepository) ActivateModel(id string) error {
	model, exists := r.models[id]
	if !exists {
		return fmt.Errorf("model version with ID %s not found", id)
	}

	// Deactivate all other versions of the same model
	for _, m := range r.models {
		if m.Name == model.Name && m.ID != id {
			m.IsActive = false
			m.Status = domain.ModelStatusInactive
		}
	}

	// Activate the selected model
	now := time.Now()
	model.IsActive = true
	model.Status = domain.ModelStatusActive
	model.ActivatedAt = &now
	model.UpdatedAt = now

	return nil
}

// DeactivateModel deactivates a model version
func (r *InMemoryModelRepository) DeactivateModel(id string) error {
	model, exists := r.models[id]
	if !exists {
		return fmt.Errorf("model version with ID %s not found", id)
	}

	model.IsActive = false
	model.Status = domain.ModelStatusInactive
	model.UpdatedAt = time.Now()

	return nil
}

// GetModelHistory retrieves the version history for a model
func (r *InMemoryModelRepository) GetModelHistory(name string) ([]*domain.ModelVersion, error) {
	var models []*domain.ModelVersion
	for _, model := range r.models {
		if model.Name == name {
			models = append(models, model)
		}
	}

	// Sort by created_at DESC, then version DESC (to match GORM behavior)
	sort.Slice(models, func(i, j int) bool {
		if models[i].CreatedAt.Equal(models[j].CreatedAt) {
			return models[i].Version > models[j].Version
		}
		return models[i].CreatedAt.After(models[j].CreatedAt)
	})

	return models, nil
}
