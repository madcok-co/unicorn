// ============================================
// 3. CORS MIDDLEWARE
// ============================================
package middleware

import (
	"strings"

	"github.com/madcok-co/unicorn"
)

type CORSMiddleware struct {
	AllowOrigins []string
	AllowMethods []string
	AllowHeaders []string
	MaxAge       int
}

func NewCORSMiddleware(origins, methods, headers []string) *CORSMiddleware {
	return &CORSMiddleware{
		AllowOrigins: origins,
		AllowMethods: methods,
		AllowHeaders: headers,
		MaxAge:       3600,
	}
}

func (m *CORSMiddleware) Handle(ctx *unicorn.Context, next func() (interface{}, error)) (interface{}, error) {
	// Set CORS headers in metadata (trigger will apply them)
	ctx.SetMetadata("cors_allow_origins", strings.Join(m.AllowOrigins, ","))
	ctx.SetMetadata("cors_allow_methods", strings.Join(m.AllowMethods, ","))
	ctx.SetMetadata("cors_allow_headers", strings.Join(m.AllowHeaders, ","))
	ctx.SetMetadata("cors_max_age", m.MaxAge)

	return next()
}
