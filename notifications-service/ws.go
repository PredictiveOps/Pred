package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSMessage is the message sent over WebSocket to clients.
type WSMessage struct {
	Type string          `json:"type"` // "new_notification"
	Data json.RawMessage `json:"data"` // Notification object
}

// Client represents a WebSocket connection for a specific tenant.
type Client struct {
	tenantID string
	conn     *websocket.Conn
	send     chan []byte
	hub      *Hub
}

// Hub manages all WebSocket connections grouped by tenant_id.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]bool // map[tenantID]map[*Client]bool
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]map[*Client]bool),
	}
}

// Register adds a new client to the hub for a specific tenant.
func (h *Hub) Register(tenantID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[tenantID] == nil {
		h.clients[tenantID] = make(map[*Client]bool)
	}
	h.clients[tenantID][client] = true
	log.Printf("WebSocket client registered for tenant %s (total: %d)", tenantID, len(h.clients[tenantID]))
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(tenantID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clientsForTenant, ok := h.clients[tenantID]; ok {
		if _, exists := clientsForTenant[client]; exists {
			delete(clientsForTenant, client)
			close(client.send)

			if len(clientsForTenant) == 0 {
				delete(h.clients, tenantID)
			}
			log.Printf("WebSocket client unregistered for tenant %s", tenantID)
		}
	}
}

// Broadcast sends a message to all clients for a specific tenant.
func (h *Hub) Broadcast(tenantID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clientsForTenant, ok := h.clients[tenantID]; ok {
		for client := range clientsForTenant {
			select {
			case client.send <- message:
			default:
				// Client's send channel is full, skip
				log.Printf("WebSocket send buffer full for tenant %s, skipping", tenantID)
			}
		}
	}
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c.tenantID, c)
		c.conn.Close()
	}()

	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for tenant %s: %v", c.tenantID, err)
			}
			break
		}
	}
}

// writePump sends messages to the WebSocket connection.
func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for message := range c.send {
		w, err := c.conn.NextWriter(websocket.TextMessage)
		if err != nil {
			return
		}
		w.Write(message)
		if err := w.Close(); err != nil {
			return
		}
	}
}
