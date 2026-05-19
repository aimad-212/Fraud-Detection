package ml

import (
	"math"
	"testing"
	"time"

	"fraud-detection/internal/domain"
	"github.com/google/uuid"
)

func TestOptimizedFeatureEngineer_Creation(t *testing.T) {
	ofe := NewOptimizedFeatureEngineer()

	if ofe == nil {
		t.Fatal("Expected non-nil OptimizedFeatureEngineer")
	}

	if ofe.userCache == nil {
		t.Error("Expected user cache to be initialized")
	}

	if ofe.featureWeights == nil {
		t.Error("Expected feature weights to be initialized")
	}
}

func TestOptimizedFeatureEngineer_UserFeaturesCaching(t *testing.T) {
	ofe := NewOptimizedFeatureEngineer()
	userID := uuid.New().String()

	// Create test user history
	userHistory := []*domain.Transaction{
		{
			ID:        uuid.New(),
			UserID:    uuid.MustParse(userID),
			Amount:    100.0,
			CreatedAt: time.Now().Add(-2 * time.Hour),
			Country:   "US",
			Merchant:  "Test Store",
		},
		{
			ID:        uuid.New(),
			UserID:    uuid.MustParse(userID),
			Amount:    200.0,
			CreatedAt: time.Now().Add(-1 * time.Hour),
			Country:   "US",
			Merchant:  "Amazon",
		},
	}

	// First call should compute features
	features1 := ofe.getOrUpdateUserFeatures(userID, userHistory)
	if features1.TransactionCount != 2 {
		t.Errorf("Expected 2 transactions, got %d", features1.TransactionCount)
	}

	// Second call should use cache
	features2 := ofe.getOrUpdateUserFeatures(userID, userHistory)
	if features1 != features2 {
		t.Error("Expected cached features to be returned")
	}
}

func TestOptimizedFeatureEngineer_ComputeUserFeatures(t *testing.T) {
	ofe := NewOptimizedFeatureEngineer()
	userID := uuid.New().String()

	// Create test user history with varying amounts
	userHistory := []*domain.Transaction{
		{Amount: 100.0, CreatedAt: time.Now().Add(-3 * time.Hour)},
		{Amount: 200.0, CreatedAt: time.Now().Add(-2 * time.Hour)},
		{Amount: 300.0, CreatedAt: time.Now().Add(-1 * time.Hour)},
	}

	for _, tx := range userHistory {
		tx.ID = uuid.New()
		tx.UserID = uuid.MustParse(userID)
	}

	features := ofe.computeUserFeatures(userID, userHistory)

	// Test basic statistics
	if features.TransactionCount != 3 {
		t.Errorf("Expected 3 transactions, got %d", features.TransactionCount)
	}

	expectedAvg := (100.0 + 200.0 + 300.0) / 3.0
	if features.AverageAmount != expectedAvg {
		t.Errorf("Expected average amount %.2f, got %.2f", expectedAvg, features.AverageAmount)
	}

	if features.MedianAmount != 200.0 {
		t.Errorf("Expected median amount 200.0, got %.2f", features.MedianAmount)
	}
}

func TestOptimizedFeatureEngineer_EngineerFeaturesOptimized(t *testing.T) {
	ofe := NewOptimizedFeatureEngineer()

	// Create test transaction
	transaction := &domain.Transaction{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Amount:    150.0,
		Currency:  "USD",
		Merchant:  "Test Store",
		Country:   "US",
		CreatedAt: time.Now(),
	}

	// Create user history
	userHistory := []*domain.Transaction{
		{
			ID:        uuid.New(),
			UserID:    transaction.UserID,
			Amount:    100.0,
			CreatedAt: time.Now().Add(-1 * time.Hour),
		},
	}

	features := ofe.EngineerFeaturesOptimized(transaction, userHistory)

	if features == nil {
		t.Fatal("Expected non-nil features")
	}

	// Test that features are optimized
	if features.Amount == 0 {
		t.Error("Expected non-zero amount feature")
	}
}

func TestOptimizedFeatureEngineer_GlobalStats(t *testing.T) {
	ofe := NewOptimizedFeatureEngineer()

	// Create test transactions
	transactions := []*domain.Transaction{
		{Amount: 100.0, RiskScore: 0.1},
		{Amount: 200.0, RiskScore: 0.8},
		{Amount: 300.0, RiskScore: 0.2},
	}

	for i, tx := range transactions {
		tx.ID = uuid.New()
		tx.UserID = uuid.New()
		tx.CreatedAt = time.Now().Add(time.Duration(i) * time.Hour)
	}

	ofe.UpdateGlobalStats(transactions)

	stats := ofe.globalStats
	if stats.TotalTransactions != 3 {
		t.Errorf("Expected 3 total transactions, got %d", stats.TotalTransactions)
	}

	expectedTotal := 100.0 + 200.0 + 300.0
	if stats.TotalAmount != expectedTotal {
		t.Errorf("Expected total amount %.2f, got %.2f", expectedTotal, stats.TotalAmount)
	}

	expectedAvg := expectedTotal / 3.0
	if stats.AverageAmount != expectedAvg {
		t.Errorf("Expected average amount %.2f, got %.2f", expectedAvg, stats.AverageAmount)
	}
}

func TestOptimizedFeatureEngineer_FeatureImportance(t *testing.T) {
	ofe := NewOptimizedFeatureEngineer()

	importance := ofe.GetFeatureImportance()

	if len(importance) == 0 {
		t.Error("Expected non-empty feature importance list")
	}

	// Check that features are sorted by score (descending)
	for i := 1; i < len(importance); i++ {
		if importance[i-1].Score < importance[i].Score {
			t.Errorf("Features not sorted by score: %.2f < %.2f", importance[i-1].Score, importance[i].Score)
		}
	}

	// Check that user_behavior has high importance
	found := false
	for _, feat := range importance {
		if feat.Name == "user_behavior" {
			found = true
			if feat.Score < 0.9 {
				t.Errorf("Expected user_behavior score >= 0.9, got %.2f", feat.Score)
			}
			break
		}
	}
	if !found {
		t.Error("Expected user_behavior in feature importance list")
	}
}

func TestOptimizedFeatureEngineer_CacheStats(t *testing.T) {
	ofe := NewOptimizedFeatureEngineer()

	stats := ofe.GetCacheStats()

	if stats == nil {
		t.Error("Expected non-nil cache stats")
	}

	if _, exists := stats["cached_users"]; !exists {
		t.Error("Expected cached_users in stats")
	}

	if _, exists := stats["cache_expiry"]; !exists {
		t.Error("Expected cache_expiry in stats")
	}
}

func TestOptimizedFeatureEngineer_FeatureHash(t *testing.T) {
	ofe := NewOptimizedFeatureEngineer()

	// Create test features
	features := &TransactionFeatures{
		Amount:    100.0,
		HourOfDay: 14.0,
		DayOfWeek: 2.0,
	}

	hash1 := ofe.GenerateFeatureHash(features)
	hash2 := ofe.GenerateFeatureHash(features)

	if hash1 != hash2 {
		t.Error("Expected same hash for identical features")
	}

	// Modify features
	features.Amount = 200.0
	hash3 := ofe.GenerateFeatureHash(features)

	if hash1 == hash3 {
		t.Error("Expected different hash for different features")
	}
}

func TestOptimizedFeatureEngineer_RiskScoreCalculation(t *testing.T) {
	ofe := NewOptimizedFeatureEngineer()

	// Test low risk user
	lowRiskFeatures := &UserFeatures{
		TransactionCount: 50,
		VelocityScore:    2.0,
		AmountStdDev:     50.0,
		AverageAmount:    100.0,
	}

	risk := ofe.calculateUserRiskScore(lowRiskFeatures)
	if risk > 0.5 {
		t.Errorf("Expected low risk score < 0.5, got %.2f", risk)
	}

	// Test high risk user
	highRiskFeatures := &UserFeatures{
		TransactionCount: 2,     // New user
		VelocityScore:    10.0,  // High velocity
		AmountStdDev:     200.0, // High variability
		AverageAmount:    100.0,
	}

	risk = ofe.calculateUserRiskScore(highRiskFeatures)
	if risk < 0.5 {
		t.Errorf("Expected high risk score > 0.5, got %.2f", risk)
	}
}

func TestCalculateStdDev(t *testing.T) {
	values := []float64{10.0, 20.0, 30.0, 40.0, 50.0}
	mean := 30.0

	stdDev := calculateStdDev(values, mean)
	expected := 14.142135623730951 // sqrt(200)

	if math.Abs(stdDev-expected) > 0.001 {
		t.Errorf("Expected std dev %.2f, got %.2f", expected, stdDev)
	}
}

func TestCalculateMedian(t *testing.T) {
	// Test odd number of values
	values1 := []float64{10.0, 20.0, 30.0}
	median1 := calculateMedian(values1)
	if median1 != 20.0 {
		t.Errorf("Expected median 20.0, got %.2f", median1)
	}

	// Test even number of values
	values2 := []float64{10.0, 20.0, 30.0, 40.0}
	median2 := calculateMedian(values2)
	expected2 := (20.0 + 30.0) / 2.0
	if median2 != expected2 {
		t.Errorf("Expected median %.2f, got %.2f", expected2, median2)
	}

	// Test empty slice
	median3 := calculateMedian([]float64{})
	if median3 != 0.0 {
		t.Errorf("Expected median 0.0 for empty slice, got %.2f", median3)
	}
}
