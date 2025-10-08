package middleware

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/madcok-co/unicorn"
)

// ============================================
// 1. AUTH MIDDLEWARE (JWT)
// ============================================

type AuthMiddleware struct {
	SecretKey string
	Algorithm string
}

func NewAuthMiddleware(secretKey string) *AuthMiddleware {
	return &AuthMiddleware{
		SecretKey: secretKey,
		Algorithm: "HS256",
	}
}

func (m *AuthMiddleware) Handle(ctx *unicorn.Context, next func() (interface{}, error)) (interface{}, error) {
	// Get token from metadata
	token := ctx.GetMetadataString("authorization")
	if token == "" {
		return nil, errors.New("unauthorized: no token provided")
	}

	// Remove "Bearer " prefix
	token = strings.TrimPrefix(token, "Bearer ")

	// Validate token
	claims, err := m.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	// Set user info in context
	ctx.SetMetadata("user_id", claims["user_id"])
	ctx.SetMetadata("user_email", claims["email"])

	return next()
}

func (m *AuthMiddleware) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.SecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
