package validation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

func (e *ValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("%s: %s (value: %s)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

func (ve *ValidationErrors) Error() string {
	var messages []string
	for _, err := range ve.Errors {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// Add adds a new validation error
func (ve *ValidationErrors) Add(field, message, value string) {
	ve.Errors = append(ve.Errors, ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// HasErrors returns true if there are validation errors
func (ve *ValidationErrors) HasErrors() bool {
	return len(ve.Errors) > 0
}

// Validator interface for validation functions
type Validator interface {
	Validate(value interface{}) error
}

// RequiredValidator validates that a value is not empty
type RequiredValidator struct {
	Field string
}

func (v *RequiredValidator) Validate(value interface{}) error {
	if value == nil {
		return &ValidationError{
			Field:   v.Field,
			Message: "is required",
		}
	}

	switch val := value.(type) {
	case string:
		if strings.TrimSpace(val) == "" {
			return &ValidationError{
				Field:   v.Field,
				Message: "is required",
			}
		}
	case []string:
		if len(val) == 0 {
			return &ValidationError{
				Field:   v.Field,
				Message: "is required",
			}
		}
	default:
		// For other types, check if it's the zero value
		if fmt.Sprintf("%v", val) == "" || fmt.Sprintf("%v", val) == "0" || fmt.Sprintf("%v", val) == "false" {
			return &ValidationError{
				Field:   v.Field,
				Message: "is required",
			}
		}
	}

	return nil
}

// UUIDValidator validates that a string is a valid UUID
type UUIDValidator struct {
	Field string
}

func (v *UUIDValidator) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Field:   v.Field,
			Message: "must be a string",
		}
	}

	if _, err := uuid.Parse(str); err != nil {
		return &ValidationError{
			Field:   v.Field,
			Message: "must be a valid UUID",
			Value:   str,
		}
	}

	return nil
}

// EmailValidator validates that a string is a valid email
type EmailValidator struct {
	Field string
}

func (v *EmailValidator) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Field:   v.Field,
			Message: "must be a string",
		}
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(str) {
		return &ValidationError{
			Field:   v.Field,
			Message: "must be a valid email address",
			Value:   str,
		}
	}

	return nil
}

// LengthValidator validates string length
type LengthValidator struct {
	Field string
	Min   int
	Max   int
	Exact int
}

func (v *LengthValidator) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Field:   v.Field,
			Message: "must be a string",
		}
	}

	length := utf8.RuneCountInString(str)

	if v.Exact > 0 {
		if length != v.Exact {
			return &ValidationError{
				Field:   v.Field,
				Message: fmt.Sprintf("must be exactly %d characters", v.Exact),
				Value:   str,
			}
		}
		return nil
	}

	if v.Min > 0 && length < v.Min {
		return &ValidationError{
			Field:   v.Field,
			Message: fmt.Sprintf("must be at least %d characters", v.Min),
			Value:   str,
		}
	}

	if v.Max > 0 && length > v.Max {
		return &ValidationError{
			Field:   v.Field,
			Message: fmt.Sprintf("must be at most %d characters", v.Max),
			Value:   str,
		}
	}

	return nil
}

// NumericValidator validates that a value is numeric
type NumericValidator struct {
	Field string
	Min   *float64
	Max   *float64
}

func (v *NumericValidator) Validate(value interface{}) error {
	var num float64
	var ok bool

	switch val := value.(type) {
	case float64:
		num, ok = val, true
	case float32:
		num, ok = float64(val), true
	case int:
		num, ok = float64(val), true
	case int64:
		num, ok = float64(val), true
	case int32:
		num, ok = float64(val), true
	case string:
		parsed, err := strconv.ParseFloat(val, 64)
		if err != nil {
			ok = false
		} else {
			num, ok = parsed, true
		}
	default:
		ok = false
	}

	if !ok {
		return &ValidationError{
			Field:   v.Field,
			Message: "must be a number",
		}
	}

	if v.Min != nil && num < *v.Min {
		return &ValidationError{
			Field:   v.Field,
			Message: fmt.Sprintf("must be at least %.2f", *v.Min),
			Value:   fmt.Sprintf("%.2f", num),
		}
	}

	if v.Max != nil && num > *v.Max {
		return &ValidationError{
			Field:   v.Field,
			Message: fmt.Sprintf("must be at most %.2f", *v.Max),
			Value:   fmt.Sprintf("%.2f", num),
		}
	}

	return nil
}

// RegexValidator validates that a string matches a regex pattern
type RegexValidator struct {
	Field   string
	Pattern string
	Message string
}

func (v *RegexValidator) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Field:   v.Field,
			Message: "must be a string",
		}
	}

	regex, err := regexp.Compile(v.Pattern)
	if err != nil {
		return &ValidationError{
			Field:   v.Field,
			Message: "invalid regex pattern in validator",
		}
	}

	if !regex.MatchString(str) {
		message := v.Message
		if message == "" {
			message = "format is invalid"
		}
		return &ValidationError{
			Field:   v.Field,
			Message: message,
			Value:   str,
		}
	}

	return nil
}

// AlphaNumericValidator validates that a string contains only alphanumeric characters
type AlphaNumericValidator struct {
	Field string
}

func (v *AlphaNumericValidator) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Field:   v.Field,
			Message: "must be a string",
		}
	}

	for _, r := range str {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return &ValidationError{
				Field:   v.Field,
				Message: "must contain only alphanumeric characters",
				Value:   str,
			}
		}
	}

	return nil
}

// PasswordValidator validates password strength
type PasswordValidator struct {
	Field            string
	MinLength        int
	RequireUppercase bool
	RequireLowercase bool
	RequireNumbers   bool
	RequireSymbols   bool
}

func (v *PasswordValidator) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Field:   v.Field,
			Message: "must be a string",
		}
	}

	if len(str) < v.MinLength {
		return &ValidationError{
			Field:   v.Field,
			Message: fmt.Sprintf("must be at least %d characters", v.MinLength),
			Value:   str,
		}
	}

	var hasUpper, hasLower, hasNumber, hasSymbol bool

	for _, r := range str {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasNumber = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}

	if v.RequireUppercase && !hasUpper {
		return &ValidationError{
			Field:   v.Field,
			Message: "must contain at least one uppercase letter",
			Value:   str,
		}
	}

	if v.RequireLowercase && !hasLower {
		return &ValidationError{
			Field:   v.Field,
			Message: "must contain at least one lowercase letter",
			Value:   str,
		}
	}

	if v.RequireNumbers && !hasNumber {
		return &ValidationError{
			Field:   v.Field,
			Message: "must contain at least one number",
			Value:   str,
		}
	}

	if v.RequireSymbols && !hasSymbol {
		return &ValidationError{
			Field:   v.Field,
			Message: "must contain at least one symbol",
			Value:   str,
		}
	}

	return nil
}

// InValidator validates that a value is in a list of allowed values
type InValidator struct {
	Field   string
	Allowed []string
}

func (v *InValidator) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Field:   v.Field,
			Message: "must be a string",
		}
	}

	for _, allowed := range v.Allowed {
		if str == allowed {
			return nil
		}
	}

	return &ValidationError{
		Field:   v.Field,
		Message: fmt.Sprintf("must be one of: %s", strings.Join(v.Allowed, ", ")),
		Value:   str,
	}
}

// Sanitizer interface for sanitization functions
type Sanitizer interface {
	Sanitize(value string) string
}

// TrimSanitizer trims whitespace from a string
type TrimSanitizer struct{}

func (s *TrimSanitizer) Sanitize(value string) string {
	return strings.TrimSpace(value)
}

// LowercaseSanitizer converts a string to lowercase
type LowercaseSanitizer struct{}

func (s *LowercaseSanitizer) Sanitize(value string) string {
	return strings.ToLower(value)
}

// UppercaseSanitizer converts a string to uppercase
type UppercaseSanitizer struct{}

func (s *UppercaseSanitizer) Sanitize(value string) string {
	return strings.ToUpper(value)
}

// HTMLSanitizer removes HTML tags from a string
type HTMLSanitizer struct{}

func (s *HTMLSanitizer) Sanitize(value string) string {
	// Simple HTML tag removal - in production, use a proper HTML sanitizer library
	// Remove script tags completely (including content)
	scriptRegex := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	value = scriptRegex.ReplaceAllString(value, "")
	// Remove other HTML tags but keep content
	htmlRegex := regexp.MustCompile(`<[^>]*>`)
	return htmlRegex.ReplaceAllString(value, "")
}

// SQLInjectionSanitizer removes potential SQL injection patterns
type SQLInjectionSanitizer struct{}

func (s *SQLInjectionSanitizer) Sanitize(value string) string {
	// Remove common SQL injection patterns
	patterns := []string{
		`(?i)\b(union|select|insert|update|delete|drop|create|alter|exec|execute|table|from|where|into|values|users)\b`,
		`(?i)(script|javascript|vbscript)`,
		`['";\\]`,
		`--`,
		`/\*.*?\*/`,
		`(?i)\b(or|and)\s+\d+\s*=\s*\d+`,
		`(?i)\bor\b.*?\b1\b.*?\b=\b.*?\b1\b`,
	}

	result := value
	for _, pattern := range patterns {
		regex := regexp.MustCompile(pattern)
		result = regex.ReplaceAllString(result, "")
	}

	// Clean up extra whitespace
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")
	result = strings.TrimSpace(result)

	return result
}

// XssSanitizer removes potential XSS patterns
type XssSanitizer struct{}

func (s *XssSanitizer) Sanitize(value string) string {
	// Remove common XSS patterns
	patterns := []string{
		`(?i)<script[^>]*>.*?</script>`,
		`(?i)javascript:`,
		`(?i)vbscript:`,
		`(?i)on\w+\s*=`,
		`(?i)<iframe[^>]*>.*?</iframe>`,
		`(?i)<object[^>]*>.*?</object>`,
		`(?i)<embed[^>]*>.*?</embed>`,
		`(?i)<link[^>]*>`,
		`(?i)<meta[^>]*>`,
	}

	result := value
	for _, pattern := range patterns {
		regex := regexp.MustCompile(pattern)
		result = regex.ReplaceAllString(result, "")
	}

	// Remove any remaining HTML tags
	htmlRegex := regexp.MustCompile(`<[^>]*>`)
	result = htmlRegex.ReplaceAllString(result, "")

	return result
}
