package repository

import (
	"errors"
	"sync"

	"fraud-detection/internal/domain"

	"github.com/google/uuid"
)

// TransactionRepository defines the interface for transaction data operations
type TransactionRepository interface {
	Create(transaction *domain.Transaction) error
	GetByID(id uuid.UUID) (*domain.Transaction, error)
	GetByUserID(userID string) ([]*domain.Transaction, error)
	Update(transaction *domain.Transaction) error
	Delete(id uuid.UUID) error
	ListTransactions(limit, offset int) ([]*domain.Transaction, error)
	GetByStatus(status domain.TransactionStatus) ([]*domain.Transaction, error)
}

// InMemoryTransactionRepository implements TransactionRepository using in-memory storage
type InMemoryTransactionRepository struct {
	mu           sync.RWMutex
	transactions map[uuid.UUID]*domain.Transaction
	userIndex    map[string][]*domain.Transaction
	statusIndex  map[domain.TransactionStatus][]*domain.Transaction
}

// NewInMemoryTransactionRepository creates a new in-memory transaction repository
func NewInMemoryTransactionRepository() *InMemoryTransactionRepository {
	return &InMemoryTransactionRepository{
		transactions: make(map[uuid.UUID]*domain.Transaction),
		userIndex:    make(map[string][]*domain.Transaction),
		statusIndex:  make(map[domain.TransactionStatus][]*domain.Transaction),
	}
}

// Create adds a new transaction to the repository
func (r *InMemoryTransactionRepository) Create(transaction *domain.Transaction) error {
	if transaction == nil {
		return errors.New("transaction cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if transaction already exists
	if _, exists := r.transactions[transaction.ID]; exists {
		return errors.New("transaction already exists")
	}

	// Create a copy to avoid external modifications
	tx := *transaction
	r.transactions[tx.ID] = &tx

	// Update user index
	r.userIndex[tx.UserID.String()] = append(r.userIndex[tx.UserID.String()], &tx)

	// Update status index
	r.statusIndex[tx.Status] = append(r.statusIndex[tx.Status], &tx)

	return nil
}

// GetByID retrieves a transaction by its ID
func (r *InMemoryTransactionRepository) GetByID(id uuid.UUID) (*domain.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	transaction, exists := r.transactions[id]
	if !exists {
		return nil, errors.New("transaction not found")
	}

	// Return a copy to prevent external modifications
	tx := *transaction
	return &tx, nil
}

// GetByUserID retrieves all transactions for a specific user
func (r *InMemoryTransactionRepository) GetByUserID(userID string) ([]*domain.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	transactions, exists := r.userIndex[userID]
	if !exists {
		return []*domain.Transaction{}, nil
	}

	// Return copies to prevent external modifications
	result := make([]*domain.Transaction, len(transactions))
	for i, tx := range transactions {
		txCopy := *tx
		result[i] = &txCopy
	}

	return result, nil
}

// Update modifies an existing transaction
func (r *InMemoryTransactionRepository) Update(transaction *domain.Transaction) error {
	if transaction == nil {
		return errors.New("transaction cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.transactions[transaction.ID]
	if !exists {
		return errors.New("transaction not found")
	}

	// Remove from old status index
	r.removeFromStatusIndex(existing.Status, transaction.ID)

	// Update transaction
	*existing = *transaction

	// Add to new status index
	r.statusIndex[transaction.Status] = append(r.statusIndex[transaction.Status], existing)

	return nil
}

// Delete removes a transaction from the repository
func (r *InMemoryTransactionRepository) Delete(id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	transaction, exists := r.transactions[id]
	if !exists {
		return errors.New("transaction not found")
	}

	// Remove from user index
	if userTxs, exists := r.userIndex[transaction.UserID.String()]; exists {
		r.userIndex[transaction.UserID.String()] = r.removeTransactionFromSlice(userTxs, id)
	}

	// Remove from status index
	r.removeFromStatusIndex(transaction.Status, id)

	// Remove from main storage
	delete(r.transactions, id)

	return nil
}

// ListTransactions returns a paginated list of transactions with limit and offset
func (r *InMemoryTransactionRepository) ListTransactions(limit, offset int) ([]*domain.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	allTxs := make([]*domain.Transaction, 0, len(r.transactions))
	for _, tx := range r.transactions {
		txCopy := *tx
		allTxs = append(allTxs, &txCopy)
	}

	// Apply pagination with limits
	// Default limit: 50, max limit: 100 to prevent memory issues
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	// Calculate pagination range
	start := offset
	if start > len(allTxs) {
		start = len(allTxs)
	}

	end := start + limit
	if end > len(allTxs) {
		end = len(allTxs)
	}

	// Return paginated results
	return allTxs[start:end], nil
}

// List returns a paginated list of transactions (legacy method for backward compatibility)
func (r *InMemoryTransactionRepository) List(limit, offset int) ([]*domain.Transaction, error) {
	return r.ListTransactions(limit, offset)
}

// GetByStatus retrieves transactions by status
func (r *InMemoryTransactionRepository) GetByStatus(status domain.TransactionStatus) ([]*domain.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	transactions, exists := r.statusIndex[status]
	if !exists {
		return []*domain.Transaction{}, nil
	}

	// Return copies to prevent external modifications
	result := make([]*domain.Transaction, len(transactions))
	for i, tx := range transactions {
		txCopy := *tx
		result[i] = &txCopy
	}

	return result, nil
}

// Helper methods

func (r *InMemoryTransactionRepository) removeFromStatusIndex(status domain.TransactionStatus, txID uuid.UUID) {
	if txs, exists := r.statusIndex[status]; exists {
		r.statusIndex[status] = r.removeTransactionFromSlice(txs, txID)
	}
}

func (r *InMemoryTransactionRepository) removeTransactionFromSlice(txs []*domain.Transaction, txID uuid.UUID) []*domain.Transaction {
	for i, tx := range txs {
		if tx.ID == txID {
			return append(txs[:i], txs[i+1:]...)
		}
	}
	return txs
}
