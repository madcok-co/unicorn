package sidecar

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/handler"
)

// ============ HTTPSidecar ============

func TestHTTPSidecar_Name(t *testing.T) {
	registry := handler.NewRegistry()
	s := NewHTTPSidecar(registry, nil)

	if s.Name() != "http-trigger" {
		t.Errorf("expected 'http-trigger', got %s", s.Name())
	}
}

func TestHTTPSidecar_WithName(t *testing.T) {
	registry := handler.NewRegistry()
	s := NewHTTPSidecar(registry, nil)
	s.WithName("public-api")

	if s.Name() != "public-api" {
		t.Errorf("expected 'public-api', got %s", s.Name())
	}
}

func TestHTTPSidecar_StopWithoutStart(t *testing.T) {
	registry := handler.NewRegistry()
	s := NewHTTPSidecar(registry, nil)

	// Stop before Start should not panic
	err := s.Stop(context.Background())
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestHTTPSidecar_StartAndCancel(t *testing.T) {
	registry := handler.NewRegistry()
	cfg := &struct {
		Host         string
		Port         int
		ReadTimeout  time.Duration
		WriteTimeout time.Duration
		IdleTimeout  time.Duration
	}{Host: "127.0.0.1", Port: 0, ReadTimeout: time.Second, WriteTimeout: time.Second, IdleTimeout: time.Second}

	// Use a type assertion to pass the config to the adapter
	_ = cfg
	s := NewHTTPSidecar(registry, nil)

	// Start blocks until ctx cancelled, cancel after short delay
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- s.Start(ctx)
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}

// ============ BrokerSidecar ============

func TestBrokerSidecar_Name(t *testing.T) {
	registry := handler.NewRegistry()
	s := NewBrokerSidecar(nil, registry, nil)

	if s.Name() != "broker-trigger" {
		t.Errorf("expected 'broker-trigger', got %s", s.Name())
	}
}

func TestBrokerSidecar_WithName(t *testing.T) {
	registry := handler.NewRegistry()
	s := NewBrokerSidecar(nil, registry, nil)
	s.WithName("kafka-consumer")

	if s.Name() != "kafka-consumer" {
		t.Errorf("expected 'kafka-consumer', got %s", s.Name())
	}
}

// ============ CronSidecar ============

func TestCronSidecar_Name(t *testing.T) {
	registry := handler.NewRegistry()
	s := NewCronSidecar(registry, nil, nil)

	if s.Name() != "cron-trigger" {
		t.Errorf("expected 'cron-trigger', got %s", s.Name())
	}
}

func TestCronSidecar_WithName(t *testing.T) {
	registry := handler.NewRegistry()
	s := NewCronSidecar(registry, nil, nil)
	s.WithName("daily-jobs")

	if s.Name() != "daily-jobs" {
		t.Errorf("expected 'daily-jobs', got %s", s.Name())
	}
}

func TestCronSidecar_StopWithoutStart(t *testing.T) {
	registry := handler.NewRegistry()
	s := NewCronSidecar(registry, nil, nil)

	// Stop before Start should not panic
	err := s.Stop(context.Background())
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// ============ Adapter field exposure ============

func TestHTTPSidecar_AdapterExposed(t *testing.T) {
	registry := handler.NewRegistry()
	s := NewHTTPSidecar(registry, nil)

	if s.Adapter == nil {
		t.Error("Adapter field should be exposed")
	}
}

func TestBrokerSidecar_AdapterExposed(t *testing.T) {
	registry := handler.NewRegistry()
	s := NewBrokerSidecar(nil, registry, nil)

	if s.Adapter == nil {
		t.Error("Adapter field should be exposed")
	}
}

func TestCronSidecar_AdapterExposed(t *testing.T) {
	registry := handler.NewRegistry()
	s := NewCronSidecar(registry, nil, nil)

	if s.Adapter == nil {
		t.Error("Adapter field should be exposed")
	}
}
