package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/madcok-co/unicorn/core/internal/codegen"
)

const version = "0.1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}

// run dispatches CLI commands from the given args slice.
// It returns an error when the command should exit with a non-zero code
// (the error messages are already printed to stdout by the sub-commands).
func run(args []string) error {
	if len(args) < 1 {
		printUsage()
		return fmt.Errorf("no command provided")
	}

	command := args[0]
	rest := args[1:]

	switch command {
	case "new", "init":
		return runNew(rest)

	case "generate", "g":
		return runGenerate(rest)

	case "run":
		return runRun(rest)

	case "services", "svc":
		return runServices(rest)

	case "version", "-v", "--version":
		fmt.Printf("Unicorn Framework v%s\n", version)
		return nil

	case "help", "-h", "--help":
		printUsage()
		return nil

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		return fmt.Errorf("unknown command: %s", command)
	}
}

func runNew(args []string) error {
	if len(args) < 1 {
		fmt.Println("Error: project name required")
		fmt.Println("Usage: unicorn new <project-name>")
		return fmt.Errorf("project name required")
	}
	projectName := args[0]
	if err := codegen.GenerateProject(projectName); err != nil {
		fmt.Printf("Error: %v\n", err)
		return err
	}
	fmt.Printf("Project '%s' created successfully!\n", projectName)
	fmt.Println("\nNext steps:")
	fmt.Printf("  cd %s\n", projectName)
	fmt.Println("  go mod tidy")
	fmt.Println("  go run cmd/server/main.go")
	return nil
}

func runGenerate(args []string) error {
	if len(args) < 1 {
		fmt.Println("Error: generator type required")
		fmt.Println("Usage: unicorn generate <type> <name>")
		fmt.Println("\nAvailable types:")
		fmt.Println("  handler   - Generate a new handler")
		fmt.Println("  model     - Generate a new model")
		fmt.Println("  service   - Generate a new service")
		return fmt.Errorf("generator type required")
	}
	genType := args[0]
	if len(args) < 2 {
		fmt.Printf("Error: name required for %s\n", genType)
		return fmt.Errorf("name required for %s", genType)
	}
	name := args[1]

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
		return fmt.Errorf("unknown generator type: %s", genType)
	}

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return err
	}
	fmt.Printf("%s '%s' generated successfully!\n", genType, name)
	return nil
}

func runRun(args []string) error {
	services := []string{}
	portStrategy := "shared"
	port := 8080

	for _, arg := range args {
		switch {
		case arg == "--all" || arg == "-a":
			services = []string{}

		case strings.HasPrefix(arg, "--service=") || strings.HasPrefix(arg, "-s="):
			val := strings.TrimPrefix(arg, "--service=")
			val = strings.TrimPrefix(val, "-s=")
			services = strings.Split(val, ",")

		case strings.HasPrefix(arg, "--port=") || strings.HasPrefix(arg, "-p="):
			val := strings.TrimPrefix(arg, "--port=")
			val = strings.TrimPrefix(val, "-p=")
			_, _ = fmt.Sscanf(val, "%d", &port)

		case arg == "--separate":
			portStrategy = "separate"

		case arg == "--shared":
			portStrategy = "shared"

		case arg == "--help" || arg == "-h":
			printRunUsage()
			return nil

		default:
			if !strings.HasPrefix(arg, "-") {
				services = append(services, arg)
			}
		}
	}

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
	return nil
}

func runServices(args []string) error {
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
	return nil
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
