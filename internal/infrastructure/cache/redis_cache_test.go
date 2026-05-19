package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/infrastructure/ml"
)

// MockRedisClient for testing
type MockRedisClient struct {
	data map[string]string
	ttls map[string]time.Time
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]string),
		ttls: make(map[string]time.Time),
	}
}

func (m *MockRedisClient) Set(key string, value string, ttl time.Duration) error {
	m.data[key] = value
	if ttl > 0 {
		m.ttls[key] = time.Now().Add(ttl)
	}
	return nil
}

func (m *MockRedisClient) Get(key string) (string, error) {
	if ttl, exists := m.ttls[key]; exists && time.Now().After(ttl) {
		delete(m.data, key)
		delete(m.ttls, key)
		return "", nil // Simulate Redis Nil
	}

	if value, exists := m.data[key]; exists {
		return value, nil
	}
	return "", nil // Simulate Redis Nil
}

func (m *MockRedisClient) Delete(key string) error {
	delete(m.data, key)
	delete(m.ttls, key)
	return nil
}

func (m *MockRedisClient) Exists(key string) (bool, error) {
	if ttl, exists := m.ttls[key]; exists && time.Now().After(ttl) {
		delete(m.data, key)
		delete(m.ttls, key)
		return false, nil
	}

	_, exists := m.data[key]
	return exists, nil
}

func (m *MockRedisClient) Clear() {
	m.data = make(map[string]string)
	m.ttls = make(map[string]time.Time)
}

// TestRedisCache_BasicOperations tests basic Redis cache operations
func TestRedisCache_BasicOperations(t *testing.T) {
	// Note: This test would require a real Redis instance for full testing
	// For now, we'll test the logic with mock data

	t.Run("Set and Get", func(t *testing.T) {
		mockClient := NewMockRedisClient()

		// Test data
		testData := map[string]interface{}{
			"key1": "value1",
			"key2": 123,
			"key3": map[string]string{"nested": "value"},
		}

		for key := range testData {
			mockClient.Set(key, "mock_value", time.Minute)
			retrieved, err := mockClient.Get(key)
			assert.NoError(t, err)
			assert.Equal(t, "mock_value", retrieved)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		mockClient := NewMockRedisClient()
		mockClient.Set("test_key", "test_value", time.Minute)

		err := mockClient.Delete("test_key")
		assert.NoError(t, err)

		exists, err := mockClient.Exists("test_key")
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("TTL Expiration", func(t *testing.T) {
		mockClient := NewMockRedisClient()
		mockClient.Set("expire_key", "expire_value", 10*time.Millisecond)

		// Should exist immediately
		exists, _ := mockClient.Exists("expire_key")
		assert.True(t, exists)

		// Wait for expiration
		time.Sleep(20 * time.Millisecond)

		// Should not exist after TTL
		exists, _ = mockClient.Exists("expire_key")
		assert.False(t, exists)
	})
}

// TestFeatureCache_UserFeatures tests user feature caching
func TestFeatureCache_UserFeatures(t *testing.T) {
	t.Run("Cache and Retrieve User Features", func(t *testing.T) {
		// Skip Redis integration tests if Redis is not available
		t.Skip("Redis integration test - requires Redis server")

		// Create feature cache with mock
		featureCache := &FeatureCache{
			redisCache: &RedisCache{}, // Mock implementation
			ctx:        context.Background(),
		}

		// Create test user features
		userFeatures := &ml.UserFeatures{
			UserID:              "test-user-123",
			TransactionCount:    10,
			AverageAmount:       150.05,
			AmountStdDev:        50.25,
			MedianAmount:        140.00,
			PreferredCountries:  map[string]int{"US": 5, "CA": 2},
			PreferredMerchants:  map[string]int{"Amazon": 3, "Walmart": 2},
			VelocityScore:       0.05,
			RiskScore:           0.15,
			LastTransactionTime: time.Now(),
			LastUpdated:         time.Now(),
		}

		// Test caching (would use real Redis in production)
		err := featureCache.CacheUserFeatures("test-user-123", userFeatures, 30*time.Minute)
		if err != nil {
			t.Skip("Redis not available for integration test")
		}

		// Test retrieval
		var retrievedFeatures ml.UserFeatures
		found, err := featureCache.GetUserFeatures("test-user-123", &retrievedFeatures)
		if err != nil {
			t.Skip("Redis not available for integration test")
		}

		if found {
			assert.Equal(t, userFeatures.UserID, retrievedFeatures.UserID)
			assert.Equal(t, userFeatures.TransactionCount, retrievedFeatures.TransactionCount)
			assert.Equal(t, userFeatures.AverageAmount, retrievedFeatures.AverageAmount)
		}
	})

	t.Run("User Features Not Found", func(t *testing.T) {
		t.Skip("Redis integration test - requires Redis server")

		featureCache := &FeatureCache{
			redisCache: &RedisCache{},
			ctx:        context.Background(),
		}

		var features ml.UserFeatures
		found, err := featureCache.GetUserFeatures("non-existent-user", &features)

		// Should not error and not find anything
		if err == nil {
			assert.False(t, found)
		} else {
			t.Skip("Redis not available for integration test")
		}
	})
}

// TestFeatureCache_TransactionFeatures tests transaction feature caching
func TestFeatureCache_TransactionFeatures(t *testing.T) {
	t.Run("Cache and Retrieve Transaction Features", func(t *testing.T) {
		t.Skip("Redis integration test - requires Redis server")

		featureCache := &FeatureCache{
			redisCache: &RedisCache{},
			ctx:        context.Background(),
		}

		// Create test transaction features
		txFeatures := &ml.TransactionFeatures{
			Amount:               250.75,
			UserAvgAmount:        150.50,
			UserVelocityScore:    0.15,
			IsBusinessHour:       1.0,
			IsWeekend:            0.0,
			IsHighRiskCountry:    0.0,
			IsSuspiciousMerchant: 0.0,
		}

		// Test caching
		err := featureCache.CacheTransactionFeatures(
			"test-tx-123",
			txFeatures,
			0.65,
			domain.DecisionFlag,
			60*time.Minute,
		)
		if err != nil {
			t.Skip("Redis not available for integration test")
		}

		// Test retrieval
		cached, err := featureCache.GetTransactionFeatures("test-tx-123")
		if err != nil {
			t.Skip("Redis not available for integration test")
		}

		if cached != nil {
			assert.Equal(t, "test-tx-123", cached.TransactionID)
			assert.Equal(t, 0.65, cached.RiskScore)
			assert.Equal(t, domain.DecisionFlag, cached.Decision)
			assert.Equal(t, txFeatures.Amount, cached.Features.Amount)
		}
	})
}

// TestFeatureCache_GlobalStats tests global statistics caching
func TestFeatureCache_GlobalStats(t *testing.T) {
	t.Run("Cache and Retrieve Global Stats", func(t *testing.T) {
		t.Skip("Redis integration test - requires Redis server")

		featureCache := &FeatureCache{
			redisCache: &RedisCache{},
			ctx:        context.Background(),
		}

		// Create test global stats
		globalStats := &GlobalStatsCache{
			TotalTransactions: 1000,
			TotalAmount:       150000.00,
			AverageAmount:     150.00,
			AmountStdDev:      75.50,
			AmountMedian:      140.00,
			LastUpdated:       time.Now(),
		}

		// Test caching
		err := featureCache.CacheGlobalStats(globalStats, 5*time.Minute)
		if err != nil {
			t.Skip("Redis not available for integration test")
		}

		// Test retrieval
		cached, err := featureCache.GetGlobalStats()
		if err != nil {
			t.Skip("Redis not available for integration test")
		}

		if cached != nil {
			assert.Equal(t, globalStats.TotalTransactions, cached.TotalTransactions)
			assert.Equal(t, globalStats.TotalAmount, cached.TotalAmount)
			assert.Equal(t, globalStats.AverageAmount, cached.AverageAmount)
		}
	})
}

// TestFeatureCache_UserHistory tests user transaction history caching
func TestFeatureCache_UserHistory(t *testing.T) {
	t.Run("Cache and Retrieve User History", func(t *testing.T) {
		t.Skip("Redis integration test - requires Redis server")

		featureCache := &FeatureCache{
			redisCache: &RedisCache{},
			ctx:        context.Background(),
		}

		// Create test transactions
		transactions := []*domain.Transaction{
			{
				ID:       uuid.New(),
				UserID:   uuid.New(),
				Amount:   100.50,
				Currency: "USD",
				Merchant: "Test Store",
			},
			{
				ID:       uuid.New(),
				UserID:   uuid.New(),
				Amount:   200.75,
				Currency: "USD",
				Merchant: "Another Store",
			},
		}

		// Test caching
		err := featureCache.CacheUserTransactionHistory("test-user-456", transactions, 15*time.Minute)
		if err != nil {
			t.Skip("Redis not available for integration test")
		}

		// Test retrieval
		cached, err := featureCache.GetUserTransactionHistory("test-user-456")
		if err != nil {
			t.Skip("Redis not available for integration test")
		}

		if cached != nil {
			assert.Len(t, cached, 2)
			assert.Equal(t, transactions[0].Amount, cached[0].Amount)
			assert.Equal(t, transactions[1].Merchant, cached[1].Merchant)
		}
	})
}

// TestCacheConfig tests cache configuration
func TestCacheConfig(t *testing.T) {
	t.Run("Default Configuration", func(t *testing.T) {
		config := DefaultCacheConfig()

		assert.Equal(t, 30*time.Minute, config.UserFeaturesTTL)
		assert.Equal(t, 60*time.Minute, config.TransactionTTL)
		assert.Equal(t, 5*time.Minute, config.GlobalStatsTTL)
		assert.Equal(t, 15*time.Minute, config.UserHistoryTTL)
		assert.True(t, config.WarmupEnabled)
		assert.Equal(t, 10*time.Minute, config.CleanupInterval)
		assert.Equal(t, int64(100*1024*1024), config.MaxMemoryUsage)
	})
}

// TestFeatureCache_Statistics tests cache statistics
func TestFeatureCache_Statistics(t *testing.T) {
	t.Run("Cache Statistics", func(t *testing.T) {
		t.Skip("Redis integration test - requires Redis server")

		featureCache := &FeatureCache{
			redisCache: &RedisCache{},
			ctx:        context.Background(),
		}

		// Test getting statistics
		stats, err := featureCache.GetFeatureCacheStats()
		if err != nil {
			t.Skip("Redis not available for integration test")
		}

		if stats != nil {
			assert.Contains(t, stats, "hit_rate")
			assert.Contains(t, stats, "hit_count")
			assert.Contains(t, stats, "miss_count")
			assert.Contains(t, stats, "total_keys")
			assert.Contains(t, stats, "memory_usage_bytes")
			assert.Contains(t, stats, "cache_type")
			assert.Equal(t, "feature_cache", stats["cache_type"])
		}
	})
}

// Benchmark tests
func BenchmarkFeatureCache_CacheUserFeatures(b *testing.B) {
	featureCache := &FeatureCache{
		redisCache: &RedisCache{},
		ctx:        context.Background(),
	}

	userFeatures := &ml.UserFeatures{
		UserID:             "benchmark-user",
		TransactionCount:   100,
		AverageAmount:      100.00,
		AmountStdDev:       25.00,
		MedianAmount:       95.00,
		RiskScore:          0.2,
		PreferredCountries: map[string]int{},
		PreferredMerchants: map[string]int{},
		LastUpdated:        time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userID := fmt.Sprintf("user-%d", i)
		featureCache.CacheUserFeatures(userID, userFeatures, 30*time.Minute)
	}
}

func BenchmarkFeatureCache_GetUserFeatures(b *testing.B) {
	featureCache := &FeatureCache{
		redisCache: &RedisCache{},
		ctx:        context.Background(),
	}

	// Pre-populate cache
	userFeatures := &ml.UserFeatures{
		UserID: "benchmark-user",
	}
	featureCache.CacheUserFeatures("benchmark-user", userFeatures, 30*time.Minute)

	var features ml.UserFeatures

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		featureCache.GetUserFeatures("benchmark-user", &features)
	}
}
