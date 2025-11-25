# How to Build and Run Examples

This directory contains 3 separate example applications. **DO NOT run `go build` without specifying a file**, as they all have `main()` functions and will conflict.

## Build Individual Examples

### 1. Basic Example (main.go)
```bash
go build -o basic main.go
./basic
```

### 2. Enhanced Example (main_enhanced.go)
```bash
go build -o enhanced main_enhanced.go
./enhanced
```

### 3. Complete Example (main_complete.go)
```bash
export JWT_SECRET="your-secret-key-min-32-chars-long"
go build -o complete main_complete.go
./complete
```

## Or Run Directly

```bash
# Basic
go run main.go

# Enhanced
go run main_enhanced.go

# Complete (with JWT secret)
JWT_SECRET="your-secret-key-min-32-chars-long" go run main_complete.go
```

## Common Mistake

❌ **WRONG** - This will cause "redeclared" errors:
```bash
go build
```

✅ **CORRECT** - Always specify which file to build:
```bash
go build main.go
# or
go build main_enhanced.go
# or
go build main_complete.go
```

## Why?

Each file (`main.go`, `main_enhanced.go`, `main_complete.go`) is a complete standalone application with its own `main()` function. They are NOT meant to be compiled together.
