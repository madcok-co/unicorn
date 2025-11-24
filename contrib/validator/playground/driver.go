// Package playground provides a go-playground/validator implementation of the unicorn Validator interface.
//
// Usage:
//
//	import (
//	    "github.com/madcok-co/unicorn/contrib/validator/playground"
//	)
//
//	driver := playground.NewDriver()
//	app.SetValidator(driver)
//
//	// With custom config
//	driver := playground.NewDriverWithConfig(&playground.Config{
//	    UseJSONNames: true,
//	})
package playground

import (
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// Driver implements contracts.Validator using go-playground/validator
type Driver struct {
	validate     *validator.Validate
	translations map[string]string
	mu           sync.RWMutex
}

// Config for the validator driver
type Config struct {
	// UseJSONNames uses JSON tag names in error messages
	UseJSONNames bool

	// Custom messages for validation tags
	Messages map[string]string
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		UseJSONNames: true,
		Messages:     defaultMessages(),
	}
}

func defaultMessages() map[string]string {
	return map[string]string{
		"required":   "{field} is required",
		"email":      "{field} must be a valid email address",
		"min":        "{field} must be at least {param} characters",
		"max":        "{field} must be at most {param} characters",
		"len":        "{field} must be exactly {param} characters",
		"gte":        "{field} must be greater than or equal to {param}",
		"lte":        "{field} must be less than or equal to {param}",
		"gt":         "{field} must be greater than {param}",
		"lt":         "{field} must be less than {param}",
		"eq":         "{field} must be equal to {param}",
		"ne":         "{field} must not be equal to {param}",
		"oneof":      "{field} must be one of: {param}",
		"url":        "{field} must be a valid URL",
		"uuid":       "{field} must be a valid UUID",
		"alpha":      "{field} must contain only alphabetic characters",
		"alphanum":   "{field} must contain only alphanumeric characters",
		"numeric":    "{field} must be a valid number",
		"boolean":    "{field} must be a boolean value",
		"lowercase":  "{field} must be lowercase",
		"uppercase":  "{field} must be uppercase",
		"contains":   "{field} must contain '{param}'",
		"excludes":   "{field} must not contain '{param}'",
		"startswith": "{field} must start with '{param}'",
		"endswith":   "{field} must end with '{param}'",
		"datetime":   "{field} must be a valid datetime",
		"json":       "{field} must be valid JSON",
	}
}

// NewDriver creates a new validator driver with default settings
func NewDriver() *Driver {
	return NewDriverWithConfig(DefaultConfig())
}

// NewDriverWithConfig creates a new validator driver with custom config
func NewDriverWithConfig(cfg *Config) *Driver {
	v := validator.New()

	// Use JSON tag names if configured
	if cfg.UseJSONNames {
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			if name == "" {
				return fld.Name
			}
			return name
		})
	}

	translations := defaultMessages()
	if cfg.Messages != nil {
		for k, v := range cfg.Messages {
			translations[k] = v
		}
	}

	return &Driver{
		validate:     v,
		translations: translations,
	}
}

// Validator returns the underlying validator instance
func (d *Driver) Validator() *validator.Validate {
	return d.validate
}

// Validate validates a struct based on tags
func (d *Driver) Validate(data any) error {
	err := d.validate.Struct(data)
	if err == nil {
		return nil
	}

	// Convert to contracts.ValidationErrors
	var validationErrors contracts.ValidationErrors

	if errs, ok := err.(validator.ValidationErrors); ok {
		for _, e := range errs {
			validationErrors = append(validationErrors, contracts.ValidationError{
				Field:   e.Field(),
				Tag:     e.Tag(),
				Value:   e.Value(),
				Message: d.formatMessage(e),
			})
		}
	}

	return validationErrors
}

// ValidateField validates a single field value
func (d *Driver) ValidateField(field any, tag string) error {
	err := d.validate.Var(field, tag)
	if err == nil {
		return nil
	}

	var validationErrors contracts.ValidationErrors

	if errs, ok := err.(validator.ValidationErrors); ok {
		for _, e := range errs {
			validationErrors = append(validationErrors, contracts.ValidationError{
				Field:   "value",
				Tag:     e.Tag(),
				Value:   e.Value(),
				Message: d.formatMessage(e),
			})
		}
	}

	return validationErrors
}

// RegisterValidation registers a custom validation function
func (d *Driver) RegisterValidation(tag string, fn contracts.ValidationFunc) error {
	return d.validate.RegisterValidation(tag, func(fl validator.FieldLevel) bool {
		return fn(fl.Field().Interface())
	})
}

// RegisterTranslation registers a custom error message for a tag
func (d *Driver) RegisterTranslation(tag string, message string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.translations[tag] = message
	return nil
}

// formatMessage formats the error message for a validation error
func (d *Driver) formatMessage(e validator.FieldError) string {
	d.mu.RLock()
	template, ok := d.translations[e.Tag()]
	d.mu.RUnlock()

	if !ok {
		template = "{field} failed validation for '{tag}'"
	}

	message := template
	message = strings.ReplaceAll(message, "{field}", e.Field())
	message = strings.ReplaceAll(message, "{tag}", e.Tag())
	message = strings.ReplaceAll(message, "{param}", e.Param())
	message = strings.ReplaceAll(message, "{value}", formatValue(e.Value()))

	return message
}

func formatValue(v any) string {
	if v == nil {
		return "nil"
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Ensure Driver implements contracts.Validator
var _ contracts.Validator = (*Driver)(nil)
