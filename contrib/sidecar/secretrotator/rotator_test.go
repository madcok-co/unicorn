package secretrotator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ============ ForceOnStart (P6 fix) ============

func TestRotator_ForceOnStart_False_NoCallbackOnStartup(t *testing.T) {
	// With ForceOnStart=false, OnRotate must NOT fire on the initial fetch
	// even though the previous value was "". (P6 fix)
	var callCount atomic.Int32

	r := New()
	r.Watch(&WatchEntry{
		Name:     "db-password",
		Interval: time.Hour,
		Fetch: func(_ context.Context) (string, error) {
			return "secret123", nil
		},
		OnRotate: func(_ context.Context, _, _, _ string) error {
			callCount.Add(1)
			return nil
		},
		ForceOnStart: false,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	r.Start(ctx)

	if n := callCount.Load(); n != 0 {
		t.Fatalf("ForceOnStart=false: expected 0 OnRotate calls at startup, got %d", n)
	}
}

func TestRotator_ForceOnStart_True_CallbackFiredOnStartup(t *testing.T) {
	// With ForceOnStart=true, OnRotate must fire once on the initial fetch.
	var callCount atomic.Int32
	var gotNewVal string

	r := New()
	r.Watch(&WatchEntry{
		Name:     "api-key",
		Interval: time.Hour,
		Fetch: func(_ context.Context) (string, error) {
			return "initial-key", nil
		},
		OnRotate: func(_ context.Context, _, _, newVal string) error {
			callCount.Add(1)
			gotNewVal = newVal
			return nil
		},
		ForceOnStart: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	r.Start(ctx)

	if n := callCount.Load(); n != 1 {
		t.Fatalf("ForceOnStart=true: expected 1 OnRotate call, got %d", n)
	}
	if gotNewVal != "initial-key" {
		t.Fatalf("expected newVal='initial-key', got %q", gotNewVal)
	}
}

// ============ Value-change callbacks ============

func TestRotator_OnRotate_FiredOnValueChange(t *testing.T) {
	// After the initial fetch, OnRotate must fire when the value changes.
	fetchVals := []string{"v1", "v2"}
	var fetchIdx atomic.Int32

	var rotations []string
	r := New()
	r.Watch(&WatchEntry{
		Name:     "cert",
		Interval: 20 * time.Millisecond,
		Fetch: func(_ context.Context) (string, error) {
			i := int(fetchIdx.Add(1)) - 1
			if i < len(fetchVals) {
				return fetchVals[i], nil
			}
			return fetchVals[len(fetchVals)-1], nil
		},
		OnRotate: func(_ context.Context, _, _, newVal string) error {
			rotations = append(rotations, newVal)
			return nil
		},
		ForceOnStart: false,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	r.Start(ctx)

	// Initial: v1 (no callback), after 20ms: v2 (callback fired)
	if len(rotations) == 0 {
		t.Fatal("expected at least one OnRotate call when value changes")
	}
	if rotations[0] != "v2" {
		t.Fatalf("expected newVal='v2', got %q", rotations[0])
	}
}

func TestRotator_OnRotate_NotFiredWhenValueUnchanged(t *testing.T) {
	var callCount atomic.Int32

	r := New()
	r.Watch(&WatchEntry{
		Name:     "stable",
		Interval: 20 * time.Millisecond,
		Fetch: func(_ context.Context) (string, error) {
			return "same-value", nil // never changes
		},
		OnRotate: func(_ context.Context, _, _, _ string) error {
			callCount.Add(1)
			return nil
		},
		ForceOnStart: false,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	r.Start(ctx)

	if n := callCount.Load(); n != 0 {
		t.Fatalf("expected 0 calls when value is stable, got %d", n)
	}
}

// ============ CurrentValue ============

func TestRotator_CurrentValue(t *testing.T) {
	r := New()
	r.Watch(&WatchEntry{
		Name:     "token",
		Interval: time.Hour,
		Fetch: func(_ context.Context) (string, error) {
			return "bearer-xyz", nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	r.Start(ctx)

	v, ok := r.CurrentValue("token")
	if !ok {
		t.Fatal("expected CurrentValue to be set after Start")
	}
	if v != "bearer-xyz" {
		t.Fatalf("expected 'bearer-xyz', got %q", v)
	}
}

func TestRotator_CurrentValue_MissingKey(t *testing.T) {
	r := New()
	_, ok := r.CurrentValue("nonexistent")
	if ok {
		t.Fatal("expected false for unregistered secret")
	}
}

// ============ Multiple secrets ============

func TestRotator_MultipleSecrets_Independent(t *testing.T) {
	r := New()
	r.Watch(&WatchEntry{
		Name:     "db-pass",
		Interval: time.Hour,
		Fetch:    func(_ context.Context) (string, error) { return "db-secret", nil },
	})
	r.Watch(&WatchEntry{
		Name:     "api-key",
		Interval: time.Hour,
		Fetch:    func(_ context.Context) (string, error) { return "api-secret", nil },
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	r.Start(ctx)

	db, ok1 := r.CurrentValue("db-pass")
	api, ok2 := r.CurrentValue("api-key")

	if !ok1 || db != "db-secret" {
		t.Fatalf("db-pass: ok=%v val=%q", ok1, db)
	}
	if !ok2 || api != "api-secret" {
		t.Fatalf("api-key: ok=%v val=%q", ok2, api)
	}
}

// ============ FetchFromFile ============

func TestFetchFromFile_ReadsContent(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "secret-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("  super-secret\n  ")
	f.Close()

	fetch := FetchFromFile(f.Name())
	val, err := fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// TrimSpace must be applied
	if val != "super-secret" {
		t.Fatalf("expected 'super-secret', got %q", val)
	}
}

func TestFetchFromFile_MissingFile(t *testing.T) {
	fetch := FetchFromFile("/nonexistent/path/secret.txt")
	_, err := fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// ============ FetchFromEnv ============

func TestFetchFromEnv_ReadsValue(t *testing.T) {
	t.Setenv("TEST_SECRET_KEY", "env-value-123")
	fetch := FetchFromEnv("TEST_SECRET_KEY")
	val, err := fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "env-value-123" {
		t.Fatalf("expected 'env-value-123', got %q", val)
	}
}

func TestFetchFromEnv_EmptyVar(t *testing.T) {
	t.Setenv("EMPTY_TEST_KEY", "")
	fetch := FetchFromEnv("EMPTY_TEST_KEY")
	_, err := fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for empty env var")
	}
}

// ============ FetchFromAWSSecretsManager ============

func TestFetchFromAWSSecretsManager_CallsGetter(t *testing.T) {
	var gotSecretID string
	getter := AWSSecretGetter(func(_ context.Context, secretID string) (string, error) {
		gotSecretID = secretID
		return "plain-secret-value", nil
	})

	fetch := FetchFromAWSSecretsManager("my/secret/id", getter)
	val, err := fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSecretID != "my/secret/id" {
		t.Fatalf("expected secretID 'my/secret/id', got %q", gotSecretID)
	}
	if val != "plain-secret-value" {
		t.Fatalf("expected 'plain-secret-value', got %q", val)
	}
}

func TestFetchFromAWSSecretsManager_GetterError(t *testing.T) {
	getter := AWSSecretGetter(func(_ context.Context, _ string) (string, error) {
		return "", fmt.Errorf("access denied")
	})
	fetch := FetchFromAWSSecretsManager("secret/id", getter)
	_, err := fetch(context.Background())
	if err == nil {
		t.Fatal("expected error from getter")
	}
}

// ============ FetchFromAWSSecretsManagerJSON ============

func TestFetchFromAWSSecretsManagerJSON_ExtractsField(t *testing.T) {
	payload := map[string]string{
		"username": "admin",
		"password": "hunter2",
		"host":     "db.us-east-1.rds.amazonaws.com",
	}
	raw, _ := json.Marshal(payload)

	getter := AWSSecretGetter(func(_ context.Context, _ string) (string, error) {
		return string(raw), nil
	})

	fetch := FetchFromAWSSecretsManagerJSON("arn:rds/creds", "password", getter)
	val, err := fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "hunter2" {
		t.Fatalf("expected 'hunter2', got %q", val)
	}
}

func TestFetchFromAWSSecretsManagerJSON_MissingField(t *testing.T) {
	payload, _ := json.Marshal(map[string]string{"username": "admin"})
	getter := AWSSecretGetter(func(_ context.Context, _ string) (string, error) {
		return string(payload), nil
	})

	fetch := FetchFromAWSSecretsManagerJSON("secret", "password", getter)
	_, err := fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for missing field")
	}
}

func TestFetchFromAWSSecretsManagerJSON_InvalidJSON(t *testing.T) {
	getter := AWSSecretGetter(func(_ context.Context, _ string) (string, error) {
		return "not-json", nil
	})
	fetch := FetchFromAWSSecretsManagerJSON("secret", "field", getter)
	_, err := fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ============ FetchFromVault ============

func TestFetchFromVault_OK(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"data": map[string]string{
				"value": "vault-secret-value",
			},
		},
	}
	srv := mockVault(t, http.StatusOK, resp)

	fetch := FetchFromVault(&VaultConfig{
		Addr:       srv.URL,
		Token:      "root",
		SecretPath: "secret/data/myapp",
	})

	val, err := fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "vault-secret-value" {
		t.Fatalf("expected 'vault-secret-value', got %q", val)
	}
}

func TestFetchFromVault_CustomField(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"data": map[string]string{
				"db_password": "db-secret",
				"api_key":     "key-value",
			},
		},
	}
	srv := mockVault(t, http.StatusOK, resp)

	fetch := FetchFromVault(&VaultConfig{
		Addr:       srv.URL,
		Token:      "root",
		SecretPath: "secret/data/myapp",
		Field:      "db_password",
	})

	val, err := fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "db-secret" {
		t.Fatalf("expected 'db-secret', got %q", val)
	}
}

func TestFetchFromVault_ErrorBody_NotLeaked(t *testing.T) {
	// When Vault returns a non-200, the error message must NOT include the
	// response body — it may contain the token or secret data (C3 fix).
	sensitiveBody := "permission denied: token=hvs.SUPER_SECRET"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, sensitiveBody)
	}))
	t.Cleanup(srv.Close)

	fetch := FetchFromVault(&VaultConfig{
		Addr:       srv.URL,
		Token:      "bad-token",
		SecretPath: "secret/data/test",
	})

	_, err := fetch(context.Background())
	if err == nil {
		t.Fatal("expected error on 403")
	}

	if strings.Contains(err.Error(), sensitiveBody) {
		t.Fatalf("error message leaks sensitive response body (C3 fix): %v", err)
	}
	if strings.Contains(err.Error(), "hvs.SUPER_SECRET") {
		t.Fatalf("error message leaks Vault token: %v", err)
	}
}

func TestFetchFromVault_InvalidPath_DotDot(t *testing.T) {
	// SecretPath containing ".." must be rejected up-front (C5 fix).
	fetch := FetchFromVault(&VaultConfig{
		Addr:       "http://127.0.0.1:8200",
		Token:      "root",
		SecretPath: "secret/../../etc/passwd",
	})

	_, err := fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for path with ..")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected 'invalid' in error, got: %v", err)
	}
}

func TestFetchFromVault_InvalidPath_QueryString(t *testing.T) {
	// SecretPath containing "?" must be rejected (C5 fix).
	fetch := FetchFromVault(&VaultConfig{
		Addr:       "http://127.0.0.1:8200",
		Token:      "root",
		SecretPath: "secret/data/myapp?evil=inject",
	})

	_, err := fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for path with query string")
	}
}

func TestFetchFromVault_FieldNotFound(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"data": map[string]string{"other_field": "x"},
		},
	}
	srv := mockVault(t, http.StatusOK, resp)

	fetch := FetchFromVault(&VaultConfig{
		Addr:       srv.URL,
		Token:      "root",
		SecretPath: "secret/data/myapp",
		Field:      "nonexistent",
	})

	_, err := fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for missing field")
	}
}

func TestFetchFromVault_SendsToken(t *testing.T) {
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Vault-Token")
		resp := map[string]any{
			"data": map[string]any{
				"data": map[string]string{"value": "x"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	fetch := FetchFromVault(&VaultConfig{
		Addr:       srv.URL,
		Token:      "hvs.test-token",
		SecretPath: "secret/data/myapp",
	})

	fetch(context.Background())

	if gotToken != "hvs.test-token" {
		t.Fatalf("expected X-Vault-Token header, got %q", gotToken)
	}
}

// ============ SecretRotator name / stop ============

func TestRotator_Name(t *testing.T) {
	r := New()
	r.Watch(&WatchEntry{Name: "s1", Interval: time.Hour, Fetch: func(_ context.Context) (string, error) { return "", nil }})
	r.Watch(&WatchEntry{Name: "s2", Interval: time.Hour, Fetch: func(_ context.Context) (string, error) { return "", nil }})

	name := r.Name()
	if !strings.Contains(name, "2") {
		t.Fatalf("expected name to include count, got %q", name)
	}
}

func TestRotator_Stop_ReturnsNil(t *testing.T) {
	r := New()
	if err := r.Stop(context.Background()); err != nil {
		t.Fatalf("Stop should return nil, got: %v", err)
	}
}

// ============ Helper ============

func mockVault(t *testing.T, status int, body any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		if body != nil {
			json.NewEncoder(w).Encode(body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}
