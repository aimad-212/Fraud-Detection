package monitoring

import (
	"context"
	"database/sql"
	"time"

	"fraud-detection/internal/infrastructure/messaging"
	"fraud-detection/internal/infrastructure/ml"
	"fraud-detection/internal/repository"
)

// HealthChecker provides health check functionality for all system components
type HealthChecker struct {
	db          *sql.DB
	nats        *messaging.NATSPublisher
	modelScorer ml.ModelScorer
	repo        repository.TransactionRepository
}

// HealthStatus represents the health status of a component
type HealthStatus struct {
	Status  string                 `json:"status"` // "healthy", "unhealthy", "degraded"
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// SystemHealth represents the overall system health
type SystemHealth struct {
	Status     string                  `json:"status"`
	Timestamp  time.Time               `json:"timestamp"`
	Version    string                  `json:"version"`
	Uptime     time.Duration           `json:"uptime"`
	Components map[string]HealthStatus `json:"components"`
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(db *sql.DB, nats *messaging.NATSPublisher, modelScorer ml.ModelScorer, repo repository.TransactionRepository) *HealthChecker {
	return &HealthChecker{
		db:          db,
		nats:        nats,
		modelScorer: modelScorer,
		repo:        repo,
	}
}

// CheckSystemHealth performs comprehensive health checks
func (hc *HealthChecker) CheckSystemHealth(ctx context.Context) *SystemHealth {
	health := &SystemHealth{
		Status:     "healthy",
		Timestamp:  time.Now(),
		Version:    "1.0.0", // TODO: Make this configurable
		Components: make(map[string]HealthStatus),
	}

	// Check database
	health.Components["database"] = hc.checkDatabase(ctx)

	// Check NATS
	health.Components["nats"] = hc.checkNATS(ctx)

	// Check ML model
	health.Components["ml_model"] = hc.checkMLModel(ctx)

	// Check repository
	health.Components["repository"] = hc.checkRepository(ctx)

	// Determine overall status
	for _, component := range health.Components {
		if component.Status == "unhealthy" {
			health.Status = "unhealthy"
			break
		} else if component.Status == "degraded" && health.Status == "healthy" {
			health.Status = "degraded"
		}
	}

	return health
}

// checkDatabase checks database connectivity and performance
func (hc *HealthChecker) checkDatabase(ctx context.Context) HealthStatus {
	if hc.db == nil {
		return HealthStatus{
			Status:  "degraded",
			Message: "Database not configured (using in-memory storage)",
		}
	}

	// Test database connectivity
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	start := time.Now()
	err := hc.db.PingContext(ctx)
	duration := time.Since(start)

	if err != nil {
		return HealthStatus{
			Status:  "unhealthy",
			Message: "Database connection failed",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	// Check if response time is acceptable
	if duration > 2*time.Second {
		return HealthStatus{
			Status:  "degraded",
			Message: "Database response time is slow",
			Details: map[string]interface{}{
				"response_time_ms": duration.Milliseconds(),
			},
		}
	}

	return HealthStatus{
		Status:  "healthy",
		Message: "Database is healthy",
		Details: map[string]interface{}{
			"response_time_ms": duration.Milliseconds(),
		},
	}
}

// checkNATS checks NATS connectivity
func (hc *HealthChecker) checkNATS(ctx context.Context) HealthStatus {
	if hc.nats == nil {
		return HealthStatus{
			Status:  "degraded",
			Message: "NATS not configured",
		}
	}

	if !hc.nats.IsConnected() {
		return HealthStatus{
			Status:  "unhealthy",
			Message: "NATS connection is down",
		}
	}

	return HealthStatus{
		Status:  "healthy",
		Message: "NATS is connected",
	}
}

// checkMLModel checks ML model availability
func (hc *HealthChecker) checkMLModel(ctx context.Context) HealthStatus {
	if hc.modelScorer == nil {
		return HealthStatus{
			Status:  "degraded",
			Message: "ML model not configured",
		}
	}

	if !hc.modelScorer.IsLoaded() {
		return HealthStatus{
			Status:  "unhealthy",
			Message: "ML model is not loaded",
		}
	}

	// Test model scoring with a simple input
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	start := time.Now()
	_, err := hc.modelScorer.Score([]float32{100.0, 0.5, 0.0})
	duration := time.Since(start)

	if err != nil {
		return HealthStatus{
			Status:  "unhealthy",
			Message: "ML model scoring failed",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	if duration > 1*time.Second {
		return HealthStatus{
			Status:  "degraded",
			Message: "ML model scoring is slow",
			Details: map[string]interface{}{
				"response_time_ms": duration.Milliseconds(),
			},
		}
	}

	return HealthStatus{
		Status:  "healthy",
		Message: "ML model is healthy",
		Details: map[string]interface{}{
			"response_time_ms": duration.Milliseconds(),
		},
	}
}

// checkRepository checks repository functionality
func (hc *HealthChecker) checkRepository(ctx context.Context) HealthStatus {
	if hc.repo == nil {
		return HealthStatus{
			Status:  "unhealthy",
			Message: "Repository not configured",
		}
	}

	// Test basic repository operations
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	start := time.Now()

	// Try to list transactions (should work even if empty)
	_, err := hc.repo.ListTransactions(1, 0)
	duration := time.Since(start)

	if err != nil {
		return HealthStatus{
			Status:  "unhealthy",
			Message: "Repository query failed",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	if duration > 1*time.Second {
		return HealthStatus{
			Status:  "degraded",
			Message: "Repository query is slow",
			Details: map[string]interface{}{
				"response_time_ms": duration.Milliseconds(),
			},
		}
	}

	return HealthStatus{
		Status:  "healthy",
		Message: "Repository is healthy",
		Details: map[string]interface{}{
			"response_time_ms": duration.Milliseconds(),
		},
	}
}

// CheckReadiness checks if the system is ready to serve traffic
func (hc *HealthChecker) CheckReadiness(ctx context.Context) HealthStatus {
	health := hc.CheckSystemHealth(ctx)

	// Readiness requires all critical components to be healthy
	criticalComponents := []string{"database", "repository"}

	for _, component := range criticalComponents {
		if status, exists := health.Components[component]; exists {
			if status.Status == "unhealthy" {
				return HealthStatus{
					Status:  "unhealthy",
					Message: "Critical component is unhealthy",
					Details: map[string]interface{}{
						"component": component,
						"status":    status,
					},
				}
			}
		}
	}

	return HealthStatus{
		Status:  "healthy",
		Message: "System is ready",
	}
}

// CheckLiveness checks if the system is alive (basic liveness probe)
func (hc *HealthChecker) CheckLiveness(ctx context.Context) HealthStatus {
	// Liveness is simpler - just check if the process is running
	return HealthStatus{
		Status:  "healthy",
		Message: "Service is alive",
	}
}
