// ============================================
// UNICORN Framework - Validation System
// Struct validation with 15+ built-in rules
// ============================================

package unicorn

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

var (
	// ErrValidationFailed indicates validation failed
	ErrValidationFailed = errors.New("validation failed")

	// Common regex patterns
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	urlRegex   = regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
)

// ValidationError represents a single field validation error.
type ValidationError struct {
	Field   string
	Tag     string
	Message string
	Value   interface{}
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []*ValidationError

// Error implements the error interface.
func (errs ValidationErrors) Error() string {
	if len(errs) == 0 {
		return ""
	}

	var messages []string
	for _, err := range errs {
		messages = append(messages, err.Error())
	}

	return strings.Join(messages, "; ")
}

// Validator handles struct validation.
type Validator struct {
	rules map[string]ValidationRule
}

// ValidationRule represents a validation rule function.
type ValidationRule func(field reflect.Value, param string) error

// NewValidator creates a new validator with built-in rules.
func NewValidator() *Validator {
	v := &Validator{
		rules: make(map[string]ValidationRule),
	}

	// Register built-in rules
	v.registerBuiltinRules()

	return v
}

// registerBuiltinRules registers all built-in validation rules.
func (v *Validator) registerBuiltinRules() {
	v.rules["required"] = v.validateRequired
	v.rules["min"] = v.validateMin
	v.rules["max"] = v.validateMax
	v.rules["len"] = v.validateLen
	v.rules["email"] = v.validateEmail
	v.rules["url"] = v.validateURL
	v.rules["oneof"] = v.validateOneOf
	v.rules["gt"] = v.validateGt
	v.rules["gte"] = v.validateGte
	v.rules["lt"] = v.validateLt
	v.rules["lte"] = v.validateLte
	v.rules["eq"] = v.validateEq
	v.rules["ne"] = v.validateNe
	v.rules["alpha"] = v.validateAlpha
	v.rules["alphanumeric"] = v.validateAlphanumeric
}

// Validate validates a struct based on `validate` tags.
func (v *Validator) Validate(target interface{}) error {
	targetValue := reflect.ValueOf(target)

	// Handle pointer
	if targetValue.Kind() == reflect.Ptr {
		targetValue = targetValue.Elem()
	}

	if targetValue.Kind() != reflect.Struct {
		return errors.New("target must be a struct")
	}

	var validationErrors ValidationErrors
	targetType := targetValue.Type()

	// Iterate through fields
	for i := 0; i < targetValue.NumField(); i++ {
		field := targetValue.Field(i)
		fieldType := targetType.Field(i)

		// Get validate tag
		tag := fieldType.Tag.Get("validate")
		if tag == "" {
			continue
		}

		// Parse validation rules
		rules := strings.Split(tag, ",")

		for _, rule := range rules {
			// Parse rule and parameter
			parts := strings.SplitN(rule, "=", 2)
			ruleName := strings.TrimSpace(parts[0])
			param := ""
			if len(parts) > 1 {
				param = strings.TrimSpace(parts[1])
			}

			// Get validation function
			validateFn, ok := v.rules[ruleName]
			if !ok {
				continue // Skip unknown rules
			}

			// Validate field
			if err := validateFn(field, param); err != nil {
				validationErrors = append(validationErrors, &ValidationError{
					Field:   fieldType.Name,
					Tag:     ruleName,
					Message: err.Error(),
					Value:   field.Interface(),
				})
			}
		}
	}

	if len(validationErrors) > 0 {
		return validationErrors
	}

	return nil
}

// RegisterRule registers a custom validation rule.
func (v *Validator) RegisterRule(name string, rule ValidationRule) {
	v.rules[name] = rule
}

// Built-in validation rules

func (v *Validator) validateRequired(field reflect.Value, param string) error {
	if isZero(field) {
		return errors.New("field is required")
	}
	return nil
}

func (v *Validator) validateMin(field reflect.Value, param string) error {
	min, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return err
	}

	switch field.Kind() {
	case reflect.String:
		if float64(len(field.String())) < min {
			return fmt.Errorf("length must be at least %v", min)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if float64(field.Int()) < min {
			return fmt.Errorf("value must be at least %v", min)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if float64(field.Uint()) < min {
			return fmt.Errorf("value must be at least %v", min)
		}
	case reflect.Float32, reflect.Float64:
		if field.Float() < min {
			return fmt.Errorf("value must be at least %v", min)
		}
	case reflect.Slice, reflect.Map, reflect.Array:
		if float64(field.Len()) < min {
			return fmt.Errorf("length must be at least %v", min)
		}
	}

	return nil
}

func (v *Validator) validateMax(field reflect.Value, param string) error {
	max, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return err
	}

	switch field.Kind() {
	case reflect.String:
		if float64(len(field.String())) > max {
			return fmt.Errorf("length must be at most %v", max)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if float64(field.Int()) > max {
			return fmt.Errorf("value must be at most %v", max)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if float64(field.Uint()) > max {
			return fmt.Errorf("value must be at most %v", max)
		}
	case reflect.Float32, reflect.Float64:
		if field.Float() > max {
			return fmt.Errorf("value must be at most %v", max)
		}
	case reflect.Slice, reflect.Map, reflect.Array:
		if float64(field.Len()) > max {
			return fmt.Errorf("length must be at most %v", max)
		}
	}

	return nil
}

func (v *Validator) validateLen(field reflect.Value, param string) error {
	length, err := strconv.Atoi(param)
	if err != nil {
		return err
	}

	switch field.Kind() {
	case reflect.String:
		if len(field.String()) != length {
			return fmt.Errorf("length must be exactly %v", length)
		}
	case reflect.Slice, reflect.Map, reflect.Array:
		if field.Len() != length {
			return fmt.Errorf("length must be exactly %v", length)
		}
	}

	return nil
}

func (v *Validator) validateEmail(field reflect.Value, param string) error {
	if field.Kind() != reflect.String {
		return errors.New("email validation only works on strings")
	}

	if !emailRegex.MatchString(field.String()) {
		return errors.New("must be a valid email address")
	}

	return nil
}

func (v *Validator) validateURL(field reflect.Value, param string) error {
	if field.Kind() != reflect.String {
		return errors.New("url validation only works on strings")
	}

	if !urlRegex.MatchString(field.String()) {
		return errors.New("must be a valid URL")
	}

	return nil
}

func (v *Validator) validateOneOf(field reflect.Value, param string) error {
	if field.Kind() != reflect.String {
		return errors.New("oneof validation only works on strings")
	}

	options := strings.Split(param, " ")
	value := field.String()

	for _, option := range options {
		if value == option {
			return nil
		}
	}

	return fmt.Errorf("must be one of: %s", param)
}

func (v *Validator) validateGt(field reflect.Value, param string) error {
	target, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return err
	}

	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if float64(field.Int()) <= target {
			return fmt.Errorf("must be greater than %v", target)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if float64(field.Uint()) <= target {
			return fmt.Errorf("must be greater than %v", target)
		}
	case reflect.Float32, reflect.Float64:
		if field.Float() <= target {
			return fmt.Errorf("must be greater than %v", target)
		}
	}

	return nil
}

func (v *Validator) validateGte(field reflect.Value, param string) error {
	target, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return err
	}

	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if float64(field.Int()) < target {
			return fmt.Errorf("must be greater than or equal to %v", target)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if float64(field.Uint()) < target {
			return fmt.Errorf("must be greater than or equal to %v", target)
		}
	case reflect.Float32, reflect.Float64:
		if field.Float() < target {
			return fmt.Errorf("must be greater than or equal to %v", target)
		}
	}

	return nil
}

func (v *Validator) validateLt(field reflect.Value, param string) error {
	target, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return err
	}

	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if float64(field.Int()) >= target {
			return fmt.Errorf("must be less than %v", target)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if float64(field.Uint()) >= target {
			return fmt.Errorf("must be less than %v", target)
		}
	case reflect.Float32, reflect.Float64:
		if field.Float() >= target {
			return fmt.Errorf("must be less than %v", target)
		}
	}

	return nil
}

func (v *Validator) validateLte(field reflect.Value, param string) error {
	target, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return err
	}

	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if float64(field.Int()) > target {
			return fmt.Errorf("must be less than or equal to %v", target)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if float64(field.Uint()) > target {
			return fmt.Errorf("must be less than or equal to %v", target)
		}
	case reflect.Float32, reflect.Float64:
		if field.Float() > target {
			return fmt.Errorf("must be less than or equal to %v", target)
		}
	}

	return nil
}

func (v *Validator) validateEq(field reflect.Value, param string) error {
	target, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return err
	}

	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if float64(field.Int()) != target {
			return fmt.Errorf("must be equal to %v", target)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if float64(field.Uint()) != target {
			return fmt.Errorf("must be equal to %v", target)
		}
	case reflect.Float32, reflect.Float64:
		if field.Float() != target {
			return fmt.Errorf("must be equal to %v", target)
		}
	}

	return nil
}

func (v *Validator) validateNe(field reflect.Value, param string) error {
	target, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return err
	}

	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if float64(field.Int()) == target {
			return fmt.Errorf("must not be equal to %v", target)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if float64(field.Uint()) == target {
			return fmt.Errorf("must not be equal to %v", target)
		}
	case reflect.Float32, reflect.Float64:
		if field.Float() == target {
			return fmt.Errorf("must not be equal to %v", target)
		}
	}

	return nil
}

func (v *Validator) validateAlpha(field reflect.Value, param string) error {
	if field.Kind() != reflect.String {
		return errors.New("alpha validation only works on strings")
	}

	str := field.String()
	for _, r := range str {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return errors.New("must contain only alphabetic characters")
		}
	}

	return nil
}

func (v *Validator) validateAlphanumeric(field reflect.Value, param string) error {
	if field.Kind() != reflect.String {
		return errors.New("alphanumeric validation only works on strings")
	}

	str := field.String()
	for _, r := range str {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return errors.New("must contain only alphanumeric characters")
		}
	}

	return nil
}

// Helper function to check if a value is zero
func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	}
	return false
}

// Global validator instance
var globalValidator = NewValidator()

// Validate validates a struct using the global validator.
func Validate(target interface{}) error {
	return globalValidator.Validate(target)
}

// RegisterValidationRule registers a custom validation rule globally.
func RegisterValidationRule(name string, rule ValidationRule) {
	globalValidator.RegisterRule(name, rule)
}
