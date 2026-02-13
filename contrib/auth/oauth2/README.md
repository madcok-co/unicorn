# OAuth2/OIDC Authentication Driver

Production-ready OAuth2/OIDC implementation for Unicorn Framework supporting multiple providers.

## Supported Providers

- **Google** - Google OAuth2
- **GitHub** - GitHub OAuth2
- **Microsoft** - Azure AD/Microsoft OAuth2
- **Generic OIDC** - Any OIDC-compliant provider

## Installation

```bash
go get github.com/madcok-co/unicorn/contrib/auth/oauth2
go get golang.org/x/oauth2
```

## Quick Start

### Google OAuth2

```go
import (
    "github.com/madcok-co/unicorn/contrib/auth/oauth2"
    "github.com/madcok-co/unicorn/core/pkg/app"
)

func main() {
    // Initialize OAuth2 driver
    auth := oauth2.NewDriver(&oauth2.Config{
        Provider:     oauth2.ProviderGoogle,
        ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
        ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
        RedirectURL:  "http://localhost:8080/auth/callback",
        Scopes:       []string{"email", "profile"},
    })

    // Create app and set auth
    application := app.New(&app.Config{
        Name:       "my-app",
        EnableHTTP: true,
    })
    
    application.SetAuth(auth)
    
    // Register handlers
    application.RegisterHandler(LoginHandler).HTTP("GET", "/auth/login").Done()
    application.RegisterHandler(CallbackHandler).HTTP("GET", "/auth/callback").Done()
    application.RegisterHandler(ProfileHandler).HTTP("GET", "/profile").Done()
    
    application.Start()
}
```

### GitHub OAuth2

```go
auth := oauth2.NewDriver(&oauth2.Config{
    Provider:     oauth2.ProviderGitHub,
    ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
    ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
    RedirectURL:  "http://localhost:8080/auth/github/callback",
    Scopes:       []string{"user:email"},
})
```

### Microsoft Azure AD

```go
auth := oauth2.NewDriver(&oauth2.Config{
    Provider:     oauth2.ProviderMicrosoft,
    ClientID:     os.Getenv("AZURE_CLIENT_ID"),
    ClientSecret: os.Getenv("AZURE_CLIENT_SECRET"),
    RedirectURL:  "http://localhost:8080/auth/microsoft/callback",
    Scopes:       []string{"User.Read"},
})
```

### Generic OIDC Provider

```go
auth := oauth2.NewDriver(&oauth2.Config{
    Provider:     oauth2.ProviderGeneric,
    ClientID:     os.Getenv("OIDC_CLIENT_ID"),
    ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
    RedirectURL:  "http://localhost:8080/auth/callback",
    AuthURL:      "https://auth.example.com/oauth2/authorize",
    TokenURL:     "https://auth.example.com/oauth2/token",
    UserInfoURL:  "https://auth.example.com/oauth2/userinfo",
    Scopes:       []string{"openid", "email", "profile"},
})
```

## Complete Example

```go
package main

import (
    "fmt"
    "os"
    
    "github.com/madcok-co/unicorn/contrib/auth/oauth2"
    "github.com/madcok-co/unicorn/core/pkg/app"
    "github.com/madcok-co/unicorn/core/pkg/context"
    "github.com/madcok-co/unicorn/core/pkg/contracts"
    httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
)

func main() {
    // Initialize OAuth2 driver
    auth := oauth2.NewDriver(&oauth2.Config{
        Provider:     oauth2.ProviderGoogle,
        ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
        ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
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

    // Register handlers
    application.RegisterHandler(Login).HTTP("GET", "/auth/login").Done()
    application.RegisterHandler(Callback).HTTP("GET", "/auth/callback").Done()
    application.RegisterHandler(Profile).HTTP("GET", "/profile").Done()
    application.RegisterHandler(Refresh).HTTP("POST", "/auth/refresh").Done()
    application.RegisterHandler(Logout).HTTP("POST", "/auth/logout").Done()

    application.Start()
}

// Login redirects user to OAuth provider
func Login(ctx *context.Context, req struct{}) (map[string]string, error) {
    auth := ctx.Auth().(*oauth2.Driver)
    
    // Generate secure random state for CSRF protection
    state := generateSecureState() // implement this properly
    
    // Store state in session/cache for validation
    // ctx.Cache().Set(ctx.Context(), "oauth_state:"+state, "1", 10*time.Minute)
    
    authURL := auth.GetAuthURL(state)
    
    return map[string]string{
        "redirect_url": authURL,
    }, nil
}

// Callback handles OAuth callback
type CallbackRequest struct {
    Code  string `query:"code"`
    State string `query:"state"`
    Error string `query:"error"`
}

func Callback(ctx *context.Context, req CallbackRequest) (map[string]any, error) {
    // Handle OAuth error
    if req.Error != "" {
        return nil, fmt.Errorf("oauth error: %s", req.Error)
    }
    
    // Validate state (CSRF protection)
    // valid := ctx.Cache().Get(ctx.Context(), "oauth_state:"+req.State)
    // if valid == "" {
    //     return nil, fmt.Errorf("invalid state")
    // }
    
    // Exchange code for tokens
    identity, err := ctx.Auth().Authenticate(ctx.Context(), contracts.Credentials{
        Type: "oauth2",
        Code: req.Code,
    })
    if err != nil {
        return nil, fmt.Errorf("authentication failed: %w", err)
    }
    
    // Store identity in session/generate JWT
    // accessToken := identity.Metadata["access_token"].(string)
    // refreshToken := identity.Metadata["refresh_token"].(string)
    
    return map[string]any{
        "user": map[string]any{
            "id":    identity.ID,
            "email": identity.Email,
            "name":  identity.Name,
        },
        "message": "logged in successfully",
    }, nil
}

// Profile returns authenticated user profile
type ProfileRequest struct {
    Authorization string `header:"Authorization"`
}

func Profile(ctx *context.Context, req ProfileRequest) (*contracts.Identity, error) {
    // Extract bearer token
    token := extractBearerToken(req.Authorization)
    if token == "" {
        return nil, fmt.Errorf("missing authorization header")
    }
    
    // Validate token and get identity
    identity, err := ctx.Auth().Validate(ctx.Context(), token)
    if err != nil {
        return nil, fmt.Errorf("unauthorized: %w", err)
    }
    
    return identity, nil
}

// Refresh refreshes expired access token
type RefreshRequest struct {
    RefreshToken string `json:"refresh_token"`
}

func Refresh(ctx *context.Context, req RefreshRequest) (*contracts.TokenPair, error) {
    newTokens, err := ctx.Auth().Refresh(ctx.Context(), req.RefreshToken)
    if err != nil {
        return nil, fmt.Errorf("failed to refresh token: %w", err)
    }
    
    return newTokens, nil
}

// Logout revokes access token
type LogoutRequest struct {
    AccessToken string `json:"access_token"`
}

func Logout(ctx *context.Context, req LogoutRequest) (map[string]string, error) {
    if err := ctx.Auth().Revoke(ctx.Context(), req.AccessToken); err != nil {
        return nil, fmt.Errorf("failed to revoke token: %w", err)
    }
    
    return map[string]string{
        "message": "logged out successfully",
    }, nil
}

func extractBearerToken(auth string) string {
    if len(auth) > 7 && auth[:7] == "Bearer " {
        return auth[7:]
    }
    return ""
}

func generateSecureState() string {
    // Implement secure random string generation
    // Use crypto/rand for production
    return "random-secure-state"
}
```

## Configuration

### Config Options

```go
type Config struct {
    // Provider type (google, github, microsoft, generic)
    Provider Provider

    // OAuth2 credentials
    ClientID     string
    ClientSecret string
    RedirectURL  string

    // Scopes to request
    Scopes []string

    // For generic OIDC provider
    AuthURL     string // Authorization endpoint
    TokenURL    string // Token endpoint
    UserInfoURL string // UserInfo endpoint

    // Token validation
    ValidateToken bool          // Validate token with provider
    CacheTTL      time.Duration // Cache TTL for validation

    // Custom HTTP client
    HTTPClient *http.Client
}
```

### Provider-Specific Scopes

**Google:**
- `email` - Email address
- `profile` - Basic profile info
- `openid` - OpenID Connect

**GitHub:**
- `user` - Full user profile
- `user:email` - Email addresses
- `read:user` - Read user profile

**Microsoft:**
- `User.Read` - Read user profile
- `Mail.Read` - Read user mail
- `Calendars.Read` - Read calendars

## OAuth2 Flow

```
1. User clicks "Login"
   → GET /auth/login
   → Returns redirect URL to OAuth provider

2. User authorizes on provider
   → Provider redirects to callback URL with code

3. App exchanges code for tokens
   → GET /auth/callback?code=xxx&state=yyy
   → Validates state (CSRF protection)
   → Exchanges code for access_token + refresh_token
   → Returns user identity

4. App uses access token
   → GET /profile
   → Header: Authorization: Bearer {access_token}
   → Returns user profile

5. Token expires
   → POST /auth/refresh
   → Body: {"refresh_token": "xxx"}
   → Returns new access_token

6. User logs out
   → POST /auth/logout
   → Body: {"access_token": "xxx"}
   → Revokes token with provider
```

## Security Best Practices

### 1. Always Use HTTPS in Production

```go
RedirectURL: "https://yourdomain.com/auth/callback", // Not http://
```

### 2. Implement CSRF Protection with State

```go
import "crypto/rand"

func generateSecureState() string {
    b := make([]byte, 32)
    rand.Read(b)
    return base64.URLEncoding.EncodeToString(b)
}

// Store state in cache/session
state := generateSecureState()
cache.Set("oauth_state:"+userID, state, 10*time.Minute)

// Validate on callback
storedState := cache.Get("oauth_state:"+userID)
if storedState != req.State {
    return errors.New("invalid state")
}
```

### 3. Store Tokens Securely

```go
// Don't store tokens in localStorage or cookies without encryption
// Use secure, httpOnly cookies or server-side sessions

// Good: Server-side session
session.Set("access_token", token.AccessToken)
session.Set("refresh_token", token.RefreshToken)

// Bad: Client-side localStorage
// localStorage.setItem("access_token", token)
```

### 4. Validate Tokens

```go
auth := oauth2.NewDriver(&oauth2.Config{
    ValidateToken: true, // Always validate in production
    CacheTTL:      5 * time.Minute,
})
```

### 5. Use Minimal Scopes

```go
// Good: Request only what you need
Scopes: []string{"email", "profile"}

// Bad: Request excessive permissions
Scopes: []string{"email", "profile", "drive", "calendar", "contacts"}
```

## Error Handling

```go
identity, err := ctx.Auth().Authenticate(ctx.Context(), creds)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "invalid_grant"):
        // Authorization code expired or invalid
        return nil, fmt.Errorf("authentication failed, please try again")
    case strings.Contains(err.Error(), "access_denied"):
        // User denied authorization
        return nil, fmt.Errorf("access denied by user")
    default:
        // Other errors
        return nil, fmt.Errorf("authentication error: %w", err)
    }
}
```

## Testing

```bash
# Run tests
cd contrib/auth/oauth2
go test -v

# Run tests with coverage
go test -v -cover

# Run specific test
go test -v -run TestValidate_Google
```

## Environment Setup

### Google OAuth2

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing
3. Enable Google+ API
4. Create OAuth 2.0 credentials
5. Set authorized redirect URIs

```bash
export GOOGLE_CLIENT_ID="your-client-id.apps.googleusercontent.com"
export GOOGLE_CLIENT_SECRET="your-client-secret"
```

### GitHub OAuth2

1. Go to GitHub Settings → Developer settings → OAuth Apps
2. Create new OAuth App
3. Set Authorization callback URL

```bash
export GITHUB_CLIENT_ID="your-github-client-id"
export GITHUB_CLIENT_SECRET="your-github-client-secret"
```

### Microsoft Azure AD

1. Go to [Azure Portal](https://portal.azure.com/)
2. Azure Active Directory → App registrations → New registration
3. Add Redirect URI
4. Create client secret

```bash
export AZURE_CLIENT_ID="your-azure-client-id"
export AZURE_CLIENT_SECRET="your-azure-client-secret"
```

## Integration with Middleware

Combine with authorization middleware for protected routes:

```go
import (
    "github.com/madcok-co/unicorn/core/pkg/middleware"
)

// Protected route requiring authentication
application.RegisterHandler(GetUserData).
    HTTP("GET", "/api/user/data").
    Use(middleware.Auth()). // Requires valid auth
    Done()
```

## License

MIT License - see LICENSE file for details
