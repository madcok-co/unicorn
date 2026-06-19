package websocket

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// readPump reads messages from the WebSocket connection and forwards
// them to the OnMessage callback. It also handles pong responses for
// keep-alive.
func (c *Client) readPump(adapter *Adapter) {
	defer func() {
		adapter.mu.Lock()
		delete(adapter.clients, c.ID)
		adapter.mu.Unlock()

		c.Conn.Close()

		if adapter.config.OnDisconnect != nil {
			adapter.config.OnDisconnect(c.ID)
		}
	}()

	c.Conn.SetReadLimit(adapter.config.MaxMsgSize)
	c.Conn.SetReadDeadline(time.Now().Add(adapter.config.PongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(adapter.config.PongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[websocket] read error for client %s: %v", c.ID, err)
			}
			return
		}

		if adapter.config.OnMessage != nil {
			adapter.config.OnMessage(c.ID, message)
		}
	}
}

// writePump writes messages from the Send channel to the WebSocket
// connection. It also handles periodic ping messages for keep-alive.
func (c *Client) writePump(adapter *Adapter) {
	ticker := time.NewTicker(adapter.config.PingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(adapter.config.WriteTimeout))
			if !ok {
				// Channel closed, send close message and exit
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("[websocket] write error for client %s: %v", c.ID, err)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(adapter.config.WriteTimeout))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[websocket] ping error for client %s: %v", c.ID, err)
				return
			}
		}
	}
}
