// Package oauth2 provides OAuth2/OIDC implementation of the unicorn Authenticator interface.
//
// Supports multiple providers:
//   - Google
//   - GitHub
//   - Microsoft Azure AD
//   - Generic OIDC
//
// Usage:
//
//	import (
//	    "github.com/madcok-co/unicorn/contrib/auth/oauth2"
//	)
//
//	// Using Google OAuth2
//	driver := oauth2.NewDriver(&oauth2.Config{
//	    Provider:     oauth2.ProviderGoogle,
//	    ClientID:     "your-client-id",
//	    ClientSecret: "your-client-secret",
//	    RedirectURL:  "http://localhost:8080/auth/callback",
//	    Scopes:       []string{"email", "profile"},
//	})
//
//	app.SetAuth(driver)
package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
)

// Provider represents OAuth2 provider type
type Provider string

const (
	ProviderGoogle    Provider = "google"
	ProviderGitHub    Provider = "github"
	ProviderMicrosoft Provider = "microsoft"
	ProviderGeneric   Provider = "generic"
)

// Driver implements contracts.Authenticator using OAuth2
type Driver struct {
	config       *Config
	oauth2Config *oauth2.Config
	httpClient   *http.Client
}

// Config for creating a new OAuth2 driver
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
	ValidateToken bool // Validate token with provider
	CacheTTL      time.Duration

	// Custom HTTP client
	HTTPClient *http.Client
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		Provider:      ProviderGoogle,
		Scopes:        []string{"email", "profile"},
		ValidateToken: true,
		CacheTTL:      5 * time.Minute,
	}
}

// NewDriver creates a new OAuth2 driver
func NewDriver(cfg *Config) *Driver {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Set default HTTP client if not provided
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	// Build oauth2 config based on provider
	oauth2Cfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       cfg.Scopes,
	}

	switch cfg.Provider {
	case ProviderGoogle:
		oauth2Cfg.Endpoint = endpoints.Google
		if cfg.UserInfoURL == "" {
			cfg.UserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
		}
	case ProviderGitHub:
		oauth2Cfg.Endpoint = endpoints.GitHub
		if cfg.UserInfoURL == "" {
			cfg.UserInfoURL = "https://api.github.com/user"
		}
	case ProviderMicrosoft:
		oauth2Cfg.Endpoint = endpoints.AzureAD("")
		if cfg.UserInfoURL == "" {
			cfg.UserInfoURL = "https://graph.microsoft.com/v1.0/me"
		}
	case ProviderGeneric:
		oauth2Cfg.Endpoint = oauth2.Endpoint{
			AuthURL:  cfg.AuthURL,
			TokenURL: cfg.TokenURL,
		}
	}

	return &Driver{
		config:       cfg,
		oauth2Config: oauth2Cfg,
		httpClient:   cfg.HTTPClient,
	}
}

// Authenticate validates credentials and returns identity
// For OAuth2, this exchanges authorization code for tokens
func (d *Driver) Authenticate(ctx context.Context, creds contracts.Credentials) (*contracts.Identity, error) {
	if creds.Type != "oauth2" {
		return nil, fmt.Errorf("unsupported credential type: %s", creds.Type)
	}

	// Exchange authorization code for token
	token, err := d.oauth2Config.Exchange(ctx, creds.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	// Get user info from provider
	userInfo, err := d.getUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	// Build identity
	identity := &contracts.Identity{
		ID:        userInfo.ID,
		Type:      "user",
		Name:      userInfo.Name,
		Email:     userInfo.Email,
		ExpiresAt: token.Expiry,
		IssuedAt:  time.Now(),
		Metadata: map[string]any{
			"provider":     string(d.config.Provider),
			"avatar_url":   userInfo.Picture,
			"access_token": token.AccessToken,
		},
	}

	// Add refresh token if available
	if token.RefreshToken != "" {
		identity.Metadata["refresh_token"] = token.RefreshToken
	}

	// Extract scopes
	if scopes, ok := token.Extra("scope").(string); ok {
		identity.Scopes = strings.Split(scopes, " ")
	} else {
		identity.Scopes = d.config.Scopes
	}

	return identity, nil
}

// Validate validates a token/session
func (d *Driver) Validate(ctx context.Context, token string) (*contracts.Identity, error) {
	// Get user info using access token
	userInfo, err := d.getUserInfo(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	identity := &contracts.Identity{
		ID:       userInfo.ID,
		Type:     "user",
		Name:     userInfo.Name,
		Email:    userInfo.Email,
		IssuedAt: time.Now(),
		Metadata: map[string]any{
			"provider":   string(d.config.Provider),
			"avatar_url": userInfo.Picture,
		},
	}

	return identity, nil
}

// Refresh refreshes an expired token
func (d *Driver) Refresh(ctx context.Context, refreshToken string) (*contracts.TokenPair, error) {
	// Create token source from refresh token
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	tokenSource := d.oauth2Config.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return &contracts.TokenPair{
		AccessToken:  newToken.AccessToken,
		RefreshToken: newToken.RefreshToken,
		TokenType:    newToken.TokenType,
		ExpiresIn:    int64(time.Until(newToken.Expiry).Seconds()),
	}, nil
}

// Revoke revokes a token
func (d *Driver) Revoke(ctx context.Context, token string) error {
	// Build revoke URL based on provider
	var revokeURL string
	switch d.config.Provider {
	case ProviderGoogle:
		revokeURL = "https://oauth2.googleapis.com/revoke"
	case ProviderGitHub:
		// GitHub uses DELETE request to revoke
		req, err := http.NewRequestWithContext(ctx, "DELETE",
			"https://api.github.com/applications/"+d.config.ClientID+"/token",
			strings.NewReader(fmt.Sprintf(`{"access_token":"%s"}`, token)))
		if err != nil {
			return fmt.Errorf("failed to create revoke request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.SetBasicAuth(d.config.ClientID, d.config.ClientSecret)

		resp, err := d.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to revoke token: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("failed to revoke token: status %d", resp.StatusCode)
		}
		return nil
	case ProviderMicrosoft:
		revokeURL = "https://login.microsoftonline.com/common/oauth2/v2.0/logout"
	default:
		return fmt.Errorf("revoke not supported for provider: %s", d.config.Provider)
	}

	// Standard revoke request (Google, Microsoft)
	data := url.Values{}
	data.Set("token", token)

	resp, err := d.httpClient.PostForm(revokeURL, data)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to revoke token: status %d", resp.StatusCode)
	}

	return nil
}

// GetAuthURL returns the authorization URL for OAuth2 flow
func (d *Driver) GetAuthURL(state string) string {
	return d.oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// GetConfig returns the OAuth2 config
func (d *Driver) GetConfig() *oauth2.Config {
	return d.oauth2Config
}

// getUserInfo fetches user information from the provider
func (d *Driver) getUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", d.config.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user info: status %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse based on provider
	var userInfo UserInfo
	switch d.config.Provider {
	case ProviderGoogle:
		var googleUser struct {
			ID      string `json:"id"`
			Email   string `json:"email"`
			Name    string `json:"name"`
			Picture string `json:"picture"`
		}
		if err := json.Unmarshal(body, &googleUser); err != nil {
			return nil, err
		}
		userInfo = UserInfo{
			ID:      googleUser.ID,
			Email:   googleUser.Email,
			Name:    googleUser.Name,
			Picture: googleUser.Picture,
		}
	case ProviderGitHub:
		var githubUser struct {
			ID        int64  `json:"id"`
			Login     string `json:"login"`
			Email     string `json:"email"`
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		}
		if err := json.Unmarshal(body, &githubUser); err != nil {
			return nil, err
		}
		userInfo = UserInfo{
			ID:      fmt.Sprintf("%d", githubUser.ID),
			Email:   githubUser.Email,
			Name:    githubUser.Name,
			Picture: githubUser.AvatarURL,
		}
		if userInfo.Name == "" {
			userInfo.Name = githubUser.Login
		}
	case ProviderMicrosoft:
		var msUser struct {
			ID                string `json:"id"`
			UserPrincipalName string `json:"userPrincipalName"`
			DisplayName       string `json:"displayName"`
			Mail              string `json:"mail"`
		}
		if err := json.Unmarshal(body, &msUser); err != nil {
			return nil, err
		}
		userInfo = UserInfo{
			ID:    msUser.ID,
			Email: msUser.Mail,
			Name:  msUser.DisplayName,
		}
		if userInfo.Email == "" {
			userInfo.Email = msUser.UserPrincipalName
		}
	default:
		// Generic OIDC
		var genericUser map[string]any
		if err := json.Unmarshal(body, &genericUser); err != nil {
			return nil, err
		}
		userInfo = UserInfo{
			ID:      getString(genericUser, "sub", "id"),
			Email:   getString(genericUser, "email"),
			Name:    getString(genericUser, "name", "display_name"),
			Picture: getString(genericUser, "picture", "avatar_url"),
		}
	}

	return &userInfo, nil
}

// UserInfo represents user information from OAuth2 provider
type UserInfo struct {
	ID      string
	Email   string
	Name    string
	Picture string
}

// getString gets string value from map with fallback keys
func getString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			if str, ok := val.(string); ok {
				return str
			}
		}
	}
	return ""
}

// Ensure Driver implements contracts.Authenticator
var _ contracts.Authenticator = (*Driver)(nil)
