package ml

import (
	"math"
	"strings"
	"time"

	"fraud-detection/internal/domain"
)

// FeatureEngineer converts transaction data into ML feature vectors
type FeatureEngineer struct {
	// Configuration for feature engineering
	maxAmount           float64
	amountBuckets       []float64
	suspiciousMerchants []string
	highRiskCountries   []string
}

// NewFeatureEngineer creates a new feature engineer
func NewFeatureEngineer() *FeatureEngineer {
	return &FeatureEngineer{
		maxAmount:     10000.0, // Maximum expected transaction amount
		amountBuckets: []float64{10, 50, 100, 500, 1000, 5000},
		suspiciousMerchants: []string{
			"suspicious-merchant",
			"high-risk-vendor",
			"fake-store",
			"test-merchant",
		},
		highRiskCountries: []string{
			"KP", // North Korea
			"IR", // Iran
			"SY", // Syria
			"CU", // Cuba
			"VE", // Venezuela
		},
	}
}

// TransactionFeatures represents the feature vector for a transaction
type TransactionFeatures struct {
	// Raw features
	Amount       float32
	AmountLog    float32
	AmountBucket []float32 // One-hot encoded amount buckets

	// Time features
	HourOfDay      float32
	DayOfWeek      float32
	IsWeekend      float32
	IsBusinessHour float32

	// Geographic features
	IsDomestic        float32
	IsHighRiskCountry float32

	// Merchant features
	IsSuspiciousMerchant float32
	MerchantCategory     []float32 // One-hot encoded categories

	// User behavior features (would be computed from history)
	UserTransactionCount float32
	UserAvgAmount        float32
	UserAmountStd        float32
	UserVelocityScore    float32

	// Device/User agent features
	IsMobileDevice  float32
	IsKnownDevice   float32
	UserAgentLength float32
}

// ToVector converts features to a flat float32 vector for ML models
func (f *TransactionFeatures) ToVector() []float32 {
	vector := []float32{
		f.Amount,
		f.AmountLog,
		f.HourOfDay,
		f.DayOfWeek,
		f.IsWeekend,
		f.IsBusinessHour,
		f.IsDomestic,
		f.IsHighRiskCountry,
		f.IsSuspiciousMerchant,
		f.UserTransactionCount,
		f.UserAvgAmount,
		f.UserAmountStd,
		f.UserVelocityScore,
		f.IsMobileDevice,
		f.IsKnownDevice,
		f.UserAgentLength,
	}

	// Add one-hot encoded features
	vector = append(vector, f.AmountBucket...)
	vector = append(vector, f.MerchantCategory...)

	return vector
}

// VectorSize returns the expected size of the feature vector
func (f *FeatureEngineer) VectorSize() int {
	// Base features: 16
	baseFeatures := 16
	// Amount buckets: len(amountBuckets) + 1 (for > max)
	amountBuckets := len(f.amountBuckets) + 1
	// Merchant categories: let's assume 5 categories for now
	merchantCategories := 5

	return baseFeatures + amountBuckets + merchantCategories
}

// EngineerFeatures creates feature vector from transaction data
func (f *FeatureEngineer) EngineerFeatures(transaction *domain.Transaction, userHistory []*domain.Transaction) *TransactionFeatures {
	features := &TransactionFeatures{}

	// Amount features
	features.Amount = float32(transaction.Amount)
	features.AmountLog = float32(math.Log10(math.Max(1.0, transaction.Amount)))

	// Amount bucket encoding
	features.AmountBucket = f.encodeAmountBucket(transaction.Amount)

	// Time features
	if !transaction.CreatedAt.IsZero() {
		features.HourOfDay = float32(transaction.CreatedAt.Hour()) / 23.0   // Normalize to [0,1]
		features.DayOfWeek = float32(transaction.CreatedAt.Weekday()) / 7.0 // Normalize to [0,1]
		features.IsWeekend = boolToFloat32(transaction.CreatedAt.Weekday() == time.Saturday || transaction.CreatedAt.Weekday() == time.Sunday)
		features.IsBusinessHour = boolToFloat32(transaction.CreatedAt.Hour() >= 9 && transaction.CreatedAt.Hour() <= 17)
	}

	// Geographic features
	features.IsDomestic = boolToFloat32(transaction.Country == "" || transaction.Country == "US")
	features.IsHighRiskCountry = boolToFloat32(f.isHighRiskCountry(transaction.Country))

	// Merchant features
	features.IsSuspiciousMerchant = boolToFloat32(f.isSuspiciousMerchant(transaction.Merchant))
	features.MerchantCategory = f.encodeMerchantCategory(transaction.Merchant)

	// User behavior features (computed from history)
	if len(userHistory) > 0 {
		features.UserTransactionCount = float32(len(userHistory))

		// Calculate average and std deviation
		var sum, sumSquares float64
		for _, tx := range userHistory {
			sum += tx.Amount
			sumSquares += tx.Amount * tx.Amount
		}
		avg := sum / float64(len(userHistory))
		variance := (sumSquares / float64(len(userHistory))) - (avg * avg)
		std := math.Sqrt(math.Max(0, variance))

		features.UserAvgAmount = float32(avg)
		features.UserAmountStd = float32(std)

		// Simple velocity score: transactions in last 24 hours
		now := time.Now()
		recentCount := 0
		for _, tx := range userHistory {
			if now.Sub(tx.CreatedAt) < 24*time.Hour {
				recentCount++
			}
		}
		features.UserVelocityScore = float32(recentCount) / 10.0 // Normalize
	}

	// Device/User agent features
	features.IsMobileDevice = boolToFloat32(f.isMobileDevice(transaction.UserAgent))
	features.IsKnownDevice = 1.0                                           // Assume known for now (would check against device registry)
	features.UserAgentLength = float32(len(transaction.UserAgent)) / 200.0 // Normalize

	return features
}

// encodeAmountBucket creates one-hot encoding for amount buckets
func (f *FeatureEngineer) encodeAmountBucket(amount float64) []float32 {
	buckets := make([]float32, len(f.amountBuckets)+1) // +1 for amounts > max

	for i, threshold := range f.amountBuckets {
		if amount <= threshold {
			buckets[i] = 1.0
			return buckets
		}
	}

	// Amount is larger than all thresholds
	buckets[len(buckets)-1] = 1.0
	return buckets
}

// encodeMerchantCategory creates one-hot encoding for merchant categories
func (f *FeatureEngineer) encodeMerchantCategory(merchant string) []float32 {
	categories := []float32{0, 0, 0, 0, 0} // 5 categories

	merchant = strings.ToLower(merchant)

	// Simple categorization based on keywords
	switch {
	case strings.Contains(merchant, "grocery") || strings.Contains(merchant, "food") || strings.Contains(merchant, "restaurant") || strings.Contains(merchant, "whole foods"):
		categories[0] = 1.0 // Food & Dining
	case strings.Contains(merchant, "gas") || strings.Contains(merchant, "fuel") || strings.Contains(merchant, "auto") || strings.Contains(merchant, "shell") || strings.Contains(merchant, "station"):
		categories[1] = 1.0 // Transportation
	case strings.Contains(merchant, "amazon") || strings.Contains(merchant, "retail") || strings.Contains(merchant, "online") || (strings.Contains(merchant, "store") && !strings.Contains(merchant, "unknown")):
		categories[2] = 1.0 // Retail
	case strings.Contains(merchant, "hotel") || strings.Contains(merchant, "travel") || strings.Contains(merchant, "airline") || strings.Contains(merchant, "delta"):
		categories[3] = 1.0 // Travel
	case strings.Contains(merchant, "netflix") || strings.Contains(merchant, "digital") || strings.Contains(merchant, "service") || strings.Contains(merchant, "subscription"):
		categories[4] = 1.0 // Digital Services
	default:
		categories[0] = 1.0 // Default to Food & Dining
	}

	return categories
}

// isHighRiskCountry checks if country is in high-risk list
func (f *FeatureEngineer) isHighRiskCountry(country string) bool {
	for _, highRisk := range f.highRiskCountries {
		if strings.EqualFold(country, highRisk) {
			return true
		}
	}
	return false
}

// isSuspiciousMerchant checks if merchant is in suspicious list
func (f *FeatureEngineer) isSuspiciousMerchant(merchant string) bool {
	merchant = strings.ToLower(merchant)
	for _, suspicious := range f.suspiciousMerchants {
		if strings.Contains(merchant, suspicious) {
			return true
		}
	}
	return false
}

// isMobileDevice checks if user agent indicates mobile device
func (f *FeatureEngineer) isMobileDevice(userAgent string) bool {
	userAgent = strings.ToLower(userAgent)
	return strings.Contains(userAgent, "mobile") ||
		strings.Contains(userAgent, "android") ||
		strings.Contains(userAgent, "iphone") ||
		strings.Contains(userAgent, "ipad")
}

// boolToFloat32 converts boolean to float32
func boolToFloat32(b bool) float32 {
	if b {
		return 1.0
	}
	return 0.0
}
