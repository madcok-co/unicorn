package contracts

// Validator adalah generic interface untuk validation
type Validator interface {
	// Validate validates a struct based on tags
	Validate(data any) error

	// ValidateField validates a single field
	ValidateField(field any, tag string) error

	// RegisterValidation registers a custom validation
	RegisterValidation(tag string, fn ValidationFunc) error

	// RegisterTranslation registers custom error message
	RegisterTranslation(tag string, message string) error
}

// ValidationFunc adalah function untuk custom validation
type ValidationFunc func(field any) bool

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   any    `json:"value,omitempty"`
	Message string `json:"message"`
}

// ValidationErrors adalah collection of validation errors
type ValidationErrors []ValidationError

// Error implements error interface
func (v ValidationErrors) Error() string {
	if len(v) == 0 {
		return ""
	}
	return v[0].Message
}

// HasErrors returns true if there are validation errors
func (v ValidationErrors) HasErrors() bool {
	return len(v) > 0
}

// GetFieldErrors returns errors for a specific field
func (v ValidationErrors) GetFieldErrors(field string) []ValidationError {
	var errors []ValidationError
	for _, e := range v {
		if e.Field == field {
			errors = append(errors, e)
		}
	}
	return errors
}

// ToMap converts errors to map format
func (v ValidationErrors) ToMap() map[string][]string {
	result := make(map[string][]string)
	for _, e := range v {
		result[e.Field] = append(result[e.Field], e.Message)
	}
	return result
}

// ValidatorConfig untuk konfigurasi validator
type ValidatorConfig struct {
	// Tag name for validation (default: "validate")
	TagName string

	// Use JSON field names in error messages
	UseJSONNames bool

	// Custom messages
	Messages map[string]string
}
