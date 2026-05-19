package cache

import (
	"context"
	"fmt"
	"time"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/infrastructure/ml"
)

// FeatureCache manages caching of fraud detection features
type FeatureCache struct {
	redisCache *RedisCache
	ctx        context.Context
}

// UserFeatureCache represents cached user features
type UserFeatureCache struct {
	UserID              string           `json:"user_id"`
	Features            *ml.UserFeatures `json:"features"`
	LastTransactionTime time.Time        `json:"last_transaction_time"`
	TransactionCount    int              `json:"transaction_count"`
	CacheTime           time.Time        `json:"cache_time"`
	TTL                 time.Duration    `json:"ttl"`
}

// TransactionFeatureCache represents cached transaction features
type TransactionFeatureCache struct {
	TransactionID string                  `json:"transaction_id"`
	Features      *ml.TransactionFeatures `json:"features"`
	RiskScore     float64                 `json:"risk_score"`
	Decision      domain.Decision         `json:"decision"`
	CacheTime     time.Time               `json:"cache_time"`
}

// GlobalStatsCache represents cached global statistics
type GlobalStatsCache struct {
	TotalTransactions int64     `json:"total_transactions"`
	TotalAmount       float64   `json:"total_amount"`
	AverageAmount     float64   `json:"average_amount"`
	AmountStdDev      float64   `json:"amount_std_dev"`
	AmountMedian      float64   `json:"amount_median"`
	LastUpdated       time.Time `json:"last_updated"`
	CacheTime         time.Time `json:"cache_time"`
}

// NewFeatureCache creates a new feature cache manager
func NewFeatureCache(redisCache *RedisCache) *FeatureCache {
	return &FeatureCache{
		redisCache: redisCache,
		ctx:        context.Background(),
	}
}

// CacheUserFeatures caches user features with TTL
func (fc *FeatureCache) CacheUserFeatures(userID string, features *ml.UserFeatures, ttl time.Duration) error {
	key := fmt.Sprintf("user_features:%s", userID)

	cacheItem := &UserFeatureCache{
		UserID:              userID,
		Features:            features,
		LastTransactionTime: time.Now(),
		CacheTime:           time.Now(),
		TTL:                 ttl,
	}

	return fc.redisCache.Set(key, cacheItem, ttl)
}

// GetUserFeatures retrieves cached user features
func (fc *FeatureCache) GetUserFeatures(userID string, dest *ml.UserFeatures) (bool, error) {
	key := fmt.Sprintf("user_features:%s", userID)

	var cacheItem UserFeatureCache
	found, err := fc.redisCache.Get(key, &cacheItem)
	if err != nil {
		return false, err
	}

	if found {
		*dest = *cacheItem.Features
		return true, nil
	}

	return false, nil
}

// InvalidateUserFeatures removes cached user features
func (fc *FeatureCache) InvalidateUserFeatures(userID string) error {
	key := fmt.Sprintf("user_features:%s", userID)
	return fc.redisCache.Delete(key)
}

// CacheTransactionFeatures caches transaction features
func (fc *FeatureCache) CacheTransactionFeatures(transactionID string, features *ml.TransactionFeatures, riskScore float64, decision domain.Decision, ttl time.Duration) error {
	key := fmt.Sprintf("tx_features:%s", transactionID)

	cacheItem := &TransactionFeatureCache{
		TransactionID: transactionID,
		Features:      features,
		RiskScore:     riskScore,
		Decision:      decision,
		CacheTime:     time.Now(),
	}

	return fc.redisCache.Set(key, cacheItem, ttl)
}

// GetTransactionFeatures retrieves cached transaction features
func (fc *FeatureCache) GetTransactionFeatures(transactionID string) (*TransactionFeatureCache, error) {
	key := fmt.Sprintf("tx_features:%s", transactionID)

	var cacheItem TransactionFeatureCache
	found, err := fc.redisCache.Get(key, &cacheItem)
	if err != nil {
		return nil, err
	}

	if found {
		return &cacheItem, nil
	}

	return nil, nil
}

// CacheGlobalStats caches global transaction statistics
func (fc *FeatureCache) CacheGlobalStats(stats *GlobalStatsCache, ttl time.Duration) error {
	key := "global_stats"
	stats.CacheTime = time.Now()
	return fc.redisCache.Set(key, stats, ttl)
}

// GetGlobalStats retrieves cached global statistics
func (fc *FeatureCache) GetGlobalStats() (*GlobalStatsCache, error) {
	key := "global_stats"

	var stats GlobalStatsCache
	found, err := fc.redisCache.Get(key, &stats)
	if err != nil {
		return nil, err
	}

	if found {
		return &stats, nil
	}

	return nil, nil
}

// CacheUserTransactionHistory caches user's recent transactions
func (fc *FeatureCache) CacheUserTransactionHistory(userID string, transactions []*domain.Transaction, ttl time.Duration) error {
	key := fmt.Sprintf("user_history:%s", userID)
	return fc.redisCache.Set(key, transactions, ttl)
}

// GetUserTransactionHistory retrieves cached user transaction history
func (fc *FeatureCache) GetUserTransactionHistory(userID string) ([]*domain.Transaction, error) {
	key := fmt.Sprintf("user_history:%s", userID)

	var transactions []*domain.Transaction
	found, err := fc.redisCache.Get(key, &transactions)
	if err != nil {
		return nil, err
	}

	if found {
		return transactions, nil
	}

	return nil, nil
}

// BatchCacheUserFeatures caches multiple user features at once
func (fc *FeatureCache) BatchCacheUserFeatures(userFeatures map[string]*ml.UserFeatures, ttl time.Duration) error {
	items := make(map[string]interface{})

	for userID, features := range userFeatures {
		key := fmt.Sprintf("user_features:%s", userID)
		cacheItem := &UserFeatureCache{
			UserID:    userID,
			Features:  features,
			CacheTime: time.Now(),
			TTL:       ttl,
		}
		items[key] = cacheItem
	}

	return fc.redisCache.SetMultiple(items, ttl)
}

// WarmupCache preloads frequently accessed features
func (fc *FeatureCache) WarmupCache(userIDs []string, transactionRepo TransactionRepository) error {
	// Batch load user transaction histories
	for _, userID := range userIDs {
		transactions, err := transactionRepo.GetByUserID(userID)
		if err != nil {
			continue
		}

		// Cache user history
		if len(transactions) > 0 {
			fc.CacheUserTransactionHistory(userID, transactions, 30*time.Minute)
		}
	}

	return nil
}

// GetCacheHitRate returns cache hit rate for features
func (fc *FeatureCache) GetCacheHitRate() (float64, error) {
	stats, err := fc.redisCache.GetStats()
	if err != nil {
		return 0, err
	}
	return stats.HitRate, nil
}

// CleanupExpiredFeatures removes expired feature cache entries
func (fc *FeatureCache) CleanupExpiredFeatures() error {
	// Redis automatically handles TTL expiration
	// This method can be used for manual cleanup if needed
	patterns := []string{
		"user_features:*",
		"tx_features:*",
		"user_history:*",
	}

	for _, pattern := range patterns {
		if err := fc.redisCache.Clear(pattern); err != nil {
			return fmt.Errorf("failed to clear pattern %s: %w", pattern, err)
		}
	}

	return nil
}

// GetFeatureCacheStats returns detailed feature cache statistics
func (fc *FeatureCache) GetFeatureCacheStats() (map[string]interface{}, error) {
	stats, err := fc.redisCache.GetStats()
	if err != nil {
		return nil, err
	}

	featureStats := map[string]interface{}{
		"hit_rate":           stats.HitRate,
		"hit_count":          stats.HitCount,
		"miss_count":         stats.MissCount,
		"total_keys":         stats.TotalKeys,
		"memory_usage_bytes": stats.MemoryUsage,
		"avg_latency_ms":     stats.AvgLatency,
		"cache_type":         "feature_cache",
		"timestamp":          time.Now(),
	}

	return featureStats, nil
}

// TransactionRepository interface for dependency injection
type TransactionRepository interface {
	GetByUserID(userID string) ([]*domain.Transaction, error)
}

// CacheConfig represents cache configuration
type CacheConfig struct {
	UserFeaturesTTL time.Duration `yaml:"user_features_ttl"`
	TransactionTTL  time.Duration `yaml:"transaction_ttl"`
	GlobalStatsTTL  time.Duration `yaml:"global_stats_ttl"`
	UserHistoryTTL  time.Duration `yaml:"user_history_ttl"`
	WarmupEnabled   bool          `yaml:"warmup_enabled"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	MaxMemoryUsage  int64         `yaml:"max_memory_usage"`
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		UserFeaturesTTL: 30 * time.Minute,
		TransactionTTL:  60 * time.Minute,
		GlobalStatsTTL:  5 * time.Minute,
		UserHistoryTTL:  15 * time.Minute,
		WarmupEnabled:   true,
		CleanupInterval: 10 * time.Minute,
		MaxMemoryUsage:  100 * 1024 * 1024, // 100MB
	}
}
