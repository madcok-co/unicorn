// Example demonstrating hybrid trigger sidecar deployment modes.
//
// This example shows how to run triggers (HTTP, broker, cron) as isolated
// sidecar processes alongside cross-cutting sidecars (management, config watch).
//
// Three deployment modes are demonstrated:
//  1. Default inline mode  — triggers run inside App.Start()
//  2. Sidecar mode — triggers run as independent sidecars via RunSidecars()
//  3. Hybrid — mixed inline + sidecar triggers
//
// Run:
//
//	cd core/examples/sidecar-mode
//	go run main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/madcok-co/unicorn/contrib/sidecar/management"

	brokerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/broker/memory"
	cronAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/cron"
	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
	"github.com/madcok-co/unicorn/core/pkg/app"
	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/sidecar"
)

// ============ Request/Response DTOs ============

type GreetRequest struct {
	Name string `json:"name"`
}

type GreetResponse struct {
	Message string `json:"message"`
}

type TaskPayload struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ============ Business Handlers ============

func Greet(ctx *ucontext.Context, req GreetRequest) (*GreetResponse, error) {
	return &GreetResponse{
		Message: fmt.Sprintf("Hello, %s!", req.Name),
	}, nil
}

func ProcessTask(ctx *ucontext.Context, req TaskPayload) error {
	log.Printf("processing task %s: %s", req.ID, req.Name)
	return nil
}

func ScheduledCleanup(ctx *ucontext.Context) error {
	log.Println("running scheduled cleanup job")
	return nil
}

// ============ Custom Protocol Sidecar ============
// Example: a custom TCP/UDP/WebSocket listener as a sidecar.

// ProtocolConsumerConfig holds configuration for the custom consumer.
type ProtocolConsumerConfig struct {
	Address string
}

// protocolConsumer listens on a custom TCP address and forwards messages
// to the handler registry via channel. This simulates any protocol consumer
// (FreeSWITCH ESL, SIP, WebSocket, MQTT, etc).
type protocolConsumer struct {
	config ProtocolConsumerConfig
	done   chan struct{}
}

func NewProtocolConsumer(cfg ProtocolConsumerConfig) *protocolConsumer {
	return &protocolConsumer{
		config: cfg,
		done:   make(chan struct{}),
	}
}

func (p *protocolConsumer) Name() string { return "custom-protocol" }

func (p *protocolConsumer) Start(ctx context.Context) error {
	log.Printf("[sidecar] %s listening on %s", p.Name(), p.config.Address)
	// In production: accept TCP connections, parse frames, forward to handler.
	// Here we simulate receiving a message every 5 seconds.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[sidecar] %s shutting down", p.Name())
			return nil
		case <-ticker.C:
			log.Printf("[sidecar] %s received message from upstream", p.Name())
		}
	}
}

func (p *protocolConsumer) Stop(ctx context.Context) error {
	close(p.done)
	log.Printf("[sidecar] %s stopped", p.Name())
	return nil
}

// ============ Mode Selection ============

func main() {
	mode := "sidecar" // Change to "inline" or "hybrid" to test other modes

	// Create app with shared handler registry and infrastructure
	application := app.New(&app.Config{
		Name:       "unicorn-sidecar-demo",
		Version:    "1.0.0",
		EnableHTTP: mode == "inline",
	})

	// Register handlers — same handlers, any deployment mode
	application.RegisterHandler(Greet).
		Named("greet").
		HTTP("GET", "/greet").
		Done()

	application.RegisterHandler(ProcessTask).
		Named("process-task").
		HTTP("POST", "/tasks").
		Message("task.assign").
		Done()

	application.RegisterHandler(ScheduledCleanup).
		Named("cleanup").
		Cron("@every 30s").
		Done()

	switch mode {
	case "inline":
		runInline(application)

	case "sidecar":
		runSidecarMode(application)

	case "hybrid":
		runHybridMode(application)
	}
}

// ============ Mode A: Inline (Default) ============

func runInline(app *app.App) {
	log.Println("=== Mode: Inline triggers ===")
	log.Println("HTTP + Broker + Cron run inside App.Start()")
	log.Println("Sidecars are auxiliary only (management, config watch)")
	log.Println()

	// Cross-cutting sidecars
	app.AddSidecar(management.New(&management.Config{
		Port:          9090,
		EnablePprof:   true,
		EnableMetrics: true,
	}))

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

// ============ Mode B: Full Sidecar Mode ============

func runSidecarMode(app *app.App) {
	log.Println("=== Mode: Full sidecar ===")
	log.Println("Triggers run as isolated sidecars via RunSidecars()")
	log.Println("HTTP crashes don't affect broker consumer")
	log.Println()

	// --- Trigger sidecars ---

	// HTTP trigger as sidecar
	httpSidecar := sidecar.NewHTTPSidecar(app.Registry(), &httpAdapter.Config{
		Host: "0.0.0.0",
		Port: 8080,
	})
	httpSidecar.Adapter.SetAppAdapters(app.Adapters())
	app.AddSidecar(httpSidecar)

	// Broker trigger as sidecar
	memBroker := brokerAdapter.New()
	app.SetBroker(memBroker)
	brokerSidecar := sidecar.NewBrokerSidecar(memBroker, app.Registry(), nil)
	brokerSidecar.Adapter.SetAppAdapters(app.Adapters())
	app.AddSidecar(brokerSidecar)

	// Cron trigger as sidecar
	simpleScheduler := cronAdapter.NewSimpleScheduler()
	app.SetCronScheduler(simpleScheduler)
	cronSidecar := sidecar.NewCronSidecar(app.Registry(), simpleScheduler, nil)
	app.AddSidecar(cronSidecar)

	// --- Cross-cutting sidecars ---

	// Management server: health, metrics, pprof
	app.AddSidecar(management.New(&management.Config{
		Port:          9090,
		EnablePprof:   true,
		EnableMetrics: true,
	}))

	// Custom protocol consumer (e.g. WebSocket, TCP, MQTT)
	app.AddSidecar(NewProtocolConsumer(ProtocolConsumerConfig{
		Address: "0.0.0.0:9091",
	}))

	// Start only sidecars — no built-in adapters
	if err := app.RunSidecars(); err != nil {
		log.Fatal(err)
	}
}

// ============ Mode C: Hybrid (Mixed) ============

func runHybridMode(app *app.App) {
	log.Println("=== Mode: Hybrid ===")
	log.Println("Broker consumer runs inline")
	log.Println("HTTP server runs as isolated sidecar")
	log.Println()

	// Setup inline broker (this will start automatically with RunServices)
	memBroker := brokerAdapter.New()
	app.SetBroker(memBroker)

	// HTTP trigger as sidecar (isolated from inline broker)
	httpSidecar := sidecar.NewHTTPSidecar(app.Registry(), &httpAdapter.Config{
		Host: "0.0.0.0",
		Port: 8080,
	})
	httpSidecar.Adapter.SetAppAdapters(app.Adapters())
	app.AddSidecar(httpSidecar)

	// Cross-cutting sidecars
	app.AddSidecar(management.New(&management.Config{
		Port:          9090,
		EnableMetrics: true,
	}))

	// Block main goroutine so sidecars keep running
	// In production, use app.RunServices() or app.RunSidecars()
	// depending on which triggers should be inline.
	// Here we just wait for signal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
