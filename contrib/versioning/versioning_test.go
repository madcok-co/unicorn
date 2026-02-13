package versioning

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Strategy != StrategyURL {
		t.Errorf("expected default strategy to be URL, got %s", cfg.Strategy)
	}

	if cfg.DefaultVersion != "v1" {
		t.Errorf("expected default version to be v1, got %s", cfg.DefaultVersion)
	}

	if len(cfg.SupportedVersions) != 1 {
		t.Errorf("expected 1 supported version, got %d", len(cfg.SupportedVersions))
	}
}

func TestResolveVersion_URL(t *testing.T) {
	manager := NewManager(&Config{
		Strategy:          StrategyURL,
		DefaultVersion:    "v1",
		SupportedVersions: []string{"v1", "v2"},
	})

	tests := []struct {
		path     string
		expected string
	}{
		{"/v1/users", "v1"},
		{"/v2/users", "v2"},
		{"/v1/users/123", "v1"},
		{"/v2.1/posts", "v2.1"},
		{"/v1.0.0/items", "v1.0.0"},
		{"/users", "v1"}, // falls back to default
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		version, err := manager.ResolveVersion(req)

		if err != nil && tt.expected != "v1" {
			t.Errorf("unexpected error for %s: %v", tt.path, err)
		}

		if version != tt.expected {
			t.Errorf("path %s: expected %s, got %s", tt.path, tt.expected, version)
		}
	}
}

func TestResolveVersion_Header(t *testing.T) {
	manager := NewManager(&Config{
		Strategy:       StrategyHeader,
		HeaderName:     "API-Version",
		DefaultVersion: "v1",
	})

	tests := []struct {
		name     string
		header   string
		value    string
		expected string
	}{
		{"API-Version header", "API-Version", "v2", "v2"},
		{"API-Version without v", "API-Version", "2", "v2"},
		{"Accept header with version", "Accept", "application/vnd.api+json;version=2", "v2"},
		{"no header", "", "", "v1"}, // falls back to default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/users", nil)
			if tt.header != "" {
				req.Header.Set(tt.header, tt.value)
			}

			version, _ := manager.ResolveVersion(req)

			if version != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, version)
			}
		})
	}
}

func TestResolveVersion_Query(t *testing.T) {
	manager := NewManager(&Config{
		Strategy:       StrategyQuery,
		QueryParam:     "version",
		DefaultVersion: "v1",
	})

	tests := []struct {
		url      string
		expected string
	}{
		{"/users?version=v2", "v2"},
		{"/users?version=2", "v2"},
		{"/users?version=2.1", "v2.1"},
		{"/users", "v1"}, // falls back to default
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.url, nil)
		version, _ := manager.ResolveVersion(req)

		if version != tt.expected {
			t.Errorf("url %s: expected %s, got %s", tt.url, tt.expected, version)
		}
	}
}

func TestResolveVersion_Custom(t *testing.T) {
	manager := NewManager(&Config{
		Strategy: StrategyCustom,
		Resolver: func(r *http.Request) (string, error) {
			// Custom logic: read from custom header
			version := r.Header.Get("X-Custom-Version")
			if version == "" {
				return "v1", nil
			}
			return version, nil
		},
	})

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("X-Custom-Version", "v3")

	version, err := manager.ResolveVersion(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if version != "v3" {
		t.Errorf("expected v3, got %s", version)
	}
}

func TestIsSupported(t *testing.T) {
	manager := NewManager(&Config{
		SupportedVersions: []string{"v1", "v2", "v2.1"},
	})

	tests := []struct {
		version  string
		expected bool
	}{
		{"v1", true},
		{"v2", true},
		{"v2.1", true},
		{"v3", false},
		{"1", true},   // normalized to v1
		{"2.1", true}, // normalized to v2.1
	}

	for _, tt := range tests {
		result := manager.IsSupported(tt.version)
		if result != tt.expected {
			t.Errorf("IsSupported(%s) = %v, want %v", tt.version, result, tt.expected)
		}
	}
}

func TestResolveVersion_StrictMode(t *testing.T) {
	manager := NewManager(&Config{
		Strategy:          StrategyURL,
		SupportedVersions: []string{"v1", "v2"},
		StrictMode:        true,
	})

	// Valid version
	req := httptest.NewRequest("GET", "/v1/users", nil)
	_, err := manager.ResolveVersion(req)
	if err != nil {
		t.Errorf("unexpected error for supported version: %v", err)
	}

	// Invalid version in strict mode
	req = httptest.NewRequest("GET", "/v99/users", nil)
	_, err = manager.ResolveVersion(req)
	if err == nil {
		t.Error("expected error for unsupported version in strict mode")
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input         string
		expectedMajor int
		expectedMinor int
		expectedPatch int
	}{
		{"v1", 1, 0, 0},
		{"v2", 2, 0, 0},
		{"v1.0", 1, 0, 0},
		{"v2.1", 2, 1, 0},
		{"v1.0.0", 1, 0, 0},
		{"v2.1.3", 2, 1, 3},
		{"1", 1, 0, 0},
		{"2.1.3", 2, 1, 3},
	}

	for _, tt := range tests {
		version, err := ParseVersion(tt.input)
		if err != nil {
			t.Fatalf("ParseVersion(%s) error: %v", tt.input, err)
		}

		if version.Major != tt.expectedMajor {
			t.Errorf("%s: expected major %d, got %d", tt.input, tt.expectedMajor, version.Major)
		}

		if version.Minor != tt.expectedMinor {
			t.Errorf("%s: expected minor %d, got %d", tt.input, tt.expectedMinor, version.Minor)
		}

		if version.Patch != tt.expectedPatch {
			t.Errorf("%s: expected patch %d, got %d", tt.input, tt.expectedPatch, version.Patch)
		}
	}
}

func TestVersion_String(t *testing.T) {
	tests := []struct {
		version  *Version
		expected string
	}{
		{&Version{Major: 1}, "v1"},
		{&Version{Major: 2, Minor: 1}, "v2.1"},
		{&Version{Major: 1, Minor: 0, Patch: 5}, "v1.0.5"},
		{&Version{Major: 2, Minor: 1, Patch: 3}, "v2.1.3"},
	}

	for _, tt := range tests {
		result := tt.version.String()
		if result != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, result)
		}
	}
}

func TestVersion_Compare(t *testing.T) {
	v1 := &Version{Major: 1, Minor: 0, Patch: 0}
	v2 := &Version{Major: 2, Minor: 0, Patch: 0}
	v2_1 := &Version{Major: 2, Minor: 1, Patch: 0}
	v2_1_3 := &Version{Major: 2, Minor: 1, Patch: 3}
	v2_1_5 := &Version{Major: 2, Minor: 1, Patch: 5}

	tests := []struct {
		v1       *Version
		v2       *Version
		expected int
	}{
		{v1, v2, -1},         // 1.0.0 < 2.0.0
		{v2, v1, 1},          // 2.0.0 > 1.0.0
		{v1, v1, 0},          // 1.0.0 == 1.0.0
		{v2, v2_1, -1},       // 2.0.0 < 2.1.0
		{v2_1_3, v2_1_5, -1}, // 2.1.3 < 2.1.5
		{v2_1_5, v2_1_3, 1},  // 2.1.5 > 2.1.3
	}

	for _, tt := range tests {
		result := tt.v1.Compare(tt.v2)
		if result != tt.expected {
			t.Errorf("%s.Compare(%s) = %d, want %d",
				tt.v1.String(), tt.v2.String(), result, tt.expected)
		}
	}
}

func TestVersion_IsGreaterThan(t *testing.T) {
	v1 := &Version{Major: 1}
	v2 := &Version{Major: 2}

	if !v2.IsGreaterThan(v1) {
		t.Error("expected v2 > v1")
	}

	if v1.IsGreaterThan(v2) {
		t.Error("expected v1 not > v2")
	}
}

func TestVersion_IsLessThan(t *testing.T) {
	v1 := &Version{Major: 1}
	v2 := &Version{Major: 2}

	if !v1.IsLessThan(v2) {
		t.Error("expected v1 < v2")
	}

	if v2.IsLessThan(v1) {
		t.Error("expected v2 not < v1")
	}
}

func TestVersion_IsEqual(t *testing.T) {
	v1a := &Version{Major: 1, Minor: 0}
	v1b := &Version{Major: 1, Minor: 0}
	v2 := &Version{Major: 2}

	if !v1a.IsEqual(v1b) {
		t.Error("expected v1a == v1b")
	}

	if v1a.IsEqual(v2) {
		t.Error("expected v1a != v2")
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1", "v1"},
		{"v1", "v1"},
		{"V1", "v1"},
		{"2.1", "v2.1"},
		{"v2.1", "v2.1"},
		{" v1 ", "v1"},
	}

	for _, tt := range tests {
		result := normalizeVersion(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExtractVersionFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/v1/users", "v1"},
		{"/v2/posts", "v2"},
		{"/v2.1/items", "v2.1"},
		{"/v1.0.0/data", "v1.0.0"},
		{"/users", ""},
		{"/api/v1/users", ""},
	}

	for _, tt := range tests {
		result := ExtractVersionFromPath(tt.path)
		if result != tt.expected {
			t.Errorf("ExtractVersionFromPath(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestRemoveVersionFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/v1/users", "/users"},
		{"/v2/posts", "/posts"},
		{"/v2.1/items", "/items"},
		{"/v1.0.0/data", "/data"},
		{"/users", "/users"},
	}

	for _, tt := range tests {
		result := RemoveVersionFromPath(tt.path)
		if result != tt.expected {
			t.Errorf("RemoveVersionFromPath(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestBuildVersionedPath(t *testing.T) {
	tests := []struct {
		version  string
		path     string
		expected string
	}{
		{"v1", "/users", "/v1/users"},
		{"v2", "users", "/v2/users"},
		{"1", "/posts", "/v1/posts"},
		{"v2.1", "/items", "/v2.1/items"},
	}

	for _, tt := range tests {
		result := BuildVersionedPath(tt.version, tt.path)
		if result != tt.expected {
			t.Errorf("BuildVersionedPath(%q, %q) = %q, want %q",
				tt.version, tt.path, result, tt.expected)
		}
	}
}

func TestGetLatestVersion(t *testing.T) {
	manager := NewManager(&Config{
		SupportedVersions: []string{"v1", "v2", "v2.1", "v3"},
		DefaultVersion:    "v1",
	})

	latest := manager.GetLatestVersion()
	if latest != "v3" {
		t.Errorf("expected latest version to be v3, got %s", latest)
	}
}

func TestGetLatestVersion_Empty(t *testing.T) {
	manager := NewManager(&Config{
		SupportedVersions: []string{},
		DefaultVersion:    "v1",
	})

	latest := manager.GetLatestVersion()
	if latest != "v1" {
		t.Errorf("expected default version v1, got %s", latest)
	}
}

func TestParseVersion_Invalid(t *testing.T) {
	tests := []string{
		"vx",
		"v1.x",
		"invalid",
	}

	for _, tt := range tests {
		_, err := ParseVersion(tt)
		if err == nil {
			t.Errorf("expected error for invalid version: %s", tt)
		}
	}
}

func TestResolveFromURL_WithPrefix(t *testing.T) {
	manager := NewManager(&Config{
		Strategy:  StrategyURL,
		URLPrefix: "/api",
	})

	req := httptest.NewRequest("GET", "/api/v2/users", nil)
	version, err := manager.ResolveVersion(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if version != "v2" {
		t.Errorf("expected v2, got %s", version)
	}
}
