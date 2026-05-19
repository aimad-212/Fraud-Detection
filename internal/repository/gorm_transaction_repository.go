package repository

import (
	"errors"
	"time"

	"fraud-detection/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MetricsRecorder defines the interface for recording metrics
type MetricsRecorder interface {
	RecordDatabaseQuery(duration time.Duration, err error)
}

// GORMTransactionRepository implements TransactionRepository using GORM
type GORMTransactionRepository struct {
	db      *gorm.DB
	metrics MetricsRecorder
}

// NewGORMTransactionRepository creates a new GORM transaction repository
func NewGORMTransactionRepository(db *gorm.DB) *GORMTransactionRepository {
	return &GORMTransactionRepository{
		db: db,
	}
}

// SetMetrics sets the metrics recorder
func (r *GORMTransactionRepository) SetMetrics(metrics MetricsRecorder) {
	r.metrics = metrics
}

// GetDB returns the underlying GORM database instance
func (r *GORMTransactionRepository) GetDB() *gorm.DB {
	return r.db
}

// Create adds a new transaction to the database
func (r *GORMTransactionRepository) Create(transaction *domain.Transaction) error {
	if transaction == nil {
		return errors.New("transaction cannot be nil")
	}

	start := time.Now()

	// Check if transaction already exists
	var existing domain.Transaction
	if err := r.db.Where("id = ?", transaction.ID).First(&existing).Error; err == nil {
		if r.metrics != nil {
			r.metrics.RecordDatabaseQuery(time.Since(start), errors.New("transaction already exists"))
		}
		return errors.New("transaction already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		if r.metrics != nil {
			r.metrics.RecordDatabaseQuery(time.Since(start), err)
		}
		return err
	}

	// Create the transaction
	err := r.db.Create(transaction).Error
	if r.metrics != nil {
		r.metrics.RecordDatabaseQuery(time.Since(start), err)
	}

	return err
}

// GetByID retrieves a transaction by its ID
func (r *GORMTransactionRepository) GetByID(id uuid.UUID) (*domain.Transaction, error) {
	var transaction domain.Transaction
	err := r.db.Where("id = ?", id).First(&transaction).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("transaction not found")
	}
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

// GetByUserID retrieves all transactions for a specific user
func (r *GORMTransactionRepository) GetByUserID(userID string) ([]*domain.Transaction, error) {
	var transactions []*domain.Transaction
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&transactions).Error
	return transactions, err
}

// Update modifies an existing transaction
func (r *GORMTransactionRepository) Update(transaction *domain.Transaction) error {
	if transaction == nil {
		return errors.New("transaction cannot be nil")
	}

	result := r.db.Save(transaction)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("transaction not found")
	}
	return nil
}

// Delete removes a transaction from the database
func (r *GORMTransactionRepository) Delete(id uuid.UUID) error {
	result := r.db.Delete(&domain.Transaction{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("transaction not found")
	}
	return nil
}

// ListTransactions returns a paginated list of transactions with limit and offset
func (r *GORMTransactionRepository) ListTransactions(limit, offset int) ([]*domain.Transaction, error) {
	start := time.Now()
	var transactions []*domain.Transaction
	err := r.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&transactions).Error
	if r.metrics != nil {
		r.metrics.RecordDatabaseQuery(time.Since(start), err)
	}
	return transactions, err
}

// GetByStatus retrieves transactions by status
func (r *GORMTransactionRepository) GetByStatus(status domain.TransactionStatus) ([]*domain.Transaction, error) {
	var transactions []*domain.Transaction
	err := r.db.Where("status = ?", status).Order("created_at DESC").Find(&transactions).Error
	return transactions, err
}

// AutoMigrate runs database migrations
func (r *GORMTransactionRepository) AutoMigrate() error {
	return r.db.AutoMigrate(&domain.Transaction{})
}

// GetTransactionStats returns basic statistics
func (r *GORMTransactionRepository) GetTransactionStats() (map[string]interface{}, error) {
	var stats struct {
		TotalTransactions int64   `json:"total_transactions"`
		TotalAmount       float64 `json:"total_amount"`
		AvgAmount         float64 `json:"avg_amount"`
	}

	// Get total count and sum
	err := r.db.Model(&domain.Transaction{}).Count(&stats.TotalTransactions).Error
	if err != nil {
		return nil, err
	}

	// Get sum and average
	err = r.db.Model(&domain.Transaction{}).
		Select("SUM(amount) as total_amount, AVG(amount) as avg_amount").
		Scan(&stats).Error
	if err != nil {
		return nil, err
	}

	// Get status distribution
	var statusCounts []struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	err = r.db.Model(&domain.Transaction{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statusCounts).Error
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"total_transactions":  stats.TotalTransactions,
		"total_amount":        stats.TotalAmount,
		"avg_amount":          stats.AvgAmount,
		"status_distribution": statusCounts,
	}

	return result, nil
}

// GetUserTransactionStats returns statistics for a specific user
func (r *GORMTransactionRepository) GetUserTransactionStats(userID string) (map[string]interface{}, error) {
	var stats struct {
		TotalTransactions int64   `json:"total_transactions"`
		TotalAmount       float64 `json:"total_amount"`
		AvgAmount         float64 `json:"avg_amount"`
		LastTransaction   string  `json:"last_transaction"`
	}

	// Get user statistics
	err := r.db.Model(&domain.Transaction{}).
		Where("user_id = ?", userID).
		Select("COUNT(*) as total_transactions, SUM(amount) as total_amount, AVG(amount) as avg_amount, MAX(created_at) as last_transaction").
		Scan(&stats).Error
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"user_id":            userID,
		"total_transactions": stats.TotalTransactions,
		"total_amount":       stats.TotalAmount,
		"avg_amount":         stats.AvgAmount,
		"last_transaction":   stats.LastTransaction,
	}

	return result, nil
}
