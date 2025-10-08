// WebSocket trigger
package triggers

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/madcok-co/unicorn"
)

type WebSocketTrigger struct {
	addr     string
	upgrader websocket.Upgrader
	clients  map[*websocket.Conn]bool
	mu       sync.RWMutex
}

func NewWebSocketTrigger(addr string) *WebSocketTrigger
func (t *WebSocketTrigger) Start() error
func (t *WebSocketTrigger) Stop() error
func (t *WebSocketTrigger) RegisterService(def *unicorn.Definition) error
func (t *WebSocketTrigger) Broadcast(message []byte) error
