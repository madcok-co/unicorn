package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn and returns everything written to os.Stdout.
// os.Stdout is restored before returning.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// captureRun calls run(args) and captures its stdout output.
func captureRun(args []string) (output string, err error) {
	output = captureStdout(func() {
		err = run(args)
	})
	return
}

// ---------------------------------------------------------------------------
// Command dispatch tests
// ---------------------------------------------------------------------------

func TestRunNoArgs(t *testing.T) {
	output, err := captureRun([]string{})
	if err == nil {
		t.Fatal("expected error for no args, got nil")
	}
	if !strings.Contains(output, "Unicorn Framework CLI") {
		t.Errorf("output should contain usage text, got: %s", output)
	}
	if !strings.Contains(err.Error(), "no command provided") {
		t.Errorf("error should mention no command, got: %v", err)
	}
}

func TestRunVersion(t *testing.T) {
	for _, arg := range []string{"version", "-v", "--version"} {
		t.Run(arg, func(t *testing.T) {
			output, err := captureRun([]string{arg})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !strings.Contains(output, "Unicorn Framework v"+version) {
				t.Errorf("expected version %s in output, got: %s", version, output)
			}
		})
	}
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		t.Run(arg, func(t *testing.T) {
			output, err := captureRun([]string{arg})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !strings.Contains(output, "Unicorn Framework CLI") {
				t.Errorf("expected usage in output, got: %s", output)
			}
		})
	}
}

func TestRunUnknownCommand(t *testing.T) {
	output, err := captureRun([]string{"foobar"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(output, "Unknown command: foobar") {
		t.Errorf("expected unknown command message, got: %s", output)
	}
	if !strings.Contains(output, "Unicorn Framework CLI") {
		t.Errorf("expected usage in output, got: %s", output)
	}
}

func TestRunServices(t *testing.T) {
	for _, arg := range []string{"services", "svc"} {
		t.Run(arg, func(t *testing.T) {
			output, err := captureRun([]string{arg})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !strings.Contains(output, "Service Management") {
				t.Errorf("expected 'Service Management' in output, got: %s", output)
			}
			if !strings.Contains(output, "app.Services().All()") {
				t.Errorf("expected Services snippet in output, got: %s", output)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// "new" command tests
// ---------------------------------------------------------------------------

func TestRunNewNoName(t *testing.T) {
	output, err := captureRun([]string{"new"})
	if err == nil {
		t.Fatal("expected error when project name is missing")
	}
	if !strings.Contains(output, "Error: project name required") {
		t.Errorf("expected name-required error, got: %s", output)
	}
}

func TestRunInitNoName(t *testing.T) {
	output, err := captureRun([]string{"init"})
	if err == nil {
		t.Fatal("expected error when project name is missing")
	}
	if !strings.Contains(output, "Error: project name required") {
		t.Errorf("expected name-required error, got: %s", output)
	}
}

func TestRunNewWithName(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	output, err := captureRun([]string{"new", "myapp"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "created successfully") {
		t.Errorf("expected success message, got: %s", output)
	}
	if !strings.Contains(output, "Next steps") {
		t.Errorf("expected next steps, got: %s", output)
	}
	if !strings.Contains(output, "cd myapp") {
		t.Errorf("expected cd instruction, got: %s", output)
	}

	// Verify the project directory was created
	if info, err := os.Stat("myapp"); err != nil || !info.IsDir() {
		t.Error("project directory was not created")
	}
}

// ---------------------------------------------------------------------------
// "generate" command tests
// ---------------------------------------------------------------------------

func TestRunGenerateNoType(t *testing.T) {
	output, err := captureRun([]string{"generate"})
	if err == nil {
		t.Fatal("expected error when generator type is missing")
	}
	if !strings.Contains(output, "Error: generator type required") {
		t.Errorf("expected type-required error, got: %s", output)
	}
	if !strings.Contains(output, "Available types:") {
		t.Errorf("expected available types listing, got: %s", output)
	}
}

func TestRunGenerateNoName(t *testing.T) {
	output, err := captureRun([]string{"generate", "handler"})
	if err == nil {
		t.Fatal("expected error when name is missing")
	}
	if !strings.Contains(output, "Error: name required for handler") {
		t.Errorf("expected name-required error, got: %s", output)
	}
}

func TestRunGenerateUnknownType(t *testing.T) {
	output, err := captureRun([]string{"generate", "unknown", "foo"})
	if err == nil {
		t.Fatal("expected error for unknown generator type")
	}
	if !strings.Contains(output, "Unknown generator type: unknown") {
		t.Errorf("expected unknown type error, got: %s", output)
	}
}

func TestRunGenerateHandler(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	output, err := captureRun([]string{"generate", "handler", "user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "handler 'user' generated successfully") {
		t.Errorf("expected handler success, got: %s", output)
	}
	if _, err := os.Stat("internal/handlers/user.go"); os.IsNotExist(err) {
		t.Error("handler file was not created")
	}
}

func TestRunGenerateHandlerShort(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	output, err := captureRun([]string{"g", "h", "profile"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "generated successfully") {
		t.Errorf("expected success, got: %s", output)
	}
	if _, err := os.Stat("internal/handlers/profile.go"); os.IsNotExist(err) {
		t.Error("handler file was not created")
	}
}

func TestRunGenerateModel(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	output, err := captureRun([]string{"generate", "model", "product"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "model 'product' generated successfully") {
		t.Errorf("expected model success, got: %s", output)
	}
	if _, err := os.Stat("internal/models/product.go"); os.IsNotExist(err) {
		t.Error("model file was not created")
	}
}

func TestRunGenerateService(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	output, err := captureRun([]string{"generate", "service", "order"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "service 'order' generated successfully") {
		t.Errorf("expected service success, got: %s", output)
	}
	if _, err := os.Stat("internal/services/order.go"); os.IsNotExist(err) {
		t.Error("service file was not created")
	}
}

// ---------------------------------------------------------------------------
// "run" command tests
// ---------------------------------------------------------------------------

func TestRunHelpFlag(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			output, err := captureRun([]string{"run", arg})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !strings.Contains(output, "Run Application") {
				t.Errorf("expected run usage, got: %s", output)
			}
		})
	}
}

func TestRunDefaultConfig(t *testing.T) {
	output, err := captureRun([]string{"run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []string{
		"Run Configuration:",
		"Port Strategy: shared",
		"Base Port: 8080",
		"Services: all",
	}
	for _, want := range checks {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q, got: %s", want, output)
		}
	}
}

func TestRunWithSeparateFlag(t *testing.T) {
	output, err := captureRun([]string{"run", "--separate"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Port Strategy: separate") {
		t.Errorf("expected separate port strategy, got: %s", output)
	}
}

func TestRunWithCustomPort(t *testing.T) {
	output, err := captureRun([]string{"run", "--port=3000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Base Port: 3000") {
		t.Errorf("expected port 3000, got: %s", output)
	}
}

func TestRunWithShortPortFlag(t *testing.T) {
	output, err := captureRun([]string{"run", "-p=9090"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Base Port: 9090") {
		t.Errorf("expected port 9090, got: %s", output)
	}
}

func TestRunWithServiceFlag(t *testing.T) {
	output, err := captureRun([]string{"run", "--service=user-service,order-service"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Services: user-service, order-service") {
		t.Errorf("expected specific services, got: %s", output)
	}
}

func TestRunWithShortServiceFlag(t *testing.T) {
	output, err := captureRun([]string{"run", "-s=auth-service"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Services: auth-service") {
		t.Errorf("expected auth-service, got: %s", output)
	}
}

func TestRunWithPositionalServices(t *testing.T) {
	output, err := captureRun([]string{"run", "user-service", "order-service"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Services: user-service, order-service") {
		t.Errorf("expected positional services, got: %s", output)
	}
}

func TestRunAllFlagOverridesServices(t *testing.T) {
	output, err := captureRun([]string{"run", "--service=foo", "--all"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Services: all") {
		t.Errorf("expected all services after --all, got: %s", output)
	}
}

func TestRunCombinedFlags(t *testing.T) {
	output, err := captureRun([]string{"run", "--separate", "--port=5555", "--service=svc1,svc2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Port Strategy: separate") {
		t.Errorf("expected separate strategy, got: %s", output)
	}
	if !strings.Contains(output, "Base Port: 5555") {
		t.Errorf("expected port 5555, got: %s", output)
	}
	if !strings.Contains(output, "Services: svc1, svc2") {
		t.Errorf("expected svc1,svc2, got: %s", output)
	}
}

// ---------------------------------------------------------------------------
// Short-aliases (g / h / m / s / init / svc) dispatch
// ---------------------------------------------------------------------------

func TestRunShortAliases(t *testing.T) {
	t.Run("g alias for generate", func(t *testing.T) {
		output, err := captureRun([]string{"g"})
		if err == nil {
			t.Fatal("expected error for g with no type")
		}
		if !strings.Contains(output, "Error: generator type required") {
			t.Errorf("expected generator error, got: %s", output)
		}
	})

	t.Run("init alias for new", func(t *testing.T) {
		output, err := captureRun([]string{"init"})
		if err == nil {
			t.Fatal("expected error for init with no name")
		}
		if !strings.Contains(output, "Error: project name required") {
			t.Errorf("expected project name error, got: %s", output)
		}
	})
}

// ---------------------------------------------------------------------------
// run function returns error on failure
// ---------------------------------------------------------------------------

func TestRunReturnsErrorForFailurePaths(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no args", []string{}},
		{"unknown command", []string{"bogus"}},
		{"new no name", []string{"new"}},
		{"generate no type", []string{"generate"}},
		{"generate no name", []string{"generate", "handler"}},
		{"generate unknown type", []string{"generate", "xyz", "foo"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := captureRun(tt.args)
			if err == nil {
				t.Errorf("expected error for args %v", tt.args)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// run function returns nil for success paths
// ---------------------------------------------------------------------------

func TestRunReturnsNilForSuccessPaths(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"version", []string{"version"}},
		{"-v", []string{"-v"}},
		{"--version", []string{"--version"}},
		{"help", []string{"help"}},
		{"-h", []string{"-h"}},
		{"--help", []string{"--help"}},
		{"services", []string{"services"}},
		{"svc", []string{"svc"}},
		{"run no args", []string{"run"}},
		{"run --help", []string{"run", "--help"}},
		{"run -h", []string{"run", "-h"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := captureRun(tt.args)
			if err != nil {
				t.Errorf("unexpected error for %v: %v", tt.args, err)
			}
		})
	}
}
