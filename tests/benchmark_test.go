package tests

import (
	"testing"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/infrastructure/ml"
	"fraud-detection/internal/repository"
	"fraud-detection/internal/usecase"
	"github.com/google/uuid"
	"go.uber.org/zap/zaptest"
)

// BenchmarkMLScoring benchmarks ML model scoring performance
func BenchmarkMLScoring(b *testing.B) {
	scorer := ml.NewMockModelScorer(0.5)

	// Generate test features
	features := []float32{100.0, 0.5, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := scorer.Score(features)
		if err != nil {
			b.Fatalf("Failed to score features: %v", err)
		}
	}
}

// BenchmarkDatabaseOperations benchmarks database CRUD operations
func BenchmarkDatabaseOperations(b *testing.B) {
	repo := repository.NewInMemoryTransactionRepository()

	b.Run("Create", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				tx := domain.NewTransaction(uuid.New(), float64(i%5000+10), "USD")
				err := repo.Create(tx)
				if err != nil {
					b.Fatalf("Failed to create transaction: %v", err)
				}
				i++
			}
		})
	})

	b.Run("Read", func(b *testing.B) {
		// Pre-populate with transactions
		transactions := make([]*domain.Transaction, 1000)
		for i := 0; i < 1000; i++ {
			tx := domain.NewTransaction(uuid.New(), float64(i%5000+10), "USD")
			repo.Create(tx)
			transactions[i] = tx
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if i >= len(transactions) {
					i = 0
				}
				_, err := repo.GetByID(transactions[i].ID)
				if err != nil {
					b.Fatalf("Failed to get transaction: %v", err)
				}
				i++
			}
		})
	})

	b.Run("List", func(b *testing.B) {
		// Pre-populate with transactions
		for i := 0; i < 1000; i++ {
			tx := domain.NewTransaction(uuid.New(), float64(i%5000+10), "USD")
			repo.Create(tx)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := repo.List(50, 0)
			if err != nil {
				b.Fatalf("Failed to list transactions: %v", err)
			}
		}
	})
}

// BenchmarkConcurrentOperations benchmarks concurrent operations
func BenchmarkConcurrentOperations(b *testing.B) {
	repo := repository.NewInMemoryTransactionRepository()
	logger := zaptest.NewLogger(b)
	useCase := usecase.NewTransactionUseCase(repo, nil, logger)

	b.Run("ConcurrentCreates", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				userID := uuid.New()
				amount := float64(i%5000 + 10)
				_, err := useCase.CreateTransaction(userID, amount, "USD", "test-merchant")
				if err != nil {
					b.Fatalf("Failed to create transaction: %v", err)
				}
				i++
			}
		})
	})

	b.Run("MixedOperations", func(b *testing.B) {
		// Pre-create some transactions
		transactions := make([]*domain.Transaction, 100)
		for i := 0; i < 100; i++ {
			userID := uuid.New()
			tx, _ := useCase.CreateTransaction(userID, float64(i%5000+10), "USD", "test-merchant")
			transactions[i] = tx
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if i >= len(transactions) {
					i = 0
				}

				// Mix of read and write operations
				if i%3 == 0 {
					// Create new transaction
					userID := uuid.New()
					useCase.CreateTransaction(userID, float64(i%5000+10), "USD", "test-merchant")
				} else if i%3 == 1 {
					// Read existing transaction
					useCase.GetTransaction(transactions[i%len(transactions)].ID)
				} else {
					// Process transaction
					useCase.ProcessTransaction(transactions[i%len(transactions)])
				}
				i++
			}
		})
	})
}

// BenchmarkMemoryUsage benchmarks memory allocation patterns
func BenchmarkMemoryUsage(b *testing.B) {
	repo := repository.NewInMemoryTransactionRepository()
	logger := zaptest.NewLogger(b)
	useCase := usecase.NewTransactionUseCase(repo, nil, logger)

	b.Run("MemoryEfficient", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			userID := uuid.New()
			tx, err := useCase.CreateTransaction(userID, 100.0, "USD", "test-merchant")
			if err != nil {
				b.Fatalf("Failed to create transaction: %v", err)
			}
			_ = tx
		}
	})
}

// BenchmarkRuleEngine benchmarks rule engine performance
func BenchmarkRuleEngine(b *testing.B) {
	ruleEngine := usecase.NewRulesEngine(nil) // No rules for benchmark

	// Create sample transaction
	tx := domain.NewTransaction(uuid.New(), 150.50, "USD")
	tx.Merchant = "amazon.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Evaluate transaction to get violations, then calculate risk score
		violations := ruleEngine.EvaluateTransaction(tx, nil)
		riskScore := ruleEngine.CalculateRiskScore(violations)
		_ = riskScore
	}
}
