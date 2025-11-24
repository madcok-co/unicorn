package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/madcok-co/unicorn/core/internal/codegen"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "new", "init":
		cmdNew()

	case "generate", "g":
		cmdGenerate()

	case "run":
		cmdRun()

	case "services", "svc":
		cmdServices()

	case "version", "-v", "--version":
		fmt.Printf("Unicorn Framework v%s\n", version)

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func cmdNew() {
	if len(os.Args) < 3 {
		fmt.Println("Error: project name required")
		fmt.Println("Usage: unicorn new <project-name>")
		os.Exit(1)
	}
	projectName := os.Args[2]
	if err := codegen.GenerateProject(projectName); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Project '%s' created successfully!\n", projectName)
	fmt.Println("\nNext steps:")
	fmt.Printf("  cd %s\n", projectName)
	fmt.Println("  go mod tidy")
	fmt.Println("  go run cmd/server/main.go")
}

func cmdGenerate() {
	if len(os.Args) < 3 {
		fmt.Println("Error: generator type required")
		fmt.Println("Usage: unicorn generate <type> <name>")
		fmt.Println("\nAvailable types:")
		fmt.Println("  handler   - Generate a new handler")
		fmt.Println("  model     - Generate a new model")
		fmt.Println("  service   - Generate a new service")
		os.Exit(1)
	}
	genType := os.Args[2]
	if len(os.Args) < 4 {
		fmt.Printf("Error: name required for %s\n", genType)
		os.Exit(1)
	}
	name := os.Args[3]

	var err error
	switch genType {
	case "handler", "h":
		err = codegen.GenerateHandler(name)
	case "model", "m":
		err = codegen.GenerateModel(name)
	case "service", "s":
		err = codegen.GenerateService(name)
	default:
		fmt.Printf("Unknown generator type: %s\n", genType)
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s '%s' generated successfully!\n", genType, name)
}

func cmdRun() {
	// Parse flags
	services := []string{}
	portStrategy := "shared"
	port := 8080

	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]

		switch {
		case arg == "--all" || arg == "-a":
			// Run all services (default)
			services = []string{}

		case strings.HasPrefix(arg, "--service=") || strings.HasPrefix(arg, "-s="):
			// --service=user-service,order-service
			val := strings.TrimPrefix(arg, "--service=")
			val = strings.TrimPrefix(val, "-s=")
			services = strings.Split(val, ",")

		case strings.HasPrefix(arg, "--port=") || strings.HasPrefix(arg, "-p="):
			// --port=8080
			val := strings.TrimPrefix(arg, "--port=")
			val = strings.TrimPrefix(val, "-p=")
			_, _ = fmt.Sscanf(val, "%d", &port) // Parse error returns default port value

		case arg == "--separate":
			// Each service on separate port
			portStrategy = "separate"

		case arg == "--shared":
			// All services on shared port (default)
			portStrategy = "shared"

		case arg == "--help" || arg == "-h":
			printRunUsage()
			os.Exit(0)

		default:
			// Treat as service name if no prefix
			if !strings.HasPrefix(arg, "-") {
				services = append(services, arg)
			}
		}
	}

	// Generate run configuration
	fmt.Println("Run Configuration:")
	fmt.Printf("  Port Strategy: %s\n", portStrategy)
	fmt.Printf("  Base Port: %d\n", port)
	if len(services) > 0 {
		fmt.Printf("  Services: %s\n", strings.Join(services, ", "))
	} else {
		fmt.Println("  Services: all")
	}

	fmt.Println("\nTo run your application, use:")
	fmt.Println("  go run cmd/server/main.go \\")
	if len(services) > 0 {
		fmt.Printf("    --services=%s \\\n", strings.Join(services, ","))
	}
	fmt.Printf("    --port=%d \\\n", port)
	fmt.Printf("    --port-strategy=%s\n", portStrategy)

	fmt.Println("\nOr in your main.go:")
	fmt.Println("  app.RunServices(\"" + strings.Join(services, "\", \"") + "\")")
}

func cmdServices() {
	fmt.Println("Service Management")
	fmt.Println("\nTo list services in your application, add this to your code:")
	fmt.Println("")
	fmt.Println("  for _, svc := range app.Services().All() {")
	fmt.Printf("      fmt.Printf(\"Service: %%s\\n\", svc.Name())\n")
	fmt.Printf("      fmt.Printf(\"  Handlers: %%d\\n\", len(svc.Handlers()))\n")
	fmt.Println("      for _, h := range svc.Handlers() {")
	fmt.Printf("          fmt.Printf(\"    - %%s\\n\", h.Name)\n")
	fmt.Println("      }")
	fmt.Println("  }")
}

func printRunUsage() {
	fmt.Print(`
Run Application

Usage:
  unicorn run [flags] [service-names...]

Flags:
  --all, -a              Run all services (default)
  --service=NAME, -s=NAME  Run specific services (comma-separated)
  --port=PORT, -p=PORT   Base port for HTTP server (default: 8080)
  --separate             Run each service on separate port
  --shared               Run all services on shared port (default)

Examples:
  unicorn run                           # Run all services
  unicorn run --service=user-service    # Run only user-service
  unicorn run user-service order-service  # Run multiple services
  unicorn run --separate --port=8080    # Each service gets its own port
`)
}

func printUsage() {
	fmt.Print(`
Unicorn Framework CLI

Usage:
  unicorn <command> [arguments]

Commands:
  new, init       Create a new Unicorn project
                  Usage: unicorn new <project-name>

  generate, g     Generate code (handler, model, service)
                  Usage: unicorn generate <type> <name>
                  Types: handler, model, service

  run             Run application services
                  Usage: unicorn run [--service=name] [--port=8080]
                  Use 'unicorn run --help' for more info

  services, svc   Service management utilities

  version, -v     Show version information

  help, -h        Show this help message

Examples:
  unicorn new myapp                    # Create new project
  unicorn g handler user               # Generate user handler
  unicorn g model product              # Generate product model
  unicorn g service order              # Generate order service
  unicorn run                          # Run all services
  unicorn run --service=user-service   # Run specific service
`)
}
