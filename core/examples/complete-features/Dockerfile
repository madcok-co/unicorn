# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download || true

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/unicorn-example main_enhanced.go

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 unicorn && \
    adduser -D -u 1000 -G unicorn unicorn

WORKDIR /home/unicorn

# Copy binary from builder
COPY --from=builder /app/bin/unicorn-example .

# Copy .env file if exists
COPY --from=builder /app/.env* ./

# Change ownership
RUN chown -R unicorn:unicorn /home/unicorn

# Switch to non-root user
USER unicorn

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./unicorn-example"]
