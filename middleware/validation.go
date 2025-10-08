// ============================================
// 7. VALIDATION MIDDLEWARE
// ============================================
package middleware

import (
	"fmt"

	"github.com/madcok-co/unicorn"
)

type ValidationMiddleware struct {
	validator *unicorn.Validator
}

func NewValidationMiddleware() *ValidationMiddleware {
	return &ValidationMiddleware{
		validator: unicorn.NewValidator(),
	}
}

func (m *ValidationMiddleware) Handle(ctx *unicorn.Context, next func() (interface{}, error)) (interface{}, error) {
	// Get request from metadata
	request := ctx.GetMetadata("request")
	if request == nil {
		return next()
	}

	// Validate if it's a struct
	if err := m.validator.Validate(request); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return next()
}
