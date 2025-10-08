// ============================================
// 5. RECOVERY MIDDLEWARE (Panic Handler)
// ============================================
package middleware

import (
	"fmt"

	"github.com/madcok-co/unicorn"
)

type RecoveryMiddleware struct {
	logger unicorn.Logger
}

func NewRecoveryMiddleware(logger unicorn.Logger) *RecoveryMiddleware {
	return &RecoveryMiddleware{
		logger: logger,
	}
}

func (m *RecoveryMiddleware) Handle(ctx *unicorn.Context, next func() (interface{}, error)) (result interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("Panic recovered",
				"request_id", ctx.RequestID(),
				"panic", r,
			)
			err = fmt.Errorf("internal server error: %v", r)
		}
	}()

	return next()
}
