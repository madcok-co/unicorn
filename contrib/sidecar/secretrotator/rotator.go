// Package secretrotator provides the SecretRotator sidecar for automatically
// rotating credentials without restarting the service. Supports Vault,
// Kubernetes mounted secrets, and custom fetcher functions.
package secretrotator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// FetchFunc retrieves the latest value of a secret.
// Errors cause the rotation to be retried on the next interval.
type FetchFunc func(ctx context.Context) (string, error)

// RotateFunc is called when a secret value changes (or on a forced refresh).
// oldValue is empty on the first fetch (initial load).
// Returning an error logs the failure but does not stop the rotation loop.
type RotateFunc func(ctx context.Context, secretName, oldValue, newValue string) error

// WatchEntry defines a single secret to monitor.
type WatchEntry struct {
	// Name is the secret identifier used in logs and callbacks.
	Name string

	// Interval controls how often the secret is re-fetched. Default: 5m
	Interval time.Duration

	// Fetch retrieves the latest secret value.
	Fetch FetchFunc

	// OnRotate is called when the value changes.
	OnRotate RotateFunc

	// ForceOnStart triggers the rotate callback on startup even if the value is unchanged.
	ForceOnStart bool
}

// SecretRotator is a sidecar that periodically fetches secrets and fires
// rotation callbacks when values change.
type SecretRotator struct {
	entries []*WatchEntry
	mu      sync.RWMutex
	values  map[string]string
}

// New creates a new SecretRotator.
func New() *SecretRotator {
	return &SecretRotator{
		values: make(map[string]string),
	}
}

// Watch registers a secret to monitor.
func (r *SecretRotator) Watch(entry *WatchEntry) *SecretRotator {
	if entry.Interval == 0 {
		entry.Interval = 5 * time.Minute
	}
	r.mu.Lock()
	r.entries = append(r.entries, entry)
	r.mu.Unlock()
	return r
}

// Name implements contracts.Sidecar.
func (r *SecretRotator) Name() string {
	r.mu.RLock()
	n := len(r.entries)
	r.mu.RUnlock()
	return fmt.Sprintf("secret-rotator(%d secrets)", n)
}

// Start implements contracts.Sidecar.
func (r *SecretRotator) Start(ctx context.Context) error {
	r.mu.RLock()
	entries := make([]*WatchEntry, len(r.entries))
	copy(entries, r.entries)
	r.mu.RUnlock()

	var wg sync.WaitGroup
	for _, e := range entries {
		e := e
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.runEntry(ctx, e)
		}()
	}

	wg.Wait()
	return nil
}

// Stop implements contracts.Sidecar.
func (r *SecretRotator) Stop(_ context.Context) error {
	return nil
}

// CurrentValue returns the currently active value for a secret, thread-safe.
func (r *SecretRotator) CurrentValue(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.values[name]
	return v, ok
}

// ============ Per-entry rotation loop ============

func (r *SecretRotator) runEntry(ctx context.Context, e *WatchEntry) {
	// Initial fetch — store the value but only fire OnRotate if ForceOnStart=true.
	// There is no "previous value" on first run, so comparing against "" would
	// always look like a change and break the ForceOnStart=false contract.
	newVal, err := e.Fetch(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[secret-rotator] initial fetch %s: %v\n", e.Name, err)
	} else {
		r.mu.Lock()
		r.values[e.Name] = newVal
		r.mu.Unlock()

		if e.ForceOnStart && e.OnRotate != nil {
			if err := e.OnRotate(ctx, e.Name, "", newVal); err != nil {
				fmt.Fprintf(os.Stderr, "[secret-rotator] rotate callback %s: %v\n", e.Name, err)
			}
		}
	}

	ticker := time.NewTicker(e.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			newVal, err := e.Fetch(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[secret-rotator] fetch %s: %v\n", e.Name, err)
				continue
			}

			r.mu.Lock()
			oldVal := r.values[e.Name]
			changed := oldVal != newVal
			if changed {
				r.values[e.Name] = newVal
			}
			r.mu.Unlock()

			if changed && e.OnRotate != nil {
				if err := e.OnRotate(ctx, e.Name, oldVal, newVal); err != nil {
					fmt.Fprintf(os.Stderr, "[secret-rotator] rotate callback %s: %v\n", e.Name, err)
				}
			}
		}
	}
}

// ============ Built-in FetchFunc factories ============

// FetchFromFile reads a secret from a file (Kubernetes mounted secret / Docker secret).
// The file is re-read on every interval, so it supports Kubernetes secret rotation.
func FetchFromFile(path string) FetchFunc {
	return func(_ context.Context) (string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read secret file %s: %w", path, err)
		}
		return string(bytes.TrimSpace(data)), nil
	}
}

// FetchFromEnv reads a secret from an environment variable.
func FetchFromEnv(envKey string) FetchFunc {
	return func(_ context.Context) (string, error) {
		v := os.Getenv(envKey)
		if v == "" {
			return "", fmt.Errorf("env %s is empty", envKey)
		}
		return v, nil
	}
}

// ============ AWS Secrets Manager ============

// AWSSecretGetter fetches the raw string value of a secret from AWS Secrets Manager.
// Implement this using the AWS SDK — the framework stays SDK-version agnostic:
//
//	import (
//	    "github.com/aws/aws-sdk-go-v2/aws"
//	    "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
//	)
//
//	client := secretsmanager.NewFromConfig(awsCfg)
//	getter := secretrotator.AWSSecretGetter(func(ctx context.Context, secretID string) (string, error) {
//	    out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
//	        SecretId: aws.String(secretID),
//	    })
//	    if err != nil {
//	        return "", err
//	    }
//	    if out.SecretString != nil {
//	        return *out.SecretString, nil
//	    }
//	    return string(out.SecretBinary), nil
//	})
type AWSSecretGetter func(ctx context.Context, secretID string) (string, error)

// FetchFromAWSSecretsManager creates a FetchFunc that retrieves a plain-string
// secret from AWS Secrets Manager. Use this when the secret value is a single string.
//
// For JSON-formatted secrets (multiple fields), use FetchFromAWSSecretsManagerJSON.
func FetchFromAWSSecretsManager(secretID string, getter AWSSecretGetter) FetchFunc {
	return func(ctx context.Context) (string, error) {
		val, err := getter(ctx, secretID)
		if err != nil {
			return "", fmt.Errorf("aws secrets manager get %q: %w", secretID, err)
		}
		return val, nil
	}
}

// FetchFromAWSSecretsManagerJSON creates a FetchFunc that retrieves a specific
// field from a JSON-formatted AWS Secrets Manager secret.
//
// AWS Secrets Manager typically stores RDS/Redshift credentials as JSON:
//
//	{"username": "admin", "password": "s3cr3t", "host": "db.us-east-1.rds.amazonaws.com", "port": "5432"}
//
// Use field = "password" to extract only the password value.
func FetchFromAWSSecretsManagerJSON(secretID, field string, getter AWSSecretGetter) FetchFunc {
	return func(ctx context.Context) (string, error) {
		raw, err := getter(ctx, secretID)
		if err != nil {
			return "", fmt.Errorf("aws secrets manager get %q: %w", secretID, err)
		}
		var data map[string]string
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			return "", fmt.Errorf("parse secret %q as JSON: %w", secretID, err)
		}
		val, ok := data[field]
		if !ok {
			return "", fmt.Errorf("field %q not found in secret %q", field, secretID)
		}
		return val, nil
	}
}

// ============ HashiCorp Vault ============

// VaultConfig holds configuration for FetchFromVault.
type VaultConfig struct {
	// Addr is the Vault server address. Default: http://127.0.0.1:8200
	Addr string

	// Token is the Vault token, typically injected via a Kubernetes service account.
	Token string

	// SecretPath is the KV secret path, e.g. "secret/data/myapp/db"
	SecretPath string

	// Field is the field name within the secret data. Default: "value"
	Field string

	// HTTPClient is optional. Default: 10s timeout
	HTTPClient *http.Client
}

// FetchFromVault fetches a secret from HashiCorp Vault KV v2 via the HTTP API.
// Does not require the Vault SDK — uses stdlib net/http only.
func FetchFromVault(config *VaultConfig) FetchFunc {
	if config.Addr == "" {
		config.Addr = "http://127.0.0.1:8200"
	}
	if config.Field == "" {
		config.Field = "value"
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}

	// Reject paths with traversal sequences or query components up front.
	if strings.Contains(config.SecretPath, "..") || strings.ContainsAny(config.SecretPath, "?#") {
		return func(_ context.Context) (string, error) {
			return "", fmt.Errorf("invalid vault secret path: %q", config.SecretPath)
		}
	}

	return func(ctx context.Context) (string, error) {
		url := config.Addr + "/v1/" + config.SecretPath
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("X-Vault-Token", config.Token)

		resp, err := config.HTTPClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("vault request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			// Discard body — it may contain sensitive data (token, secret values).
			io.Copy(io.Discard, resp.Body)
			return "", fmt.Errorf("vault returned HTTP %d for path %s", resp.StatusCode, config.SecretPath)
		}

		// Vault KV v2 response structure
		var result struct {
			Data struct {
				Data map[string]string `json:"data"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", fmt.Errorf("decode vault response: %w", err)
		}

		val, ok := result.Data.Data[config.Field]
		if !ok {
			return "", fmt.Errorf("field %q not found in vault secret %s", config.Field, config.SecretPath)
		}
		return val, nil
	}
}
