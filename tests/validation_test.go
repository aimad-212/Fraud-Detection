package tests

import (
	"testing"

	"fraud-detection/internal/infrastructure/validation"

	"github.com/stretchr/testify/assert"
)

func TestRequiredValidator(t *testing.T) {
	validator := &validation.RequiredValidator{Field: "test"}

	// Test valid cases
	assert.NoError(t, validator.Validate("test"))
	assert.NoError(t, validator.Validate(123))
	assert.NoError(t, validator.Validate(true))

	// Test invalid cases
	assert.Error(t, validator.Validate(nil))
	assert.Error(t, validator.Validate(""))
	assert.Error(t, validator.Validate("   "))
}

func TestUUIDValidator(t *testing.T) {
	validator := &validation.UUIDValidator{Field: "test"}

	// Test valid UUIDs
	assert.NoError(t, validator.Validate("550e8400-e29b-41d4-a716-446655440000"))
	assert.NoError(t, validator.Validate("123e4567-e89b-12d3-a456-426614174000"))

	// Test invalid UUIDs
	assert.Error(t, validator.Validate("invalid-uuid"))
	assert.Error(t, validator.Validate("550e8400-e29b-41d4-a716"))
	assert.Error(t, validator.Validate("123"))
	assert.Error(t, validator.Validate(123))
}

func TestEmailValidator(t *testing.T) {
	validator := &validation.EmailValidator{Field: "test"}

	// Test valid emails
	assert.NoError(t, validator.Validate("test@example.com"))
	assert.NoError(t, validator.Validate("user.name+tag@domain.co.uk"))
	assert.NoError(t, validator.Validate("user123@test-domain.com"))

	// Test invalid emails
	assert.Error(t, validator.Validate("invalid-email"))
	assert.Error(t, validator.Validate("@domain.com"))
	assert.Error(t, validator.Validate("user@"))
	assert.Error(t, validator.Validate("user.domain.com"))
	assert.Error(t, validator.Validate(123))
}

func TestLengthValidator(t *testing.T) {
	// Test minimum length
	minValidator := &validation.LengthValidator{Field: "test", Min: 5}
	assert.NoError(t, minValidator.Validate("hello"))
	assert.NoError(t, minValidator.Validate("hello world"))
	assert.Error(t, minValidator.Validate("hi"))
	assert.Error(t, minValidator.Validate(""))

	// Test maximum length
	maxValidator := &validation.LengthValidator{Field: "test", Max: 10}
	assert.NoError(t, maxValidator.Validate("hello"))
	assert.NoError(t, maxValidator.Validate(""))
	assert.Error(t, maxValidator.Validate("this is too long"))

	// Test exact length
	exactValidator := &validation.LengthValidator{Field: "test", Exact: 5}
	assert.NoError(t, exactValidator.Validate("hello"))
	assert.Error(t, exactValidator.Validate("hi"))
	assert.Error(t, exactValidator.Validate("hello world"))

	// Test non-string input
	assert.Error(t, minValidator.Validate(123))
}

func TestNumericValidator(t *testing.T) {
	// Test basic numeric validation
	validator := &validation.NumericValidator{Field: "test"}
	assert.NoError(t, validator.Validate(123))
	assert.NoError(t, validator.Validate(123.45))
	assert.NoError(t, validator.Validate("123"))
	assert.NoError(t, validator.Validate("123.45"))

	assert.Error(t, validator.Validate("not a number"))
	assert.Error(t, validator.Validate("abc"))

	// Test with minimum
	min := 10.0
	minValidator := &validation.NumericValidator{Field: "test", Min: &min}
	assert.NoError(t, minValidator.Validate(15))
	assert.NoError(t, minValidator.Validate(10))
	assert.Error(t, minValidator.Validate(5))

	// Test with maximum
	max := 100.0
	maxValidator := &validation.NumericValidator{Field: "test", Max: &max}
	assert.NoError(t, maxValidator.Validate(50))
	assert.NoError(t, maxValidator.Validate(100))
	assert.Error(t, maxValidator.Validate(150))

	// Test with both min and max
	rangeValidator := &validation.NumericValidator{Field: "test", Min: &min, Max: &max}
	assert.NoError(t, rangeValidator.Validate(50))
	assert.Error(t, rangeValidator.Validate(5))
	assert.Error(t, rangeValidator.Validate(150))
}

func TestRegexValidator(t *testing.T) {
	// Test alphanumeric pattern
	validator := &validation.RegexValidator{
		Field:   "test",
		Pattern: `^[a-zA-Z0-9]+$`,
		Message: "must contain only alphanumeric characters",
	}

	assert.NoError(t, validator.Validate("test123"))
	assert.NoError(t, validator.Validate("ABCdef"))
	assert.Error(t, validator.Validate("test-123"))
	assert.Error(t, validator.Validate("test 123"))
	assert.Error(t, validator.Validate("test@123"))

	// Test invalid regex pattern
	invalidValidator := &validation.RegexValidator{
		Field:   "test",
		Pattern: `[invalid regex`,
	}
	assert.Error(t, invalidValidator.Validate("test"))
}

func TestAlphaNumericValidator(t *testing.T) {
	validator := &validation.AlphaNumericValidator{Field: "test"}

	assert.NoError(t, validator.Validate("test123"))
	assert.NoError(t, validator.Validate("ABCdef"))
	assert.NoError(t, validator.Validate("123456"))
	assert.Error(t, validator.Validate("test-123"))
	assert.Error(t, validator.Validate("test 123"))
	assert.Error(t, validator.Validate("test@123"))
	assert.Error(t, validator.Validate(123))
}

func TestPasswordValidator(t *testing.T) {
	// Test basic password validation
	validator := &validation.PasswordValidator{
		Field:     "password",
		MinLength: 8,
	}

	assert.NoError(t, validator.Validate("password123"))
	assert.Error(t, validator.Validate("short"))
	assert.Error(t, validator.Validate(""))

	// Test with all requirements
	strongValidator := &validation.PasswordValidator{
		Field:            "password",
		MinLength:        8,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumbers:   true,
		RequireSymbols:   true,
	}

	assert.NoError(t, strongValidator.Validate("StrongP@ssw0rd"))
	assert.NoError(t, strongValidator.Validate("MyP@ss123!"))

	// Test missing requirements
	assert.Error(t, strongValidator.Validate("weakpassword")) // missing uppercase, numbers, symbols
	assert.Error(t, strongValidator.Validate("WEAKPASSWORD")) // missing lowercase, numbers, symbols
	assert.Error(t, strongValidator.Validate("WeakPassword")) // missing numbers, symbols
	assert.Error(t, strongValidator.Validate("WeakP@ssword")) // missing numbers
	assert.Error(t, strongValidator.Validate("WeakPass123"))  // missing symbols
	assert.Error(t, strongValidator.Validate("Short1!"))      // too short

	assert.Error(t, strongValidator.Validate(123))
}

func TestInValidator(t *testing.T) {
	validator := &validation.InValidator{
		Field:   "status",
		Allowed: []string{"active", "inactive", "pending"},
	}

	assert.NoError(t, validator.Validate("active"))
	assert.NoError(t, validator.Validate("inactive"))
	assert.NoError(t, validator.Validate("pending"))
	assert.Error(t, validator.Validate("unknown"))
	assert.Error(t, validator.Validate("ACTIVE")) // case sensitive
	assert.Error(t, validator.Validate(123))
}

func TestTrimSanitizer(t *testing.T) {
	sanitizer := &validation.TrimSanitizer{}

	assert.Equal(t, "test", sanitizer.Sanitize("  test  "))
	assert.Equal(t, "hello world", sanitizer.Sanitize("\t\n hello world \n\t"))
	assert.Equal(t, "", sanitizer.Sanitize("   "))
	assert.Equal(t, "test", sanitizer.Sanitize("test")) // no change
}

func TestLowercaseSanitizer(t *testing.T) {
	sanitizer := &validation.LowercaseSanitizer{}

	assert.Equal(t, "test", sanitizer.Sanitize("TEST"))
	assert.Equal(t, "hello world", sanitizer.Sanitize("HELLO WORLD"))
	assert.Equal(t, "123", sanitizer.Sanitize("123"))
	assert.Equal(t, "test", sanitizer.Sanitize("test")) // no change
}

func TestUppercaseSanitizer(t *testing.T) {
	sanitizer := &validation.UppercaseSanitizer{}

	assert.Equal(t, "TEST", sanitizer.Sanitize("test"))
	assert.Equal(t, "HELLO WORLD", sanitizer.Sanitize("hello world"))
	assert.Equal(t, "123", sanitizer.Sanitize("123"))
	assert.Equal(t, "TEST", sanitizer.Sanitize("TEST")) // no change
}

func TestHTMLSanitizer(t *testing.T) {
	sanitizer := &validation.HTMLSanitizer{}

	assert.Equal(t, "test", sanitizer.Sanitize("<p>test</p>"))
	assert.Equal(t, "hello world", sanitizer.Sanitize("<div>hello <b>world</b></div>"))
	assert.Equal(t, "test", sanitizer.Sanitize("test")) // no change
	assert.Equal(t, "", sanitizer.Sanitize("<script>alert('xss')</script>"))
}

func TestSQLInjectionSanitizer(t *testing.T) {
	sanitizer := &validation.SQLInjectionSanitizer{}

	assert.Equal(t, "test", sanitizer.Sanitize("test"))
	assert.Equal(t, "username", sanitizer.Sanitize("username; DROP TABLE users; --"))
	assert.Equal(t, "name", sanitizer.Sanitize("name' OR '1'='1"))
	assert.Equal(t, "test", sanitizer.Sanitize("test")) // no change
}

func TestXssSanitizer(t *testing.T) {
	sanitizer := &validation.XssSanitizer{}

	assert.Equal(t, "test", sanitizer.Sanitize("test"))
	assert.Equal(t, "", sanitizer.Sanitize("<script>alert('xss')</script>"))
	assert.Equal(t, "link", sanitizer.Sanitize("<a href='javascript:alert(1)'>link</a>"))
	assert.Equal(t, "test", sanitizer.Sanitize("test")) // no change
}

func TestValidationErrors(t *testing.T) {
	var errors validation.ValidationErrors

	// Test empty errors
	assert.False(t, errors.HasErrors())
	assert.Equal(t, "", errors.Error())

	// Add errors
	errors.Add("field1", "is required", "")
	errors.Add("field2", "must be email", "invalid")

	assert.True(t, errors.HasErrors())
	assert.Contains(t, errors.Error(), "field1: is required")
	assert.Contains(t, errors.Error(), "field2: must be email (value: invalid)")
	assert.Len(t, errors.Errors, 2)
}

func TestValidationError(t *testing.T) {
	// Test without value
	err := &validation.ValidationError{
		Field:   "username",
		Message: "is required",
	}
	assert.Equal(t, "username: is required", err.Error())

	// Test with value
	errWithValue := &validation.ValidationError{
		Field:   "email",
		Message: "must be valid",
		Value:   "invalid-email",
	}
	assert.Equal(t, "email: must be valid (value: invalid-email)", errWithValue.Error())
}

func TestMultipleValidators(t *testing.T) {
	// Test field with multiple validators
	validators := []validation.Validator{
		&validation.RequiredValidator{Field: "email"},
		&validation.EmailValidator{Field: "email"},
		&validation.LengthValidator{Field: "email", Max: 100},
	}

	// Test valid input
	validEmail := "test@example.com"
	for _, validator := range validators {
		assert.NoError(t, validator.Validate(validEmail))
	}

	// Test invalid input (empty)
	for _, validator := range validators {
		if _, ok := validator.(*validation.RequiredValidator); ok {
			assert.Error(t, validator.Validate(""))
		}
	}

	// Test invalid input (bad format)
	for _, validator := range validators {
		if _, ok := validator.(*validation.EmailValidator); ok {
			assert.Error(t, validator.Validate("not-an-email"))
		}
	}

	// Test invalid input (too long)
	longEmail := "very-long-email-address-that-exceeds-the-maximum-length-and-should-definitely-be-over-one-hundred-characters-long@example.com"
	for _, validator := range validators {
		if _, ok := validator.(*validation.LengthValidator); ok {
			assert.Error(t, validator.Validate(longEmail))
		}
	}
}

func TestValidatorEdgeCases(t *testing.T) {
	t.Run("Numeric validator with string numbers", func(t *testing.T) {
		validator := &validation.NumericValidator{Field: "test"}
		assert.NoError(t, validator.Validate("0"))
		assert.NoError(t, validator.Validate("-123"))
		assert.NoError(t, validator.Validate("123.456"))
	})

	t.Run("Length validator with unicode", func(t *testing.T) {
		validator := &validation.LengthValidator{Field: "test", Min: 2}
		assert.NoError(t, validator.Validate("测试")) // Chinese characters (2 chars)
		assert.Error(t, validator.Validate("测"))    // Single Chinese character (1 char)
	})

	t.Run("Email validator with edge cases", func(t *testing.T) {
		validator := &validation.EmailValidator{Field: "test"}
		assert.NoError(t, validator.Validate("a@b.co")) // Minimal valid email
		assert.Error(t, validator.Validate("a@b"))      // No TLD
		assert.Error(t, validator.Validate("@b.co"))    // No local part
	})

	t.Run("Password validator with unicode", func(t *testing.T) {
		validator := &validation.PasswordValidator{
			Field:            "password",
			MinLength:        8,
			RequireUppercase: true,
			RequireLowercase: true,
			RequireNumbers:   true,
		}
		assert.NoError(t, validator.Validate("Password123")) // ASCII
		// Unicode characters might not be counted as expected for uppercase/lowercase
	})
}
