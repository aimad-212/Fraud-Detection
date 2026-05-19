package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"fraud-detection/internal/config"
	httpHandler "fraud-detection/internal/delivery/http"
	"fraud-detection/internal/domain"
	"fraud-detection/internal/infrastructure/audit"
	"fraud-detection/internal/infrastructure/auth"
	"fraud-detection/internal/infrastructure/cache"
	"fraud-detection/internal/infrastructure/messaging"
	"fraud-detection/internal/infrastructure/ml"
	"fraud-detection/internal/infrastructure/monitoring"
	"fraud-detection/internal/infrastructure/ratelimit"
	"fraud-detection/internal/infrastructure/validation"
	"fraud-detection/internal/repository"
	"fraud-detection/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// usecaseLoggerAdapter adapts zap.Logger to usecase.Logger interface
type usecaseLoggerAdapter struct {
	logger *zap.Logger
}

func (l *usecaseLoggerAdapter) Error(msg string, args ...interface{}) {
	l.logger.Error(msg, zap.Any("args", args))
}

func (l *usecaseLoggerAdapter) Warn(msg string, args ...interface{}) {
	l.logger.Warn(msg, zap.Any("args", args))
}

func (l *usecaseLoggerAdapter) Info(msg string, args ...interface{}) {
	l.logger.Info(msg, zap.Any("args", args))
}

func main() {
	fmt.Println("Starting application...")

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting Fraud Detection Service")

	// Load configuration
	appConfig := config.NewConfig()

	// Try to load from config file
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "configs/config.yaml"
	}

	if cfg, err := config.LoadFromFile(configFile); err != nil {
		logger.Warn("Failed to load config file, using defaults", zap.Error(err))
	} else {
		appConfig = cfg
	}

	// Override with environment variables
	appConfig.LoadFromEnv()

	// Validate configuration
	if err := appConfig.Validate(); err != nil {
		logger.Fatal("Configuration validation failed", zap.Error(err))
	}

	logger.Info("Configuration loaded successfully")

	// Initialize monitoring if enabled
	var metrics *monitoring.Metrics
	var healthChecker *monitoring.HealthChecker
	var systemMetricsCollector *monitoring.SystemMetricsCollector

	if appConfig.Monitoring.Enabled {
		// Initialize metrics
		metrics = monitoring.NewMetrics()
		logger.Info("Initialized Prometheus metrics")

		// Initialize system metrics collector
		systemMetricsCollector = monitoring.NewSystemMetricsCollector(metrics)
		systemMetricsCollector.Start()
		logger.Info("Started system metrics collector")

		// Health checker will be initialized after other components
	}

	// Initialize repository based on database type
	var transactionRepo repository.TransactionRepository
	if appConfig.Database.IsMemory() {
		transactionRepo = repository.NewInMemoryTransactionRepository()
		logger.Info("Initialized in-memory transaction repository")
	} else {
		db, err := appConfig.Database.Connect()
		if err != nil {
			logger.Fatal("Failed to connect to database", zap.Error(err))
		}

		// Run migrations
		gormRepo := repository.NewGORMTransactionRepository(db)
		if err := gormRepo.AutoMigrate(); err != nil {
			logger.Fatal("Failed to run database migrations", zap.Error(err))
		}

		transactionRepo = gormRepo
		logger.Info("Initialized PostgreSQL transaction repository")

		// Set metrics on repository if monitoring is enabled
		if appConfig.Monitoring.Enabled && metrics != nil {
			if gormRepo, ok := transactionRepo.(*repository.GORMTransactionRepository); ok {
				gormRepo.SetMetrics(metrics)
				logger.Info("Set metrics on GORM repository")
			}
		}
	}

	// Initialize NATS messaging if enabled
	var publisher usecase.TransactionPublisher
	var subscriber *messaging.NATSSubscriber

	if appConfig.Messaging.Enabled {
		// Initialize NATS publisher
		natsPublisher, err := messaging.NewNATSPublisher(
			appConfig.Messaging.URL,
			appConfig.Messaging.Subject,
			logger,
		)
		if err != nil {
			logger.Fatal("Failed to initialize NATS publisher", zap.Error(err))
		}
		publisher = natsPublisher
		logger.Info("Initialized NATS publisher")

		// Initialize transaction event processor
		eventProcessor := usecase.NewTransactionEventProcessor(transactionRepo, logger)

		// Initialize NATS subscriber
		natsSubscriber, err := messaging.NewNATSSubscriber(
			appConfig.Messaging.URL,
			appConfig.Messaging.Subject,
			appConfig.Messaging.QueueGroup,
			eventProcessor,
			logger,
		)
		if err != nil {
			logger.Fatal("Failed to initialize NATS subscriber", zap.Error(err))
		}
		subscriber = natsSubscriber

		// Start subscriber
		if err := subscriber.Start(); err != nil {
			logger.Fatal("Failed to start NATS subscriber", zap.Error(err))
		}
		logger.Info("Started NATS subscriber")

		// Ensure cleanup on exit for NATS components
		defer func() {
			if publisher != nil {
				if p, ok := publisher.(*messaging.NATSPublisher); ok {
					p.Close()
				}
			}
			if subscriber != nil {
				subscriber.Stop()
			}
		}()
	} else {
		logger.Info("NATS messaging disabled")
	}

	// Initialize ML model scorer if enabled
	var modelScorer ml.ModelScorer
	if appConfig.FraudDetection.EnableML {
		logger.Info("Initializing ML model scorer")
		modelType := appConfig.FraudDetection.MLConfig.ModelType
		if modelType == "" {
			modelType = "onnx" // default
		}

		scorer, err := ml.ModelScorerFactory(appConfig.FraudDetection.MLConfig.ModelPath, modelType)
		if err != nil {
			logger.Warn("Failed to initialize model scorer, using mock scorer",
				zap.String("model_type", modelType),
				zap.String("model_path", appConfig.FraudDetection.MLConfig.ModelPath),
				zap.Error(err))
			modelScorer = ml.NewMockModelScorer(0.3) // Default moderate risk
		} else {
			modelScorer = scorer
			logger.Info("Initialized model scorer",
				zap.String("model_type", modelType),
				zap.String("model_path", appConfig.FraudDetection.MLConfig.ModelPath))
		}
	} else {
		logger.Info("ML model scoring disabled")
	}

	// Initialize Redis cache if enabled
	var redisCache *cache.RedisCache
	if appConfig.Redis.Enabled {
		logger.Info("Initializing Redis cache")
		redisAddr := fmt.Sprintf("%s:%d", appConfig.Redis.Host, appConfig.Redis.Port)
		redisCacheInstance, err := cache.NewRedisCache(redisAddr, appConfig.Redis.Password, appConfig.Redis.DB)
		if err != nil {
			logger.Error("Failed to initialize Redis cache", zap.Error(err))
		} else {
			redisCache = redisCacheInstance
			logger.Info("Redis cache initialized successfully")
		}
	} else {
		logger.Info("Redis cache disabled")
	}

	// Initialize decision service and rule loader if ML is enabled
	var decisionService *usecase.DecisionService
	var ruleLoader *usecase.RuleLoader
	if modelScorer != nil {
		// Initialize feature engineer (use usecase version, not ml version)
		featureEngineer := usecase.NewFeatureEngineer()

		// Initialize rule loader with hot-reload support
		rulesFile := "configs/rules.yaml"
		if _, err := os.Stat(rulesFile); os.IsNotExist(err) {
			// Create default rules file if it doesn't exist
			logger.Warn("Rules file not found, creating default rules file", zap.String("file", rulesFile))
			tempRuleLoader := usecase.NewRuleLoader()
			defaultRules := tempRuleLoader.CreateDefaultRules()
			if err := tempRuleLoader.SaveRulesToFile(rulesFile, defaultRules); err != nil {
				logger.Error("Failed to create default rules file", zap.Error(err))
				// Fall back to in-memory rules
				rules := defaultRules
				rulesEngine := usecase.NewRulesEngine(rules)
				loggerAdapter := &usecaseLoggerAdapter{logger: logger}
				decisionService = usecase.NewDecisionServiceWithCache(rulesEngine, featureEngineer, transactionRepo, loggerAdapter, redisCache)
				logger.Info("Initialized decision service with default rules")
			} else {
				logger.Info("Created default rules file", zap.String("file", rulesFile))
			}
		}

		// Create rule reload callback
		onRulesReload := func(rules []*domain.Rule) {
			logger.Info("Rules reloaded", zap.Int("rule_count", len(rules)))
			// Update the rules engine in the decision service
			if decisionService != nil {
				newRulesEngine := usecase.NewRulesEngine(rules)
				decisionService.UpdateRulesEngine(newRulesEngine)
			}
		}

		// Initialize rule loader with file watching
		var err error
		ruleLoader, err = usecase.NewRuleLoaderWithFile(rulesFile, onRulesReload)
		if err != nil {
			logger.Error("Failed to initialize rule loader with file watching, using default rules", zap.Error(err))
			// Fall back to default rules
			tempRuleLoader := usecase.NewRuleLoader()
			rules := tempRuleLoader.CreateDefaultRules()
			rulesEngine := usecase.NewRulesEngine(rules)
			loggerAdapter := &usecaseLoggerAdapter{logger: logger}
			decisionService = usecase.NewDecisionServiceWithMLAndMetrics(rulesEngine, featureEngineer, transactionRepo, loggerAdapter, modelScorer, redisCache, metrics)
			logger.Info("Initialized decision service with default rules and ML integration")
		} else {
			rules := ruleLoader.GetRules()
			rulesEngine := usecase.NewRulesEngine(rules)
			loggerAdapter := &usecaseLoggerAdapter{logger: logger}
			var redisCacheToPass ml.CacheInterface
			if appConfig.Redis.Enabled && redisCache != nil {
				redisCacheToPass = redisCache
			}
			decisionService = usecase.NewDecisionServiceWithMLAndMetrics(rulesEngine, featureEngineer, transactionRepo, loggerAdapter, modelScorer, redisCacheToPass, metrics)
			logger.Info("Initialized decision service with hot-reload rules and ML integration",
				zap.String("rules_file", rulesFile),
				zap.Int("rule_count", len(rules)),
				zap.Bool("ml_enabled", modelScorer != nil))
		}
	}

	// Initialize use case
	transactionUseCase := usecase.NewTransactionUseCase(transactionRepo, publisher, logger)

	// Set decision service if available
	if decisionService != nil {
		if uc, ok := transactionUseCase.(*usecase.TransactionUseCaseImpl); ok {
			uc.SetDecisionService(decisionService)
			logger.Info("Set decision service on transaction use case")
		}
	}

	// Set metrics on use case if monitoring is enabled
	if appConfig.Monitoring.Enabled && metrics != nil {
		if uc, ok := transactionUseCase.(*usecase.TransactionUseCaseImpl); ok {
			uc.SetMetrics(metrics)
			logger.Info("Set metrics on transaction use case")
		}
	}

	logger.Info("Initialized transaction use case")

	// Initialize JWT authentication service
	jwtConfig := auth.JWTConfig{
		SecretKey:          appConfig.JWT.SecretKey,
		Issuer:             appConfig.JWT.Issuer,
		AccessTokenExpiry:  appConfig.JWT.AccessTokenExpiry,
		RefreshTokenExpiry: appConfig.JWT.RefreshTokenExpiry,
	}
	jwtService, err := auth.NewJWTService(jwtConfig)
	if err != nil {
		logger.Fatal("Failed to initialize JWT service", zap.Error(err))
	}
	logger.Info("Initialized JWT authentication service")

	// Initialize user repository and service
	userRepo := repository.NewInMemoryUserRepository()
	loggerAdapter := &usecaseLoggerAdapter{logger: logger}
	userService := usecase.NewUserService(userRepo, jwtService, loggerAdapter)

	// Create default admin user if no users exist
	if err := userRepo.CreateDefaultAdmin(); err != nil {
		logger.Warn("Failed to create default admin user", zap.Error(err))
	} else {
		logger.Info("Created default admin user")
	}

	// Ensure cleanup on exit for all components
	defer func() {
		if systemMetricsCollector != nil {
			systemMetricsCollector.Stop()
		}
		if ruleLoader != nil {
			ruleLoader.Close()
		}
	}()

	// Initialize rate limiting if enabled
	var rateLimitMiddleware *httpHandler.RateLimitMiddleware
	if appConfig.RateLimit.Enabled {
		var redisClient *redis.Client
		if appConfig.Redis.Enabled {
			// Create Redis client for rate limiting
			redisAddr := fmt.Sprintf("%s:%d", appConfig.Redis.Host, appConfig.Redis.Port)
			redisClient = redis.NewClient(&redis.Options{
				Addr:     redisAddr,
				Password: appConfig.Redis.Password,
				DB:       appConfig.Redis.DB,
			})
		}

		rateLimiter := ratelimit.NewRateLimiter(redisClient, "fraud-detection")
		rateLimitMiddleware = httpHandler.NewRateLimitMiddleware(rateLimiter)

		// Configure rate limits for different endpoint types
		rateLimitMiddleware.AddEndpointConfig("/api/v1/auth", httpHandler.AuthEndpoints)
		rateLimitMiddleware.AddEndpointConfig("/api/v1/transactions", httpHandler.TransactionEndpoints)
		rateLimitMiddleware.AddEndpointConfig("/api/v1/users", httpHandler.AdminEndpoints)

		logger.Info("Initialized rate limiting middleware")
	} else {
		logger.Info("Rate limiting disabled")
	}

	// Initialize audit logging if enabled
	var auditLogger audit.AuditLogger
	auditConfig := audit.Config{
		Enabled:                appConfig.Audit.Enabled,
		LogLevel:               appConfig.Audit.LogLevel,
		OutputFile:             appConfig.Audit.OutputFile,
		MaxFileSize:            appConfig.Audit.MaxFileSize,
		MaxBackups:             appConfig.Audit.MaxBackups,
		MaxAge:                 appConfig.Audit.MaxAge,
		RotateOnStartup:        appConfig.Audit.RotateOnStartup,
		AuditAuthEvents:        appConfig.Audit.AuditAuthEvents,
		AuditTransactionEvents: appConfig.Audit.AuditTransactionEvents,
		AuditAdminEvents:       appConfig.Audit.AuditAdminEvents,
		AuditSecurityEvents:    appConfig.Audit.AuditSecurityEvents,
		AuditDataAccess:        appConfig.Audit.AuditDataAccess,
	}

	auditLogger, err = audit.NewAuditLogger(auditConfig)
	if err != nil {
		logger.Fatal("Failed to initialize audit logger", zap.Error(err))
	}
	defer auditLogger.Close()

	if appConfig.Audit.Enabled {
		logger.Info("Initialized audit logging",
			zap.String("output_file", appConfig.Audit.OutputFile),
			zap.String("log_level", appConfig.Audit.LogLevel))
	} else {
		logger.Info("Audit logging disabled")
	}

	// Initialize model manager and handler
	var modelHandler *httpHandler.ModelHandler
	var modelRepo repository.ModelRepository

	// Initialize model repository (using in-memory for now, can be extended to use database)
	modelRepo = repository.NewInMemoryModelRepository()
	logger.Info("Initialized in-memory model repository")

	// Create models directory for file storage
	modelsDir := "models"
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		logger.Error("Failed to create models directory", zap.Error(err))
	} else {
		logger.Info("Created models directory", zap.String("path", modelsDir))
	}

	// Initialize model manager
	modelManager := usecase.NewModelManager(modelRepo, logger, modelsDir)
	modelHandler = httpHandler.NewModelHandler(modelManager, logger)
	logger.Info("Initialized model manager and handler")

	// DEBUG: Check if model handler is initialized
	if modelHandler == nil {
		logger.Fatal("Model handler is nil!")
	} else {
		logger.Info("Model handler successfully initialized")
	}

	// Initialize HTTP handlers
	transactionHandler := httpHandler.NewTransactionHandler(transactionUseCase, logger)
	userHandler := httpHandler.NewUserHandler(userService, loggerAdapter)
	authMiddleware := httpHandler.NewAuthMiddleware(jwtService)
	logger.Info("Initialized HTTP handlers and authentication middleware")

	// Initialize health checker if monitoring is enabled
	if appConfig.Monitoring.Enabled {
		var db *sql.DB
		if !appConfig.Database.IsMemory() {
			if gormRepo, ok := transactionRepo.(*repository.GORMTransactionRepository); ok {
				sqlDB, err := gormRepo.GetDB().DB()
				if err != nil {
					logger.Warn("Failed to get underlying SQL DB for health checks", zap.Error(err))
				} else {
					db = sqlDB
				}
			}
		}

		var natsPublisher *messaging.NATSPublisher
		if publisher != nil {
			if p, ok := publisher.(*messaging.NATSPublisher); ok {
				natsPublisher = p
			}
		}

		var modelScorer ml.ModelScorer
		// Initialize ML model scorer with graceful fallback
		mlModelType := os.Getenv("ML_MODEL_TYPE")
		mlModelPath := os.Getenv("ML_MODEL_PATH")
		if mlModelType == "" {
			mlModelType = "mock" // Default to mock
		}
		if mlModelPath == "" {
			mlModelPath = "" // Some models don't need a path (mock)
		}
		scorer, err := ml.ModelScorerFactory(mlModelPath, mlModelType)
		if err != nil {
			logger.Warn("Failed to initialize model scorer, using mock", zap.Error(err))
			scorer = ml.NewMockModelScorer(0.5)
		}
		modelScorer = scorer
		logger.Info("Initialized ML model scorer", zap.String("type", mlModelType))

		healthChecker = monitoring.NewHealthChecker(db, natsPublisher, modelScorer, transactionRepo)
		logger.Info("Initialized health checker")
	}

	// Initialize Gin router
	router := gin.Default()

	// Add middleware
	router.Use(gin.Recovery())

	// Add CORS middleware if enabled
	if appConfig.CORS.Enabled {
		corsMiddleware := httpHandler.NewCorsMiddleware(appConfig.CORS)
		router.Use(corsMiddleware.Middleware())
		logger.Info("Added CORS middleware")
	}

	// Add security headers middleware if enabled
	if appConfig.Security.Enabled {
		securityMiddleware := httpHandler.NewSecurityMiddleware(appConfig.Security)
		router.Use(securityMiddleware.Middleware())
		logger.Info("Added security headers middleware")
	}

	// Add rate limiting middleware if enabled
	if rateLimitMiddleware != nil {
		router.Use(rateLimitMiddleware.RateLimit())
		logger.Info("Added rate limiting middleware")
	}

	// Add validation middleware if enabled
	if appConfig.Validation.Enabled {
		validationMiddlewareConfig := httpHandler.ValidationConfig{
			Enabled:         appConfig.Validation.Enabled,
			SanitizeEnabled: appConfig.Validation.SanitizeEnabled,
			MaxBodySize:     int64(appConfig.Validation.MaxBodySize),
			CustomRules:     make(map[string]map[string][]validation.Validator),
			FieldSanitizers: make(map[string][]validation.Sanitizer),
			SkipPaths: []string{
				"/health",
				"/health/ready",
				"/health/detailed",
				"/metrics",
			},
		}
		validationMiddleware := httpHandler.NewValidationMiddleware(validationMiddlewareConfig)
		router.Use(validationMiddleware.Middleware())
		logger.Info("Added validation middleware")
	}

	// Add audit middleware if enabled
	if appConfig.Audit.Enabled {
		auditMiddlewareConfig := httpHandler.AuditMiddlewareConfig{
			SkipPaths: []string{
				"/health",
				"/health/ready",
				"/health/detailed",
				"/metrics",
			},
			MaxRequestBodySize:  1024, // 1KB
			MaxResponseBodySize: 1024, // 1KB
			LogRequestHeaders:   true,
			LogResponseHeaders:  false, // Usually not needed for response headers
		}
		auditMiddleware := httpHandler.NewAuditMiddleware(auditLogger, auditMiddlewareConfig)
		router.Use(auditMiddleware.Middleware())
		logger.Info("Added audit middleware")
	}

	// Add monitoring middleware if enabled
	if appConfig.Monitoring.Enabled && metrics != nil {
		router.Use(monitoring.MetricsMiddleware(metrics))
		logger.Info("Added metrics middleware")
	}

	// Health check endpoints
	if appConfig.Monitoring.Enabled && healthChecker != nil {
		router.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "healthy",
				"message": "Service is alive",
			})
		})
		router.GET("/health/ready", func(c *gin.Context) {
			health := healthChecker.CheckReadiness(c.Request.Context())
			statusCode := http.StatusOK
			if health.Status != "healthy" {
				statusCode = http.StatusServiceUnavailable
			}
			c.JSON(statusCode, health)
		})
		router.GET("/health/detailed", func(c *gin.Context) {
			health := healthChecker.CheckSystemHealth(c.Request.Context())
			statusCode := http.StatusOK
			if health.Status != "healthy" {
				statusCode = http.StatusServiceUnavailable
			}
			c.JSON(statusCode, health)
		})
		logger.Info("Added health check endpoints")
	} else {
		// Basic health check endpoint
		router.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "healthy",
				"service": "fraud-detection",
			})
		})
	}

	// Metrics endpoint for Prometheus
	if appConfig.Monitoring.Enabled {
		router.GET(appConfig.Monitoring.MetricsPath, gin.WrapH(promhttp.Handler()))
		logger.Info("Added Prometheus metrics endpoint", zap.String("path", appConfig.Monitoring.MetricsPath))
	}

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Authentication routes (public)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", userHandler.Register)
			auth.POST("/login", userHandler.Login)
			auth.POST("/refresh", userHandler.RefreshToken)
		}

		// User routes (protected)
		users := v1.Group("/users")
		users.Use(authMiddleware.RequireAuth())
		{
			users.GET("/profile", userHandler.GetProfile)
			users.PUT("/profile", userHandler.UpdateProfile)
			users.POST("/change-password", userHandler.ChangePassword)
			users.POST("/logout", userHandler.Logout)

			// Admin only routes
			admin := users.Group("")
			admin.Use(authMiddleware.RequireRole(string(domain.RoleAdmin)))
			{
				admin.GET("", userHandler.ListUsers)
				admin.DELETE("/:id", userHandler.DeleteUser)
			}
		}

		// Transaction routes (protected)
		transactions := v1.Group("/transactions")
		transactions.Use(authMiddleware.RequireAuth())
		{
			transactions.POST("", transactionHandler.CreateTransaction)
			transactions.GET("", transactionHandler.ListTransactions)
			transactions.GET("/:id", transactionHandler.GetTransaction)
			transactions.PUT("/:id/status", transactionHandler.UpdateTransactionStatus)
			transactions.DELETE("/:id", transactionHandler.DeleteTransaction)
		}

		// User transaction routes (protected)
		v1.GET("/users/:userId/transactions", authMiddleware.RequireAuth(), transactionHandler.GetUserTransactions)

		// Model management routes - separate group with admin auth
		models := v1.Group("/models")
		models.Use(authMiddleware.RequireAuth())
		models.Use(authMiddleware.RequireRole(string(domain.RoleAdmin)))
		{
			models.POST("/upload", modelHandler.UploadModel)
			models.GET("", modelHandler.ListModels)
			models.GET("/:modelId", modelHandler.GetModel)
			models.DELETE("/:modelId", modelHandler.DeleteModel)
			models.POST("/:modelId/activate", modelHandler.ActivateModel)
			models.POST("/:modelId/deactivate", modelHandler.DeactivateModel)
		}

		// Model operations by name - admin only
		v1.GET("/models/by-name/:modelName/active", authMiddleware.RequireAuth(), authMiddleware.RequireRole(string(domain.RoleAdmin)), modelHandler.GetActiveModel)
		v1.GET("/models/by-name/:modelName/history", authMiddleware.RequireAuth(), authMiddleware.RequireRole(string(domain.RoleAdmin)), modelHandler.GetModelHistory)
		v1.POST("/models/by-name/:modelName/rollback/:version", authMiddleware.RequireAuth(), authMiddleware.RequireRole(string(domain.RoleAdmin)), modelHandler.RollbackModel)
		v1.GET("/models/by-name/:modelName/metrics", authMiddleware.RequireAuth(), authMiddleware.RequireRole(string(domain.RoleAdmin)), modelHandler.GetModelMetrics)
	}

	// Start server
	addr := fmt.Sprintf("%s:%d", appConfig.Server.Host, appConfig.Server.Port)

	logger.Info("Starting HTTP server", zap.String("address", addr))
	if err := router.Run(addr); err != nil {
		logger.Fatal("Failed to start HTTP server", zap.Error(err))
	}
}

func init() {
	// Set Gin mode based on environment
	if gin.Mode() == "" {
		gin.SetMode(gin.ReleaseMode)
	}
}
