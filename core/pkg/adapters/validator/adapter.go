// Package validator provides a generic validator adapter
// that wraps any validation library (go-playground/validator, ozzo-validation, etc.)
package validator

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// Driver is the interface that any validator must implement
type Driver interface {
	// Validate validates a struct and returns validation errors
	Validate(v any) error
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   any    `json:"value,omitempty"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	msgs := make([]string, len(e))
	for i, err := range e {
		msgs[i] = err.Message
	}
	return strings.Join(msgs, "; ")
}

// HasErrors returns true if there are validation errors
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// ByField returns errors for a specific field
func (e ValidationErrors) ByField(field string) []ValidationError {
	var result []ValidationError
	for _, err := range e {
		if err.Field == field {
			result = append(result, err)
		}
	}
	return result
}

// ToMap converts errors to map for JSON response
func (e ValidationErrors) ToMap() map[string]string {
	result := make(map[string]string)
	for _, err := range e {
		result[err.Field] = err.Message
	}
	return result
}

// Adapter wraps validation driver
type Adapter struct {
	driver          Driver
	customMessages  map[string]string
	fieldNameMapper func(field string) string
}

// New creates a new validator adapter
func New(driver Driver) *Adapter {
	return &Adapter{
		driver:         driver,
		customMessages: make(map[string]string),
	}
}

// WithMessages sets custom error messages
// Key format: "tag" or "field.tag"
func (a *Adapter) WithMessages(messages map[string]string) *Adapter {
	a.customMessages = messages
	return a
}

// WithFieldNameMapper sets function to map field names (e.g., to JSON tags)
func (a *Adapter) WithFieldNameMapper(mapper func(string) string) *Adapter {
	a.fieldNameMapper = mapper
	return a
}

// Validate validates a struct
func (a *Adapter) Validate(v any) error {
	return a.driver.Validate(v)
}

// ============ Built-in Simple Validator ============

// SimpleValidator provides basic validation without external dependencies
type SimpleValidator struct {
	rules map[string]RuleFunc
}

// RuleFunc is a validation rule function
type RuleFunc func(value any, param string) bool

// NewSimpleValidator creates a new simple validator
func NewSimpleValidator() *SimpleValidator {
	v := &SimpleValidator{
		rules: make(map[string]RuleFunc),
	}

	// Register default rules
	v.RegisterRule("required", ruleRequired)
	v.RegisterRule("email", ruleEmail)
	v.RegisterRule("min", ruleMin)
	v.RegisterRule("max", ruleMax)
	v.RegisterRule("len", ruleLen)
	v.RegisterRule("gte", ruleGte)
	v.RegisterRule("lte", ruleLte)
	v.RegisterRule("gt", ruleGt)
	v.RegisterRule("lt", ruleLt)
	v.RegisterRule("oneof", ruleOneOf)
	v.RegisterRule("url", ruleURL)
	v.RegisterRule("uuid", ruleUUID)
	v.RegisterRule("alpha", ruleAlpha)
	v.RegisterRule("alphanum", ruleAlphaNum)
	v.RegisterRule("numeric", ruleNumeric)

	return v
}

// RegisterRule registers a custom validation rule
func (v *SimpleValidator) RegisterRule(name string, fn RuleFunc) {
	v.rules[name] = fn
}

// Validate validates a struct using `validate` tags
func (v *SimpleValidator) Validate(obj any) error {
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return fmt.Errorf("validation requires a struct")
	}

	var errors ValidationErrors

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		// Get validation tag
		tag := field.Tag.Get("validate")
		if tag == "" || tag == "-" {
			continue
		}

		// Get field name (prefer json tag)
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" && parts[0] != "-" {
				fieldName = parts[0]
			}
		}

		// Parse and validate rules
		rules := strings.Split(tag, ",")
		for _, rule := range rules {
			ruleName, param := parseRule(rule)

			ruleFn, ok := v.rules[ruleName]
			if !ok {
				continue
			}

			if !ruleFn(fieldVal.Interface(), param) {
				errors = append(errors, ValidationError{
					Field:   fieldName,
					Tag:     ruleName,
					Value:   fieldVal.Interface(),
					Message: formatMessage(fieldName, ruleName, param),
				})
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

func parseRule(rule string) (name, param string) {
	parts := strings.SplitN(rule, "=", 2)
	name = parts[0]
	if len(parts) > 1 {
		param = parts[1]
	}
	return
}

func formatMessage(field, rule, param string) string {
	switch rule {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, param)
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, param)
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", field, param)
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, param)
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, param)
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, param)
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, param)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, param)
	case "url":
		return fmt.Sprintf("%s must be a valid URL", field)
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", field)
	case "alpha":
		return fmt.Sprintf("%s must contain only letters", field)
	case "alphanum":
		return fmt.Sprintf("%s must contain only letters and numbers", field)
	case "numeric":
		return fmt.Sprintf("%s must be numeric", field)
	default:
		return fmt.Sprintf("%s failed validation: %s", field, rule)
	}
}

// ============ Validation Rules ============

func ruleRequired(value any, _ string) bool {
	if value == nil {
		return false
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return strings.TrimSpace(v.String()) != ""
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() > 0
	case reflect.Ptr, reflect.Interface:
		return !v.IsNil()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return v.Float() != 0
	case reflect.Bool:
		return true // bool is always "present"
	default:
		return true
	}
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

func ruleEmail(value any, _ string) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	return emailRegex.MatchString(s)
}

func ruleMin(value any, param string) bool {
	min := parseInt(param)
	v := reflect.ValueOf(value)

	switch v.Kind() {
	case reflect.String:
		return len(v.String()) >= min
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() >= min
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() >= int64(min)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() >= uint64(min)
	case reflect.Float32, reflect.Float64:
		return v.Float() >= float64(min)
	default:
		return true
	}
}

func ruleMax(value any, param string) bool {
	max := parseInt(param)
	v := reflect.ValueOf(value)

	switch v.Kind() {
	case reflect.String:
		return len(v.String()) <= max
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() <= max
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() <= int64(max)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() <= uint64(max)
	case reflect.Float32, reflect.Float64:
		return v.Float() <= float64(max)
	default:
		return true
	}
}

func ruleLen(value any, param string) bool {
	length := parseInt(param)
	v := reflect.ValueOf(value)

	switch v.Kind() {
	case reflect.String:
		return len(v.String()) == length
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == length
	default:
		return true
	}
}

func ruleGte(value any, param string) bool {
	return ruleMin(value, param)
}

func ruleLte(value any, param string) bool {
	return ruleMax(value, param)
}

func ruleGt(value any, param string) bool {
	gt := parseInt(param)
	v := reflect.ValueOf(value)

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() > int64(gt)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() > uint64(gt)
	case reflect.Float32, reflect.Float64:
		return v.Float() > float64(gt)
	default:
		return true
	}
}

func ruleLt(value any, param string) bool {
	lt := parseInt(param)
	v := reflect.ValueOf(value)

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() < int64(lt)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() < uint64(lt)
	case reflect.Float32, reflect.Float64:
		return v.Float() < float64(lt)
	default:
		return true
	}
}

func ruleOneOf(value any, param string) bool {
	s := fmt.Sprintf("%v", value)
	options := strings.Split(param, " ")
	for _, opt := range options {
		if s == opt {
			return true
		}
	}
	return false
}

var urlRegex = regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)

func ruleURL(value any, _ string) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	return urlRegex.MatchString(s)
}

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ruleUUID(value any, _ string) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	return uuidRegex.MatchString(s)
}

var alphaRegex = regexp.MustCompile(`^[a-zA-Z]+$`)

func ruleAlpha(value any, _ string) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	return alphaRegex.MatchString(s)
}

var alphaNumRegex = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

func ruleAlphaNum(value any, _ string) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	return alphaNumRegex.MatchString(s)
}

var numericRegex = regexp.MustCompile(`^[0-9]+$`)

func ruleNumeric(value any, _ string) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	return numericRegex.MatchString(s)
}

func parseInt(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n) // Error ignored - returns 0 on parse failure which is acceptable
	return n
}

// ============ go-playground/validator Wrapper ============

// PlaygroundValidator is the interface that go-playground/validator implements
type PlaygroundValidator interface {
	Struct(s any) error
	StructCtx(ctx any, s any) error
}

// PlaygroundValidatorError is the interface for validator.ValidationErrors
type PlaygroundValidatorError interface {
	Field() string
	Tag() string
	Value() any
}

// PlaygroundDriver wraps go-playground/validator
type PlaygroundDriver struct {
	validator PlaygroundValidator
}

// WrapPlayground wraps go-playground/validator
// Usage:
//
//	import "github.com/go-playground/validator/v10"
//	v := validator.New()
//	driver := WrapPlayground(v)
//	adapter := New(driver)
func WrapPlayground(v PlaygroundValidator) *PlaygroundDriver {
	return &PlaygroundDriver{validator: v}
}

func (d *PlaygroundDriver) Validate(v any) error {
	err := d.validator.Struct(v)
	if err == nil {
		return nil
	}

	// Try to convert to ValidationErrors
	// This works if the error is of type validator.ValidationErrors
	if errs, ok := err.(interface{ Error() string }); ok {
		// Return as-is if we can't convert
		return fmt.Errorf("validation failed: %s", errs.Error())
	}

	return err
}

// ============ Helper Functions ============

// Validate validates struct using default simple validator
func Validate(v any) error {
	return NewSimpleValidator().Validate(v)
}

// MustValidate validates struct and panics on error
func MustValidate(v any) {
	if err := Validate(v); err != nil {
		panic(err)
	}
}
