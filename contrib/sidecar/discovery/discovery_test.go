package discovery

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============ Registration ============

func TestServiceRegistrar_Register_SendsCorrectPayload(t *testing.T) {
	var received consulServiceRegistration
	srv := mockConsul(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/v1/agent/service/register" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	})

	svc := &ServiceDefinition{
		ID:      "payment-svc-1",
		Name:    "payment-service",
		Address: "10.0.0.5",
		Port:    8080,
		Tags:    []string{"v2", "canary"},
	}
	r := NewConsul(svc, &Config{
		ConsulAddr:        srv.URL,
		HeartbeatInterval: time.Hour, // prevent heartbeat during test
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	r.Start(ctx) // blocks until ctx cancelled; registers then waits

	if received.ID != "payment-svc-1" {
		t.Errorf("expected ID payment-svc-1, got %s", received.ID)
	}
	if received.Name != "payment-service" {
		t.Errorf("expected Name payment-service, got %s", received.Name)
	}
	if len(received.Tags) != 2 {
		t.Errorf("expected 2 tags, got %v", received.Tags)
	}
}

func TestServiceRegistrar_Register_ACLToken(t *testing.T) {
	var gotToken string
	srv := mockConsul(t, func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Consul-Token")
		w.WriteHeader(http.StatusOK)
	})

	svc := &ServiceDefinition{Name: "svc", Address: "127.0.0.1", Port: 9090}
	r := NewConsul(svc, &Config{
		ConsulAddr:        srv.URL,
		Token:             "my-acl-token",
		HeartbeatInterval: time.Hour,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	r.Start(ctx)

	if gotToken != "my-acl-token" {
		t.Errorf("expected ACL token in header, got %q", gotToken)
	}
}

func TestServiceRegistrar_Register_ConsulError(t *testing.T) {
	srv := mockConsul(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	svc := &ServiceDefinition{Name: "svc", Address: "127.0.0.1", Port: 9090}
	r := NewConsul(svc, &Config{
		ConsulAddr:        srv.URL,
		HeartbeatInterval: time.Hour,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := r.Start(ctx)
	if err == nil {
		t.Fatal("expected error when Consul returns 500")
	}
}

// ============ Deregistration ============

func TestServiceRegistrar_Deregister_Stop(t *testing.T) {
	var paths []string
	srv := mockConsul(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	})

	svc := &ServiceDefinition{Name: "svc", ID: "svc-1", Address: "127.0.0.1", Port: 9090}
	r := NewConsul(svc, &Config{ConsulAddr: srv.URL, HeartbeatInterval: time.Hour})

	r.Stop(context.Background())

	found := false
	for _, p := range paths {
		if p == "/v1/agent/service/deregister/svc-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected deregister request, got paths: %v", paths)
	}
}

// ============ URL injection protection (C4 fix) ============

func TestServiceRegistrar_ServiceID_URLEncoded(t *testing.T) {
	var gotRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// RequestURI preserves the raw percent-encoding as sent by the client.
		gotRequestURI = r.RequestURI
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	// ID with slashes that could cause path traversal if not encoded.
	svc := &ServiceDefinition{
		Name:    "api",
		ID:      "api/v2/../secret",
		Address: "127.0.0.1",
		Port:    8080,
	}
	r := NewConsul(svc, &Config{ConsulAddr: srv.URL, HeartbeatInterval: time.Hour})
	r.Stop(context.Background())

	// Slashes in the ID must be percent-encoded (%2F) in the request URI so they
	// cannot act as path separators and cause traversal (C4 fix).
	if !strings.Contains(gotRequestURI, "%2F") {
		t.Fatalf("expected percent-encoded slashes in RequestURI (C4 fix), got %q", gotRequestURI)
	}
}

func TestServiceRegistrar_ServiceID_SpecialCharsEncoded(t *testing.T) {
	var gotRawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.RequestURI preserves percent-encoding
		gotRawPath = r.RequestURI
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	svc := &ServiceDefinition{
		Name:    "api",
		ID:      "svc with spaces and /slashes",
		Address: "127.0.0.1",
		Port:    8080,
	}
	r := NewConsul(svc, &Config{ConsulAddr: srv.URL, HeartbeatInterval: time.Hour})
	r.Stop(context.Background())

	// Spaces → %20, slashes → %2F
	if !strings.Contains(gotRawPath, "%20") || !strings.Contains(gotRawPath, "%2F") {
		t.Fatalf("expected percent-encoded ID in URL, got %q", gotRawPath)
	}
}

// ============ TTL heartbeat ============

func TestServiceRegistrar_Heartbeat_PassesTTL(t *testing.T) {
	var checkPaths []string
	srv := mockConsul(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/check/pass/") {
			checkPaths = append(checkPaths, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})

	svc := &ServiceDefinition{
		Name:    "svc",
		ID:      "svc-1",
		Address: "127.0.0.1",
		Port:    9090,
	}
	r := NewConsul(svc, &Config{
		ConsulAddr:        srv.URL,
		HeartbeatInterval: 20 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	r.Start(ctx)

	if len(checkPaths) == 0 {
		t.Fatal("expected at least one TTL heartbeat")
	}
}

// ============ Defaults ============

func TestServiceDefinition_Defaults(t *testing.T) {
	svc := &ServiceDefinition{
		Name:    "api",
		Address: "10.0.0.1",
		Port:    8080,
	}
	svc.defaults()

	if svc.ID == "" {
		t.Fatal("ID should be defaulted to name-address-port")
	}
	if svc.HealthCheckURL == "" {
		t.Fatal("HealthCheckURL should be defaulted")
	}
	if svc.HealthCheckInterval == 0 {
		t.Fatal("HealthCheckInterval should be defaulted")
	}
}

// ============ Name ============

func TestServiceRegistrar_Name(t *testing.T) {
	svc := &ServiceDefinition{Name: "payment", Address: "10.0.0.1", Port: 8080}
	r := NewConsul(svc, nil)
	name := r.Name()
	if !strings.Contains(name, "payment") {
		t.Fatalf("Name() should contain service name, got %q", name)
	}
}

// ============ formatDuration ============

func TestFormatDuration_NonZero(t *testing.T) {
	got := formatDuration(5 * time.Second)
	if got != "5s" {
		t.Fatalf("expected '5s', got %q", got)
	}
}

func TestFormatDuration_Zero(t *testing.T) {
	got := formatDuration(0)
	if got != "" {
		t.Fatalf("expected empty string for zero duration, got %q", got)
	}
}

// ============ Helper ============

func mockConsul(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}
