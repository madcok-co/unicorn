// ============================================
// 2. GRPC TRIGGER
// ============================================
package triggers

import (
	"fmt"
	"sync"

	"github.com/madcok-co/unicorn"
	"google.golang.org/grpc"
)

type GRPCTrigger struct {
	addr     string
	server   *grpc.Server
	services map[string]*unicorn.Definition
	mu       sync.RWMutex
}

func NewGRPCTrigger(addr string) *GRPCTrigger {
	return &GRPCTrigger{
		addr:     addr,
		services: make(map[string]*unicorn.Definition),
		server:   grpc.NewServer(),
	}
}

func (t *GRPCTrigger) RegisterService(def *unicorn.Definition) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.services[def.Name] = def
	return nil
}

func (t *GRPCTrigger) Start() error {
	// Start gRPC server
	// Note: Full gRPC implementation requires protobuf definitions
	// This is a simplified version
	return fmt.Errorf("gRPC trigger requires protobuf definitions - see documentation")
}

func (t *GRPCTrigger) Stop() error {
	if t.server != nil {
		t.server.GracefulStop()
	}
	return nil
}
