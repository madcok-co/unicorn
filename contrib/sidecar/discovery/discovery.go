// Package discovery provides the ServiceRegistrar sidecar for automatically
// registering and deregistering a service with a service registry (Consul, etc.)
// on app start and stop. Uses stdlib net/http — no external Consul SDK required.
package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ServiceDefinition describes the service instance to register.
type ServiceDefinition struct {
	// ID is the unique identifier for this instance. Default: Name-Address-Port
	ID string

	// Name is the logical service name (e.g. "payment-service")
	Name string

	// Address is the IP or hostname of this instance
	Address string

	// Port is the main HTTP port of the service
	Port int

	// Tags for filtering in the service mesh (e.g. ["v2", "canary"])
	Tags []string

	// Meta holds arbitrary key-value metadata
	Meta map[string]string

	// HealthCheckURL is polled by Consul for health checks.
	// Defaults to http://Address:Port/health/live
	HealthCheckURL string

	// HealthCheckInterval controls how often Consul polls the health check. Default: 10s
	HealthCheckInterval time.Duration

	// HealthCheckTimeout is the per-check timeout. Default: 5s
	HealthCheckTimeout time.Duration

	// DeregisterAfter is how long Consul waits before removing a continuously unhealthy service. Default: 1m
	DeregisterAfter time.Duration
}

func (s *ServiceDefinition) defaults() {
	if s.ID == "" {
		s.ID = fmt.Sprintf("%s-%s-%d", s.Name, s.Address, s.Port)
	}
	if s.HealthCheckURL == "" {
		s.HealthCheckURL = fmt.Sprintf("http://%s:%d/health/live", s.Address, s.Port)
	}
	if s.HealthCheckInterval == 0 {
		s.HealthCheckInterval = 10 * time.Second
	}
	if s.HealthCheckTimeout == 0 {
		s.HealthCheckTimeout = 5 * time.Second
	}
	if s.DeregisterAfter == 0 {
		s.DeregisterAfter = time.Minute
	}
}

// Config holds ServiceRegistrar configuration.
type Config struct {
	// ConsulAddr is the Consul agent address. Default: http://127.0.0.1:8500
	ConsulAddr string

	// Token is the Consul ACL token (optional)
	Token string

	// HeartbeatInterval for TTL-based checks. Default: 5s
	HeartbeatInterval time.Duration

	// HTTPClient for Consul communication. Default: 10s timeout
	HTTPClient *http.Client
}

// ServiceRegistrar is a sidecar that automatically registers and deregisters
// a service with Consul. Supports HTTP health checks and TTL heartbeats.
type ServiceRegistrar struct {
	config  *Config
	service *ServiceDefinition
	client  *http.Client
}

// NewConsul creates a ServiceRegistrar targeting a Consul agent.
func NewConsul(service *ServiceDefinition, config *Config) *ServiceRegistrar {
	if config == nil {
		config = &Config{}
	}
	if config.ConsulAddr == "" {
		config.ConsulAddr = "http://127.0.0.1:8500"
	}
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 5 * time.Second
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}

	service.defaults()

	return &ServiceRegistrar{
		config:  config,
		service: service,
		client:  config.HTTPClient,
	}
}

// Name implements contracts.Sidecar.
func (r *ServiceRegistrar) Name() string {
	return fmt.Sprintf("service-registrar(%s@%s:%d)", r.service.Name, r.service.Address, r.service.Port)
}

// Start implements contracts.Sidecar. Registers the service then maintains the TTL heartbeat.
func (r *ServiceRegistrar) Start(ctx context.Context) error {
	if err := r.register(ctx); err != nil {
		return fmt.Errorf("consul register %s: %w", r.service.ID, err)
	}

	ticker := time.NewTicker(r.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_ = r.passTTL(ctx)
		}
	}
}

// Stop implements contracts.Sidecar. Deregisters the service from Consul.
func (r *ServiceRegistrar) Stop(ctx context.Context) error {
	return r.deregister(ctx)
}

// ============ Consul HTTP API ============

type consulServiceRegistration struct {
	ID                string            `json:"ID"`
	Name              string            `json:"Name"`
	Address           string            `json:"Address"`
	Port              int               `json:"Port"`
	Tags              []string          `json:"Tags,omitempty"`
	Meta              map[string]string `json:"Meta,omitempty"`
	Check             *consulCheck      `json:"Check,omitempty"`
	EnableTagOverride bool              `json:"EnableTagOverride"`
}

type consulCheck struct {
	HTTP                           string `json:"HTTP,omitempty"`
	Interval                       string `json:"Interval,omitempty"`
	Timeout                        string `json:"Timeout,omitempty"`
	DeregisterCriticalServiceAfter string `json:"DeregisterCriticalServiceAfter,omitempty"`
	TLSSkipVerify                  bool   `json:"TLSSkipVerify,omitempty"`
}

func (r *ServiceRegistrar) register(ctx context.Context) error {
	svc := &consulServiceRegistration{
		ID:      r.service.ID,
		Name:    r.service.Name,
		Address: r.service.Address,
		Port:    r.service.Port,
		Tags:    r.service.Tags,
		Meta:    r.service.Meta,
		Check: &consulCheck{
			HTTP:                           r.service.HealthCheckURL,
			Interval:                       formatDuration(r.service.HealthCheckInterval),
			Timeout:                        formatDuration(r.service.HealthCheckTimeout),
			DeregisterCriticalServiceAfter: formatDuration(r.service.DeregisterAfter),
		},
	}

	body, err := json.Marshal(svc)
	if err != nil {
		return err
	}

	url := r.config.ConsulAddr + "/v1/agent/service/register"
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.config.Token != "" {
		req.Header.Set("X-Consul-Token", r.config.Token)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("consul returned %d", resp.StatusCode)
	}
	return nil
}

func (r *ServiceRegistrar) deregister(ctx context.Context) error {
	url := r.config.ConsulAddr + "/v1/agent/service/deregister/" + r.service.ID
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return err
	}
	if r.config.Token != "" {
		req.Header.Set("X-Consul-Token", r.config.Token)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("consul deregister returned %d", resp.StatusCode)
	}
	return nil
}

// passTTL sends a passing status for the TTL check (used when the service uses TTL-based checks).
func (r *ServiceRegistrar) passTTL(ctx context.Context) error {
	checkID := "service:" + r.service.ID
	url := r.config.ConsulAddr + "/v1/agent/check/pass/" + checkID
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return err
	}
	if r.config.Token != "" {
		req.Header.Set("X-Consul-Token", r.config.Token)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return ""
	}
	return d.String()
}
