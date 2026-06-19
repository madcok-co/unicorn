package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ============ Test helpers ============

// newTestServer creates an httptest server with the adapter's WS handler
// and returns the adapter, server URL, and a cleanup function.
func newTestServer(t *testing.T, cfg *Config) (*Adapter, string, func()) {
	t.Helper()
	adapter := NewAdapter(cfg)

	mux := http.NewServeMux()
	mux.HandleFunc(adapter.config.Path, adapter.handleWebSocket)

	srv := httptest.NewServer(mux)

	cleanup := func() {
		srv.Close()
	}

	return adapter, srv.URL, cleanup
}

// connectWS creates a WebSocket client connected to the test server.
func connectWS(t *testing.T, url string, path string) *websocket.Conn {
	t.Helper()
	wsURL := strings.Replace(url, "http://", "ws://", 1) + path
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	return conn
}

// ============ NewAdapter tests ============

func TestNewAdapter_Defaults(t *testing.T) {
	a := NewAdapter(nil)

	if a.config.Host != "0.0.0.0" {
		t.Errorf("expected Host 0.0.0.0, got %s", a.config.Host)
	}
	if a.config.Port != 8081 {
		t.Errorf("expected Port 8081, got %d", a.config.Port)
	}
	if a.config.Path != "/ws" {
		t.Errorf("expected Path /ws, got %s", a.config.Path)
	}
	if a.config.MaxMsgSize != 4096 {
		t.Errorf("expected MaxMsgSize 4096, got %d", a.config.MaxMsgSize)
	}
	if a.config.PingPeriod != 54*time.Second {
		t.Errorf("expected PingPeriod 54s, got %v", a.config.PingPeriod)
	}
	if a.config.PongWait != 60*time.Second {
		t.Errorf("expected PongWait 60s, got %v", a.config.PongWait)
	}
	if a.config.WriteTimeout != 10*time.Second {
		t.Errorf("expected WriteTimeout 10s, got %v", a.config.WriteTimeout)
	}
}

func TestNewAdapter_CustomConfig(t *testing.T) {
	a := NewAdapter(&Config{
		Host:       "127.0.0.1",
		Port:       9090,
		Path:       "/custom",
		MaxMsgSize: 8192,
	})

	if a.config.Host != "127.0.0.1" {
		t.Errorf("expected Host 127.0.0.1, got %s", a.config.Host)
	}
	if a.config.Port != 9090 {
		t.Errorf("expected Port 9090, got %d", a.config.Port)
	}
	if a.config.Path != "/custom" {
		t.Errorf("expected Path /custom, got %s", a.config.Path)
	}
	if a.config.MaxMsgSize != 8192 {
		t.Errorf("expected MaxMsgSize 8192, got %d", a.config.MaxMsgSize)
	}
}

func TestNewAdapter_PartialDefaults(t *testing.T) {
	// Only override some fields, others get defaults
	a := NewAdapter(&Config{
		Port: 3000,
	})

	if a.config.Host != "0.0.0.0" {
		t.Errorf("expected default Host, got %s", a.config.Host)
	}
	if a.config.Port != 3000 {
		t.Errorf("expected Port 3000, got %d", a.config.Port)
	}
	if a.config.Path != "/ws" {
		t.Errorf("expected default Path, got %s", a.config.Path)
	}
	if a.config.PingPeriod != 54*time.Second {
		t.Errorf("expected default PingPeriod, got %v", a.config.PingPeriod)
	}
}

// ============ Name test ============

func TestName(t *testing.T) {
	a := NewAdapter(nil)
	if a.Name() != "websocket" {
		t.Errorf("expected Name 'websocket', got %q", a.Name())
	}
}

// ============ ClientCount tests ============

func TestClientCount_Zero(t *testing.T) {
	a := NewAdapter(nil)
	if a.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", a.ClientCount())
	}
}

// ============ Broadcast_NoClients test ============

func TestBroadcast_NoClients(t *testing.T) {
	a := NewAdapter(nil)
	// Should not panic when no clients
	a.Broadcast([]byte("hello"))
}

// ============ SendTo_NotFound test ============

func TestSendTo_NotFound(t *testing.T) {
	a := NewAdapter(nil)
	err := a.SendTo("nonexistent", []byte("hello"))
	if err == nil {
		t.Fatal("expected error for unknown client")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// ============ KickClient_NotFound test ============

func TestKickClient_NotFound(t *testing.T) {
	a := NewAdapter(nil)
	err := a.KickClient("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown client")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// ============ ClientIDs test ============

func TestClientIDs_Empty(t *testing.T) {
	a := NewAdapter(nil)
	ids := a.ClientIDs()
	if len(ids) != 0 {
		t.Errorf("expected empty slice, got %v", ids)
	}
}

// ============ UpgraderConfig tests ============

func TestUpgraderConfig(t *testing.T) {
	a := NewAdapter(nil)

	// Check CORS is permissive (CheckOrigin returns true)
	if !a.upgrader.CheckOrigin(nil) {
		t.Error("expected CheckOrigin to return true by default")
	}

	if a.upgrader.ReadBufferSize != 1024 {
		t.Errorf("expected ReadBufferSize 1024, got %d", a.upgrader.ReadBufferSize)
	}
	if a.upgrader.WriteBufferSize != 1024 {
		t.Errorf("expected WriteBufferSize 1024, got %d", a.upgrader.WriteBufferSize)
	}
}

// ============ WebSocket Connect/Disconnect tests ============

func TestWebSocket_Connect(t *testing.T) {
	a, url, cleanup := newTestServer(t, nil)
	defer cleanup()

	conn := connectWS(t, url, "/ws")
	defer conn.Close()

	// Wait for connection to register
	time.Sleep(50 * time.Millisecond)

	if a.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", a.ClientCount())
	}

	ids := a.ClientIDs()
	if len(ids) != 1 {
		t.Fatalf("expected 1 client ID, got %d", len(ids))
	}
	if ids[0] == "" {
		t.Error("client ID should not be empty")
	}
	if len(ids[0]) != 32 {
		t.Errorf("expected hex-encoded 16-byte ID (32 chars), got %d: %s", len(ids[0]), ids[0])
	}
}

func TestWebSocket_Disconnect(t *testing.T) {
	a, url, cleanup := newTestServer(t, nil)
	defer cleanup()

	conn := connectWS(t, url, "/ws")
	time.Sleep(50 * time.Millisecond)

	if a.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", a.ClientCount())
	}

	conn.Close()
	time.Sleep(100 * time.Millisecond)

	if a.ClientCount() != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", a.ClientCount())
	}
}

// ============ Send/Receive tests ============

func TestWebSocket_SendReceive(t *testing.T) {
	var receivedMsg []byte
	var receivedClientID string
	done := make(chan struct{})

	_, url, cleanup := newTestServer(t, &Config{
		OnMessage: func(clientID string, msg []byte) {
			receivedClientID = clientID
			receivedMsg = msg
			close(done)
		},
	})
	defer cleanup()

	conn := connectWS(t, url, "/ws")
	defer conn.Close()

	// Send from client to server
	err := conn.WriteMessage(websocket.TextMessage, []byte("hello server"))
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}

	if string(receivedMsg) != "hello server" {
		t.Errorf("expected 'hello server', got %q", receivedMsg)
	}
	if receivedClientID == "" {
		t.Error("client ID should not be empty")
	}
}

// ============ Server-to-Client Send tests ============

func TestWebSocket_ServerToClient(t *testing.T) {
	a, url, cleanup := newTestServer(t, nil)
	defer cleanup()

	conn := connectWS(t, url, "/ws")
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Get the client ID
	ids := a.ClientIDs()
	if len(ids) != 1 {
		t.Fatalf("expected 1 client, got %d", len(ids))
	}
	clientID := ids[0]

	// Send from server to client
	err := a.SendTo(clientID, []byte("hello client"))
	if err != nil {
		t.Fatalf("SendTo failed: %v", err)
	}

	// Read on client
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if string(msg) != "hello client" {
		t.Errorf("expected 'hello client', got %q", msg)
	}
}

// ============ Broadcast tests ============

func TestWebSocket_Broadcast(t *testing.T) {
	a, url, cleanup := newTestServer(t, nil)
	defer cleanup()

	const numClients = 3
	conns := make([]*websocket.Conn, numClients)

	for i := range numClients {
		conns[i] = connectWS(t, url, "/ws")
		defer conns[i].Close()
	}

	time.Sleep(100 * time.Millisecond)

	if a.ClientCount() != numClients {
		t.Fatalf("expected %d clients, got %d", numClients, a.ClientCount())
	}

	// Broadcast to all
	a.Broadcast([]byte("broadcast"))

	// All clients should receive
	for i, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("client %d failed to read: %v", i, err)
		}
		if string(msg) != "broadcast" {
			t.Errorf("client %d: expected 'broadcast', got %q", i, msg)
		}
	}
}

// ============ KickClient test ============

func TestWebSocket_KickClient(t *testing.T) {
	a, url, cleanup := newTestServer(t, nil)
	defer cleanup()

	conn := connectWS(t, url, "/ws")
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	ids := a.ClientIDs()
	if len(ids) != 1 {
		t.Fatalf("expected 1 client, got %d", len(ids))
	}
	clientID := ids[0]

	err := a.KickClient(clientID)
	if err != nil {
		t.Fatalf("KickClient failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if a.ClientCount() != 0 {
		t.Errorf("expected 0 clients after kick, got %d", a.ClientCount())
	}

	// Verify the connection is closed from client side
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected read error after kick")
	}
}

// ============ OnConnect/OnDisconnect callbacks ============

func TestWebSocket_OnConnectCallback(t *testing.T) {
	var connectedID string
	done := make(chan struct{})

	_, url, cleanup := newTestServer(t, &Config{
		OnConnect: func(clientID string) {
			connectedID = clientID
			close(done)
		},
	})
	defer cleanup()

	conn := connectWS(t, url, "/ws")
	defer conn.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnConnect callback")
	}

	if connectedID == "" {
		t.Error("OnConnect callback should receive client ID")
	}
}

func TestWebSocket_OnDisconnectCallback(t *testing.T) {
	var disconnectedID string
	done := make(chan struct{})

	_, url, cleanup := newTestServer(t, &Config{
		OnDisconnect: func(clientID string) {
			disconnectedID = clientID
			close(done)
		},
	})
	defer cleanup()

	conn := connectWS(t, url, "/ws")
	time.Sleep(50 * time.Millisecond)

	conn.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnDisconnect callback")
	}

	if disconnectedID == "" {
		t.Error("OnDisconnect callback should receive client ID")
	}
}

// ============ Start/Stop test ============

func TestStartStop(t *testing.T) {
	a := NewAdapter(&Config{
		Host: "127.0.0.1",
		Port: 0, // use random port
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Start is blocking so run in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- a.Start(ctx)
	}()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Cancel context to trigger Stop
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for Start to return after context cancel")
	}
}

// ============ Stop with connected clients ============

func TestStop_CleansUpClients(t *testing.T) {
	a, url, cleanup := newTestServer(t, &Config{
		Host: "127.0.0.1",
		Port: 0,
	})
	defer cleanup()

	// Connect real WebSocket clients
	conn1 := connectWS(t, url, "/ws")
	conn2 := connectWS(t, url, "/ws")
	defer func() {
		conn1.Close()
		conn2.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	if a.ClientCount() != 2 {
		t.Fatalf("expected 2 clients, got %d", a.ClientCount())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := a.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	if a.ClientCount() != 0 {
		t.Errorf("expected 0 clients after Stop, got %d", a.ClientCount())
	}
}

// ============ Concurrency tests ============

func TestConcurrentAccess(t *testing.T) {
	a, url, cleanup := newTestServer(t, nil)
	defer cleanup()

	const numClients = 10
	var wg sync.WaitGroup

	// Connect multiple clients concurrently
	for range numClients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn := connectWS(t, url, "/ws")
			// Keep connection open briefly
			time.Sleep(200 * time.Millisecond)
			conn.Close()
		}()
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	// Concurrent broadcasts shouldn't panic or deadlock
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.Broadcast([]byte("test"))
			a.ClientCount()
			a.ClientIDs()
		}()
	}
	wg.Wait()
}

// ============ Multiple clients, targeted send ============

func TestWebSocket_TargetedSend(t *testing.T) {
	a, url, cleanup := newTestServer(t, nil)
	defer cleanup()

	conn1 := connectWS(t, url, "/ws")
	defer conn1.Close()
	conn2 := connectWS(t, url, "/ws")
	defer conn2.Close()

	time.Sleep(100 * time.Millisecond)

	ids := a.ClientIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(ids))
	}

	// Send to first client only
	err := a.SendTo(ids[0], []byte("only you"))
	if err != nil {
		t.Fatalf("SendTo failed: %v", err)
	}

	// conn1 should receive the message
	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg1, err := conn1.ReadMessage()
	if err != nil {
		t.Fatalf("conn1 read failed: %v", err)
	}
	if string(msg1) != "only you" {
		t.Errorf("conn1: expected 'only you', got %q", msg1)
	}

	// conn2 should NOT receive the message
	conn2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err = conn2.ReadMessage()
	if err == nil {
		t.Error("conn2 should not have received the targeted message")
	}
}

// ============ generateClientID uniqueness ============

func TestGenerateClientID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for range 1000 {
		id := generateClientID()
		if seen[id] {
			t.Fatal("duplicate client ID generated")
		}
		seen[id] = true
		if len(id) != 32 {
			t.Fatalf("expected 32-char hex ID, got %d: %s", len(id), id)
		}
	}
}

// ============ Max message size test ============

func TestWebSocket_MaxMessageSize(t *testing.T) {
	_, url, cleanup := newTestServer(t, &Config{
		MaxMsgSize: 10,
	})
	defer cleanup()

	conn := connectWS(t, url, "/ws")
	defer conn.Close()

	// Send a message larger than MaxMsgSize
	err := conn.WriteMessage(websocket.TextMessage, []byte("this message is way too long"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// The server should close the connection due to size limit
	time.Sleep(100 * time.Millisecond)

	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected connection to be closed due to oversized message")
	}
}
