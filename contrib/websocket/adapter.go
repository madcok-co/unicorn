package websocket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Config holds WebSocket adapter configuration.
type Config struct {
	Host string
	Port int
	Path string // WebSocket endpoint path, default "/ws"

	// MaxMsgSize is the maximum message size in bytes.
	MaxMsgSize int64

	// Heartbeat
	PingPeriod   time.Duration
	PongWait     time.Duration
	WriteTimeout time.Duration

	// Callbacks
	OnConnect    func(clientID string)
	OnDisconnect func(clientID string)
	OnMessage    func(clientID string, msg []byte)
}

// Client represents a connected WebSocket client.
type Client struct {
	ID   string
	Conn *websocket.Conn
	Send chan []byte
}

// Adapter is a WebSocket sidecar that manages WebSocket connections.
type Adapter struct {
	config   *Config
	clients  map[string]*Client
	mu       sync.RWMutex
	server   *http.Server
	upgrader websocket.Upgrader
}

// NewAdapter creates a new WebSocket adapter with sensible defaults.
func NewAdapter(cfg *Config) *Adapter {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	if cfg.Port == 0 {
		cfg.Port = 8081
	}
	if cfg.Path == "" {
		cfg.Path = "/ws"
	}
	if cfg.MaxMsgSize == 0 {
		cfg.MaxMsgSize = 4096
	}
	if cfg.PingPeriod == 0 {
		cfg.PingPeriod = 54 * time.Second
	}
	if cfg.PongWait == 0 {
		cfg.PongWait = 60 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 10 * time.Second
	}

	return &Adapter{
		config:  cfg,
		clients: make(map[string]*Client),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// Name implements contracts.Sidecar.
func (a *Adapter) Name() string {
	return "websocket"
}

// Start implements contracts.Sidecar. Blocks until ctx is cancelled.
func (a *Adapter) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc(a.config.Path, a.handleWebSocket)

	addr := fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)

	a.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("[websocket] listening on %s%s", addr, a.config.Path)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return a.Stop(context.Background())
	case err := <-errCh:
		return fmt.Errorf("websocket server error: %w", err)
	}
}

// Stop implements contracts.Sidecar. Gracefully shuts down all connections.
func (a *Adapter) Stop(ctx context.Context) error {
	// Close all client connections
	a.mu.Lock()
	for id, client := range a.clients {
		close(client.Send)
		if client.Conn != nil {
			client.Conn.Close()
		}
		delete(a.clients, id)
	}
	a.mu.Unlock()

	// Shutdown HTTP server
	if a.server != nil {
		return a.server.Shutdown(ctx)
	}
	return nil
}

// Broadcast sends a message to all connected clients.
func (a *Adapter) Broadcast(msg []byte) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, client := range a.clients {
		select {
		case client.Send <- msg:
		default:
			// Client send buffer is full, skip
		}
	}
}

// SendTo sends a message to a specific client by ID.
func (a *Adapter) SendTo(clientID string, msg []byte) error {
	a.mu.RLock()
	client, ok := a.clients[clientID]
	a.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %q not found", clientID)
	}

	select {
	case client.Send <- msg:
		return nil
	default:
		return fmt.Errorf("client %q send buffer full", clientID)
	}
}

// ClientCount returns the number of currently connected clients.
func (a *Adapter) ClientCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.clients)
}

// ClientIDs returns a list of all connected client IDs.
func (a *Adapter) ClientIDs() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	ids := make([]string, 0, len(a.clients))
	for id := range a.clients {
		ids = append(ids, id)
	}
	return ids
}

// KickClient force-disconnects a client by ID.
func (a *Adapter) KickClient(clientID string) error {
	a.mu.Lock()
	client, ok := a.clients[clientID]
	if ok {
		delete(a.clients, clientID)
	}
	a.mu.Unlock()

	if !ok {
		return fmt.Errorf("client %q not found", clientID)
	}

	close(client.Send)
	return client.Conn.Close()
}

// handleWebSocket handles incoming WebSocket upgrade requests.
func (a *Adapter) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[websocket] upgrade error: %v", err)
		return
	}

	clientID := generateClientID()
	client := &Client{
		ID:   clientID,
		Conn: conn,
		Send: make(chan []byte, 256),
	}

	a.mu.Lock()
	a.clients[clientID] = client
	a.mu.Unlock()

	if a.config.OnConnect != nil {
		a.config.OnConnect(clientID)
	}

	go client.writePump(a)
	go client.readPump(a)
}

// generateClientID generates a unique client identifier.
func generateClientID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
