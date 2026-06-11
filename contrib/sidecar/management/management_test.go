package management

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============ Health endpoint tests ============

func TestManagementServer_Liveness(t *testing.T) {
	s := New(nil)
	rec := serveHTTP(s, "GET", "/health/live", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	mustDecode(t, rec, &body)
	if body["status"] != string(StatusUp) {
		t.Fatalf("expected status up, got %v", body["status"])
	}
}

func TestManagementServer_Startup_BeforeComplete(t *testing.T) {
	s := New(nil)
	rec := serveHTTP(s, "GET", "/health/startup", nil)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestManagementServer_Startup_AfterComplete(t *testing.T) {
	s := New(nil)
	s.SetStartupComplete()
	rec := serveHTTP(s, "GET", "/health/startup", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestManagementServer_Readiness_NotReady(t *testing.T) {
	s := New(nil)
	s.SetReady(false)
	rec := serveHTTP(s, "GET", "/health/ready", nil)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestManagementServer_Readiness_Ready_NoCheckers(t *testing.T) {
	s := New(nil)
	s.SetReady(true) // simulate post-Start state
	rec := serveHTTP(s, "GET", "/health/ready", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestManagementServer_Readiness_AllUp(t *testing.T) {
	s := New(nil)
	s.SetReady(true)
	s.AddChecker("db", func(_ context.Context) HealthResult {
		return HealthResult{Status: StatusUp}
	})
	s.AddChecker("cache", func(_ context.Context) HealthResult {
		return HealthResult{Status: StatusUp}
	})
	rec := serveHTTP(s, "GET", "/health/ready", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestManagementServer_Readiness_OneDown(t *testing.T) {
	s := New(nil)
	s.SetReady(true)
	s.AddChecker("db", func(_ context.Context) HealthResult {
		return HealthResult{Status: StatusDown, Message: "connection refused"}
	})
	rec := serveHTTP(s, "GET", "/health/ready", nil)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var body map[string]any
	mustDecode(t, rec, &body)
	if body["status"] != string(StatusDown) {
		t.Fatalf("expected status down, got %v", body["status"])
	}
}

func TestManagementServer_Readiness_Degraded(t *testing.T) {
	s := New(nil)
	s.SetReady(true)
	s.AddChecker("db", func(_ context.Context) HealthResult {
		return HealthResult{Status: StatusDegraded}
	})
	rec := serveHTTP(s, "GET", "/health/ready", nil)

	// Degraded still returns 200 so traffic continues
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (degraded is still OK), got %d", rec.Code)
	}
	var body map[string]any
	mustDecode(t, rec, &body)
	if body["status"] != string(StatusDegraded) {
		t.Fatalf("expected status degraded, got %v", body["status"])
	}
}

func TestManagementServer_Health_Aggregate(t *testing.T) {
	s := New(nil)
	s.AddChecker("svc-a", func(_ context.Context) HealthResult {
		return HealthResult{Status: StatusUp}
	})
	s.AddChecker("svc-b", func(_ context.Context) HealthResult {
		return HealthResult{Status: StatusDown, Message: "timeout"}
	})
	rec := serveHTTP(s, "GET", "/health", nil)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when any checker is down, got %d", rec.Code)
	}
	var body map[string]any
	mustDecode(t, rec, &body)
	components, ok := body["components"].(map[string]any)
	if !ok || len(components) != 2 {
		t.Fatalf("expected 2 components, got %v", body["components"])
	}
}

func TestManagementServer_Concurrent_Checkers(t *testing.T) {
	s := New(nil)

	var mu sync.Mutex
	calls := make(map[string]int)
	for i := range 5 {
		name := fmt.Sprintf("svc-%d", i)
		s.AddChecker(name, func(_ context.Context) HealthResult {
			mu.Lock()
			calls[name]++
			mu.Unlock()
			return HealthResult{Status: StatusUp}
		})
	}

	for range 10 {
		rec := serveHTTP(s, "GET", "/health", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	}
}

// ============ Metrics tests ============

func TestManagementServer_Metrics_Enabled(t *testing.T) {
	s := New(&Config{EnableMetrics: true})
	rec := serveHTTP(s, "GET", "/metrics", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Fatalf("expected text/plain content-type, got %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "go_goroutines") {
		t.Fatal("expected go_goroutines in metrics output")
	}
}

func TestManagementServer_Metrics_NotEnabled(t *testing.T) {
	s := New(&Config{EnableMetrics: false})
	rec := serveHTTP(s, "GET", "/metrics", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when metrics disabled, got %d", rec.Code)
	}
}

func TestManagementServer_Metrics_CustomProvider(t *testing.T) {
	s := New(&Config{EnableMetrics: true})
	s.AddMetricProvider(func() []MetricPoint {
		return []MetricPoint{
			{Name: "my_custom_counter", Help: "a custom metric", Type: "counter", Value: 42},
		}
	})
	rec := serveHTTP(s, "GET", "/metrics", nil)

	body := rec.Body.String()
	if !strings.Contains(body, "my_custom_counter 42") {
		t.Fatalf("expected custom metric in output, got:\n%s", body)
	}
	if !strings.Contains(body, "# HELP my_custom_counter") {
		t.Fatalf("expected HELP line in output, got:\n%s", body)
	}
}

func TestManagementServer_Metrics_LabelOrdering(t *testing.T) {
	// Labels must be emitted in sorted order for deterministic output (P2 fix).
	labels := map[string]string{"z": "last", "a": "first", "m": "middle"}
	got := formatLabels(labels)
	want := `a="first",m="middle",z="last"`
	if got != want {
		t.Fatalf("labels not sorted:\n  got  %q\n  want %q", got, want)
	}
}

func TestManagementServer_Metrics_LabelEscaping(t *testing.T) {
	// Label values containing backslash, quote, and newline must be escaped (P3 fix).
	tests := []struct {
		input string
		want  string
	}{
		{`val"ue`, `val\"ue`},
		{`val\ue`, `val\\ue`},
		{"val\nue", `val\nue`},
		{`a"b\c` + "\nd", `a\"b\\c\nd`},
	}
	for _, tc := range tests {
		got := prometheusEscape(tc.input)
		if got != tc.want {
			t.Errorf("escape(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestManagementServer_Metrics_LabelFormatting(t *testing.T) {
	// Full formatting with escaped labels must produce valid Prometheus syntax.
	labels := map[string]string{"env": `prod"dc1`, "region": "us-east-1"}
	got := formatLabels(labels)
	want := `env="prod\"dc1",region="us-east-1"`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestManagementServer_Metrics_Caching(t *testing.T) {
	// Within the TTL, a second /metrics call must return the cached snapshot (P1 fix).
	s := New(&Config{EnableMetrics: true, MetricsCacheTTL: 10 * time.Minute})

	rec1 := serveHTTP(s, "GET", "/metrics", nil)
	rec2 := serveHTTP(s, "GET", "/metrics", nil)

	if rec1.Body.String() != rec2.Body.String() {
		t.Fatal("expected identical cached response on second scrape within TTL")
	}
}

func TestManagementServer_Metrics_CacheExpires(t *testing.T) {
	// After the TTL expires, fresh runtime stats should be collected.
	s := New(&Config{EnableMetrics: true, MetricsCacheTTL: 1 * time.Millisecond})

	s.collectRuntimeMetrics() // populate cache

	time.Sleep(5 * time.Millisecond) // wait for TTL to expire

	// Second call should succeed (no panic, no stale crash)
	points := s.collectRuntimeMetrics()
	if len(points) == 0 {
		t.Fatal("expected non-empty metrics after cache expiry")
	}
}

func TestManagementServer_Metrics_GoInfo_LabelPresent(t *testing.T) {
	s := New(&Config{EnableMetrics: true})
	rec := serveHTTP(s, "GET", "/metrics", nil)

	body := rec.Body.String()
	if !strings.Contains(body, `go_info{version="`) {
		t.Fatalf("expected go_info with version label:\n%s", body)
	}
}

// ============ Auth tests (C1 fix) ============

func TestManagementServer_Auth_BearerToken_Required(t *testing.T) {
	s := New(&Config{EnableMetrics: true, BearerToken: "secret"})

	rec := serveHTTP(s, "GET", "/metrics", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}
}

func TestManagementServer_Auth_BearerToken_Valid(t *testing.T) {
	s := New(&Config{EnableMetrics: true, BearerToken: "secret"})

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid token, got %d", rec.Code)
	}
}

func TestManagementServer_Auth_BearerToken_WrongToken(t *testing.T) {
	s := New(&Config{EnableMetrics: true, BearerToken: "secret"})

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong token, got %d", rec.Code)
	}
}

func TestManagementServer_Auth_BearerToken_BadFormat(t *testing.T) {
	s := New(&Config{EnableMetrics: true, BearerToken: "secret"})

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // wrong scheme
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong scheme, got %d", rec.Code)
	}
}

func TestManagementServer_Auth_AllowedCIDR_Allowed(t *testing.T) {
	s := New(&Config{
		EnableMetrics: true,
		AllowedCIDRs:  []string{"127.0.0.0/8"},
	})

	// httptest.NewRequest default RemoteAddr is "192.0.2.1:1234"
	// We override it to be within the allowed CIDR.
	req := httptest.NewRequest("GET", "/metrics", nil)
	req.RemoteAddr = "127.0.0.1:9999"
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from allowed IP, got %d", rec.Code)
	}
}

func TestManagementServer_Auth_AllowedCIDR_Denied(t *testing.T) {
	s := New(&Config{
		EnableMetrics: true,
		AllowedCIDRs:  []string{"10.0.0.0/8"},
	})

	// httptest default RemoteAddr "192.0.2.1:1234" is outside 10.0.0.0/8
	rec := serveHTTP(s, "GET", "/metrics", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 from denied IP, got %d", rec.Code)
	}
}

func TestManagementServer_Auth_HealthEndpoints_AlwaysPublic(t *testing.T) {
	// Health endpoints must remain accessible even when auth is configured (C1 fix).
	s := New(&Config{BearerToken: "secret"})

	paths := []string{"/health", "/health/live", "/health/ready", "/health/startup"}
	for _, path := range paths {
		rec := serveHTTP(s, "GET", path, nil)
		if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
			t.Errorf("health endpoint %s returned auth error %d — should always be public", path, rec.Code)
		}
	}
}

// ============ Server lifecycle tests (R1 fix) ============

func TestManagementServer_StopBeforeStart_NoopSafe(t *testing.T) {
	// Stop() before Start() must return nil and not panic (R1 fix — nil server deref).
	s := New(nil)
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop before Start should return nil, got: %v", err)
	}
}

func TestManagementServer_StopBeforeStart_Concurrent(t *testing.T) {
	// Verify no data race when Stop is called while Start has not yet assigned s.server.
	s := New(&Config{Port: 0, Host: "127.0.0.1"})

	var wg sync.WaitGroup
	wg.Add(2)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	go func() {
		defer wg.Done()
		_ = s.Start(ctx)
	}()
	go func() {
		defer wg.Done()
		_ = s.Stop(context.Background())
	}()

	wg.Wait()
}

// ============ Helper functions ============

func serveHTTP(s *ManagementServer, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	return rec
}

func mustDecode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
