package tests

import (
	"testing"
	"time"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/infrastructure/ml"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockModelScorer(t *testing.T) {
	scorer := ml.NewMockModelScorer(0.5)

	// Test scoring - mock scorer always returns the configured score
	score, err := scorer.Score([]float32{100.0, 0.5})
	require.NoError(t, err)
	assert.Equal(t, 0.5, score)

	score, err = scorer.Score([]float32{500.0})
	require.NoError(t, err)
	assert.Equal(t, 0.5, score)

	score, err = scorer.Score([]float32{2000.0})
	require.NoError(t, err)
	assert.Equal(t, 0.5, score)

	// Test loaded status
	assert.True(t, scorer.IsLoaded())

	// Test close
	err = scorer.Close()
	require.NoError(t, err)
	assert.False(t, scorer.IsLoaded())

	// Test scoring after close
	_, err = scorer.Score([]float32{100.0})
	assert.Error(t, err)
}

func TestONNXModelScorer(t *testing.T) {
	// Test with non-existent model file - should handle gracefully
	scorer, err := ml.NewONNXModelScorer("fake-model.onnx")

	// ONNX runtime requires native library installation
	// If the library is not available, we expect an error
	// This is expected in CI/CD environments without ONNX runtime
	if err != nil {
		// Expected error when ONNX runtime library is not installed
		assert.Contains(t, err.Error(), "ONNX runtime", "Should mention ONNX runtime issue")
		t.Logf("ONNX runtime not available (expected in CI/CD): %v", err)
		return
	}

	// If we got here, ONNX runtime is available
	assert.NotNil(t, scorer)

	// Test loaded status (depends on model validity)
	t.Logf("ONNX scorer loaded: %v", scorer.IsLoaded())

	// Test scoring when loaded
	score, err := scorer.Score([]float32{100.0, 0.5})
	if scorer.IsLoaded() {
		assert.NoError(t, err)
		assert.Greater(t, score, 0.0)
	} else {
		assert.Error(t, err)
		assert.Equal(t, 0.0, score)
	}

	// Test close
	err = scorer.Close()
	assert.NoError(t, err)
}

func TestFeatureEngineer_VectorSize(t *testing.T) {
	engineer := ml.NewFeatureEngineer()

	// Expected size: 16 base + 7 amount buckets (6 thresholds + 1 overflow) + 5 merchant categories
	expectedSize := 16 + 7 + 5
	assert.Equal(t, expectedSize, engineer.VectorSize())
}

func TestFeatureEngineer_EngineerFeatures(t *testing.T) {
	engineer := ml.NewFeatureEngineer()

	// Create test transaction
	userID, _ := uuid.Parse("550e8400-e29b-41d4-a716-446655440000")
	transaction := domain.NewTransaction(userID, 250.75, "USD")
	transaction.Merchant = "Test Grocery Store"
	transaction.Country = "US"
	transaction.IPAddress = "192.168.1.1"
	transaction.UserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)"
	transaction.DeviceID = "device123"
	transaction.CreatedAt = time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC) // Monday afternoon

	// Create some user history
	history := []*domain.Transaction{
		domain.NewTransaction(userID, 100.0, "USD"),
		domain.NewTransaction(userID, 200.0, "USD"),
		domain.NewTransaction(userID, 50.0, "USD"),
	}

	features := engineer.EngineerFeatures(transaction, history)

	// Test basic features
	assert.Equal(t, float32(250.75), features.Amount)
	assert.True(t, features.AmountLog > 0)                             // Log of amount should be positive
	assert.True(t, features.HourOfDay >= 0 && features.HourOfDay <= 1) // Normalized hour
	assert.Equal(t, float32(1.0/7.0), features.DayOfWeek)              // Monday = 1/7
	assert.Equal(t, float32(1.0), features.IsBusinessHour)             // 14:30 is business hours
	assert.Equal(t, float32(1.0), features.IsDomestic)                 // US is domestic
	assert.Equal(t, float32(0.0), features.IsHighRiskCountry)          // US is not high risk
	assert.Equal(t, float32(0.0), features.IsSuspiciousMerchant)       // Not suspicious
	assert.Equal(t, float32(1.0), features.IsMobileDevice)             // iPhone user agent

	// Test user behavior features
	assert.Equal(t, float32(3), features.UserTransactionCount)
	assert.True(t, features.UserAvgAmount > 0)
	assert.True(t, features.UserVelocityScore >= 0)

	// Test feature vector conversion
	vector := features.ToVector()
	assert.Len(t, vector, engineer.VectorSize())

	// Most values should be normalized to [0,1], but some like Amount may not be
	for i, v := range vector {
		if i != 0 && i != 1 && i != 9 && i != 10 && i != 11 { // Skip Amount, AmountLog, UserTransactionCount, UserAvgAmount, UserAmountStd
			assert.True(t, v >= 0 && v <= 1, "Feature %d should be normalized to [0,1], got %f", i, v)
		}
		assert.True(t, v >= 0, "Feature %d should be non-negative, got %f", i, v)
	}
}

func TestFeatureEngineer_AmountBuckets(t *testing.T) {
	engineer := ml.NewFeatureEngineer()

	// Test through feature engineering
	userID, _ := uuid.Parse("550e8400-e29b-41d4-a716-446655440000")

	tests := []struct {
		amount   float64
		expected []float32 // One-hot encoded buckets
	}{
		{5.0, []float32{1, 0, 0, 0, 0, 0, 0}},    // First bucket (<= 10)
		{30.0, []float32{0, 1, 0, 0, 0, 0, 0}},   // Second bucket (10 < x <= 50)
		{75.0, []float32{0, 0, 1, 0, 0, 0, 0}},   // Third bucket (50 < x <= 100)
		{250.0, []float32{0, 0, 0, 1, 0, 0, 0}},  // Fourth bucket (100 < x <= 500)
		{750.0, []float32{0, 0, 0, 0, 1, 0, 0}},  // Fifth bucket (500 < x <= 1000)
		{2500.0, []float32{0, 0, 0, 0, 0, 1, 0}}, // Sixth bucket (1000 < x <= 5000)
		{7500.0, []float32{0, 0, 0, 0, 0, 0, 1}}, // Overflow bucket (> 5000)
	}

	for _, test := range tests {
		transaction := domain.NewTransaction(userID, test.amount, "USD")
		features := engineer.EngineerFeatures(transaction, nil)
		assert.Equal(t, test.expected, features.AmountBucket, "Amount %f bucket encoding failed", test.amount)
	}
}

func TestFeatureEngineer_MerchantCategories(t *testing.T) {
	engineer := ml.NewFeatureEngineer()

	// Test through feature engineering
	userID, _ := uuid.Parse("550e8400-e29b-41d4-a716-446655440000")

	tests := []struct {
		merchant string
		expected []float32 // One-hot encoded categories
	}{
		{"Whole Foods Grocery", []float32{1, 0, 0, 0, 0}},  // Food
		{"Shell Gas Station", []float32{0, 1, 0, 0, 0}},    // Transportation
		{"Amazon Online Store", []float32{0, 0, 1, 0, 0}},  // Retail
		{"Delta Airlines", []float32{0, 0, 0, 1, 0}},       // Travel
		{"Netflix Subscription", []float32{0, 0, 0, 0, 1}}, // Digital
		{"Unknown Store", []float32{1, 0, 0, 0, 0}},        // Default to Food
	}

	for _, test := range tests {
		transaction := domain.NewTransaction(userID, 100.0, "USD")
		transaction.Merchant = test.merchant
		features := engineer.EngineerFeatures(transaction, nil)
		assert.Equal(t, test.expected, features.MerchantCategory, "Merchant '%s' category encoding failed", test.merchant)
	}
}

func TestFeatureEngineer_TimeFeatures(t *testing.T) {
	engineer := ml.NewFeatureEngineer()

	userID, _ := uuid.Parse("550e8400-e29b-41d4-a716-446655440000")
	transaction := domain.NewTransaction(userID, 100.0, "USD")

	// Test weekend detection
	transaction.CreatedAt = time.Date(2024, 1, 13, 12, 0, 0, 0, time.UTC) // Saturday
	features := engineer.EngineerFeatures(transaction, nil)
	assert.Equal(t, float32(1.0), features.IsWeekend)

	// Test weekday
	transaction.CreatedAt = time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC) // Monday
	features = engineer.EngineerFeatures(transaction, nil)
	assert.Equal(t, float32(0.0), features.IsWeekend)

	// Test business hours
	transaction.CreatedAt = time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC) // 2:30 PM
	features = engineer.EngineerFeatures(transaction, nil)
	assert.Equal(t, float32(1.0), features.IsBusinessHour)

	// Test non-business hours
	transaction.CreatedAt = time.Date(2024, 1, 15, 22, 0, 0, 0, time.UTC) // 10:00 PM
	features = engineer.EngineerFeatures(transaction, nil)
	assert.Equal(t, float32(0.0), features.IsBusinessHour)
}

func TestFeatureEngineer_GeographicFeatures(t *testing.T) {
	engineer := ml.NewFeatureEngineer()

	userID, _ := uuid.Parse("550e8400-e29b-41d4-a716-446655440000")
	transaction := domain.NewTransaction(userID, 100.0, "USD")

	// Test domestic (US)
	transaction.Country = "US"
	features := engineer.EngineerFeatures(transaction, nil)
	assert.Equal(t, float32(1.0), features.IsDomestic)
	assert.Equal(t, float32(0.0), features.IsHighRiskCountry)

	// Test international non-risk
	transaction.Country = "CA"
	features = engineer.EngineerFeatures(transaction, nil)
	assert.Equal(t, float32(0.0), features.IsDomestic)
	assert.Equal(t, float32(0.0), features.IsHighRiskCountry)

	// Test high-risk country
	transaction.Country = "KP" // North Korea
	features = engineer.EngineerFeatures(transaction, nil)
	assert.Equal(t, float32(0.0), features.IsDomestic)
	assert.Equal(t, float32(1.0), features.IsHighRiskCountry)
}

func BenchmarkFeatureEngineering(b *testing.B) {
	engineer := ml.NewFeatureEngineer()

	userID, _ := uuid.Parse("550e8400-e29b-41d4-a716-446655440000")
	transaction := domain.NewTransaction(userID, 250.0, "USD")
	transaction.Merchant = "Test Store"
	transaction.Country = "US"
	transaction.CreatedAt = time.Now()

	history := []*domain.Transaction{
		domain.NewTransaction(userID, 100.0, "USD"),
		domain.NewTransaction(userID, 200.0, "USD"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engineer.EngineerFeatures(transaction, history)
	}
}

func BenchmarkModelScoring(b *testing.B) {
	scorer := ml.NewMockModelScorer(0.5)
	features := []float32{250.0, 0.5, 0.7, 0.0, 1.0, 1.0, 0.0, 0.0, 0.0, 5.0, 150.0, 50.0, 2.0, 0.0, 1.0, 0.8}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = scorer.Score(features)
	}
}
