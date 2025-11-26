package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"github.com/madcok-co/unicorn/core/pkg/handler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
)

// Adapter adalah gRPC server adapter
type Adapter struct {
	server   *grpc.Server
	registry *handler.Registry
	config   *Config
	listener net.Listener

	// Middleware untuk semua RPC calls
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor

	// Service registrations
	serviceRegistrations []ServiceRegistration
}

// Config untuk gRPC adapter
type Config struct {
	Host string
	Port int

	// TLS Configuration
	TLS *contracts.TLSConfig

	// Max message sizes
	MaxRecvMsgSize int
	MaxSendMsgSize int

	// Connection parameters
	MaxConnectionIdle     time.Duration
	MaxConnectionAge      time.Duration
	MaxConnectionAgeGrace time.Duration
	KeepAliveTime         time.Duration
	KeepAliveTimeout      time.Duration

	// Enable reflection for debugging
	EnableReflection bool

	// Additional gRPC server options
	ServerOptions []grpc.ServerOption
}

// ServiceRegistration represents a gRPC service registration
type ServiceRegistration struct {
	Desc    *grpc.ServiceDesc
	Handler interface{}
}

// New creates a new gRPC adapter
func New(registry *handler.Registry, config *Config) *Adapter {
	if config == nil {
		config = &Config{
			Host:                  "0.0.0.0",
			Port:                  9090,
			MaxRecvMsgSize:        4 << 20, // 4MB
			MaxSendMsgSize:        4 << 20, // 4MB
			MaxConnectionIdle:     15 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Second,
			KeepAliveTime:         5 * time.Minute,
			KeepAliveTimeout:      1 * time.Minute,
			EnableReflection:      true,
		}
	}

	return &Adapter{
		registry:             registry,
		config:               config,
		unaryInterceptors:    make([]grpc.UnaryServerInterceptor, 0),
		streamInterceptors:   make([]grpc.StreamServerInterceptor, 0),
		serviceRegistrations: make([]ServiceRegistration, 0),
	}
}

// UseUnaryInterceptor adds a unary interceptor
func (a *Adapter) UseUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) {
	a.unaryInterceptors = append(a.unaryInterceptors, interceptor)
}

// UseStreamInterceptor adds a stream interceptor
func (a *Adapter) UseStreamInterceptor(interceptor grpc.StreamServerInterceptor) {
	a.streamInterceptors = append(a.streamInterceptors, interceptor)
}

// RegisterService registers a gRPC service
func (a *Adapter) RegisterService(desc *grpc.ServiceDesc, handler interface{}) {
	a.serviceRegistrations = append(a.serviceRegistrations, ServiceRegistration{
		Desc:    desc,
		Handler: handler,
	})
}

// Start starts the gRPC server
func (a *Adapter) Start(ctx context.Context) error {
	// Create server options
	opts := a.buildServerOptions()

	// Create gRPC server
	a.server = grpc.NewServer(opts...)

	// Register all services
	for _, reg := range a.serviceRegistrations {
		a.server.RegisterService(reg.Desc, reg.Handler)
	}

	// Enable reflection if configured
	if a.config.EnableReflection {
		reflection.Register(a.server)
	}

	// Create listener
	addr := fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	a.listener = listener

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := a.server.Serve(listener); err != nil {
			errCh <- err
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		return a.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

// buildServerOptions builds gRPC server options
func (a *Adapter) buildServerOptions() []grpc.ServerOption {
	opts := make([]grpc.ServerOption, 0)

	// Add message size options
	if a.config.MaxRecvMsgSize > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(a.config.MaxRecvMsgSize))
	}
	if a.config.MaxSendMsgSize > 0 {
		opts = append(opts, grpc.MaxSendMsgSize(a.config.MaxSendMsgSize))
	}

	// Add interceptors
	if len(a.unaryInterceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(a.unaryInterceptors...))
	}
	if len(a.streamInterceptors) > 0 {
		opts = append(opts, grpc.ChainStreamInterceptor(a.streamInterceptors...))
	}

	// Configure TLS if enabled
	if a.config.TLS != nil && a.config.TLS.Enabled {
		tlsConfig, err := a.configureTLS()
		if err == nil {
			creds := credentials.NewTLS(tlsConfig)
			opts = append(opts, grpc.Creds(creds))
		}
	}

	// Add additional server options
	opts = append(opts, a.config.ServerOptions...)

	return opts
}

// configureTLS configures TLS for the server
func (a *Adapter) configureTLS() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(a.config.TLS.CertFile, a.config.TLS.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load key pair: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
		PreferServerCipherSuites: true,
	}

	// Apply custom settings
	if a.config.TLS.MinVersion != 0 {
		tlsConfig.MinVersion = a.config.TLS.MinVersion
	}
	if a.config.TLS.MaxVersion != 0 {
		tlsConfig.MaxVersion = a.config.TLS.MaxVersion
	}
	if len(a.config.TLS.CipherSuites) > 0 {
		tlsConfig.CipherSuites = a.config.TLS.CipherSuites
	}
	if a.config.TLS.ClientAuth != 0 {
		tlsConfig.ClientAuth = a.config.TLS.ClientAuth
	}

	return tlsConfig, nil
}

// Shutdown gracefully shuts down the server
func (a *Adapter) Shutdown(ctx context.Context) error {
	if a.server == nil {
		return nil
	}

	// Create a channel to signal when GracefulStop is done
	done := make(chan struct{})
	go func() {
		a.server.GracefulStop()
		close(done)
	}()

	// Wait for either graceful stop or context timeout
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		// Force stop if context expires
		a.server.Stop()
		return ctx.Err()
	}
}

// Address returns the server address
func (a *Adapter) Address() string {
	return fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)
}

// IsTLS returns true if TLS is enabled
func (a *Adapter) IsTLS() bool {
	return a.config.TLS != nil && a.config.TLS.Enabled
}

// CreateUnaryHandler creates a gRPC unary handler from a Unicorn handler
// This is a helper for users to wrap Unicorn handlers for gRPC
func (a *Adapter) CreateUnaryHandler(h *handler.Handler) grpc.UnaryHandler {
	executor := handler.NewExecutor(h)

	return func(ctx context.Context, req interface{}) (interface{}, error) {
		// Create unicorn context
		uCtx := ucontext.New(ctx)

		// Extract metadata from gRPC context
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			headers := make(map[string]string)
			for k, v := range md {
				if len(v) > 0 {
					headers[k] = v[0]
				}
			}

			// Build request
			uReq := &ucontext.Request{
				Method:      "RPC",
				Path:        "",
				Headers:     headers,
				Params:      make(map[string]string),
				Query:       make(map[string]string),
				Body:        nil,
				TriggerType: "grpc",
			}

			uCtx.SetRequest(uReq)
		}

		// Set the request data
		uCtx.Set("grpc.request", req)

		// Execute handler
		if err := executor.Execute(uCtx); err != nil {
			return nil, err
		}

		// Get response
		resp := uCtx.Response()
		if resp.Body != nil {
			return resp.Body, nil
		}

		return nil, nil
	}
}

// ExtractMetadata extracts metadata from unicorn context and returns gRPC metadata
func ExtractMetadata(ctx *ucontext.Context) metadata.MD {
	md := metadata.MD{}

	resp := ctx.Response()
	if resp != nil && resp.Headers != nil {
		for k, v := range resp.Headers {
			md[k] = []string{v}
		}
	}

	return md
}
