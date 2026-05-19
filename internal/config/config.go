package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"fraud-detection/internal/infrastructure/database"
	"fraud-detection/internal/infrastructure/messaging"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds server configuration
type ServerConfig struct {
	Port  int    `yaml:"port"`
	Mode  string `yaml:"mode"`
	Debug bool   `yaml:"debug"`
	Host  string `yaml:"host"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// FraudDetectionConfig holds fraud detection configuration
type FraudDetectionConfig struct {
	HighRiskThreshold          float64  `yaml:"high_risk_threshold"`
	MediumRiskThreshold        float64  `yaml:"medium_risk_threshold"`
	MaxAmountThreshold         float64  `yaml:"max_amount_threshold"`
	SuspiciousMerchants        []string `yaml:"suspicious_merchants"`
	EnableGeolocation          bool     `yaml:"enable_geolocation"`
	EnableDeviceFingerprinting bool     `yaml:"enable_device_fingerprinting"`
	EnableML                   bool     `yaml:"enable_ml"`
	ModelPath                  string   `yaml:"model_path"`
	MLConfig                   MLConfig `yaml:"ml"`
}

// MLConfig holds ML-specific configuration
type MLConfig struct {
	// Model settings
	ModelType       string `yaml:"model_type"` // onnx, mock, tensorflow
	ModelPath       string `yaml:"model_path"`
	ModelVersion    string `yaml:"model_version"`
	FallbackToRules bool   `yaml:"fallback_to_rules"`

	// Hybrid decision weights
	RuleWeight           float64 `yaml:"rule_weight"`            // Weight for rule-based decisions (0.0-1.0)
	MLWeight             float64 `yaml:"ml_weight"`              // Weight for ML decisions (0.0-1.0)
	EnableDynamicWeights bool    `yaml:"enable_dynamic_weights"` // Allow dynamic weight adjustment

	// Decision thresholds
	ApproveThreshold float64 `yaml:"approve_threshold"` // Score below this is approved (0.0-1.0)
	FlagThreshold    float64 `yaml:"flag_threshold"`    // Score above this is flagged (0.0-1.0)
	RejectThreshold  float64 `yaml:"reject_threshold"`  // Score above this is rejected (0.0-1.0)

	// Feature engineering
	FeatureVectorSize  int    `yaml:"feature_vector_size"` // Size of ML feature vector
	EnableFeatureCache bool   `yaml:"enable_feature_cache"`
	FeatureCacheTTL    string `yaml:"feature_cache_ttl"` // Cache TTL for features

	// Performance settings
	MaxConcurrentPredictions int    `yaml:"max_concurrent_predictions"`
	PredictionTimeout        string `yaml:"prediction_timeout"`
	EnableBatchPrediction    bool   `yaml:"enable_batch_prediction"`
	BatchSize                int    `yaml:"batch_size"`

	// Model monitoring
	EnableDriftDetection bool    `yaml:"enable_drift_detection"`
	DriftThreshold       float64 `yaml:"drift_threshold"`
	EnableModelAccuracy  bool    `yaml:"enable_model_accuracy"`
	AccuracyWindow       string  `yaml:"accuracy_window"`

	// A/B testing
	EnableABTesting       bool    `yaml:"enable_ab_testing"`
	ABTestTrafficSplit    float64 `yaml:"ab_test_traffic_split"` // 0.0-1.0
	ABTestControlWeight   float64 `yaml:"ab_test_control_weight"`
	ABTestTreatmentWeight float64 `yaml:"ab_test_treatment_weight"`
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	SecretKey          string        `yaml:"secret_key"`
	Issuer             string        `yaml:"issuer"`
	AccessTokenExpiry  time.Duration `yaml:"access_token_expiry"`
	RefreshTokenExpiry time.Duration `yaml:"refresh_token_expiry"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled bool `yaml:"enabled"`
	// Global rate limiting
	GlobalRequests int `yaml:"global_requests"`
	GlobalWindow   int `yaml:"global_window_seconds"`
	// Per IP rate limiting
	IPRequests int `yaml:"ip_requests"`
	IPWindow   int `yaml:"ip_window_seconds"`
	// Per user rate limiting
	UserRequests int `yaml:"user_requests"`
	UserWindow   int `yaml:"user_window_seconds"`
}

// AuditConfig holds audit logging configuration
type AuditConfig struct {
	Enabled         bool   `yaml:"enabled"`
	LogLevel        string `yaml:"log_level"`
	OutputFile      string `yaml:"output_file"`
	MaxFileSize     int    `yaml:"max_file_size_mb"`
	MaxBackups      int    `yaml:"max_backups"`
	MaxAge          int    `yaml:"max_age_days"`
	RotateOnStartup bool   `yaml:"rotate_on_startup"`
	// Events to audit
	AuditAuthEvents        bool `yaml:"audit_auth_events"`
	AuditTransactionEvents bool `yaml:"audit_transaction_events"`
	AuditAdminEvents       bool `yaml:"audit_admin_events"`
	AuditSecurityEvents    bool `yaml:"audit_security_events"`
	AuditDataAccess        bool `yaml:"audit_data_access"`
}

// ValidationConfig holds input validation configuration
type ValidationConfig struct {
	Enabled         bool `yaml:"enabled"`
	SanitizeEnabled bool `yaml:"sanitize_enabled"`
	MaxBodySize     int  `yaml:"max_body_size_bytes"`
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	Enabled        bool   `yaml:"enabled"`
	PrometheusPort int    `yaml:"prometheus_port"`
	MetricsPath    string `yaml:"metrics_path"`
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age_seconds"`
}

// SecurityConfig holds security headers configuration
type SecurityConfig struct {
	Enabled bool `yaml:"enabled"`
	// HSTS (HTTP Strict Transport Security)
	HSTSEnabled           bool `yaml:"hsts_enabled"`
	HSTSMaxAge            int  `yaml:"hsts_max_age_seconds"`
	HSTSIncludeSubdomains bool `yaml:"hsts_include_subdomains"`
	HSTSPreload           bool `yaml:"hsts_preload"`
	// CSP (Content Security Policy)
	CSPEnabled bool   `yaml:"csp_enabled"`
	CSPPolicy  string `yaml:"csp_policy"`
	// Standard security headers
	XFrameOptions       string `yaml:"x_frame_options"`
	XContentTypeOptions string `yaml:"x_content_type_options"`
	XXSSProtection      string `yaml:"x_xss_protection"`
	ReferrerPolicy      string `yaml:"referrer_policy"`
	// Advanced security headers
	PermissionsPolicy string `yaml:"permissions_policy"`
	// Cross-origin policies
	COEPEnabled    bool `yaml:"coep_enabled"`
	COEPReportOnly bool `yaml:"coep_report_only"`
	CORPEnabled    bool `yaml:"corp_enabled"`
	CORPReportOnly bool `yaml:"corp_report_only"`
	COOPEnabled    bool `yaml:"coop_enabled"`
	COOPReportOnly bool `yaml:"coop_report_only"`
	// Other security settings
	RemoveServerHeader  bool   `yaml:"remove_server_header"`
	CacheControlEnabled bool   `yaml:"cache_control_enabled"`
	CacheControlValue   string `yaml:"cache_control_value"`
}

// Config holds the complete application configuration
type Config struct {
	Server         ServerConfig         `yaml:"server"`
	Logging        LoggingConfig        `yaml:"logging"`
	Database       database.Config      `yaml:"database"`
	Redis          RedisConfig          `yaml:"redis"`
	Messaging      messaging.Config     `yaml:"nats"`
	FraudDetection FraudDetectionConfig `yaml:"fraud_detection"`
	Monitoring     MonitoringConfig     `yaml:"monitoring"`
	JWT            JWTConfig            `yaml:"jwt"`
	RateLimit      RateLimitConfig      `yaml:"rate_limit"`
	Audit          AuditConfig          `yaml:"audit"`
	Validation     ValidationConfig     `yaml:"validation"`
	CORS           CORSConfig           `yaml:"cors"`
	Security       SecurityConfig       `yaml:"security"`
}

// NewConfig creates a new config with defaults
func NewConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:  8080,
			Mode:  "release",
			Debug: false,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Database: *database.NewConfig(),
		Redis: RedisConfig{
			Enabled:  false,
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       0,
		},
		Messaging: *messaging.NewConfig(),
		FraudDetection: FraudDetectionConfig{
			HighRiskThreshold:          0.8,
			MediumRiskThreshold:        0.5,
			MaxAmountThreshold:         5000.0,
			SuspiciousMerchants:        []string{"suspicious-merchant", "high-risk-vendor"},
			EnableGeolocation:          false,
			EnableDeviceFingerprinting: false,
			EnableML:                   true,
			ModelPath:                  "./ml/models/fraud_model.onnx",
			MLConfig: MLConfig{
				ModelType:                "onnx",
				ModelPath:                "./ml/models/fraud_model.onnx",
				ModelVersion:             "1.0.0",
				FallbackToRules:          true,
				RuleWeight:               0.6,
				MLWeight:                 0.4,
				EnableDynamicWeights:     false,
				ApproveThreshold:         0.3,
				FlagThreshold:            0.7,
				RejectThreshold:          0.9,
				FeatureVectorSize:        20,
				EnableFeatureCache:       true,
				FeatureCacheTTL:          "300s",
				MaxConcurrentPredictions: 100,
				PredictionTimeout:        "1000ms",
				EnableBatchPrediction:    false,
				BatchSize:                10,
				EnableDriftDetection:     false,
				DriftThreshold:           0.1,
				EnableModelAccuracy:      true,
				AccuracyWindow:           "24h",
				EnableABTesting:          false,
				ABTestTrafficSplit:       0.1,
				ABTestControlWeight:      0.5,
				ABTestTreatmentWeight:    0.5,
			},
		},
		Monitoring: MonitoringConfig{
			Enabled:        false,
			PrometheusPort: 9090,
			MetricsPath:    "/metrics",
		},
		JWT: JWTConfig{
			SecretKey:          "your-secret-key-change-in-production",
			Issuer:             "fraud-detection-service",
			AccessTokenExpiry:  15 * time.Minute,
			RefreshTokenExpiry: 7 * 24 * time.Hour,
		},
		RateLimit: RateLimitConfig{
			Enabled:        false, // Disabled by default
			GlobalRequests: 1000,
			GlobalWindow:   3600, // 1 hour
			IPRequests:     100,
			IPWindow:       60, // 1 minute
			UserRequests:   500,
			UserWindow:     60, // 1 minute
		},
		Audit: AuditConfig{
			Enabled:                false, // Disabled by default
			LogLevel:               "info",
			OutputFile:             "logs/audit.log",
			MaxFileSize:            100, // 100MB
			MaxBackups:             10,
			MaxAge:                 30, // 30 days
			RotateOnStartup:        true,
			AuditAuthEvents:        true,
			AuditTransactionEvents: true,
			AuditAdminEvents:       true,
			AuditSecurityEvents:    true,
			AuditDataAccess:        false,
		},
		Validation: ValidationConfig{
			Enabled:         true,    // Enabled by default for security
			SanitizeEnabled: true,    // Sanitization enabled by default
			MaxBodySize:     1048576, // 1MB
		},
		CORS: CORSConfig{
			Enabled:          false, // Disabled by default for security
			AllowedOrigins:   []string{"https://localhost:3000", "https://localhost:8080"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
			ExposedHeaders:   []string{"Content-Length"},
			AllowCredentials: true,
			MaxAge:           86400, // 24 hours
		},
		Security: SecurityConfig{
			Enabled:               true,     // Enabled by default for security
			HSTSEnabled:           true,     // HSTS enabled by default for HTTPS
			HSTSMaxAge:            31536000, // 1 year
			HSTSIncludeSubdomains: false,
			HSTSPreload:           false,
			CSPEnabled:            true,
			CSPPolicy:             "default-src 'self'; script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; sandbox",
			XFrameOptions:         "DENY",
			XContentTypeOptions:   "nosniff",
			XXSSProtection:        "1; mode=block",
			ReferrerPolicy:        "strict-origin-when-cross-origin",
			PermissionsPolicy:     "geolocation=(), microphone=(), camera=()",
			COEPEnabled:           false,
			COEPReportOnly:        false,
			CORPEnabled:           false,
			CORPReportOnly:        false,
			COOPEnabled:           false,
			COOPReportOnly:        false,
			RemoveServerHeader:    true,
			CacheControlEnabled:   true,
			CacheControlValue:     "no-store, no-cache, must-revalidate, proxy-revalidate",
		},
	}
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := NewConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	return config, nil
}

// LoadFromEnv loads configuration overrides from environment variables
func (c *Config) LoadFromEnv() {
	// Server
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			c.Server.Port = p
		}
	}
	if mode := os.Getenv("GIN_MODE"); mode != "" {
		c.Server.Mode = mode
	}
	if host := os.Getenv("HOST"); host != "" {
		c.Server.Host = host
	}

	// Database
	if dbType := os.Getenv("DB_TYPE"); dbType != "" {
		c.Database.Type = dbType
	}
	if host := os.Getenv("DB_HOST"); host != "" {
		c.Database.Host = host
	}
	if port := os.Getenv("DB_PORT"); port != "" {
		// Parse port
		c.Database.Port = 5432 // Default
	}
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		c.Database.Database = dbName
	}
	if user := os.Getenv("DB_USER"); user != "" {
		c.Database.Username = user
	}
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		c.Database.Password = password
	}
	if sslMode := os.Getenv("DB_SSLMODE"); sslMode != "" {
		c.Database.SSLMode = sslMode
	}

	// NATS
	if enabled := os.Getenv("NATS_ENABLED"); enabled == "false" {
		c.Messaging.Enabled = false
	}
	if url := os.Getenv("NATS_URL"); url != "" {
		c.Messaging.URL = url
	}
	if subject := os.Getenv("NATS_SUBJECT"); subject != "" {
		c.Messaging.Subject = subject
	}
	if queueGroup := os.Getenv("NATS_QUEUE_GROUP"); queueGroup != "" {
		c.Messaging.QueueGroup = queueGroup
	}

	// Monitoring
	if enabled := os.Getenv("MONITORING_ENABLED"); enabled != "" {
		if enabled == "true" {
			c.Monitoring.Enabled = true
		} else if enabled == "false" {
			c.Monitoring.Enabled = false
		}
	}
	if port := os.Getenv("PROMETHEUS_PORT"); port != "" {
		// Parse port would be needed here
		c.Monitoring.PrometheusPort = 9090 // Default for now
	}
	if path := os.Getenv("METRICS_PATH"); path != "" {
		c.Monitoring.MetricsPath = path
	}

	// Fraud Detection ML settings
	if enabled := os.Getenv("ML_ENABLED"); enabled != "" {
		if enabled == "true" {
			c.FraudDetection.EnableML = true
		} else if enabled == "false" {
			c.FraudDetection.EnableML = false
		}
	}
	if modelPath := os.Getenv("ML_MODEL_PATH"); modelPath != "" {
		c.FraudDetection.ModelPath = modelPath
	}

	// ML Configuration settings
	if modelType := os.Getenv("ML_MODEL_TYPE"); modelType != "" {
		c.FraudDetection.MLConfig.ModelType = modelType
	}
	if mlModelPath := os.Getenv("ML_CONFIG_MODEL_PATH"); mlModelPath != "" {
		c.FraudDetection.MLConfig.ModelPath = mlModelPath
	}
	if modelVersion := os.Getenv("ML_MODEL_VERSION"); modelVersion != "" {
		c.FraudDetection.MLConfig.ModelVersion = modelVersion
	}
	if fallback := os.Getenv("ML_FALLBACK_TO_RULES"); fallback != "" {
		if fallback == "true" {
			c.FraudDetection.MLConfig.FallbackToRules = true
		} else if fallback == "false" {
			c.FraudDetection.MLConfig.FallbackToRules = false
		}
	}
	if ruleWeight := os.Getenv("ML_RULE_WEIGHT"); ruleWeight != "" {
		if weight, err := strconv.ParseFloat(ruleWeight, 64); err == nil {
			c.FraudDetection.MLConfig.RuleWeight = weight
		}
	}
	if mlWeight := os.Getenv("ML_WEIGHT"); mlWeight != "" {
		if weight, err := strconv.ParseFloat(mlWeight, 64); err == nil {
			c.FraudDetection.MLConfig.MLWeight = weight
		}
	}
	if dynamicWeights := os.Getenv("ML_ENABLE_DYNAMIC_WEIGHTS"); dynamicWeights != "" {
		if dynamicWeights == "true" {
			c.FraudDetection.MLConfig.EnableDynamicWeights = true
		} else if dynamicWeights == "false" {
			c.FraudDetection.MLConfig.EnableDynamicWeights = false
		}
	}
	if approveThreshold := os.Getenv("ML_APPROVE_THRESHOLD"); approveThreshold != "" {
		if threshold, err := strconv.ParseFloat(approveThreshold, 64); err == nil {
			c.FraudDetection.MLConfig.ApproveThreshold = threshold
		}
	}
	if flagThreshold := os.Getenv("ML_FLAG_THRESHOLD"); flagThreshold != "" {
		if threshold, err := strconv.ParseFloat(flagThreshold, 64); err == nil {
			c.FraudDetection.MLConfig.FlagThreshold = threshold
		}
	}
	if rejectThreshold := os.Getenv("ML_REJECT_THRESHOLD"); rejectThreshold != "" {
		if threshold, err := strconv.ParseFloat(rejectThreshold, 64); err == nil {
			c.FraudDetection.MLConfig.RejectThreshold = threshold
		}
	}
	if featureSize := os.Getenv("ML_FEATURE_VECTOR_SIZE"); featureSize != "" {
		if size, err := strconv.Atoi(featureSize); err == nil {
			c.FraudDetection.MLConfig.FeatureVectorSize = size
		}
	}
	if featureCache := os.Getenv("ML_ENABLE_FEATURE_CACHE"); featureCache != "" {
		if featureCache == "true" {
			c.FraudDetection.MLConfig.EnableFeatureCache = true
		} else if featureCache == "false" {
			c.FraudDetection.MLConfig.EnableFeatureCache = false
		}
	}
	if featureCacheTTL := os.Getenv("ML_FEATURE_CACHE_TTL"); featureCacheTTL != "" {
		c.FraudDetection.MLConfig.FeatureCacheTTL = featureCacheTTL
	}
	if maxPredictions := os.Getenv("ML_MAX_CONCURRENT_PREDICTIONS"); maxPredictions != "" {
		if max, err := strconv.Atoi(maxPredictions); err == nil {
			c.FraudDetection.MLConfig.MaxConcurrentPredictions = max
		}
	}
	if predictionTimeout := os.Getenv("ML_PREDICTION_TIMEOUT"); predictionTimeout != "" {
		c.FraudDetection.MLConfig.PredictionTimeout = predictionTimeout
	}
	if batchPrediction := os.Getenv("ML_ENABLE_BATCH_PREDICTION"); batchPrediction != "" {
		if batchPrediction == "true" {
			c.FraudDetection.MLConfig.EnableBatchPrediction = true
		} else if batchPrediction == "false" {
			c.FraudDetection.MLConfig.EnableBatchPrediction = false
		}
	}
	if batchSize := os.Getenv("ML_BATCH_SIZE"); batchSize != "" {
		if size, err := strconv.Atoi(batchSize); err == nil {
			c.FraudDetection.MLConfig.BatchSize = size
		}
	}
	if driftDetection := os.Getenv("ML_ENABLE_DRIFT_DETECTION"); driftDetection != "" {
		if driftDetection == "true" {
			c.FraudDetection.MLConfig.EnableDriftDetection = true
		} else if driftDetection == "false" {
			c.FraudDetection.MLConfig.EnableDriftDetection = false
		}
	}
	if driftThreshold := os.Getenv("ML_DRIFT_THRESHOLD"); driftThreshold != "" {
		if threshold, err := strconv.ParseFloat(driftThreshold, 64); err == nil {
			c.FraudDetection.MLConfig.DriftThreshold = threshold
		}
	}
	if modelAccuracy := os.Getenv("ML_ENABLE_MODEL_ACCURACY"); modelAccuracy != "" {
		if modelAccuracy == "true" {
			c.FraudDetection.MLConfig.EnableModelAccuracy = true
		} else if modelAccuracy == "false" {
			c.FraudDetection.MLConfig.EnableModelAccuracy = false
		}
	}
	if accuracyWindow := os.Getenv("ML_ACCURACY_WINDOW"); accuracyWindow != "" {
		c.FraudDetection.MLConfig.AccuracyWindow = accuracyWindow
	}
	if abTesting := os.Getenv("ML_ENABLE_AB_TESTING"); abTesting != "" {
		if abTesting == "true" {
			c.FraudDetection.MLConfig.EnableABTesting = true
		} else if abTesting == "false" {
			c.FraudDetection.MLConfig.EnableABTesting = false
		}
	}
	if trafficSplit := os.Getenv("ML_AB_TEST_TRAFFIC_SPLIT"); trafficSplit != "" {
		if split, err := strconv.ParseFloat(trafficSplit, 64); err == nil {
			c.FraudDetection.MLConfig.ABTestTrafficSplit = split
		}
	}
	if controlWeight := os.Getenv("ML_AB_TEST_CONTROL_WEIGHT"); controlWeight != "" {
		if weight, err := strconv.ParseFloat(controlWeight, 64); err == nil {
			c.FraudDetection.MLConfig.ABTestControlWeight = weight
		}
	}
	if treatmentWeight := os.Getenv("ML_AB_TEST_TREATMENT_WEIGHT"); treatmentWeight != "" {
		if weight, err := strconv.ParseFloat(treatmentWeight, 64); err == nil {
			c.FraudDetection.MLConfig.ABTestTreatmentWeight = weight
		}
	}

	// JWT settings
	if secretKey := os.Getenv("JWT_SECRET_KEY"); secretKey != "" {
		c.JWT.SecretKey = secretKey
	}
	if issuer := os.Getenv("JWT_ISSUER"); issuer != "" {
		c.JWT.Issuer = issuer
	}

	// Rate limiting settings
	if enabled := os.Getenv("RATE_LIMIT_ENABLED"); enabled != "" {
		if enabled == "true" {
			c.RateLimit.Enabled = true
		} else if enabled == "false" {
			c.RateLimit.Enabled = false
		}
	}
	if globalRequests := os.Getenv("RATE_LIMIT_GLOBAL_REQUESTS"); globalRequests != "" {
		if req, err := strconv.Atoi(globalRequests); err == nil {
			c.RateLimit.GlobalRequests = req
		}
	}
	if globalWindow := os.Getenv("RATE_LIMIT_GLOBAL_WINDOW"); globalWindow != "" {
		if window, err := strconv.Atoi(globalWindow); err == nil {
			c.RateLimit.GlobalWindow = window
		}
	}
	if ipRequests := os.Getenv("RATE_LIMIT_IP_REQUESTS"); ipRequests != "" {
		if req, err := strconv.Atoi(ipRequests); err == nil {
			c.RateLimit.IPRequests = req
		}
	}
	if ipWindow := os.Getenv("RATE_LIMIT_IP_WINDOW"); ipWindow != "" {
		if window, err := strconv.Atoi(ipWindow); err == nil {
			c.RateLimit.IPWindow = window
		}
	}
	if userRequests := os.Getenv("RATE_LIMIT_USER_REQUESTS"); userRequests != "" {
		if req, err := strconv.Atoi(userRequests); err == nil {
			c.RateLimit.UserRequests = req
		}
	}
	if userWindow := os.Getenv("RATE_LIMIT_USER_WINDOW"); userWindow != "" {
		if window, err := strconv.Atoi(userWindow); err == nil {
			c.RateLimit.UserWindow = window
		}
	}

	// Audit logging settings
	if enabled := os.Getenv("AUDIT_ENABLED"); enabled != "" {
		if enabled == "true" {
			c.Audit.Enabled = true
		} else if enabled == "false" {
			c.Audit.Enabled = false
		}
	}
	if logLevel := os.Getenv("AUDIT_LOG_LEVEL"); logLevel != "" {
		c.Audit.LogLevel = logLevel
	}
	if outputFile := os.Getenv("AUDIT_OUTPUT_FILE"); outputFile != "" {
		c.Audit.OutputFile = outputFile
	}
	if maxFileSize := os.Getenv("AUDIT_MAX_FILE_SIZE"); maxFileSize != "" {
		if size, err := strconv.Atoi(maxFileSize); err == nil {
			c.Audit.MaxFileSize = size
		}
	}
	if maxBackups := os.Getenv("AUDIT_MAX_BACKUPS"); maxBackups != "" {
		if backups, err := strconv.Atoi(maxBackups); err == nil {
			c.Audit.MaxBackups = backups
		}
	}
	if maxAge := os.Getenv("AUDIT_MAX_AGE"); maxAge != "" {
		if age, err := strconv.Atoi(maxAge); err == nil {
			c.Audit.MaxAge = age
		}
	}
	if rotateOnStartup := os.Getenv("AUDIT_ROTATE_ON_STARTUP"); rotateOnStartup != "" {
		if rotateOnStartup == "true" {
			c.Audit.RotateOnStartup = true
		} else if rotateOnStartup == "false" {
			c.Audit.RotateOnStartup = false
		}
	}
	if auditAuthEvents := os.Getenv("AUDIT_AUTH_EVENTS"); auditAuthEvents != "" {
		if auditAuthEvents == "true" {
			c.Audit.AuditAuthEvents = true
		} else if auditAuthEvents == "false" {
			c.Audit.AuditAuthEvents = false
		}
	}
	if auditTransactionEvents := os.Getenv("AUDIT_TRANSACTION_EVENTS"); auditTransactionEvents != "" {
		if auditTransactionEvents == "true" {
			c.Audit.AuditTransactionEvents = true
		} else if auditTransactionEvents == "false" {
			c.Audit.AuditTransactionEvents = false
		}
	}
	if auditAdminEvents := os.Getenv("AUDIT_ADMIN_EVENTS"); auditAdminEvents != "" {
		if auditAdminEvents == "true" {
			c.Audit.AuditAdminEvents = true
		} else if auditAdminEvents == "false" {
			c.Audit.AuditAdminEvents = false
		}
	}
	if auditSecurityEvents := os.Getenv("AUDIT_SECURITY_EVENTS"); auditSecurityEvents != "" {
		if auditSecurityEvents == "true" {
			c.Audit.AuditSecurityEvents = true
		} else if auditSecurityEvents == "false" {
			c.Audit.AuditSecurityEvents = false
		}
	}
	if auditDataAccess := os.Getenv("AUDIT_DATA_ACCESS"); auditDataAccess != "" {
		if auditDataAccess == "true" {
			c.Audit.AuditDataAccess = true
		} else if auditDataAccess == "false" {
			c.Audit.AuditDataAccess = false
		}
	}

	// Validation settings
	if enabled := os.Getenv("VALIDATION_ENABLED"); enabled != "" {
		if enabled == "true" {
			c.Validation.Enabled = true
		} else if enabled == "false" {
			c.Validation.Enabled = false
		}
	}
	if sanitizeEnabled := os.Getenv("VALIDATION_SANITIZE_ENABLED"); sanitizeEnabled != "" {
		if sanitizeEnabled == "true" {
			c.Validation.SanitizeEnabled = true
		} else if sanitizeEnabled == "false" {
			c.Validation.SanitizeEnabled = false
		}
	}
	if maxBodySize := os.Getenv("VALIDATION_MAX_BODY_SIZE"); maxBodySize != "" {
		if size, err := strconv.Atoi(maxBodySize); err == nil {
			c.Validation.MaxBodySize = size
		}
	}

	// CORS settings
	if corsEnabled := os.Getenv("CORS_ENABLED"); corsEnabled != "" {
		if corsEnabled == "true" {
			c.CORS.Enabled = true
		} else if corsEnabled == "false" {
			c.CORS.Enabled = false
		}
	}
	if allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS"); allowedOrigins != "" {
		c.CORS.AllowedOrigins = strings.Split(allowedOrigins, ",")
	}
	if allowedMethods := os.Getenv("CORS_ALLOWED_METHODS"); allowedMethods != "" {
		c.CORS.AllowedMethods = strings.Split(allowedMethods, ",")
	}
	if allowedHeaders := os.Getenv("CORS_ALLOWED_HEADERS"); allowedHeaders != "" {
		c.CORS.AllowedHeaders = strings.Split(allowedHeaders, ",")
	}
	if allowCredentials := os.Getenv("CORS_ALLOW_CREDENTIALS"); allowCredentials != "" {
		if allowCredentials == "true" {
			c.CORS.AllowCredentials = true
		} else if allowCredentials == "false" {
			c.CORS.AllowCredentials = false
		}
	}
	if maxAge := os.Getenv("CORS_MAX_AGE"); maxAge != "" {
		if age, err := strconv.Atoi(maxAge); err == nil {
			c.CORS.MaxAge = age
		}
	}

	// Security settings
	if enabled := os.Getenv("SECURITY_ENABLED"); enabled != "" {
		if enabled == "true" {
			c.Security.Enabled = true
		} else if enabled == "false" {
			c.Security.Enabled = false
		}
	}
	if hstsEnabled := os.Getenv("SECURITY_HSTS_ENABLED"); hstsEnabled != "" {
		if hstsEnabled == "true" {
			c.Security.HSTSEnabled = true
		} else if hstsEnabled == "false" {
			c.Security.HSTSEnabled = false
		}
	}
	if hstsMaxAge := os.Getenv("SECURITY_HSTS_MAX_AGE"); hstsMaxAge != "" {
		if age, err := strconv.Atoi(hstsMaxAge); err == nil {
			c.Security.HSTSMaxAge = age
		}
	}
	if hstsIncludeSubdomains := os.Getenv("SECURITY_HSTS_INCLUDE_SUBDOMAINS"); hstsIncludeSubdomains != "" {
		if hstsIncludeSubdomains == "true" {
			c.Security.HSTSIncludeSubdomains = true
		} else if hstsIncludeSubdomains == "false" {
			c.Security.HSTSIncludeSubdomains = false
		}
	}
	if hstsPreload := os.Getenv("SECURITY_HSTS_PRELOAD"); hstsPreload != "" {
		if hstsPreload == "true" {
			c.Security.HSTSPreload = true
		} else if hstsPreload == "false" {
			c.Security.HSTSPreload = false
		}
	}
	if cspEnabled := os.Getenv("SECURITY_CSP_ENABLED"); cspEnabled != "" {
		if cspEnabled == "true" {
			c.Security.CSPEnabled = true
		} else if cspEnabled == "false" {
			c.Security.CSPEnabled = false
		}
	}
	if cspPolicy := os.Getenv("SECURITY_CSP_POLICY"); cspPolicy != "" {
		c.Security.CSPPolicy = cspPolicy
	}
	if xFrameOptions := os.Getenv("SECURITY_X_FRAME_OPTIONS"); xFrameOptions != "" {
		c.Security.XFrameOptions = xFrameOptions
	}
	if xContentTypeOptions := os.Getenv("SECURITY_X_CONTENT_TYPE_OPTIONS"); xContentTypeOptions != "" {
		c.Security.XContentTypeOptions = xContentTypeOptions
	}
	if xXSSProtection := os.Getenv("SECURITY_X_XSS_PROTECTION"); xXSSProtection != "" {
		c.Security.XXSSProtection = xXSSProtection
	}
	if referrerPolicy := os.Getenv("SECURITY_REFERRER_POLICY"); referrerPolicy != "" {
		c.Security.ReferrerPolicy = referrerPolicy
	}
	if permissionsPolicy := os.Getenv("SECURITY_PERMISSIONS_POLICY"); permissionsPolicy != "" {
		c.Security.PermissionsPolicy = permissionsPolicy
	}
	if removeServerHeader := os.Getenv("SECURITY_REMOVE_SERVER_HEADER"); removeServerHeader != "" {
		if removeServerHeader == "true" {
			c.Security.RemoveServerHeader = true
		} else if removeServerHeader == "false" {
			c.Security.RemoveServerHeader = false
		}
	}
	if cacheControlEnabled := os.Getenv("SECURITY_CACHE_CONTROL_ENABLED"); cacheControlEnabled != "" {
		if cacheControlEnabled == "true" {
			c.Security.CacheControlEnabled = true
		} else if cacheControlEnabled == "false" {
			c.Security.CacheControlEnabled = false
		}
	}
	if cacheControlValue := os.Getenv("SECURITY_CACHE_CONTROL_VALUE"); cacheControlValue != "" {
		c.Security.CacheControlValue = cacheControlValue
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if err := c.Database.Validate(); err != nil {
		return fmt.Errorf("database config validation failed: %w", err)
	}

	if err := c.Messaging.Validate(); err != nil {
		return fmt.Errorf("messaging config validation failed: %w", err)
	}

	return nil
}
