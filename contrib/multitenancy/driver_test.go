package multitenancy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewDriver(t *testing.T) {
	cfg := &Config{
		Strategy:   StrategyHeader,
		HeaderName: "X-Tenant",
	}

	driver := NewDriver(cfg)

	if driver == nil {
		t.Fatal("expected driver to be non-nil")
	}

	if driver.config.HeaderName != "X-Tenant" {
		t.Errorf("expected header name X-Tenant, got %s", driver.config.HeaderName)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Strategy != StrategyHeader {
		t.Errorf("expected default strategy to be header, got %s", cfg.Strategy)
	}

	if cfg.HeaderName != "X-Tenant-ID" {
		t.Errorf("expected default header name X-Tenant-ID, got %s", cfg.HeaderName)
	}
}

func TestResolveTenant_FromHeader(t *testing.T) {
	driver := NewDriver(&Config{
		Strategy:   StrategyHeader,
		HeaderName: "X-Tenant-ID",
	})

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("X-Tenant-ID", "tenant1")

	tenantID, err := driver.ResolveTenant(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenantID != "tenant1" {
		t.Errorf("expected tenant1, got %s", tenantID)
	}
}

func TestResolveTenant_FromHeaderMissing(t *testing.T) {
	driver := NewDriver(&Config{
		Strategy:   StrategyHeader,
		HeaderName: "X-Tenant-ID",
	})

	req := httptest.NewRequest("GET", "/users", nil)

	_, err := driver.ResolveTenant(req)
	if err == nil {
		t.Error("expected error for missing header")
	}
}

func TestResolveTenant_FromHeaderWithDefault(t *testing.T) {
	driver := NewDriver(&Config{
		Strategy:      StrategyHeader,
		HeaderName:    "X-Tenant-ID",
		AllowMissing:  true,
		DefaultTenant: "default",
	})

	req := httptest.NewRequest("GET", "/users", nil)

	tenantID, err := driver.ResolveTenant(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenantID != "default" {
		t.Errorf("expected default, got %s", tenantID)
	}
}

func TestResolveTenant_FromSubdomain(t *testing.T) {
	driver := NewDriver(&Config{
		Strategy: StrategySubdomain,
	})

	req := httptest.NewRequest("GET", "http://tenant1.example.com/users", nil)

	tenantID, err := driver.ResolveTenant(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenantID != "tenant1" {
		t.Errorf("expected tenant1, got %s", tenantID)
	}
}

func TestResolveTenant_FromSubdomainWithPort(t *testing.T) {
	driver := NewDriver(&Config{
		Strategy: StrategySubdomain,
	})

	req := httptest.NewRequest("GET", "http://tenant2.example.com:8080/users", nil)

	tenantID, err := driver.ResolveTenant(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenantID != "tenant2" {
		t.Errorf("expected tenant2, got %s", tenantID)
	}
}

func TestResolveTenant_FromPath(t *testing.T) {
	driver := NewDriver(&Config{
		Strategy: StrategyPath,
	})

	req := httptest.NewRequest("GET", "/tenant1/users", nil)

	tenantID, err := driver.ResolveTenant(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenantID != "tenant1" {
		t.Errorf("expected tenant1, got %s", tenantID)
	}
}

func TestResolveTenant_CustomResolver(t *testing.T) {
	driver := NewDriver(&Config{
		Strategy: StrategyCustom,
		Resolver: func(r *http.Request) (string, error) {
			// Custom logic: read from query param
			tenantID := r.URL.Query().Get("tenant")
			if tenantID == "" {
				return "", fmt.Errorf("tenant query param not found")
			}
			return tenantID, nil
		},
	})

	req := httptest.NewRequest("GET", "/users?tenant=custom-tenant", nil)

	tenantID, err := driver.ResolveTenant(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenantID != "custom-tenant" {
		t.Errorf("expected custom-tenant, got %s", tenantID)
	}
}

func TestGetTenant_WithoutStore(t *testing.T) {
	driver := NewDriver(&Config{
		Strategy: StrategyHeader,
	})

	tenant, err := driver.GetTenant(context.Background(), "tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenant.ID != "tenant1" {
		t.Errorf("expected tenant1, got %s", tenant.ID)
	}

	if !tenant.Active {
		t.Error("expected tenant to be active")
	}
}

func TestGetTenant_WithStore(t *testing.T) {
	store := &mockTenantStore{
		tenants: map[string]*Tenant{
			"tenant1": {
				ID:       "tenant1",
				Name:     "Tenant One",
				Active:   true,
				Disabled: false,
			},
		},
	}

	driver := NewDriver(&Config{
		Strategy: StrategyHeader,
		Store:    store,
	})

	tenant, err := driver.GetTenant(context.Background(), "tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenant.Name != "Tenant One" {
		t.Errorf("expected Tenant One, got %s", tenant.Name)
	}
}

func TestGetTenant_Disabled(t *testing.T) {
	store := &mockTenantStore{
		tenants: map[string]*Tenant{
			"tenant1": {
				ID:       "tenant1",
				Name:     "Tenant One",
				Active:   true,
				Disabled: true,
			},
		},
	}

	driver := NewDriver(&Config{
		Strategy: StrategyHeader,
		Store:    store,
	})

	_, err := driver.GetTenant(context.Background(), "tenant1")
	if err == nil {
		t.Error("expected error for disabled tenant")
	}
}

func TestGetTenant_NotActive(t *testing.T) {
	store := &mockTenantStore{
		tenants: map[string]*Tenant{
			"tenant1": {
				ID:       "tenant1",
				Name:     "Tenant One",
				Active:   false,
				Disabled: false,
			},
		},
	}

	driver := NewDriver(&Config{
		Strategy: StrategyHeader,
		Store:    store,
	})

	_, err := driver.GetTenant(context.Background(), "tenant1")
	if err == nil {
		t.Error("expected error for inactive tenant")
	}
}

func TestGetTenantFromRequest(t *testing.T) {
	store := &mockTenantStore{
		tenants: map[string]*Tenant{
			"tenant1": {
				ID:     "tenant1",
				Name:   "Tenant One",
				Active: true,
			},
		},
	}

	driver := NewDriver(&Config{
		Strategy:   StrategyHeader,
		HeaderName: "X-Tenant-ID",
		Store:      store,
	})

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("X-Tenant-ID", "tenant1")

	tenant, err := driver.GetTenantFromRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenant.Name != "Tenant One" {
		t.Errorf("expected Tenant One, got %s", tenant.Name)
	}
}

func TestCreateTenant(t *testing.T) {
	store := &mockTenantStore{
		tenants: make(map[string]*Tenant),
	}

	driver := NewDriver(&Config{
		Strategy: StrategyHeader,
		Store:    store,
	})

	newTenant := &Tenant{
		ID:   "tenant2",
		Name: "Tenant Two",
	}

	err := driver.CreateTenant(context.Background(), newTenant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it was created
	tenant, err := driver.GetTenant(context.Background(), "tenant2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenant.Name != "Tenant Two" {
		t.Errorf("expected Tenant Two, got %s", tenant.Name)
	}

	if !tenant.Active {
		t.Error("expected tenant to be active after creation")
	}
}

func TestUpdateTenant(t *testing.T) {
	store := &mockTenantStore{
		tenants: map[string]*Tenant{
			"tenant1": {
				ID:     "tenant1",
				Name:   "Old Name",
				Active: true,
			},
		},
	}

	driver := NewDriver(&Config{
		Strategy: StrategyHeader,
		Store:    store,
	})

	updatedTenant := &Tenant{
		ID:     "tenant1",
		Name:   "New Name",
		Active: true,
	}

	err := driver.UpdateTenant(context.Background(), updatedTenant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tenant, _ := driver.GetTenant(context.Background(), "tenant1")
	if tenant.Name != "New Name" {
		t.Errorf("expected New Name, got %s", tenant.Name)
	}
}

func TestDeleteTenant(t *testing.T) {
	store := &mockTenantStore{
		tenants: map[string]*Tenant{
			"tenant1": {
				ID:     "tenant1",
				Name:   "Tenant One",
				Active: true,
			},
		},
	}

	driver := NewDriver(&Config{
		Strategy: StrategyHeader,
		Store:    store,
	})

	err := driver.DeleteTenant(context.Background(), "tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = driver.GetTenant(context.Background(), "tenant1")
	if err == nil {
		t.Error("expected error after deleting tenant")
	}
}

func TestListTenants(t *testing.T) {
	store := &mockTenantStore{
		tenants: map[string]*Tenant{
			"tenant1": {ID: "tenant1", Name: "Tenant One", Active: true},
			"tenant2": {ID: "tenant2", Name: "Tenant Two", Active: true},
			"tenant3": {ID: "tenant3", Name: "Tenant Three", Active: false},
		},
	}

	driver := NewDriver(&Config{
		Strategy: StrategyHeader,
		Store:    store,
	})

	tenants, err := driver.ListTenants(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only return active tenants
	if len(tenants) != 2 {
		t.Errorf("expected 2 active tenants, got %d", len(tenants))
	}
}

func TestTenant_HasFeature(t *testing.T) {
	tenant := &Tenant{
		ID: "tenant1",
		Features: map[string]bool{
			"advanced-analytics": true,
			"api-access":         true,
			"custom-branding":    false,
		},
	}

	if !tenant.HasFeature("advanced-analytics") {
		t.Error("expected advanced-analytics to be enabled")
	}

	if !tenant.HasFeature("api-access") {
		t.Error("expected api-access to be enabled")
	}

	if tenant.HasFeature("custom-branding") {
		t.Error("expected custom-branding to be disabled")
	}

	if tenant.HasFeature("non-existent") {
		t.Error("expected non-existent feature to be disabled")
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Test tenant ID
	ctx = SetTenantIDInContext(ctx, "tenant1")
	tenantID, ok := GetTenantIDFromContext(ctx)
	if !ok {
		t.Error("expected tenant ID to be in context")
	}
	if tenantID != "tenant1" {
		t.Errorf("expected tenant1, got %s", tenantID)
	}

	// Test tenant object
	tenant := &Tenant{
		ID:   "tenant1",
		Name: "Tenant One",
	}
	ctx = SetTenantInContext(ctx, tenant)
	retrievedTenant, ok := GetTenantFromContext(ctx)
	if !ok {
		t.Error("expected tenant to be in context")
	}
	if retrievedTenant.Name != "Tenant One" {
		t.Errorf("expected Tenant One, got %s", retrievedTenant.Name)
	}
}

// Mock TenantStore for testing
type mockTenantStore struct {
	tenants map[string]*Tenant
}

func (m *mockTenantStore) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	tenant, exists := m.tenants[id]
	if !exists {
		return nil, fmt.Errorf("tenant not found: %s", id)
	}
	return tenant, nil
}

func (m *mockTenantStore) GetTenantByDomain(ctx context.Context, domain string) (*Tenant, error) {
	for _, tenant := range m.tenants {
		if tenant.Domain == domain {
			return tenant, nil
		}
	}
	return nil, fmt.Errorf("tenant not found for domain: %s", domain)
}

func (m *mockTenantStore) ListTenants(ctx context.Context) ([]*Tenant, error) {
	tenants := make([]*Tenant, 0)
	for _, tenant := range m.tenants {
		if tenant.Active {
			tenants = append(tenants, tenant)
		}
	}
	return tenants, nil
}

func (m *mockTenantStore) CreateTenant(ctx context.Context, tenant *Tenant) error {
	if tenant.ID == "" {
		return fmt.Errorf("tenant ID is required")
	}
	m.tenants[tenant.ID] = tenant
	return nil
}

func (m *mockTenantStore) UpdateTenant(ctx context.Context, tenant *Tenant) error {
	if _, exists := m.tenants[tenant.ID]; !exists {
		return fmt.Errorf("tenant not found: %s", tenant.ID)
	}
	m.tenants[tenant.ID] = tenant
	return nil
}

func (m *mockTenantStore) DeleteTenant(ctx context.Context, id string) error {
	if _, exists := m.tenants[id]; !exists {
		return fmt.Errorf("tenant not found: %s", id)
	}
	delete(m.tenants, id)
	return nil
}
