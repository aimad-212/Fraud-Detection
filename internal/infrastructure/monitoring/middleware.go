package monitoring

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// MetricsMiddleware creates Gin middleware for collecting HTTP metrics
func MetricsMiddleware(metrics *Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start)
		statusCode := c.Writer.Status()

		metrics.RecordAPIRequest(duration, statusCode)
	}
}

// HealthMiddleware creates middleware for health check endpoints
func HealthMiddleware(healthChecker *HealthChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		switch c.Request.URL.Path {
		case "/health":
			// Basic liveness check
			health := healthChecker.CheckLiveness(ctx)
			c.JSON(http.StatusOK, gin.H{
				"status":  health.Status,
				"message": health.Message,
			})
			c.Abort()
		case "/health/ready":
			// Readiness check
			health := healthChecker.CheckReadiness(ctx)
			statusCode := http.StatusOK
			if health.Status != "healthy" {
				statusCode = http.StatusServiceUnavailable
			}
			c.JSON(statusCode, health)
			c.Abort()
		case "/health/detailed":
			// Detailed health check
			health := healthChecker.CheckSystemHealth(ctx)
			statusCode := http.StatusOK
			if health.Status != "healthy" {
				statusCode = http.StatusServiceUnavailable
			}
			c.JSON(statusCode, health)
			c.Abort()
		default:
			c.Next()
		}
	}
}

// MetricsHandler returns an HTTP handler for Prometheus metrics
func MetricsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This will be handled by the Prometheus HTTP handler
		// The actual handler will be set up in main.go
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Metrics endpoint not configured"))
	})
}

// SystemMetricsCollector periodically collects system metrics
type SystemMetricsCollector struct {
	metrics *Metrics
	stop    chan struct{}
}

// NewSystemMetricsCollector creates a new system metrics collector
func NewSystemMetricsCollector(metrics *Metrics) *SystemMetricsCollector {
	return &SystemMetricsCollector{
		metrics: metrics,
		stop:    make(chan struct{}),
	}
}

// Start begins collecting system metrics
func (s *SystemMetricsCollector) Start() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		startTime := time.Now()

		for {
			select {
			case <-ticker.C:
				// Update uptime
				uptime := time.Since(startTime).Seconds()
				s.metrics.UpdateSystemMetrics(0, 0, uptime) // TODO: Add actual memory and connection tracking

			case <-s.stop:
				return
			}
		}
	}()
}

// Stop stops collecting system metrics
func (s *SystemMetricsCollector) Stop() {
	close(s.stop)
}

// ResponseWriter wraps http.ResponseWriter to capture status code
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rw *ResponseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

func (rw *ResponseWriter) Write(data []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	return rw.ResponseWriter.Write(data)
}

func (rw *ResponseWriter) StatusCode() int {
	return rw.statusCode
}

// PrometheusMiddleware creates middleware for Prometheus metrics collection
func PrometheusMiddleware(metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := NewResponseWriter(w)

			// Call the next handler
			next.ServeHTTP(rw, r)

			// Record metrics
			duration := time.Since(start)
			statusCode := rw.StatusCode()

			// Skip metrics for health and metrics endpoints to avoid noise
			if r.URL.Path != "/health" && r.URL.Path != "/metrics" && !isHealthEndpoint(r.URL.Path) {
				metrics.RecordAPIRequest(duration, statusCode)
			}
		})
	}
}

// isHealthEndpoint checks if the path is a health endpoint
func isHealthEndpoint(path string) bool {
	return path == "/health" || path == "/health/ready" || path == "/health/detailed"
}

// DatabaseMetricsMiddleware creates middleware for database metrics
func DatabaseMetricsMiddleware(metrics *Metrics) func(func() error) func() error {
	return func(operation func() error) func() error {
		return func() error {
			start := time.Now()
			err := operation()
			duration := time.Since(start)

			metrics.RecordDatabaseQuery(duration, err)
			return err
		}
	}
}

// MLMetricsMiddleware creates middleware for ML scoring metrics
func MLMetricsMiddleware(metrics *Metrics) func(func([]float32) (float64, error)) func([]float32) (float64, error) {
	return func(scoreFunc func([]float32) (float64, error)) func([]float32) (float64, error) {
		return func(features []float32) (float64, error) {
			start := time.Now()
			score, err := scoreFunc(features)
			duration := time.Since(start)

			if err != nil {
				metrics.RecordMLScoringError()
			} else {
				metrics.RecordMLScoring(duration, score)
			}

			return score, err
		}
	}
}
