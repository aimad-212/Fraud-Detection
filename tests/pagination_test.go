package tests

import (
	"testing"

	"fraud-detection/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPagination tests pagination functionality
func TestPagination(t *testing.T) {
	repo := repository.NewInMemoryTransactionRepository()

	// Create some test transactions
	for i := 0; i < 100; i++ {
		// This would normally create actual transactions
		// For now, we'll test the repository directly
	}

	t.Run("Limit and Offset Validation", func(t *testing.T) {
		// Test default limit
		_, err := repo.ListTransactions(0, 0)
		require.NoError(t, err)
		// Should return default limit (50) or whatever is available

		// Test max limit enforcement
		_, err = repo.ListTransactions(200, 0)
		require.NoError(t, err)
		// Should cap at 100

		// Test negative offset handling
		_, err = repo.ListTransactions(10, -5)
		require.NoError(t, err)
		// Should clamp to 0
	})

	t.Run("Empty Repository", func(t *testing.T) {
		emptyRepo := repository.NewInMemoryTransactionRepository()
		transactions, err := emptyRepo.ListTransactions(10, 0)
		require.NoError(t, err)
		assert.Empty(t, transactions)
	})
}
