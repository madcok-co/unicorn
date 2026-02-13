// Package versioning provides API versioning helpers for Unicorn Framework.
//
// Supports:
//   - URL-based versioning (/v1/users, /v2/users)
//   - Header-based versioning (Accept: application/vnd.api+json;version=1)
//   - Query parameter versioning (?version=1)
//   - Custom version resolver
//
// Usage:
//
//	import (
//	    "github.com/madcok-co/unicorn/contrib/versioning"
//	)
//
//	// URL-based versioning
//	vm := versioning.NewManager(&versioning.Config{
//	    Strategy: versioning.StrategyURL,
//	    DefaultVersion: "v1",
//	})
//
//	// Register version-specific handlers
//	app.RegisterHandler(GetUserV1).HTTP("GET", "/v1/users/:id").Done()
//	app.RegisterHandler(GetUserV2).HTTP("GET", "/v2/users/:id").Done()
package versioning

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// Strategy represents versioning strategy
type Strategy string

const (
	StrategyURL    Strategy = "url"    // /v1/users
	StrategyHeader Strategy = "header" // Accept: application/vnd.api+json;version=1
	StrategyQuery  Strategy = "query"  // ?version=1
	StrategyCustom Strategy = "custom" // Custom resolver
)

// Manager manages API versioning
type Manager struct {
	config *Config
}

// Config for versioning manager
type Config struct {
	// Versioning strategy
	Strategy Strategy

	// Default version when not specified
	DefaultVersion string

	// Header name for header-based versioning
	HeaderName string // Default: "Accept" or "API-Version"

	// URL prefix pattern (for URL strategy)
	URLPrefix string // e.g., "/api"

	// Query parameter name (for query strategy)
	QueryParam string // Default: "version"

	// Supported versions
	SupportedVersions []string

	// Strict mode (reject unsupported versions)
	StrictMode bool

	// Custom version resolver
	Resolver VersionResolver
}

// VersionResolver is custom function to resolve version from request
type VersionResolver func(r *http.Request) (string, error)

// Version represents API version information
type Version struct {
	Major      int    // Major version number
	Minor      int    // Minor version number
	Patch      int    // Patch version number
	Raw        string // Raw version string (e.g., "v1", "1.0", "2.1.3")
	Normalized string // Normalized version (e.g., "1", "1.0", "2.1.3")
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Strategy:          StrategyURL,
		DefaultVersion:    "v1",
		HeaderName:        "API-Version",
		QueryParam:        "version",
		SupportedVersions: []string{"v1"},
		StrictMode:        false,
	}
}

// NewManager creates a new versioning manager
func NewManager(cfg *Config) *Manager {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if cfg.HeaderName == "" {
		cfg.HeaderName = "API-Version"
	}

	if cfg.QueryParam == "" {
		cfg.QueryParam = "version"
	}

	return &Manager{
		config: cfg,
	}
}

// ResolveVersion resolves API version from HTTP request
func (m *Manager) ResolveVersion(r *http.Request) (string, error) {
	var version string
	var err error

	switch m.config.Strategy {
	case StrategyURL:
		version, err = m.resolveFromURL(r)
	case StrategyHeader:
		version, err = m.resolveFromHeader(r)
	case StrategyQuery:
		version, err = m.resolveFromQuery(r)
	case StrategyCustom:
		if m.config.Resolver == nil {
			return "", fmt.Errorf("custom resolver not configured")
		}
		version, err = m.config.Resolver(r)
	default:
		return "", fmt.Errorf("unknown strategy: %s", m.config.Strategy)
	}

	if err != nil || version == "" {
		version = m.config.DefaultVersion
	}

	// Validate version if strict mode
	if m.config.StrictMode && !m.IsSupported(version) {
		return "", fmt.Errorf("unsupported API version: %s", version)
	}

	return version, nil
}

// resolveFromURL extracts version from URL path
func (m *Manager) resolveFromURL(r *http.Request) (string, error) {
	path := r.URL.Path

	// Remove prefix if configured
	if m.config.URLPrefix != "" {
		path = strings.TrimPrefix(path, m.config.URLPrefix)
	}

	// Match version pattern: /v1/, /v2/, /v1.0/, /v2.1.3/
	re := regexp.MustCompile(`^/v?(\d+(?:\.\d+(?:\.\d+)?)?)(?:/|$)`)
	matches := re.FindStringSubmatch(path)

	if len(matches) > 1 {
		return "v" + matches[1], nil
	}

	return "", fmt.Errorf("version not found in URL")
}

// resolveFromHeader extracts version from HTTP header
func (m *Manager) resolveFromHeader(r *http.Request) (string, error) {
	// Try custom header first (e.g., API-Version: v1)
	version := r.Header.Get(m.config.HeaderName)
	if version != "" {
		return normalizeVersion(version), nil
	}

	// Try Accept header with media type versioning
	// Accept: application/vnd.api+json;version=1
	accept := r.Header.Get("Accept")
	if accept != "" {
		re := regexp.MustCompile(`version=v?(\d+(?:\.\d+(?:\.\d+)?)?)`)
		matches := re.FindStringSubmatch(accept)
		if len(matches) > 1 {
			return "v" + matches[1], nil
		}
	}

	return "", fmt.Errorf("version not found in header")
}

// resolveFromQuery extracts version from query parameter
func (m *Manager) resolveFromQuery(r *http.Request) (string, error) {
	version := r.URL.Query().Get(m.config.QueryParam)
	if version == "" {
		return "", fmt.Errorf("version not found in query")
	}

	return normalizeVersion(version), nil
}

// IsSupported checks if version is supported
func (m *Manager) IsSupported(version string) bool {
	normalized := normalizeVersion(version)

	for _, supported := range m.config.SupportedVersions {
		if normalizeVersion(supported) == normalized {
			return true
		}
	}

	return false
}

// GetLatestVersion returns the latest supported version
func (m *Manager) GetLatestVersion() string {
	if len(m.config.SupportedVersions) == 0 {
		return m.config.DefaultVersion
	}

	// Simple approach: return last in list
	// In production, you might want semantic version comparison
	return m.config.SupportedVersions[len(m.config.SupportedVersions)-1]
}

// ParseVersion parses version string into Version struct
func ParseVersion(versionStr string) (*Version, error) {
	normalized := normalizeVersion(versionStr)

	// Remove 'v' prefix
	numericVersion := strings.TrimPrefix(normalized, "v")

	// Split by dots
	parts := strings.Split(numericVersion, ".")

	v := &Version{
		Raw:        versionStr,
		Normalized: normalized,
	}

	// Parse major version
	if len(parts) > 0 {
		major, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid major version: %s", parts[0])
		}
		v.Major = major
	}

	// Parse minor version
	if len(parts) > 1 {
		minor, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid minor version: %s", parts[1])
		}
		v.Minor = minor
	}

	// Parse patch version
	if len(parts) > 2 {
		patch, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid patch version: %s", parts[2])
		}
		v.Patch = patch
	}

	return v, nil
}

// String returns version as string
func (v *Version) String() string {
	if v.Patch > 0 {
		return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
	}
	if v.Minor > 0 {
		return fmt.Sprintf("v%d.%d", v.Major, v.Minor)
	}
	return fmt.Sprintf("v%d", v.Major)
}

// Compare compares two versions
// Returns: -1 if v < other, 0 if v == other, 1 if v > other
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}

	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}

	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	return 0
}

// IsGreaterThan checks if version is greater than other
func (v *Version) IsGreaterThan(other *Version) bool {
	return v.Compare(other) > 0
}

// IsLessThan checks if version is less than other
func (v *Version) IsLessThan(other *Version) bool {
	return v.Compare(other) < 0
}

// IsEqual checks if version equals other
func (v *Version) IsEqual(other *Version) bool {
	return v.Compare(other) == 0
}

// normalizeVersion normalizes version string
func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)

	// Add 'v' prefix if not present
	if !strings.HasPrefix(version, "v") && !strings.HasPrefix(version, "V") {
		version = "v" + version
	}

	return strings.ToLower(version)
}

// ExtractVersionFromPath extracts version from URL path
func ExtractVersionFromPath(path string) string {
	re := regexp.MustCompile(`^/v?(\d+(?:\.\d+(?:\.\d+)?)?)(?:/|$)`)
	matches := re.FindStringSubmatch(path)

	if len(matches) > 1 {
		return "v" + matches[1]
	}

	return ""
}

// RemoveVersionFromPath removes version from URL path
func RemoveVersionFromPath(path string) string {
	re := regexp.MustCompile(`^/v?\d+(?:\.\d+(?:\.\d+)?)?`)
	return re.ReplaceAllString(path, "")
}

// BuildVersionedPath builds versioned URL path
func BuildVersionedPath(version, path string) string {
	normalized := normalizeVersion(version)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "/" + normalized + path
}

// VersionInfo represents API version information for documentation
type VersionInfo struct {
	Version    string   `json:"version"`
	Status     string   `json:"status"` // "stable", "beta", "deprecated"
	Deprecated bool     `json:"deprecated"`
	SunsetDate string   `json:"sunset_date,omitempty"`
	ChangeLog  string   `json:"changelog,omitempty"`
	Links      []string `json:"links,omitempty"`
}

// Context keys
type contextKey string

const (
	VersionContextKey contextKey = "api_version"
)

// GetVersionFromContext extracts API version from context
func GetVersionFromContext(r *http.Request) (string, bool) {
	version := r.Context().Value(VersionContextKey)
	if version == nil {
		return "", false
	}
	versionStr, ok := version.(string)
	return versionStr, ok
}

// SetVersionInContext stores API version in request context
func SetVersionInContext(r *http.Request, version string) *http.Request {
	ctx := context.WithValue(r.Context(), VersionContextKey, version)
	return r.WithContext(ctx)
}

// DeprecationInfo represents deprecation information
type DeprecationInfo struct {
	Version    string `json:"version"`
	SunsetDate string `json:"sunset_date"`
	Message    string `json:"message"`
	NewVersion string `json:"new_version,omitempty"`
}

// AddDeprecationHeader adds deprecation headers to response
func AddDeprecationHeader(w http.ResponseWriter, info *DeprecationInfo) {
	w.Header().Set("Deprecation", "true")
	if info.SunsetDate != "" {
		w.Header().Set("Sunset", info.SunsetDate)
	}
	if info.NewVersion != "" {
		w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="successor-version"`, info.NewVersion))
	}
}
