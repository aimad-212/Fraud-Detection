package usecase

import (
	"fmt"
	"time"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/infrastructure/ml"
	"fraud-detection/internal/infrastructure/monitoring"
)

// DecisionService handles fraud detection decisions
type DecisionService struct {
	rulesEngine              domain.RuleEngine
	featureEngineer          *FeatureEngineer
	optimizedFeatureEngineer *ml.OptimizedFeatureEngineer
	mlModel                  ml.ModelScorer
	transactionRepo          TransactionRepository
	logger                   Logger
	metrics                  *monitoring.Metrics
	useOptimizedFeatures     bool
	enableML                 bool
	ruleWeight               float64
	mlWeight                 float64
}

// NewDecisionService creates a new decision service
func NewDecisionService(rulesEngine domain.RuleEngine, featureEngineer *FeatureEngineer, transactionRepo TransactionRepository, logger Logger) *DecisionService {
	return NewDecisionServiceWithML(rulesEngine, featureEngineer, transactionRepo, logger, nil, nil)
}

// NewDecisionServiceWithCache creates a new decision service with Redis cache
func NewDecisionServiceWithCache(rulesEngine domain.RuleEngine, featureEngineer *FeatureEngineer, transactionRepo TransactionRepository, logger Logger, redisCache ml.CacheInterface) *DecisionService {
	return NewDecisionServiceWithML(rulesEngine, featureEngineer, transactionRepo, logger, nil, redisCache)
}

// NewDecisionServiceWithML creates a new decision service with ML model integration
func NewDecisionServiceWithML(rulesEngine domain.RuleEngine, featureEngineer *FeatureEngineer, transactionRepo TransactionRepository, logger Logger, mlModel ml.ModelScorer, redisCache ml.CacheInterface) *DecisionService {
	return NewDecisionServiceWithMLAndMetrics(rulesEngine, featureEngineer, transactionRepo, logger, mlModel, redisCache, nil)
}

// NewDecisionServiceWithMLAndMetrics creates a new decision service with ML model integration and metrics
func NewDecisionServiceWithMLAndMetrics(rulesEngine domain.RuleEngine, featureEngineer *FeatureEngineer, transactionRepo TransactionRepository, logger Logger, mlModel ml.ModelScorer, redisCache ml.CacheInterface, metrics *monitoring.Metrics) *DecisionService {
	optimizedFeatureEngineer := ml.NewOptimizedFeatureEngineerWithCache(redisCache)

	return &DecisionService{
		rulesEngine:              rulesEngine,
		featureEngineer:          featureEngineer,
		optimizedFeatureEngineer: optimizedFeatureEngineer,
		mlModel:                  mlModel,
		transactionRepo:          transactionRepo,
		logger:                   logger,
		metrics:                  metrics,
		useOptimizedFeatures:     true,           // Enable optimized features by default
		enableML:                 mlModel != nil, // Enable ML if model is provided
		ruleWeight:               0.6,            // Default weights
		mlWeight:                 0.4,            // Default weights
	}
}

// UpdateRulesEngine updates the rules engine with new rules
func (ds *DecisionService) UpdateRulesEngine(newRulesEngine domain.RuleEngine) {
	ds.rulesEngine = newRulesEngine
	ds.logger.Info("Rules engine updated successfully")
}

// convertMLFeaturesToDomain converts ML features to domain features
func (ds *DecisionService) convertMLFeaturesToDomain(mlFeatures *ml.TransactionFeatures, transaction *domain.Transaction) *TransactionFeatures {
	return &TransactionFeatures{
		TransactionID: transaction.ID.String(),
		UserID:        transaction.UserID.String(),
		Amount:        float64(mlFeatures.Amount),
		Currency:      transaction.Currency,
		Merchant:      transaction.Merchant,
		Country:       transaction.Country,
		Timestamp:     transaction.CreatedAt,

		// Map ML features to domain features
		UserAverageAmount: float64(mlFeatures.UserAvgAmount),
		VelocityRatio:     float64(mlFeatures.UserVelocityScore),
		IsBusinessHour:    mlFeatures.IsBusinessHour == 1.0,
		IsWeekend:         mlFeatures.IsWeekend == 1.0,
		IsHighRiskCountry: mlFeatures.IsHighRiskCountry == 1.0,
		IsNewMerchant:     mlFeatures.IsSuspiciousMerchant == 1.0,

		// Additional mappings would go here
	}
}

// SetOptimizedFeatures enables or disables optimized feature engineering
func (ds *DecisionService) SetOptimizedFeatures(enabled bool) {
	ds.useOptimizedFeatures = enabled
	ds.logger.Info("Optimized features", "enabled", enabled)
}

// GetFeatureImportance returns feature importance from optimized engine
func (ds *DecisionService) GetFeatureImportance() []ml.FeatureImportance {
	if ds.optimizedFeatureEngineer != nil {
		return ds.optimizedFeatureEngineer.GetFeatureImportance()
	}
	return []ml.FeatureImportance{}
}

// GetCacheStats returns cache statistics from optimized engine
func (ds *DecisionService) GetCacheStats() map[string]interface{} {
	if ds.optimizedFeatureEngineer != nil {
		return ds.optimizedFeatureEngineer.GetCacheStats()
	}
	return map[string]interface{}{}
}

// EvaluateTransaction evaluates a transaction for fraud
func (ds *DecisionService) EvaluateTransaction(transaction *domain.Transaction) (*domain.DecisionResult, error) {
	startTime := time.Now()

	// Get user transaction history for context
	userTransactions, err := ds.transactionRepo.GetByUserID(transaction.UserID.String())
	if err != nil {
		ds.logger.Error("Failed to get user transactions for evaluation", "error", err, "user_id", transaction.UserID.String())
		// Continue with empty context
		userTransactions = []*domain.Transaction{}
	}

	// Create evaluation context
	context := &domain.EvaluationContext{
		UserTransactions: userTransactions,
		UserProfile:      ds.buildUserProfile(transaction.UserID.String(), userTransactions),
		IPAddress:        transaction.IPAddress,
		UserAgent:        transaction.UserAgent,
	}

	// Extract features (use optimized if available)
	var features *TransactionFeatures
	if ds.useOptimizedFeatures && ds.optimizedFeatureEngineer != nil {
		// Convert to ML features for optimization
		mlFeatures := ds.optimizedFeatureEngineer.EngineerFeaturesOptimized(transaction, context.UserTransactions)

		// Update global statistics
		ds.optimizedFeatureEngineer.UpdateGlobalStats(context.UserTransactions)

		// Convert back to domain features (simplified conversion)
		features = ds.convertMLFeaturesToDomain(mlFeatures, transaction)
	} else {
		features = ds.featureEngineer.ExtractFeatures(transaction, context)
	}

	// Evaluate rules
	violations := ds.rulesEngine.EvaluateTransaction(transaction, context)

	// Calculate rule-based risk score
	ruleScore := ds.calculateRuleScore(violations, features)

	// Calculate ML-based risk score if ML is enabled
	var mlScore float64
	var mlError error
	if ds.enableML && ds.mlModel != nil {
		// Convert features to ML vector
		featureVector := ds.convertFeaturesToMLVector(features)
		// Convert float64 to float32 for ML model
		featureVector32 := make([]float32, len(featureVector))
		for i, v := range featureVector {
			featureVector32[i] = float32(v)
		}
		mlScore, mlError = ds.mlModel.Score(featureVector32)
		if mlError != nil {
			ds.logger.Warn("ML model scoring failed, using rule-based only",
				"error", mlError,
				"transaction_id", transaction.ID)
			mlScore = 0.0 // Fallback to neutral score
			// Record ML fallback
			if ds.metrics != nil {
				ds.metrics.RecordMLFallback()
			}
		}
		// Record ML scoring metrics
		if ds.metrics != nil {
			ds.metrics.RecordMLScoring(time.Since(startTime), mlScore)
		}
	} else {
		mlScore = 0.0
	}

	// Calculate hybrid risk score
	riskScore := ds.calculateHybridScore(ruleScore, mlScore, features)

	// Make decision
	decision := ds.makeDecision(riskScore, violations)

	result := &domain.DecisionResult{
		TransactionID: transaction.ID,
		Decision:      decision,
		RiskScore:     riskScore,
		RuleScore:     ruleScore,
		MLScore:       mlScore,
		Violations:    violations,
		Features:      features.ToMap(),
		Explanation:   ds.generateHybridExplanation(decision, violations, features, ruleScore, mlScore),
		ProcessedAt:   time.Now(),
	}

	// Record hybrid decision metrics
	if ds.metrics != nil {
		ds.metrics.RecordHybridDecision(time.Since(startTime), ruleScore, mlScore, riskScore, string(decision), ds.enableML)
	}

	ds.logger.Info("Transaction evaluation completed",
		"transaction_id", transaction.ID,
		"decision", decision,
		"risk_score", riskScore,
		"violation_count", len(violations))

	return result, nil
}

// buildUserProfile builds a user profile from transaction history
func (ds *DecisionService) buildUserProfile(userID string, transactions []*domain.Transaction) *domain.UserProfile {
	if len(transactions) == 0 {
		return &domain.UserProfile{
			UserID:                   userID,
			AverageTransactionAmount: 0,
			TransactionCount:         0,
			RiskScore:                0.5, // Neutral risk for new users
		}
	}

	totalAmount := 0.0
	countries := make(map[string]bool)
	merchants := make(map[string]bool)
	lastTransaction := transactions[0].CreatedAt

	for _, tx := range transactions {
		totalAmount += tx.Amount
		if tx.Country != "" {
			countries[tx.Country] = true
		}
		if tx.Merchant != "" {
			merchants[tx.Merchant] = true
		}
		if tx.CreatedAt.After(lastTransaction) {
			lastTransaction = tx.CreatedAt
		}
	}

	averageAmount := totalAmount / float64(len(transactions))

	// Convert maps to slices
	preferredCountries := make([]string, 0, len(countries))
	for country := range countries {
		preferredCountries = append(preferredCountries, country)
	}

	preferredMerchants := make([]string, 0, len(merchants))
	for merchant := range merchants {
		preferredMerchants = append(preferredMerchants, merchant)
	}

	return &domain.UserProfile{
		UserID:                   userID,
		AverageTransactionAmount: averageAmount,
		TransactionCount:         len(transactions),
		LastTransactionTime:      lastTransaction,
		PreferredCountries:       preferredCountries,
		PreferredMerchants:       preferredMerchants,
		RiskScore:                0.1, // Low risk for established users
	}
}

// calculateRuleScore calculates rule-based risk score
func (ds *DecisionService) calculateRuleScore(violations []domain.RuleViolation, features *TransactionFeatures) float64 {
	// Base score from rule violations
	ruleScore := ds.rulesEngine.CalculateRiskScore(violations)

	// Additional risk factors from features
	featureScore := 0.0

	// High velocity increases risk
	if features.TransactionsLast1Hour > 3 {
		featureScore += 0.2
	}
	if features.TransactionsLast24Hours > 10 {
		featureScore += 0.3
	}

	// Unusual amounts increase risk
	if features.IsAmountUnusual {
		featureScore += 0.15
	}

	// New countries/merchants increase risk
	if features.IsNewCountry {
		featureScore += 0.1
	}
	if features.IsNewMerchant {
		featureScore += 0.05
	}

	// High-risk countries increase risk
	if features.IsHighRiskCountry {
		featureScore += 0.4
	}

	// Time factors
	if features.IsNightTime {
		featureScore += 0.05
	}

	// User risk score
	featureScore += features.UserRiskScore * 0.3

	// Combine scores (weighted average)
	totalScore := (ruleScore * 0.7) + (featureScore * 0.3)

	// Cap at 1.0
	if totalScore > 1.0 {
		totalScore = 1.0
	}

	return totalScore
}

// makeDecision makes the final approve/reject/flag decision
func (ds *DecisionService) makeDecision(riskScore float64, violations []domain.RuleViolation) domain.Decision {
	// Check for critical violations (automatic reject)
	for _, violation := range violations {
		if violation.Severity == domain.SeverityCritical {
			return domain.DecisionReject
		}
	}

	// Decision based on risk score
	switch {
	case riskScore >= 0.8:
		return domain.DecisionReject
	case riskScore >= 0.4:
		return domain.DecisionFlag
	default:
		return domain.DecisionApprove
	}
}

// generateExplanation generates a human-readable explanation
func (ds *DecisionService) generateExplanation(decision domain.Decision, violations []domain.RuleViolation, features *TransactionFeatures) string {
	explanation := fmt.Sprintf("Decision: %s (Risk Score: %.2f)", decision, features.UserRiskScore)

	if len(violations) > 0 {
		explanation += "\n\nRule Violations:"
		for _, v := range violations {
			explanation += fmt.Sprintf("\n- %s: %s", v.RuleName, v.Violation)
		}
	}

	// Add key feature insights
	explanation += "\n\nKey Factors:"
	if features.IsAmountUnusual {
		explanation += "\n- Unusual transaction amount"
	}
	if features.IsNewCountry {
		explanation += "\n- Transaction from new country"
	}
	if features.IsHighRiskCountry {
		explanation += "\n- Transaction from high-risk country"
	}
	if features.TransactionsLast1Hour > 3 {
		explanation += fmt.Sprintf("\n- High velocity: %d transactions in last hour", features.TransactionsLast1Hour)
	}

	return explanation
}

// convertFeaturesToMLVector converts transaction features to ML model input vector
func (ds *DecisionService) convertFeaturesToMLVector(features *TransactionFeatures) []float64 {
	// Create a comprehensive feature vector for ML model
	// This should match the expected input format of your trained ML model
	vector := make([]float64, 16) // Adjust size based on your model requirements

	// Basic transaction features
	vector[0] = features.Amount
	vector[1] = float64(features.TransactionsLast1Hour)
	vector[2] = float64(features.TransactionsLast24Hours)
	vector[3] = float64(features.TransactionsLast7Days)

	// User behavior features
	vector[4] = features.UserAverageAmount
	vector[5] = features.UserRiskScore
	vector[6] = features.VelocityRatio

	// Temporal features
	vector[7] = 0.0 // Hour of day (normalized)
	if features.IsBusinessHour {
		vector[7] = 0.5
	}
	vector[8] = 0.0 // Day of week (normalized)
	if features.IsWeekend {
		vector[8] = 1.0
	}
	vector[9] = 0.0 // Time of day
	if features.IsNightTime {
		vector[9] = 1.0
	}

	// Geographic features
	vector[10] = 0.0 // Country risk
	if features.IsHighRiskCountry {
		vector[10] = 1.0
	}
	vector[11] = 0.0 // New country
	if features.IsNewCountry {
		vector[11] = 1.0
	}

	// Merchant features
	vector[12] = 0.0 // Merchant risk
	if features.IsNewMerchant {
		vector[12] = 1.0
	}

	// Amount features
	vector[13] = 0.0 // Amount unusual
	if features.IsAmountUnusual {
		vector[13] = 1.0
	}

	// Additional derived features
	vector[14] = features.Amount / (features.UserAverageAmount + 1.0) // Amount ratio
	vector[15] = 0.0                                                  // Geographic diversity (placeholder - would need additional data)

	return vector
}

// calculateHybridScore combines rule and ML scores using weighted approach
func (ds *DecisionService) calculateHybridScore(ruleScore, mlScore float64, features *TransactionFeatures) float64 {
	// Dynamic weight adjustment based on data quality and confidence
	ruleWeight := ds.ruleWeight
	mlWeight := ds.mlWeight

	// Adjust weights based on ML model confidence
	if ds.enableML && ds.mlModel != nil {
		// If ML model is available but score is very low confidence, rely more on rules
		if mlScore < 0.1 || mlScore > 0.9 {
			ruleWeight += 0.1
			mlWeight -= 0.1
		}
	}

	// Adjust weights based on user history
	if features.TransactionsLast7Days < 5 {
		// New users - rely more on rules
		ruleWeight += 0.1
		mlWeight -= 0.1
	}

	// Normalize weights
	totalWeight := ruleWeight + mlWeight
	if totalWeight > 0 {
		ruleWeight /= totalWeight
		mlWeight /= totalWeight
	}

	// Calculate hybrid score
	hybridScore := (ruleScore * ruleWeight) + (mlScore * mlWeight)

	// Apply additional risk factors
	featureScore := 0.0

	// High velocity increases risk
	if features.TransactionsLast1Hour > 3 {
		featureScore += 0.1
	}

	// Unusual amounts increase risk
	if features.IsAmountUnusual {
		featureScore += 0.1
	}

	// High-risk countries increase risk
	if features.IsHighRiskCountry {
		featureScore += 0.2
	}

	// Combine with hybrid score
	finalScore := hybridScore + featureScore

	// Cap at 1.0
	if finalScore > 1.0 {
		finalScore = 1.0
	}

	return finalScore
}

// generateHybridExplanation generates explanation for hybrid decision
func (ds *DecisionService) generateHybridExplanation(decision domain.Decision, violations []domain.RuleViolation, features *TransactionFeatures, ruleScore, mlScore float64) string {
	explanation := fmt.Sprintf("Decision: %s (Overall Risk: %.2f)", decision, ruleScore+mlScore)

	// Add scoring breakdown
	if ds.enableML {
		explanation += fmt.Sprintf("\n\nScoring Breakdown:")
		explanation += fmt.Sprintf("\n- Rule-based Score: %.2f (%.1f%% weight)", ruleScore, ds.ruleWeight*100)
		explanation += fmt.Sprintf("\n- ML Model Score: %.2f (%.1f%% weight)", mlScore, ds.mlWeight*100)
	}

	if len(violations) > 0 {
		explanation += "\n\nRule Violations:"
		for _, v := range violations {
			explanation += fmt.Sprintf("\n- %s: %s (Severity: %s)", v.RuleName, v.Violation, v.Severity)
		}
	}

	// Add key feature insights
	explanation += "\n\nKey Factors:"
	if features.IsAmountUnusual {
		explanation += fmt.Sprintf("\n- Unusual amount: $%.2f (user avg: $%.2f)", features.Amount, features.UserAverageAmount)
	}
	if features.IsNewCountry {
		explanation += fmt.Sprintf("\n- Transaction from new country: %s", features.Country)
	}
	if features.IsHighRiskCountry {
		explanation += fmt.Sprintf("\n- High-risk country: %s", features.Country)
	}
	if features.TransactionsLast1Hour > 3 {
		explanation += fmt.Sprintf("\n- High velocity: %d transactions in last hour", features.TransactionsLast1Hour)
	}
	if features.TransactionsLast24Hours > 10 {
		explanation += fmt.Sprintf("\n- High daily volume: %d transactions in 24 hours", features.TransactionsLast24Hours)
	}

	// Add ML insights if available
	if ds.enableML && mlScore > 0 {
		explanation += fmt.Sprintf("\n- ML Model Confidence: %.1f%%", mlScore*100)
		if mlScore > 0.7 {
			explanation += " (High fraud risk detected by ML)"
		} else if mlScore < 0.3 {
			explanation += " (Low fraud risk according to ML)"
		}
	}

	return explanation
}

// SetMLWeights allows dynamic adjustment of rule/ML weights
func (ds *DecisionService) SetMLWeights(ruleWeight, mlWeight float64) {
	oldRuleWeight := ds.ruleWeight
	oldMLWeight := ds.mlWeight

	total := ruleWeight + mlWeight
	if total > 0 {
		ds.ruleWeight = ruleWeight / total
		ds.mlWeight = mlWeight / total
	}

	// Record weight change metrics
	if ds.metrics != nil {
		ds.metrics.RecordHybridWeightChange(oldRuleWeight, oldMLWeight, ds.ruleWeight, ds.mlWeight)
	}

	ds.logger.Info("ML weights updated",
		"rule_weight", ds.ruleWeight,
		"ml_weight", ds.mlWeight)
}

// EnableML enables or disables ML model integration
func (ds *DecisionService) EnableML(enabled bool) {
	ds.enableML = enabled && ds.mlModel != nil
	ds.logger.Info("ML integration", "enabled", ds.enableML)
}

// GetMLStatus returns current ML integration status
func (ds *DecisionService) GetMLStatus() map[string]interface{} {
	return map[string]interface{}{
		"ml_enabled":   ds.enableML,
		"ml_available": ds.mlModel != nil,
		"rule_weight":  ds.ruleWeight,
		"ml_weight":    ds.mlWeight,
	}
}

// Interfaces for dependency injection
type TransactionRepository interface {
	GetByUserID(userID string) ([]*domain.Transaction, error)
}

type Logger interface {
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Info(msg string, args ...interface{})
}
