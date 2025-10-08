// ============================================
// UNICORN Framework - CLI Runner
// User TIDAK perlu buat main.go!
// ============================================

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/madcok-co/unicorn"
	"github.com/madcok-co/unicorn/triggers"
	"github.com/madcok-co/unicorn/utils"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "config.yaml", "Path to config file")
	execService := flag.String("exec", "", "Execute service directly")
	execRequest := flag.String("request", "{}", "Request data for exec")
	flag.Parse()

	// If exec mode, execute and exit
	if *execService != "" {
		executeService(*execService, *execRequest)
		return
	}

	// Load config
	loader := utils.NewConfigLoader()
	cfg, err := loader.LoadWithEnvSubstitution(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger, err := utils.NewZapLogger(cfg.App.Environment == "development")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	unicorn.SetGlobalLogger(logger)
	logger.Info("UNICORN Framework starting", "version", unicorn.Version, "app", cfg.App.Name)

	// Initialize connection manager
	connMgr := unicorn.NewConnectionManager()

	if len(cfg.Databases) > 0 {
		if err := connMgr.InitializeDatabases(cfg.Databases); err != nil {
			logger.Error("Failed to initialize databases", "error", err)
			os.Exit(1)
		}
		logger.Info("Databases initialized", "count", len(cfg.Databases))
	}

	if len(cfg.Redis) > 0 {
		if err := connMgr.InitializeRedis(cfg.Redis); err != nil {
			logger.Error("Failed to initialize Redis", "error", err)
			os.Exit(1)
		}
		logger.Info("Redis instances initialized", "count", len(cfg.Redis))
	}

	if len(cfg.Kafka.Brokers) > 0 {
		if err := connMgr.InitializeKafka(cfg.Kafka); err != nil {
			logger.Error("Failed to initialize Kafka", "error", err)
			os.Exit(1)
		}
		logger.Info("Kafka initialized")
	}

	connMgr.MarkInitialized()
	defer connMgr.Close()

	// Register built-in plugins
	//plugins.RegisterBuiltinPlugins()

	// Initialize plugin manager
	pluginMgr := unicorn.NewPluginManager(unicorn.GetGlobalPluginRegistry())
	for name, pluginCfg := range cfg.Plugins {
		if pluginCfg.Enabled {
			if err := pluginMgr.Initialize(name, pluginCfg.Config); err != nil {
				logger.Warn("Failed to initialize plugin", "plugin", name, "error", err)
			} else {
				logger.Info("Plugin initialized", "plugin", name)
			}
		}
	}
	defer pluginMgr.Close()

	// Register built-in middlewares
	//middleware.RegisterBuiltinMiddlewares(logger, "your-jwt-secret")

	// Initialize metrics
	_ = utils.NewMetrics()
	logger.Info("Metrics initialized")

	// Initialize triggers
	var activeTriggers []Trigger

	// HTTP Trigger
	if cfg.Server.HTTP.Enabled {
		addr := fmt.Sprintf("%s:%d", cfg.Server.HTTP.Host, cfg.Server.HTTP.Port)
		httpTrigger := triggers.NewHTTPTrigger(addr)

		for _, def := range unicorn.ListServicesByTrigger("http") {
			httpTrigger.RegisterService(def)
		}

		activeTriggers = append(activeTriggers, httpTrigger)
		logger.Info("HTTP trigger initialized", "address", addr)
	}

	// gRPC Trigger
	if cfg.Server.GRPC.Enabled {
		addr := fmt.Sprintf("%s:%d", cfg.Server.GRPC.Host, cfg.Server.GRPC.Port)
		grpcTrigger := triggers.NewGRPCTrigger(addr)

		for _, def := range unicorn.ListServicesByTrigger("grpc") {
			grpcTrigger.RegisterService(def)
		}

		activeTriggers = append(activeTriggers, grpcTrigger)
		logger.Info("gRPC trigger initialized", "address", addr)
	}

	// Kafka Trigger
	if len(cfg.Kafka.Brokers) > 0 {
		kafkaTrigger := triggers.NewKafkaTrigger(cfg.Kafka.Brokers, cfg.Kafka.GroupID)

		for _, def := range unicorn.ListServicesByTrigger("kafka") {
			kafkaTrigger.RegisterService(def)
		}

		activeTriggers = append(activeTriggers, kafkaTrigger)
		logger.Info("Kafka trigger initialized")
	}

	// Cron Trigger
	if len(cfg.Cron.Jobs) > 0 {
		cronTrigger := triggers.NewCronTrigger()

		for _, job := range cfg.Cron.Jobs {
			if job.Enabled {
				cronTrigger.AddJob(job.Name, job.Schedule, job.ServiceName, job.Request)
				logger.Info("Cron job added", "name", job.Name, "schedule", job.Schedule)
			}
		}

		activeTriggers = append(activeTriggers, cronTrigger)
		logger.Info("Cron trigger initialized", "jobs", len(cfg.Cron.Jobs))
	}

	// CLI Trigger (always enabled)
	cliTrigger := triggers.NewCLITrigger()
	for _, def := range unicorn.ListServicesByTrigger("cli") {
		cliTrigger.RegisterService(def)
	}

	// Start all triggers
	for _, trigger := range activeTriggers {
		t := trigger // Capture for goroutine
		go func() {
			if err := t.Start(); err != nil {
				logger.Error("Trigger failed", "error", err)
			}
		}()
	}

	// Log summary
	serviceCount := unicorn.GetGlobalRegistry().Count()
	logger.Info("UNICORN Framework started",
		"services", serviceCount,
		"triggers", len(activeTriggers),
		"environment", cfg.App.Environment,
	)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down gracefully...")

	// Stop all triggers
	for _, trigger := range activeTriggers {
		trigger.Stop()
	}

	logger.Info("UNICORN Framework stopped")
}

// executeService executes a service directly via CLI
func executeService(serviceName, requestJSON string) {
	// Parse request
	var request map[string]interface{}
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing request: %v\n", err)
		os.Exit(1)
	}

	// Execute service
	result, err := unicorn.ExecuteService(context.Background(), serviceName, request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing service: %v\n", err)
		os.Exit(1)
	}

	// Print result
	output, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(output))
}

// Trigger interface for starting and stopping
type Trigger interface {
	Start() error
	Stop() error
}
