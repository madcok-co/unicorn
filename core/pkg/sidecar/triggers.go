// Package sidecar provides sidecar wrappers for built-in trigger adapters,
// allowing triggers to run as independent sidecar processes alongside the main app.
//
// This enables hybrid deployment where triggers can run either:
//   - Inline with App.Start() (default, simpler)
//   - As sidecars via app.AddSidecar() (advanced, isolation)
//
// Use cases:
//   - Graceful degradation under load (health check survives heavy traffic)
//   - Independent lifecycle (HTTP crash doesn't kill broker consumer)
//   - Runtime add/remove triggers without restart
//   - Debug/profile triggers independently
package sidecar

import (
	"context"

	"github.com/madcok-co/unicorn/core/pkg/adapters/broker"
	"github.com/madcok-co/unicorn/core/pkg/adapters/cron"
	"github.com/madcok-co/unicorn/core/pkg/adapters/http"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"github.com/madcok-co/unicorn/core/pkg/handler"
)

// ============ HTTP Trigger as Sidecar ============

// HTTPSidecar wraps the HTTP adapter as a contracts.Sidecar.
// The underlying Adapter field is exported so the App can configure it
// (e.g. set app-level adapters, add middleware).
type HTTPSidecar struct {
	Adapter *http.Adapter
	name    string
}

// NewHTTPSidecar creates a new HTTP sidecar from a handler registry and HTTP config.
func NewHTTPSidecar(registry *handler.Registry, config *http.Config) *HTTPSidecar {
	return &HTTPSidecar{
		Adapter: http.New(registry, config),
		name:    "http-trigger",
	}
}

// WithName sets a custom name for the sidecar (used in logs).
func (s *HTTPSidecar) WithName(name string) *HTTPSidecar {
	s.name = name
	return s
}

// Name returns the sidecar name.
func (s *HTTPSidecar) Name() string { return s.name }

// Start starts the HTTP server. Blocks until ctx is cancelled.
func (s *HTTPSidecar) Start(ctx context.Context) error { return s.Adapter.Start(ctx) }

// Stop gracefully shuts down the HTTP server.
func (s *HTTPSidecar) Stop(ctx context.Context) error { return s.Adapter.Shutdown(ctx) }

// ============ Broker Trigger as Sidecar ============

// BrokerSidecar wraps the broker adapter as a contracts.Sidecar.
// The underlying Adapter field is exported so the App can configure it.
type BrokerSidecar struct {
	Adapter *broker.Adapter
	name    string
}

// NewBrokerSidecar creates a new broker sidecar from a handler registry and broker.
func NewBrokerSidecar(b contracts.Broker, registry *handler.Registry, config *broker.Config) *BrokerSidecar {
	return &BrokerSidecar{
		Adapter: broker.New(b, registry, config),
		name:    "broker-trigger",
	}
}

// WithName sets a custom name for the sidecar.
func (s *BrokerSidecar) WithName(name string) *BrokerSidecar {
	s.name = name
	return s
}

// Name returns the sidecar name.
func (s *BrokerSidecar) Name() string { return s.name }

// Start starts consuming messages. Blocks until ctx is cancelled.
func (s *BrokerSidecar) Start(ctx context.Context) error { return s.Adapter.Start(ctx) }

// Stop gracefully shuts down the broker consumer.
func (s *BrokerSidecar) Stop(ctx context.Context) error { return s.Adapter.Stop(ctx) }

// ============ Cron Trigger as Sidecar ============

// CronSidecar wraps the cron adapter as a contracts.Sidecar.
// The underlying Adapter field is exported so the App can configure it.
type CronSidecar struct {
	Adapter *cron.Adapter
	name    string
}

// NewCronSidecar creates a new cron sidecar from a handler registry and scheduler.
func NewCronSidecar(registry *handler.Registry, scheduler cron.Scheduler, config *cron.Config) *CronSidecar {
	return &CronSidecar{
		Adapter: cron.New(registry, scheduler, config),
		name:    "cron-trigger",
	}
}

// WithName sets a custom name for the sidecar.
func (s *CronSidecar) WithName(name string) *CronSidecar {
	s.name = name
	return s
}

// Name returns the sidecar name.
func (s *CronSidecar) Name() string { return s.name }

// Start starts the cron scheduler. Blocks until ctx is cancelled.
func (s *CronSidecar) Start(ctx context.Context) error { return s.Adapter.Start(ctx) }

// Stop gracefully shuts down the cron scheduler.
func (s *CronSidecar) Stop(ctx context.Context) error { return s.Adapter.Stop() }
