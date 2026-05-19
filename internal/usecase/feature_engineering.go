package usecase

import (
	"math"
	"time"

	"fraud-detection/internal/domain"
)

// FeatureEngineer handles feature extraction and engineering
type FeatureEngineer struct{}

// NewFeatureEngineer creates a new feature engineer
func NewFeatureEngineer() *FeatureEngineer {
	return &FeatureEngineer{}
}

// ExtractFeatures extracts features from a transaction and context
func (fe *FeatureEngineer) ExtractFeatures(transaction *domain.Transaction, context *domain.EvaluationContext) *TransactionFeatures {
	features := &TransactionFeatures{
		TransactionID: transaction.ID.String(),
		UserID:        transaction.UserID.String(),
		Amount:        transaction.Amount,
		Currency:      transaction.Currency,
		Merchant:      transaction.Merchant,
		Country:       transaction.Country,
		Timestamp:     transaction.CreatedAt,
	}

	if context != nil {
		fe.extractUserFeatures(features, transaction, context)
		fe.extractVelocityFeatures(features, transaction, context)
		fe.extractGeographicFeatures(features, transaction, context)
		fe.extractTemporalFeatures(features, transaction, context)
	}

	return features
}

// extractUserFeatures extracts user behavior features
func (fe *FeatureEngineer) extractUserFeatures(features *TransactionFeatures, transaction *domain.Transaction, context *domain.EvaluationContext) {
	if context.UserProfile != nil {
		profile := context.UserProfile

		// Amount-based features
		features.AmountToAverageRatio = transaction.Amount / profile.AverageTransactionAmount
		features.IsAmountUnusual = math.Abs(features.AmountToAverageRatio-1.0) > 0.5

		// Frequency features
		features.UserTransactionCount = profile.TransactionCount

		// Geographic features
		features.IsNewCountry = !fe.isCountryInList(transaction.Country, profile.PreferredCountries)

		// Merchant features
		features.IsNewMerchant = !fe.isMerchantInList(transaction.Merchant, profile.PreferredMerchants)

		// Risk features
		features.UserRiskScore = profile.RiskScore
	}

	// Transaction history features
	if context.UserTransactions != nil && len(context.UserTransactions) > 0 {
		features.UserTotalTransactions = len(context.UserTransactions)

		// Calculate user's transaction statistics
		totalAmount := 0.0
		count := 0
		lastTransactionTime := time.Time{}

		for _, tx := range context.UserTransactions {
			if tx.ID != transaction.ID { // Exclude current transaction
				totalAmount += tx.Amount
				count++

				if tx.CreatedAt.After(lastTransactionTime) {
					lastTransactionTime = tx.CreatedAt
				}
			}
		}

		if count > 0 {
			features.UserAverageAmount = totalAmount / float64(count)
			features.AmountToUserAverageRatio = transaction.Amount / features.UserAverageAmount
		}

		if !lastTransactionTime.IsZero() {
			features.TimeSinceLastTransaction = transaction.CreatedAt.Sub(lastTransactionTime)
		}
	}
}

// extractVelocityFeatures extracts velocity-based features
func (fe *FeatureEngineer) extractVelocityFeatures(features *TransactionFeatures, transaction *domain.Transaction, context *domain.EvaluationContext) {
	if context.UserTransactions == nil {
		return
	}

	now := transaction.CreatedAt

	// Transactions in last 1 hour
	features.TransactionsLast1Hour = fe.countTransactionsInWindow(context.UserTransactions, transaction.ID.String(), now, 1*time.Hour)

	// Transactions in last 24 hours
	features.TransactionsLast24Hours = fe.countTransactionsInWindow(context.UserTransactions, transaction.ID.String(), now, 24*time.Hour)

	// Transactions in last 7 days
	features.TransactionsLast7Days = fe.countTransactionsInWindow(context.UserTransactions, transaction.ID.String(), now, 7*24*time.Hour)

	// Amount in last 24 hours
	features.AmountLast24Hours = fe.sumAmountInWindow(context.UserTransactions, transaction.ID.String(), now, 24*time.Hour)

	// Average amount per transaction in last 24 hours
	if features.TransactionsLast24Hours > 0 {
		features.AverageAmountLast24Hours = features.AmountLast24Hours / float64(features.TransactionsLast24Hours)
	}

	// Velocity ratios
	if features.UserAverageAmount > 0 {
		features.VelocityRatio = features.AmountLast24Hours / features.UserAverageAmount
	}
}

// extractGeographicFeatures extracts geographic features
func (fe *FeatureEngineer) extractGeographicFeatures(features *TransactionFeatures, transaction *domain.Transaction, context *domain.EvaluationContext) {
	// Basic geographic features
	features.HasCountry = transaction.Country != ""
	features.CountryCode = transaction.Country

	// Check if country is in high-risk list
	highRiskCountries := []string{"KP", "IR", "SY", "VE", "CU"}
	features.IsHighRiskCountry = fe.isCountryInList(transaction.Country, highRiskCountries)

	// Check if country is in user's preferred countries
	if context.UserProfile != nil {
		features.IsPreferredCountry = fe.isCountryInList(transaction.Country, context.UserProfile.PreferredCountries)
	}
}

// extractTemporalFeatures extracts time-based features
func (fe *FeatureEngineer) extractTemporalFeatures(features *TransactionFeatures, transaction *domain.Transaction, context *domain.EvaluationContext) {
	t := transaction.CreatedAt

	// Time features
	features.HourOfDay = t.Hour()
	features.DayOfWeek = int(t.Weekday())
	features.DayOfMonth = t.Day()
	features.Month = int(t.Month())

	// Business hour flags
	features.IsBusinessHour = features.HourOfDay >= 9 && features.HourOfDay <= 17
	features.IsWeekend = features.DayOfWeek == 0 || features.DayOfWeek == 6
	features.IsNightTime = features.HourOfDay >= 22 || features.HourOfDay <= 5

	// Seasonal features
	features.IsHolidaySeason = features.Month == 11 || features.Month == 12 // Nov-Dec
}

// Helper methods

func (fe *FeatureEngineer) countTransactionsInWindow(transactions []*domain.Transaction, excludeID string, now time.Time, window time.Duration) int {
	count := 0
	cutoff := now.Add(-window)

	for _, tx := range transactions {
		if tx.ID.String() != excludeID && tx.CreatedAt.After(cutoff) && tx.CreatedAt.Before(now) {
			count++
		}
	}

	return count
}

func (fe *FeatureEngineer) sumAmountInWindow(transactions []*domain.Transaction, excludeID string, now time.Time, window time.Duration) float64 {
	total := 0.0
	cutoff := now.Add(-window)

	for _, tx := range transactions {
		if tx.ID.String() != excludeID && tx.CreatedAt.After(cutoff) && tx.CreatedAt.Before(now) {
			total += tx.Amount
		}
	}

	return total
}

func (fe *FeatureEngineer) isCountryInList(country string, countries []string) bool {
	for _, c := range countries {
		if c == country {
			return true
		}
	}
	return false
}

func (fe *FeatureEngineer) isMerchantInList(merchant string, merchants []string) bool {
	for _, m := range merchants {
		if m == merchant {
			return true
		}
	}
	return false
}

// TransactionFeatures represents extracted features for a transaction
type TransactionFeatures struct {
	// Basic transaction features
	TransactionID string    `json:"transaction_id"`
	UserID        string    `json:"user_id"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	Merchant      string    `json:"merchant"`
	Country       string    `json:"country"`
	Timestamp     time.Time `json:"timestamp"`

	// User behavior features
	UserTotalTransactions    int           `json:"user_total_transactions"`
	UserAverageAmount        float64       `json:"user_average_amount"`
	AmountToAverageRatio     float64       `json:"amount_to_average_ratio"`
	AmountToUserAverageRatio float64       `json:"amount_to_user_average_ratio"`
	IsAmountUnusual          bool          `json:"is_amount_unusual"`
	TimeSinceLastTransaction time.Duration `json:"time_since_last_transaction"`
	UserTransactionCount     int           `json:"user_transaction_count"`
	UserRiskScore            float64       `json:"user_risk_score"`

	// Velocity features
	TransactionsLast1Hour    int     `json:"transactions_last_1_hour"`
	TransactionsLast24Hours  int     `json:"transactions_last_24_hours"`
	TransactionsLast7Days    int     `json:"transactions_last_7_days"`
	AmountLast24Hours        float64 `json:"amount_last_24_hours"`
	AverageAmountLast24Hours float64 `json:"average_amount_last_24_hours"`
	VelocityRatio            float64 `json:"velocity_ratio"`

	// Geographic features
	HasCountry         bool   `json:"has_country"`
	CountryCode        string `json:"country_code"`
	IsNewCountry       bool   `json:"is_new_country"`
	IsHighRiskCountry  bool   `json:"is_high_risk_country"`
	IsPreferredCountry bool   `json:"is_preferred_country"`

	// Merchant features
	IsNewMerchant bool `json:"is_new_merchant"`

	// Temporal features
	HourOfDay       int  `json:"hour_of_day"`
	DayOfWeek       int  `json:"day_of_week"`
	DayOfMonth      int  `json:"day_of_month"`
	Month           int  `json:"month"`
	IsBusinessHour  bool `json:"is_business_hour"`
	IsWeekend       bool `json:"is_weekend"`
	IsNightTime     bool `json:"is_night_time"`
	IsHolidaySeason bool `json:"is_holiday_season"`
}

// ToMap converts features to a map for easy access
func (tf *TransactionFeatures) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"transaction_id":               tf.TransactionID,
		"user_id":                      tf.UserID,
		"amount":                       tf.Amount,
		"currency":                     tf.Currency,
		"merchant":                     tf.Merchant,
		"country":                      tf.Country,
		"user_total_transactions":      tf.UserTotalTransactions,
		"user_average_amount":          tf.UserAverageAmount,
		"amount_to_average_ratio":      tf.AmountToAverageRatio,
		"amount_to_user_average_ratio": tf.AmountToUserAverageRatio,
		"is_amount_unusual":            tf.IsAmountUnusual,
		"time_since_last_transaction":  tf.TimeSinceLastTransaction.String(),
		"transactions_last_1_hour":     tf.TransactionsLast1Hour,
		"transactions_last_24_hours":   tf.TransactionsLast24Hours,
		"transactions_last_7_days":     tf.TransactionsLast7Days,
		"amount_last_24_hours":         tf.AmountLast24Hours,
		"velocity_ratio":               tf.VelocityRatio,
		"has_country":                  tf.HasCountry,
		"is_new_country":               tf.IsNewCountry,
		"is_high_risk_country":         tf.IsHighRiskCountry,
		"is_new_merchant":              tf.IsNewMerchant,
		"hour_of_day":                  tf.HourOfDay,
		"day_of_week":                  tf.DayOfWeek,
		"is_business_hour":             tf.IsBusinessHour,
		"is_weekend":                   tf.IsWeekend,
		"is_night_time":                tf.IsNightTime,
		"is_holiday_season":            tf.IsHolidaySeason,
	}
}
