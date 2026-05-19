package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"fraud-detection/internal/infrastructure/validation"

	"github.com/gin-gonic/gin"
)

// ValidationMiddleware provides input validation and sanitization
type ValidationMiddleware struct {
	config ValidationConfig
}

// ValidationConfig holds configuration for validation middleware
type ValidationConfig struct {
	// Enable/disable validation
	Enabled bool
	// Enable sanitization
	SanitizeEnabled bool
	// Global sanitizers to apply to all string fields
	GlobalSanitizers []validation.Sanitizer
	// Custom validation rules
	CustomRules map[string]map[string][]validation.Validator
	// Custom sanitizers per field
	FieldSanitizers map[string][]validation.Sanitizer
	// Skip validation for these paths
	SkipPaths []string
	// Maximum request body size (in bytes)
	MaxBodySize int64
}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware(config ValidationConfig) *ValidationMiddleware {
	// Set defaults
	if config.MaxBodySize == 0 {
		config.MaxBodySize = 1024 * 1024 // 1MB
	}

	// Add default global sanitizers if none provided
	if len(config.GlobalSanitizers) == 0 {
		config.GlobalSanitizers = []validation.Sanitizer{
			&validation.TrimSanitizer{},
			&validation.XssSanitizer{},
			&validation.SQLInjectionSanitizer{},
		}
	}

	return &ValidationMiddleware{
		config: config,
	}
}

// Middleware returns the Gin middleware function
func (m *ValidationMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.Enabled {
			c.Next()
			return
		}

		// Skip validation for specified paths
		if m.shouldSkipPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Check request body size
		if c.Request.ContentLength > m.config.MaxBodySize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": fmt.Sprintf("Request body too large. Maximum size is %d bytes", m.config.MaxBodySize),
			})
			c.Abort()
			return
		}

		// Only validate JSON requests
		if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
			c.Next()
			return
		}

		// Read and validate request body
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid JSON format",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Validate and sanitize based on path and method
		if err := m.validateAndSanitize(c.Request.Method, c.Request.URL.Path, body); err != nil {
			if validationErrors, ok := err.(*validation.ValidationErrors); ok {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":  "Validation failed",
					"errors": validationErrors.Errors,
				})
			} else {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Validation failed",
					"details": err.Error(),
				})
			}
			c.Abort()
			return
		}

		// Replace the request body with sanitized data
		sanitizedBody, _ := json.Marshal(body)
		c.Request.Body = &requestBodyReader{data: sanitizedBody}

		c.Next()
	}
}

// shouldSkipPath checks if the path should be skipped from validation
func (m *ValidationMiddleware) shouldSkipPath(path string) bool {
	for _, skipPath := range m.config.SkipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}
	return false
}

// validateAndSanitize validates and sanitizes the request body
func (m *ValidationMiddleware) validateAndSanitize(method, path string, body map[string]interface{}) error {
	var validationErrors validation.ValidationErrors

	// Get validation rules for this endpoint
	rules := m.getValidationRules(method, path)

	// Validate each field
	for field, validators := range rules {
		value, exists := body[field]

		// Check if field is required
		for _, validator := range validators {
			if reqValidator, ok := validator.(*validation.RequiredValidator); ok {
				if err := reqValidator.Validate(value); err != nil {
					validationErrors.Add(field, err.Error(), fmt.Sprintf("%v", value))
					continue
				}
			}
		}

		// If field doesn't exist and is not required, skip validation
		if !exists {
			continue
		}

		// Apply sanitizers
		if m.config.SanitizeEnabled {
			sanitizedValue := m.sanitizeField(field, value)
			body[field] = sanitizedValue
			value = sanitizedValue
		}

		// Apply validators
		for _, validator := range validators {
			if _, ok := validator.(*validation.RequiredValidator); ok {
				continue // Already handled above
			}

			if err := validator.Validate(value); err != nil {
				validationErrors.Add(field, err.Error(), fmt.Sprintf("%v", value))
			}
		}
	}

	if validationErrors.HasErrors() {
		return &validationErrors
	}

	return nil
}

// sanitizeField sanitizes a field value
func (m *ValidationMiddleware) sanitizeField(field string, value interface{}) interface{} {
	str, ok := value.(string)
	if !ok {
		return value
	}

	result := str

	// Apply global sanitizers
	for _, sanitizer := range m.config.GlobalSanitizers {
		result = sanitizer.Sanitize(result)
	}

	// Apply field-specific sanitizers
	if fieldSanitizers, exists := m.config.FieldSanitizers[field]; exists {
		for _, sanitizer := range fieldSanitizers {
			result = sanitizer.Sanitize(result)
		}
	}

	return result
}

// getValidationRules returns validation rules for a specific endpoint
func (m *ValidationMiddleware) getValidationRules(method, path string) map[string][]validation.Validator {
	// Check for custom rules first
	if customRules, exists := m.config.CustomRules[method+":"+path]; exists {
		return customRules
	}

	// Default validation rules based on common patterns
	rules := make(map[string][]validation.Validator)

	// Authentication endpoints
	if strings.Contains(path, "/auth/login") {
		rules["username"] = []validation.Validator{
			&validation.RequiredValidator{Field: "username"},
			&validation.LengthValidator{Field: "username", Min: 3, Max: 50},
			&validation.AlphaNumericValidator{Field: "username"},
		}
		rules["password"] = []validation.Validator{
			&validation.RequiredValidator{Field: "password"},
			&validation.PasswordValidator{
				Field:            "password",
				MinLength:        8,
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumbers:   true,
			},
		}
	}

	if strings.Contains(path, "/auth/register") {
		rules["username"] = []validation.Validator{
			&validation.RequiredValidator{Field: "username"},
			&validation.LengthValidator{Field: "username", Min: 3, Max: 50},
			&validation.AlphaNumericValidator{Field: "username"},
		}
		rules["email"] = []validation.Validator{
			&validation.RequiredValidator{Field: "email"},
			&validation.EmailValidator{Field: "email"},
		}
		rules["password"] = []validation.Validator{
			&validation.RequiredValidator{Field: "password"},
			&validation.PasswordValidator{
				Field:            "password",
				MinLength:        8,
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumbers:   true,
				RequireSymbols:   true,
			},
		}
	}

	// Transaction endpoints
	if strings.Contains(path, "/transactions") {
		if method == "POST" {
			rules["user_id"] = []validation.Validator{
				&validation.RequiredValidator{Field: "user_id"},
				&validation.UUIDValidator{Field: "user_id"},
			}
			rules["amount"] = []validation.Validator{
				&validation.RequiredValidator{Field: "amount"},
				&validation.NumericValidator{
					Field: "amount",
					Min:   &[]float64{0.01}[0],
					Max:   &[]float64{999999.99}[0],
				},
			}
			rules["currency"] = []validation.Validator{
				&validation.LengthValidator{Field: "currency", Exact: 3},
				&validation.AlphaNumericValidator{Field: "currency"},
			}
			rules["merchant"] = []validation.Validator{
				&validation.LengthValidator{Field: "merchant", Min: 1, Max: 255},
			}
			rules["country"] = []validation.Validator{
				&validation.LengthValidator{Field: "country", Exact: 2},
				&validation.AlphaNumericValidator{Field: "country"},
			}
			rules["city"] = []validation.Validator{
				&validation.LengthValidator{Field: "city", Min: 1, Max: 100},
			}
		}
	}

	// User management endpoints
	if strings.Contains(path, "/users") {
		if method == "PUT" {
			rules["email"] = []validation.Validator{
				&validation.EmailValidator{Field: "email"},
			}
			rules["username"] = []validation.Validator{
				&validation.LengthValidator{Field: "username", Min: 3, Max: 50},
				&validation.AlphaNumericValidator{Field: "username"},
			}
		}
		if strings.Contains(path, "/password") && method == "PUT" {
			rules["current_password"] = []validation.Validator{
				&validation.RequiredValidator{Field: "current_password"},
			}
			rules["new_password"] = []validation.Validator{
				&validation.RequiredValidator{Field: "new_password"},
				&validation.PasswordValidator{
					Field:            "new_password",
					MinLength:        8,
					RequireUppercase: true,
					RequireLowercase: true,
					RequireNumbers:   true,
					RequireSymbols:   true,
				},
			}
		}
	}

	return rules
}

// requestBodyReader is a custom reader for the sanitized request body
type requestBodyReader struct {
	data []byte
	pos  int
}

func (r *requestBodyReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *requestBodyReader) Close() error {
	return nil
}

// ValidateStruct validates a struct using validation tags
func ValidateStruct(s interface{}) error {
	validationErrors := &validation.ValidationErrors{}

	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %s", v.Kind())
	}

	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Get validation tags
		tag := field.Tag.Get("validate")
		if tag == "" {
			continue
		}

		// Parse validation rules from tag
		rules := parseValidationTags(tag, field.Name)

		// Apply validators
		for _, rule := range rules {
			var value interface{}
			if fieldValue.IsValid() && fieldValue.CanInterface() {
				value = fieldValue.Interface()
			}

			if err := rule.Validate(value); err != nil {
				validationErrors.Add(field.Name, err.Error(), fmt.Sprintf("%v", value))
			}
		}
	}

	if validationErrors.HasErrors() {
		return validationErrors
	}

	return nil
}

// parseValidationTags parses validation tags from struct field tags
func parseValidationTags(tag, fieldName string) []validation.Validator {
	var validators []validation.Validator

	rules := strings.Split(tag, ",")
	for _, rule := range rules {
		parts := strings.Split(rule, "=")
		ruleName := strings.TrimSpace(parts[0])

		switch ruleName {
		case "required":
			validators = append(validators, &validation.RequiredValidator{Field: fieldName})
		case "uuid":
			validators = append(validators, &validation.UUIDValidator{Field: fieldName})
		case "email":
			validators = append(validators, &validation.EmailValidator{Field: fieldName})
		case "alpha":
			validators = append(validators, &validation.AlphaNumericValidator{Field: fieldName})
		case "min":
			if len(parts) > 1 {
				if min, err := parseInt(parts[1]); err == nil {
					validators = append(validators, &validation.LengthValidator{Field: fieldName, Min: min})
				}
			}
		case "max":
			if len(parts) > 1 {
				if max, err := parseInt(parts[1]); err == nil {
					validators = append(validators, &validation.LengthValidator{Field: fieldName, Max: max})
				}
			}
		case "len":
			if len(parts) > 1 {
				if length, err := parseInt(parts[1]); err == nil {
					validators = append(validators, &validation.LengthValidator{Field: fieldName, Exact: length})
				}
			}
		case "gt":
			if len(parts) > 1 {
				if min, err := parseFloat(parts[1]); err == nil {
					validators = append(validators, &validation.NumericValidator{Field: fieldName, Min: &min})
				}
			}
		case "lt":
			if len(parts) > 1 {
				if max, err := parseFloat(parts[1]); err == nil {
					validators = append(validators, &validation.NumericValidator{Field: fieldName, Max: &max})
				}
			}
		}
	}

	return validators
}

// parseInt parses an integer from string
func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// parseFloat parses a float from string
func parseFloat(s string) (float64, error) {
	var result float64
	_, err := fmt.Sscanf(s, "%f", &result)
	return result, err
}
