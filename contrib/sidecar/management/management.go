// Package management provides the ManagementServer sidecar:
// a separate HTTP server (default port 9090) for Kubernetes probes,
// Prometheus metrics, and pprof — without polluting the main application port.
package management

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds ManagementServer configuration.
type Config struct {
	// Host and Port for the management server. Default: 0.0.0.0:9090
	Host string
	Port int

	// ReadTimeout for HTTP requests. Default: 5s
	ReadTimeout time.Duration

	// ShutdownTimeout for graceful stop. Default: 5s
	ShutdownTimeout time.Duration

	// EnablePprof enables /debug/pprof/* endpoints. Default: true
	EnablePprof bool

	// EnableMetrics enables the /metrics endpoint. Default: true
	EnableMetrics bool

	// AllowedCIDRs restricts /metrics and /debug/pprof/* to the given CIDR ranges.
	// Health endpoints (/health/*) are always public. Example: []string{"127.0.0.0/8", "10.0.0.0/8"}
	AllowedCIDRs []string

	// BearerToken, if set, requires "Authorization: Bearer <token>" on
	// /metrics and /debug/pprof/* requests. Uses constant-time comparison.
	BearerToken string

	// MetricsCacheTTL controls how long runtime metrics are cached between
	// /metrics scrapes to avoid a stop-the-world ReadMemStats on every request.
	// Default: 5s
	MetricsCacheTTL time.Duration
}

func (c *Config) addr() string {
	host := c.Host
	if host == "" {
		host = "0.0.0.0"
	}
	port := c.Port
	if port == 0 {
		port = 9090
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// HealthStatus represents the health state of a component.
type HealthStatus string

const (
	StatusUp       HealthStatus = "up"
	StatusDown     HealthStatus = "down"
	StatusDegraded HealthStatus = "degraded"
)

// HealthResult holds the outcome of a single health check.
type HealthResult struct {
	Status     HealthStatus   `json:"status"`
	Message    string         `json:"message,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
	DurationMs int64          `json:"duration_ms"`
}

// HealthChecker is a function that performs a single health check.
type HealthChecker func(ctx context.Context) HealthResult

// MetricPoint represents one metric value in Prometheus exposition format.
type MetricPoint struct {
	Name   string            // e.g. "http_requests_total"
	Help   string            // # HELP line
	Type   string            // "counter", "gauge", "histogram"
	Labels map[string]string // label key=value pairs
	Value  float64
}

// MetricProvider is a function that returns a slice of MetricPoints.
type MetricProvider func() []MetricPoint

// ManagementServer is a sidecar that exposes management endpoints on a
// dedicated port, separate from the main application port.
//
// Endpoints:
//
//	GET /health          — aggregate health check (all registered checkers)
//	GET /health/live     — liveness probe (always 200 while process is running)
//	GET /health/ready    — readiness probe (200 when all checkers pass)
//	GET /health/startup  — startup probe (200 after SetStartupComplete() is called)
//	GET /metrics         — Prometheus text format (Go runtime + custom metrics)
//	GET /debug/pprof/*   — Go runtime profiler (when EnablePprof = true)
type ManagementServer struct {
	config   *Config
	mux      *http.ServeMux
	mu       sync.RWMutex
	checkers map[string]HealthChecker
	metrics  []MetricProvider

	// serverMu guards the server field against races between Start and Stop.
	serverMu sync.Mutex
	server   *http.Server

	// metricsMu guards the metrics cache.
	metricsMu     sync.Mutex
	cachedMetrics []MetricPoint
	metricsExpiry time.Time

	ready       atomic.Bool
	startupDone atomic.Bool
}

// New creates a new ManagementServer with the given config.
// Pass nil to use defaults (port 9090, pprof and metrics enabled).
func New(config *Config) *ManagementServer {
	if config == nil {
		config = &Config{}
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 5 * time.Second
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 5 * time.Second
	}

	s := &ManagementServer{
		config:   config,
		mux:      http.NewServeMux(),
		checkers: make(map[string]HealthChecker),
	}

	s.registerRoutes()
	return s
}

// Name implements contracts.Sidecar.
func (s *ManagementServer) Name() string {
	return fmt.Sprintf("management-server(%s)", s.config.addr())
}

// AddChecker registers a named health checker.
// Checkers are called by /health and /health/ready.
func (s *ManagementServer) AddChecker(name string, fn HealthChecker) *ManagementServer {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkers[name] = fn
	return s
}

// AddMetricProvider registers an additional metric provider for the /metrics endpoint.
func (s *ManagementServer) AddMetricProvider(fn MetricProvider) *ManagementServer {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics = append(s.metrics, fn)
	return s
}

// SetReady controls whether the readiness probe returns 200.
// Use this to drain traffic during maintenance or graceful shutdown.
func (s *ManagementServer) SetReady(ready bool) {
	s.ready.Store(ready)
}

// SetStartupComplete marks startup as finished.
// Until called, /health/startup returns 503.
func (s *ManagementServer) SetStartupComplete() {
	s.startupDone.Store(true)
}

// Start implements contracts.Sidecar. Blocks until ctx is cancelled.
func (s *ManagementServer) Start(ctx context.Context) error {
	// Mark ready on start; can be overridden via SetReady(false)
	s.ready.Store(true)

	ln, err := net.Listen("tcp", s.config.addr())
	if err != nil {
		return fmt.Errorf("management server listen %s: %w", s.config.addr(), err)
	}

	srv := &http.Server{
		Handler:     s.mux,
		ReadTimeout: s.config.ReadTimeout,
	}

	// Assign under mutex so Stop() never reads a partially-initialized pointer.
	s.serverMu.Lock()
	s.server = srv
	s.serverMu.Unlock()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

// Stop implements contracts.Sidecar.
func (s *ManagementServer) Stop(ctx context.Context) error {
	// Signal not-ready so the load balancer drains traffic before shutdown
	s.ready.Store(false)

	s.serverMu.Lock()
	srv := s.server
	s.serverMu.Unlock()

	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// ============ Route Registration ============

func (s *ManagementServer) registerRoutes() {
	// Health endpoints are always public — load balancers and K8s probes need them.
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /health/live", s.handleLiveness)
	s.mux.HandleFunc("GET /health/ready", s.handleReadiness)
	s.mux.HandleFunc("GET /health/startup", s.handleStartup)

	// Metrics and pprof endpoints are gated behind optional auth middleware.
	if s.config.EnableMetrics {
		s.mux.HandleFunc("GET /metrics", s.secure(s.handleMetrics))
	}

	if s.config.EnablePprof {
		s.mux.HandleFunc("/debug/pprof/", s.secure(pprof.Index))
		s.mux.HandleFunc("/debug/pprof/cmdline", s.secure(pprof.Cmdline))
		s.mux.HandleFunc("/debug/pprof/profile", s.secure(pprof.Profile))
		s.mux.HandleFunc("/debug/pprof/symbol", s.secure(pprof.Symbol))
		s.mux.HandleFunc("/debug/pprof/trace", s.secure(pprof.Trace))
	}
}

// secure wraps a handler with IP allowlist and bearer token checks.
// Health endpoints are always public; only metrics and pprof are protected.
func (s *ManagementServer) secure(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(s.config.AllowedCIDRs) > 0 && !s.isAllowedIP(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if s.config.BearerToken != "" {
			const prefix = "Bearer "
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, prefix) ||
				subtle.ConstantTimeCompare([]byte(auth[len(prefix):]), []byte(s.config.BearerToken)) != 1 {
				w.Header().Set("WWW-Authenticate", `Bearer realm="management"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

func (s *ManagementServer) isAllowedIP(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, cidr := range s.config.AllowedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// ============ Health Handlers ============

func (s *ManagementServer) handleLiveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    StatusUp,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *ManagementServer) handleStartup(w http.ResponseWriter, r *http.Request) {
	if !s.startupDone.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":    StatusDown,
			"message":   "startup not complete",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    StatusUp,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *ManagementServer) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":    StatusDown,
			"message":   "server marked not ready",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	results := s.runCheckers(r.Context())
	overall, code := aggregateStatus(results)

	writeJSON(w, code, map[string]any{
		"status":     overall,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"components": results,
	})
}

func (s *ManagementServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	results := s.runCheckers(r.Context())
	overall, code := aggregateStatus(results)

	writeJSON(w, code, map[string]any{
		"status":     overall,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"components": results,
	})
}

// ============ Metrics Handler ============

func (s *ManagementServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	points := s.collectRuntimeMetrics()

	s.mu.RLock()
	providers := s.metrics
	s.mu.RUnlock()

	for _, p := range providers {
		points = append(points, p()...)
	}

	for _, pt := range points {
		if pt.Help != "" {
			fmt.Fprintf(w, "# HELP %s %s\n", pt.Name, pt.Help)
		}
		if pt.Type != "" {
			fmt.Fprintf(w, "# TYPE %s %s\n", pt.Name, pt.Type)
		}
		if labelStr := formatLabels(pt.Labels); labelStr != "" {
			fmt.Fprintf(w, "%s{%s} %g\n", pt.Name, labelStr, pt.Value)
		} else {
			fmt.Fprintf(w, "%s %g\n", pt.Name, pt.Value)
		}
	}
}

// collectRuntimeMetrics gathers Go runtime stats in Prometheus exposition format.
// Results are cached for MetricsCacheTTL (default 5s) to avoid a stop-the-world
// runtime.ReadMemStats call on every scrape.
func (s *ManagementServer) collectRuntimeMetrics() []MetricPoint {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	if time.Now().Before(s.metricsExpiry) {
		return s.cachedMetrics
	}

	ttl := s.config.MetricsCacheTTL
	if ttl == 0 {
		ttl = 5 * time.Second
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	points := []MetricPoint{
		{
			Name:  "go_goroutines",
			Help:  "Number of goroutines that currently exist.",
			Type:  "gauge",
			Value: float64(runtime.NumGoroutine()),
		},
		{
			Name:  "go_memstats_alloc_bytes",
			Help:  "Number of bytes allocated and still in use.",
			Type:  "gauge",
			Value: float64(ms.Alloc),
		},
		{
			Name:  "go_memstats_sys_bytes",
			Help:  "Number of bytes obtained from system.",
			Type:  "gauge",
			Value: float64(ms.Sys),
		},
		{
			Name:  "go_memstats_heap_alloc_bytes",
			Help:  "Number of heap bytes allocated and still in use.",
			Type:  "gauge",
			Value: float64(ms.HeapAlloc),
		},
		{
			Name:  "go_memstats_heap_idle_bytes",
			Help:  "Number of heap bytes waiting to be used.",
			Type:  "gauge",
			Value: float64(ms.HeapIdle),
		},
		{
			Name:  "go_memstats_heap_inuse_bytes",
			Help:  "Number of heap bytes that are in use.",
			Type:  "gauge",
			Value: float64(ms.HeapInuse),
		},
		{
			Name:  "go_memstats_stack_inuse_bytes",
			Help:  "Number of bytes in use by the stack allocator.",
			Type:  "gauge",
			Value: float64(ms.StackInuse),
		},
		{
			Name:  "go_gc_cycles_total",
			Help:  "Total number of completed GC cycles.",
			Type:  "counter",
			Value: float64(ms.NumGC),
		},
		{
			Name:   "go_info",
			Help:   "Information about the Go environment.",
			Type:   "gauge",
			Labels: map[string]string{"version": runtime.Version()},
			Value:  1,
		},
	}

	// PauseNs ring buffer is only valid after at least one GC cycle.
	if ms.NumGC > 0 {
		points = append(points, MetricPoint{
			Name:  "go_gc_duration_seconds_last",
			Help:  "Duration of the last garbage collection in seconds.",
			Type:  "gauge",
			Value: float64(ms.PauseNs[(ms.NumGC+255)%256]) / 1e9,
		})
	}

	s.cachedMetrics = points
	s.metricsExpiry = time.Now().Add(ttl)
	return points
}

// formatLabels returns a sorted, Prometheus-escaped label string.
// Deterministic ordering prevents diff noise in time-series storage.
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+`="`+prometheusEscape(labels[k])+`"`)
	}
	return strings.Join(parts, ",")
}

// prometheusEscape escapes backslash, double-quote, and newline per the
// Prometheus text format specification (section 4 of the data model).
func prometheusEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// ============ Helpers ============

func (s *ManagementServer) runCheckers(ctx context.Context) map[string]HealthResult {
	s.mu.RLock()
	checkers := make(map[string]HealthChecker, len(s.checkers))
	for k, v := range s.checkers {
		checkers[k] = v
	}
	s.mu.RUnlock()

	if len(checkers) == 0 {
		return map[string]HealthResult{}
	}

	results := make(map[string]HealthResult, len(checkers))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, checker := range checkers {
		name, checker := name, checker
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			result := checker(checkCtx)
			result.DurationMs = time.Since(start).Milliseconds()

			mu.Lock()
			results[name] = result
			mu.Unlock()
		}()
	}
	wg.Wait()
	return results
}

func aggregateStatus(results map[string]HealthResult) (HealthStatus, int) {
	overall := StatusUp
	for _, r := range results {
		if r.Status == StatusDown {
			return StatusDown, http.StatusServiceUnavailable
		}
		if r.Status == StatusDegraded {
			overall = StatusDegraded
		}
	}
	if overall == StatusDegraded {
		// Degraded still accepts traffic
		return StatusDegraded, http.StatusOK
	}
	return StatusUp, http.StatusOK
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
