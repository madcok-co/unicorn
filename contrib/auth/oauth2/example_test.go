package oauth2_test

import (
	"fmt"

	"github.com/madcok-co/unicorn/contrib/auth/oauth2"
	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
	"github.com/madcok-co/unicorn/core/pkg/app"
	"github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// Example showing basic Google OAuth2 integration
func Example() {
	// Initialize OAuth2 driver
	auth := oauth2.NewDriver(&oauth2.Config{
		Provider:     oauth2.ProviderGoogle,
		ClientID:     "your-google-client-id",
		ClientSecret: "your-google-client-secret",
		RedirectURL:  "http://localhost:8080/auth/callback",
		Scopes:       []string{"email", "profile"},
	})

	// Create app
	application := app.New(&app.Config{
		Name:       "oauth-example",
		EnableHTTP: true,
		HTTP:       &httpAdapter.Config{Port: 8080},
	})

	// Set authenticator
	application.SetAuth(auth)

	// Login handler - redirects to OAuth provider
	application.RegisterHandler(Login).
		HTTP("GET", "/auth/login").
		Done()

	// Callback handler - exchanges code for tokens
	application.RegisterHandler(Callback).
		HTTP("GET", "/auth/callback").
		Done()

	// Protected handler - requires authentication
	application.RegisterHandler(GetProfile).
		HTTP("GET", "/profile").
		Done()

	application.Start()
}

// Login redirects user to OAuth provider
func Login(ctx *context.Context, req struct{}) (map[string]string, error) {
	// Get OAuth driver
	auth := ctx.Auth().(*oauth2.Driver)

	// Generate state for CSRF protection
	state := "random-state-value" // In production, use crypto/rand

	// Get authorization URL
	authURL := auth.GetAuthURL(state)

	return map[string]string{
		"redirect_url": authURL,
	}, nil
}

// Callback handles OAuth callback and exchanges code for tokens
type CallbackRequest struct {
	Code  string `query:"code"`
	State string `query:"state"`
}

func Callback(ctx *context.Context, req CallbackRequest) (map[string]any, error) {
	// Validate state (CSRF protection)
	// In production, compare with stored state

	// Exchange code for tokens
	identity, err := ctx.Auth().Authenticate(ctx.Context(), contracts.Credentials{
		Type: "oauth2",
		Code: req.Code,
	})
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Store identity in session/JWT
	// For this example, we'll just return it

	return map[string]any{
		"user_id": identity.ID,
		"email":   identity.Email,
		"name":    identity.Name,
	}, nil
}

// GetProfile returns authenticated user profile
type GetProfileRequest struct {
	Authorization string `header:"Authorization"`
}

func GetProfile(ctx *context.Context, req GetProfileRequest) (*contracts.Identity, error) {
	// Extract token from Authorization header
	token := extractBearerToken(req.Authorization)

	// Validate token and get identity
	identity, err := ctx.Auth().Validate(ctx.Context(), token)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	return identity, nil
}

func extractBearerToken(auth string) string {
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

// Example showing GitHub OAuth2 integration
func Example_gitHub() {
	auth := oauth2.NewDriver(&oauth2.Config{
		Provider:     oauth2.ProviderGitHub,
		ClientID:     "your-github-client-id",
		ClientSecret: "your-github-client-secret",
		RedirectURL:  "http://localhost:8080/auth/github/callback",
		Scopes:       []string{"user:email"},
	})

	application := app.New(&app.Config{
		Name:       "github-oauth",
		EnableHTTP: true,
		HTTP:       &httpAdapter.Config{Port: 8080},
	})

	application.SetAuth(auth)
	// ... register handlers
	application.Start()
}

// Example showing Microsoft Azure AD OAuth2 integration
func Example_microsoft() {
	auth := oauth2.NewDriver(&oauth2.Config{
		Provider:     oauth2.ProviderMicrosoft,
		ClientID:     "your-azure-client-id",
		ClientSecret: "your-azure-client-secret",
		RedirectURL:  "http://localhost:8080/auth/microsoft/callback",
		Scopes:       []string{"User.Read"},
	})

	application := app.New(&app.Config{
		Name:       "azure-oauth",
		EnableHTTP: true,
		HTTP:       &httpAdapter.Config{Port: 8080},
	})

	application.SetAuth(auth)
	// ... register handlers
	application.Start()
}

// Example showing generic OIDC provider
func Example_genericOIDC() {
	auth := oauth2.NewDriver(&oauth2.Config{
		Provider:     oauth2.ProviderGeneric,
		ClientID:     "your-client-id",
		ClientSecret: "your-client-secret",
		RedirectURL:  "http://localhost:8080/auth/callback",
		AuthURL:      "https://auth.example.com/oauth2/authorize",
		TokenURL:     "https://auth.example.com/oauth2/token",
		UserInfoURL:  "https://auth.example.com/oauth2/userinfo",
		Scopes:       []string{"openid", "email", "profile"},
	})

	application := app.New(&app.Config{
		Name:       "generic-oidc",
		EnableHTTP: true,
		HTTP:       &httpAdapter.Config{Port: 8080},
	})

	application.SetAuth(auth)
	// ... register handlers
	application.Start()
}

// Example showing token refresh
func Example_refreshToken() {
	auth := oauth2.NewDriver(&oauth2.Config{
		Provider:     oauth2.ProviderGoogle,
		ClientID:     "your-client-id",
		ClientSecret: "your-client-secret",
	})

	application := app.New(&app.Config{
		Name: "token-refresh",
	})

	application.SetAuth(auth)

	// Handler that refreshes expired token
	application.RegisterHandler(RefreshToken).
		HTTP("POST", "/auth/refresh").
		Done()

	application.Start()
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func RefreshToken(ctx *context.Context, req RefreshTokenRequest) (*contracts.TokenPair, error) {
	// Refresh the token
	newTokens, err := ctx.Auth().Refresh(ctx.Context(), req.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return newTokens, nil
}

// Example showing token revocation (logout)
func Example_revokeToken() {
	auth := oauth2.NewDriver(&oauth2.Config{
		Provider:     oauth2.ProviderGoogle,
		ClientID:     "your-client-id",
		ClientSecret: "your-client-secret",
	})

	application := app.New(&app.Config{
		Name: "token-revoke",
	})

	application.SetAuth(auth)

	// Handler that revokes token (logout)
	application.RegisterHandler(Logout).
		HTTP("POST", "/auth/logout").
		Done()

	application.Start()
}

type LogoutRequest struct {
	AccessToken string `json:"access_token"`
}

func Logout(ctx *context.Context, req LogoutRequest) (map[string]string, error) {
	// Revoke the token
	if err := ctx.Auth().Revoke(ctx.Context(), req.AccessToken); err != nil {
		return nil, fmt.Errorf("failed to revoke token: %w", err)
	}

	return map[string]string{
		"message": "logged out successfully",
	}, nil
}
