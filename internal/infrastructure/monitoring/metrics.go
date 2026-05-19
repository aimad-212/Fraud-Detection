package monitoring

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the fraud detection service
type Metrics struct {
	// Transaction metrics
	TransactionsCreated   prometheus.Counter
	TransactionsProcessed prometheus.Counter
	TransactionsApproved  prometheus.Counter
	TransactionsFlagged   prometheus.Counter
	TransactionsRejected  prometheus.Counter
	TransactionsDeleted   prometheus.Counter

	// Processing metrics
	TransactionProcessingDuration prometheus.Histogram
	TransactionProcessingErrors   prometheus.Counter

	// Risk score metrics
	RiskScoreDistribution  prometheus.Histogram
	HighRiskTransactions   prometheus.Counter
	MediumRiskTransactions prometheus.Counter
	LowRiskTransactions    prometheus.Counter

	// NATS metrics
	NATSEventsPublished prometheus.Counter
	NATSEventsProcessed prometheus.Counter
	NATSErrors          prometheus.Counter

	// ML metrics
	MLModelScores     prometheus.Histogram
	MLScoringErrors   prometheus.Counter
	MLScoringDuration prometheus.Histogram

	// Hybrid Decision metrics
	HybridDecisionsTotal   prometheus.Counter
	HybridDecisionDuration prometheus.Histogram
	RuleScoresDistribution prometheus.Histogram
	MLScoresDistribution   prometheus.Histogram
	HybridWeightChanges    prometheus.Counter
	MLFallbacksTotal       prometheus.Counter
	MLModelVersionChanges  prometheus.Counter
	HybridDecisionAccuracy prometheus.Gauge
	MLVsRuleDecisions      *prometheus.CounterVec

	// API metrics
	APIRrequestsTotal  prometheus.Counter
	APIRequestDuration prometheus.Histogram
	APIRequestErrors   prometheus.Counter

	// Database metrics
	DatabaseQueriesTotal  prometheus.Counter
	DatabaseQueryErrors   prometheus.Counter
	DatabaseQueryDuration prometheus.Histogram

	// System metrics
	ActiveConnections prometheus.Gauge
	MemoryUsage       prometheus.Gauge
	Uptime            prometheus.Gauge
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	m := &Metrics{
		// Transaction counters
		TransactionsCreated: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_transactions_created_total",
			Help: "Total number of transactions created",
		}),
		TransactionsProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_transactions_processed_total",
			Help: "Total number of transactions processed",
		}),
		TransactionsApproved: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_transactions_approved_total",
			Help: "Total number of transactions approved",
		}),
		TransactionsFlagged: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_transactions_flagged_total",
			Help: "Total number of transactions flagged",
		}),
		TransactionsRejected: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_transactions_rejected_total",
			Help: "Total number of transactions rejected",
		}),
		TransactionsDeleted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_transactions_deleted_total",
			Help: "Total number of transactions deleted",
		}),

		// Processing metrics
		TransactionProcessingDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "fraud_detection_transaction_processing_duration_seconds",
			Help:    "Time spent processing transactions",
			Buckets: prometheus.DefBuckets,
		}),
		TransactionProcessingErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_transaction_processing_errors_total",
			Help: "Total number of transaction processing errors",
		}),

		// Risk score metrics
		RiskScoreDistribution: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "fraud_detection_risk_score_distribution",
			Help:    "Distribution of risk scores",
			Buckets: []float64{0.0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		}),
		HighRiskTransactions: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_high_risk_transactions_total",
			Help: "Total number of high-risk transactions (>0.8)",
		}),
		MediumRiskTransactions: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_medium_risk_transactions_total",
			Help: "Total number of medium-risk transactions (0.3-0.8)",
		}),
		LowRiskTransactions: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_low_risk_transactions_total",
			Help: "Total number of low-risk transactions (<0.3)",
		}),

		// NATS metrics
		NATSEventsPublished: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_nats_events_published_total",
			Help: "Total number of NATS events published",
		}),
		NATSEventsProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_nats_events_processed_total",
			Help: "Total number of NATS events processed",
		}),
		NATSErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_nats_errors_total",
			Help: "Total number of NATS errors",
		}),

		// ML metrics
		MLModelScores: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "fraud_detection_ml_model_scores",
			Help:    "Distribution of ML model scores",
			Buckets: []float64{0.0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		}),
		MLScoringErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_ml_scoring_errors_total",
			Help: "Total number of ML scoring errors",
		}),
		MLScoringDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "fraud_detection_ml_scoring_duration_seconds",
			Help:    "Time spent on ML scoring",
			Buckets: prometheus.DefBuckets,
		}),

		// Hybrid Decision metrics
		HybridDecisionsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_hybrid_decisions_total",
			Help: "Total number of hybrid decisions made",
		}),
		HybridDecisionDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "fraud_detection_hybrid_decision_duration_seconds",
			Help:    "Time spent making hybrid decisions",
			Buckets: prometheus.DefBuckets,
		}),
		RuleScoresDistribution: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "fraud_detection_rule_scores_distribution",
			Help:    "Distribution of rule-based scores",
			Buckets: []float64{0.0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		}),
		MLScoresDistribution: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "fraud_detection_ml_scores_distribution",
			Help:    "Distribution of ML model scores",
			Buckets: []float64{0.0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		}),
		HybridWeightChanges: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_hybrid_weight_changes_total",
			Help: "Total number of hybrid weight changes",
		}),
		MLFallbacksTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_ml_fallbacks_total",
			Help: "Total number of ML fallbacks to rules-only",
		}),
		MLModelVersionChanges: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_ml_model_version_changes_total",
			Help: "Total number of ML model version changes",
		}),
		HybridDecisionAccuracy: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "fraud_detection_hybrid_decision_accuracy",
			Help: "Current hybrid decision accuracy percentage",
		}),
		MLVsRuleDecisions: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "fraud_detection_ml_vs_rule_decisions_total",
			Help: "Comparison of ML vs rule decisions by outcome",
		}, []string{"decision_type", "ml_vs_rule"}),

		// API metrics
		APIRrequestsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_api_requests_total",
			Help: "Total number of API requests",
		}),
		APIRequestDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "fraud_detection_api_request_duration_seconds",
			Help:    "API request duration",
			Buckets: prometheus.DefBuckets,
		}),
		APIRequestErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_api_request_errors_total",
			Help: "Total number of API request errors",
		}),

		// Database metrics
		DatabaseQueriesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_database_queries_total",
			Help: "Total number of database queries",
		}),
		DatabaseQueryErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fraud_detection_database_query_errors_total",
			Help: "Total number of database query errors",
		}),
		DatabaseQueryDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "fraud_detection_database_query_duration_seconds",
			Help:    "Database query duration",
			Buckets: prometheus.DefBuckets,
		}),

		// System metrics
		ActiveConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "fraud_detection_active_connections",
			Help: "Number of active connections",
		}),
		MemoryUsage: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "fraud_detection_memory_usage_bytes",
			Help: "Memory usage in bytes",
		}),
		Uptime: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "fraud_detection_uptime_seconds",
			Help: "Service uptime in seconds",
		}),
	}

	return m
}

// RecordTransactionCreated records a transaction creation
func (m *Metrics) RecordTransactionCreated() {
	m.TransactionsCreated.Inc()
}

// RecordTransactionProcessed records a transaction processing with duration
func (m *Metrics) RecordTransactionProcessed(duration time.Duration, riskScore float64, status string) {
	m.TransactionsProcessed.Inc()
	m.TransactionProcessingDuration.Observe(duration.Seconds())
	m.RiskScoreDistribution.Observe(riskScore)

	switch status {
	case "approved":
		m.TransactionsApproved.Inc()
		m.LowRiskTransactions.Inc()
	case "flagged":
		m.TransactionsFlagged.Inc()
		m.MediumRiskTransactions.Inc()
	case "rejected":
		m.TransactionsRejected.Inc()
		m.HighRiskTransactions.Inc()
	}

	if riskScore > 0.8 {
		m.HighRiskTransactions.Inc()
	} else if riskScore > 0.3 {
		m.MediumRiskTransactions.Inc()
	} else {
		m.LowRiskTransactions.Inc()
	}
}

// RecordTransactionDeleted records a transaction deletion
func (m *Metrics) RecordTransactionDeleted() {
	m.TransactionsDeleted.Inc()
}

// RecordProcessingError records a transaction processing error
func (m *Metrics) RecordProcessingError() {
	m.TransactionProcessingErrors.Inc()
}

// RecordNATSEventPublished records a NATS event publication
func (m *Metrics) RecordNATSEventPublished() {
	m.NATSEventsPublished.Inc()
}

// RecordNATSEventProcessed records a NATS event processing
func (m *Metrics) RecordNATSEventProcessed() {
	m.NATSEventsProcessed.Inc()
}

// RecordNATSError records a NATS error
func (m *Metrics) RecordNATSError() {
	m.NATSErrors.Inc()
}

// RecordMLScoring records ML model scoring
func (m *Metrics) RecordMLScoring(duration time.Duration, score float64) {
	m.MLScoringDuration.Observe(duration.Seconds())
	m.MLModelScores.Observe(score)
}

// RecordMLScoringError records an ML scoring error
func (m *Metrics) RecordMLScoringError() {
	m.MLScoringErrors.Inc()
}

// RecordAPIRequest records an API request
func (m *Metrics) RecordAPIRequest(duration time.Duration, statusCode int) {
	m.APIRrequestsTotal.Inc()
	m.APIRequestDuration.Observe(duration.Seconds())

	if statusCode >= 400 {
		m.APIRequestErrors.Inc()
	}
}

// RecordDatabaseQuery records a database query
func (m *Metrics) RecordDatabaseQuery(duration time.Duration, err error) {
	m.DatabaseQueriesTotal.Inc()
	m.DatabaseQueryDuration.Observe(duration.Seconds())

	if err != nil {
		m.DatabaseQueryErrors.Inc()
	}
}

// RecordHybridDecision records a hybrid decision with detailed metrics
func (m *Metrics) RecordHybridDecision(duration time.Duration, ruleScore, mlScore, finalScore float64, decision string, mlEnabled bool) {
	m.HybridDecisionsTotal.Inc()
	m.HybridDecisionDuration.Observe(duration.Seconds())
	m.RuleScoresDistribution.Observe(ruleScore)
	m.MLScoresDistribution.Observe(mlScore)

	// Track ML vs Rule decision alignment
	if mlEnabled {
		mlDecision := m.scoreToDecision(mlScore)
		ruleDecision := m.scoreToDecision(ruleScore)

		if mlDecision == ruleDecision {
			m.MLVsRuleDecisions.WithLabelValues(decision, "aligned").Inc()
		} else {
			m.MLVsRuleDecisions.WithLabelValues(decision, "disagreed").Inc()
		}
	} else {
		m.MLVsRuleDecisions.WithLabelValues(decision, "rules_only").Inc()
	}
}

// RecordMLFallback records an ML fallback event
func (m *Metrics) RecordMLFallback() {
	m.MLFallbacksTotal.Inc()
}

// RecordHybridWeightChange records a hybrid weight change
func (m *Metrics) RecordHybridWeightChange(oldRuleWeight, oldMLWeight, newRuleWeight, newMLWeight float64) {
	m.HybridWeightChanges.Inc()
}

// RecordMLModelVersionChange records an ML model version change
func (m *Metrics) RecordMLModelVersionChange(oldVersion, newVersion string) {
	m.MLModelVersionChanges.Inc()
}

// UpdateHybridDecisionAccuracy updates the hybrid decision accuracy
func (m *Metrics) UpdateHybridDecisionAccuracy(accuracy float64) {
	m.HybridDecisionAccuracy.Set(accuracy)
}

// scoreToDecision converts a score to a decision string
func (m *Metrics) scoreToDecision(score float64) string {
	if score >= 0.7 {
		return "reject"
	} else if score >= 0.3 {
		return "flag"
	}
	return "approve"
}

// UpdateSystemMetrics updates system-level metrics
func (m *Metrics) UpdateSystemMetrics(connections int, memoryBytes int64, uptimeSeconds float64) {
	m.ActiveConnections.Set(float64(connections))
	m.MemoryUsage.Set(float64(memoryBytes))
	m.Uptime.Set(uptimeSeconds)
}
