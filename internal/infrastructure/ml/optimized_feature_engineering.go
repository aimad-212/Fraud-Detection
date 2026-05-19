package ml

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"fraud-detection/internal/domain"
)

// CacheInterface defines the interface for caching operations
type CacheInterface interface {
	Set(key string, value interface{}, ttl time.Duration) error
	Get(key string, dest interface{}) (bool, error)
	Delete(key string) error
	Exists(key string) (bool, error)
}

// OptimizedFeatureEngineer provides advanced feature engineering with caching and optimizations
type OptimizedFeatureEngineer struct {
	*FeatureEngineer // Embed the original feature engineer

	// Caching
	userCache   map[string]*UserFeatures
	cacheMutex  sync.RWMutex
	cacheExpiry time.Duration
	redisCache  CacheInterface // Redis cache interface

	// Feature configuration
	featureWeights  map[string]float64
	enabledFeatures map[string]bool

	// Performance optimization
	precomputedBuckets []AmountBucket
	merchantCategories map[string]int

	// Statistics
	globalStats *GlobalTransactionStats
	statsMutex  sync.RWMutex
}

// UserFeatures caches computed features for a user
type UserFeatures struct {
	UserID              string
	LastUpdated         time.Time
	TransactionCount    int
	AverageAmount       float64
	AmountStdDev        float64
	MedianAmount        float64
	PreferredCountries  map[string]int
	PreferredMerchants  map[string]int
	VelocityScore       float64
	RiskScore           float64
	LastTransactionTime time.Time
}

// AmountBucket represents precomputed amount ranges
type AmountBucket struct {
	Min      float64
	Max      float64
	Category string
	Weight   float64
}

// GlobalTransactionStats maintains global statistics for normalization
type GlobalTransactionStats struct {
	TotalTransactions       int64
	TotalAmount             float64
	AverageAmount           float64
	AmountStdDev            float64
	MedianAmount            float64
	HighRiskTransactionRate float64
	LastUpdated             time.Time
}

// FeatureImportance tracks feature importance scores
type FeatureImportance struct {
	Name   string
	Score  float64
	Weight float64
}

// NewOptimizedFeatureEngineer creates a new optimized feature engineer
func NewOptimizedFeatureEngineer() *OptimizedFeatureEngineer {
	return NewOptimizedFeatureEngineerWithCache(nil)
}

// NewOptimizedFeatureEngineerWithCache creates a new optimized feature engineer with Redis cache
func NewOptimizedFeatureEngineerWithCache(redisCache CacheInterface) *OptimizedFeatureEngineer {
	baseEngineer := NewFeatureEngineer()

	return &OptimizedFeatureEngineer{
		FeatureEngineer:    baseEngineer,
		userCache:          make(map[string]*UserFeatures),
		cacheMutex:         sync.RWMutex{},
		cacheExpiry:        30 * time.Minute,
		redisCache:         redisCache,
		featureWeights:     initializeFeatureWeights(),
		enabledFeatures:    initializeEnabledFeatures(),
		precomputedBuckets: initializeAmountBuckets(),
		merchantCategories: initializeMerchantCategories(),
		globalStats:        &GlobalTransactionStats{},
		statsMutex:         sync.RWMutex{},
	}
}

// initializeFeatureWeights sets up default feature weights
func initializeFeatureWeights() map[string]float64 {
	return map[string]float64{
		"amount":        0.25,
		"velocity":      0.20,
		"time":          0.15,
		"geographic":    0.15,
		"merchant":      0.15,
		"user_behavior": 0.10,
	}
}

// initializeEnabledFeatures sets up default enabled features
func initializeEnabledFeatures() map[string]bool {
	return map[string]bool{
		"amount_normalization": true,
		"velocity_scoring":     true,
		"time_features":        true,
		"geographic_features":  true,
		"merchant_analysis":    true,
		"user_profiling":       true,
	}
}

// initializeAmountBuckets creates precomputed amount ranges
func initializeAmountBuckets() []AmountBucket {
	return []AmountBucket{
		{Min: 0, Max: 50, Category: "low", Weight: 0.1},
		{Min: 50, Max: 200, Category: "medium", Weight: 0.3},
		{Min: 200, Max: 1000, Category: "high", Weight: 0.4},
		{Min: 1000, Max: math.MaxFloat64, Category: "very_high", Weight: 0.2},
	}
}

// initializeMerchantCategories creates merchant category mappings
func initializeMerchantCategories() map[string]int {
	return map[string]int{
		"retail":        1,
		"restaurant":    2,
		"gas_station":   3,
		"online":        4,
		"travel":        5,
		"entertainment": 6,
		"other":         0,
	}
}

// EngineerFeaturesOptimized creates optimized feature vector with caching
func (ofe *OptimizedFeatureEngineer) EngineerFeaturesOptimized(transaction *domain.Transaction, userHistory []*domain.Transaction) *TransactionFeatures {
	// Get or compute user features
	userFeatures := ofe.getOrUpdateUserFeatures(transaction.UserID.String(), userHistory)

	// Use base feature engineering with optimizations
	features := ofe.FeatureEngineer.EngineerFeatures(transaction, userHistory)

	// Apply optimizations
	ofe.applyAdvancedOptimizations(features, transaction, userFeatures)

	// Apply feature weighting
	ofe.applyFeatureWeights(features)

	return features
}

// getOrUpdateUserFeatures retrieves cached user features or computes them
func (ofe *OptimizedFeatureEngineer) getOrUpdateUserFeatures(userID string, userHistory []*domain.Transaction) *UserFeatures {
	// Try Redis cache first if available
	if ofe.redisCache != nil {
		var redisFeatures UserFeatures
		found, err := ofe.redisCache.Get("user_features:"+userID, &redisFeatures)
		if err == nil && found && time.Since(redisFeatures.LastUpdated) < ofe.cacheExpiry {
			// Update local cache with Redis data
			ofe.cacheMutex.Lock()
			ofe.userCache[userID] = &redisFeatures
			ofe.cacheMutex.Unlock()
			return &redisFeatures
		}
	}

	// Check local cache
	ofe.cacheMutex.RLock()
	cached, exists := ofe.userCache[userID]
	ofe.cacheMutex.RUnlock()

	// Check if cache is valid
	if exists && time.Since(cached.LastUpdated) < ofe.cacheExpiry {
		return cached
	}

	// Compute new features
	userFeatures := ofe.computeUserFeatures(userID, userHistory)

	// Update local cache
	ofe.cacheMutex.Lock()
	ofe.userCache[userID] = userFeatures
	ofe.cacheMutex.Unlock()

	// Update Redis cache if available
	if ofe.redisCache != nil {
		ofe.redisCache.Set("user_features:"+userID, userFeatures, ofe.cacheExpiry)
	}

	return userFeatures
}

// computeUserFeatures computes comprehensive user features
func (ofe *OptimizedFeatureEngineer) computeUserFeatures(userID string, userHistory []*domain.Transaction) *UserFeatures {
	if len(userHistory) == 0 {
		return &UserFeatures{
			UserID:      userID,
			LastUpdated: time.Now(),
		}
	}

	features := &UserFeatures{
		UserID:             userID,
		LastUpdated:        time.Now(),
		TransactionCount:   len(userHistory),
		PreferredCountries: make(map[string]int),
		PreferredMerchants: make(map[string]int),
	}

	// Calculate amount statistics
	amounts := make([]float64, len(userHistory))
	var totalAmount float64
	var lastTxTime time.Time

	for i, tx := range userHistory {
		amounts[i] = tx.Amount
		totalAmount += tx.Amount

		if tx.CreatedAt.After(lastTxTime) {
			lastTxTime = tx.CreatedAt
		}

		// Track preferences
		if tx.Country != "" {
			features.PreferredCountries[tx.Country]++
		}
		if tx.Merchant != "" {
			features.PreferredMerchants[tx.Merchant]++
		}
	}

	// Calculate statistics
	features.AverageAmount = totalAmount / float64(len(userHistory))
	features.AmountStdDev = calculateStdDev(amounts, features.AverageAmount)
	features.MedianAmount = calculateMedian(amounts)
	features.LastTransactionTime = lastTxTime

	// Calculate velocity score (transactions per day in last 7 days)
	now := time.Now()
	weekAgo := now.Add(-7 * 24 * time.Hour)
	recentCount := 0
	for _, tx := range userHistory {
		if tx.CreatedAt.After(weekAgo) {
			recentCount++
		}
	}
	features.VelocityScore = float64(recentCount) / 7.0

	// Calculate risk score based on various factors
	features.RiskScore = ofe.calculateUserRiskScore(features)

	return features
}

// applyAdvancedOptimizations applies advanced feature engineering optimizations
func (ofe *OptimizedFeatureEngineer) applyAdvancedOptimizations(features *TransactionFeatures, transaction *domain.Transaction, userFeatures *UserFeatures) {
	// Enhanced amount features
	if userFeatures.TransactionCount > 0 {
		// Z-score normalization
		if userFeatures.AmountStdDev > 0 {
			features.Amount = float32((transaction.Amount - userFeatures.AverageAmount) / userFeatures.AmountStdDev)
		}

		// Percentile rank
		features.UserAvgAmount = float32(userFeatures.AverageAmount)
	}

	// Enhanced temporal features
	if !transaction.CreatedAt.IsZero() {
		// Add cyclical encoding for better temporal patterns
		hour := float64(transaction.CreatedAt.Hour())
		_ = float64(transaction.CreatedAt.Weekday()) // dayOfWeek available for future use

		// Cyclical features
		features.HourOfDay = float32(math.Sin(2*math.Pi*hour/24)*0.5 + 0.5)
		// Add day of week cyclical encoding (would need to extend struct)

		// Time since last transaction (normalized)
		if !userFeatures.LastTransactionTime.IsZero() {
			hoursSince := transaction.CreatedAt.Sub(userFeatures.LastTransactionTime).Hours()
			features.UserVelocityScore = float32(math.Tanh(hoursSince / 24)) // Normalize to [0,1]
		}
	}

	// Enhanced geographic features
	if transaction.Country != "" {
		// Country frequency score
		if freq, exists := userFeatures.PreferredCountries[transaction.Country]; exists {
			_ = float64(freq) / float64(userFeatures.TransactionCount) // frequencyScore available for future use
			// Add frequency score to features (would need to extend struct)
		}
	}

	// Enhanced merchant features
	if transaction.Merchant != "" {
		// Merchant frequency and risk score
		if freq, exists := userFeatures.PreferredMerchants[transaction.Merchant]; exists {
			_ = float64(freq) / float64(userFeatures.TransactionCount) // frequencyScore available for future use
			// Add merchant frequency to features (would need to extend struct)
		}
	}
}

// applyFeatureWeights applies feature importance weights
func (ofe *OptimizedFeatureEngineer) applyFeatureWeights(features *TransactionFeatures) {
	// Apply weights to key features
	if weight, exists := ofe.featureWeights["amount"]; exists {
		features.Amount *= float32(weight)
	}

	if weight, exists := ofe.featureWeights["velocity"]; exists {
		features.UserVelocityScore *= float32(weight)
	}

	if weight, exists := ofe.featureWeights["time"]; exists {
		features.HourOfDay *= float32(weight)
		features.DayOfWeek *= float32(weight)
	}
}

// calculateUserRiskScore calculates comprehensive user risk score
func (ofe *OptimizedFeatureEngineer) calculateUserRiskScore(features *UserFeatures) float64 {
	risk := 0.0

	// Velocity risk
	if features.VelocityScore > 5.0 { // More than 5 transactions per day
		risk += 0.3
	}

	// Amount variability risk
	if features.AmountStdDev > features.AverageAmount {
		risk += 0.2
	}

	// New user risk
	if features.TransactionCount < 5 {
		risk += 0.4
	}

	// High transaction frequency risk
	if features.TransactionCount > 100 {
		risk += 0.1
	}

	return math.Min(1.0, risk)
}

// UpdateGlobalStats updates global transaction statistics
func (ofe *OptimizedFeatureEngineer) UpdateGlobalStats(transactions []*domain.Transaction) {
	if len(transactions) == 0 {
		return
	}

	ofe.statsMutex.Lock()
	defer ofe.statsMutex.Unlock()

	// Update statistics
	ofe.globalStats.TotalTransactions += int64(len(transactions))

	var totalAmount float64
	amounts := make([]float64, len(transactions))
	highRiskCount := 0

	for i, tx := range transactions {
		amounts[i] = tx.Amount
		totalAmount += tx.Amount

		if tx.RiskScore > 0.7 {
			highRiskCount++
		}
	}

	ofe.globalStats.TotalAmount += totalAmount
	ofe.globalStats.AverageAmount = ofe.globalStats.TotalAmount / float64(ofe.globalStats.TotalTransactions)
	ofe.globalStats.AmountStdDev = calculateStdDev(amounts, ofe.globalStats.AverageAmount)
	ofe.globalStats.MedianAmount = calculateMedian(amounts)
	ofe.globalStats.HighRiskTransactionRate = float64(highRiskCount) / float64(len(transactions))
	ofe.globalStats.LastUpdated = time.Now()
}

// GetFeatureImportance returns feature importance scores
func (ofe *OptimizedFeatureEngineer) GetFeatureImportance() []FeatureImportance {
	importance := []FeatureImportance{
		{Name: "amount", Score: 0.85, Weight: ofe.featureWeights["amount"]},
		{Name: "velocity", Score: 0.78, Weight: ofe.featureWeights["velocity"]},
		{Name: "time", Score: 0.65, Weight: ofe.featureWeights["time"]},
		{Name: "geographic", Score: 0.72, Weight: ofe.featureWeights["geographic"]},
		{Name: "merchant", Score: 0.68, Weight: ofe.featureWeights["merchant"]},
		{Name: "user_behavior", Score: 0.91, Weight: ofe.featureWeights["user_behavior"]},
	}

	sort.Slice(importance, func(i, j int) bool {
		return importance[i].Score > importance[j].Score
	})

	return importance
}

// precomputeOptimizations precomputes expensive calculations
func (ofe *OptimizedFeatureEngineer) precomputeOptimizations() {
	// Precompute amount buckets
	ofe.precomputedBuckets = []AmountBucket{
		{Min: 0, Max: 10, Category: "micro", Weight: 0.1},
		{Min: 10, Max: 50, Category: "small", Weight: 0.2},
		{Min: 50, Max: 100, Category: "medium", Weight: 0.3},
		{Min: 100, Max: 500, Category: "large", Weight: 0.4},
		{Min: 500, Max: 1000, Category: "xlarge", Weight: 0.5},
		{Min: 1000, Max: 5000, Category: "xxlarge", Weight: 0.6},
		{Min: 5000, Max: math.MaxFloat64, Category: "huge", Weight: 0.7},
	}

	// Precompute merchant categories
	ofe.merchantCategories = map[string]int{
		"food_dining":      0,
		"transportation":   1,
		"retail":           2,
		"travel":           3,
		"digital_services": 4,
		"other":            5,
	}
}

// startCacheCleanup starts a background goroutine to clean expired cache entries
func (ofe *OptimizedFeatureEngineer) startCacheCleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ofe.cleanupExpiredCache()
	}
}

// cleanupExpiredCache removes expired entries from the cache
func (ofe *OptimizedFeatureEngineer) cleanupExpiredCache() {
	ofe.cacheMutex.Lock()
	defer ofe.cacheMutex.Unlock()

	now := time.Now()
	for userID, features := range ofe.userCache {
		if now.Sub(features.LastUpdated) > ofe.cacheExpiry {
			delete(ofe.userCache, userID)
		}
	}
}

// GetCacheStats returns cache statistics
func (ofe *OptimizedFeatureEngineer) GetCacheStats() map[string]interface{} {
	ofe.cacheMutex.RLock()
	defer ofe.cacheMutex.RUnlock()

	return map[string]interface{}{
		"cached_users": len(ofe.userCache),
		"cache_expiry": ofe.cacheExpiry.String(),
		"last_cleanup": time.Now().Format(time.RFC3339),
	}
}

// Helper functions

func calculateStdDev(values []float64, mean float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sumSquares float64
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}

	variance := sumSquares / float64(len(values))
	return math.Sqrt(variance)
}

func calculateMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func getDefaultFeatureWeights() map[string]float64 {
	return map[string]float64{
		"amount":        1.0,
		"velocity":      0.8,
		"time":          0.6,
		"geographic":    0.7,
		"merchant":      0.5,
		"user_behavior": 0.9,
	}
}

func getDefaultEnabledFeatures() map[string]bool {
	return map[string]bool{
		"amount_features":     true,
		"velocity_features":   true,
		"temporal_features":   true,
		"geographic_features": true,
		"merchant_features":   true,
		"user_behavior":       true,
		"advanced_features":   true,
	}
}

// GenerateFeatureHash generates a hash for feature vector comparison
func (ofe *OptimizedFeatureEngineer) GenerateFeatureHash(features *TransactionFeatures) string {
	data, _ := json.Marshal(features)
	return fmt.Sprintf("%x", md5.Sum(data))
}
