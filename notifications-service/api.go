package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type NotificationResponse struct {
	ID        int64           `json:"id"`
	TenantID  string          `json:"tenant_id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for WebSocket (adjust as needed)
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func startHTTPServer(gdb *gorm.DB, hub *Hub) {
	mux := http.NewServeMux()
	mux.HandleFunc("/notifications", func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		tenantID := r.URL.Query().Get("tenant_id")
		if tenantID == "" {
			http.Error(w, "tenant_id is required", http.StatusBadRequest)
			return
		}

		limitStr := r.URL.Query().Get("limit")

		limit := 10
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
		if limit <= 0 {
			limit = 10
		}
		if limit > 100 {
			limit = 100
		}

		var notifs []NotificationResponse

		err := gdb.Raw(`
			SELECT id, tenant_id, type, payload, created_at
			FROM notifications
			WHERE tenant_id = ?
			ORDER BY created_at DESC
			LIMIT ?
		`, tenantID, limit).Scan(&notifs).Error

		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(notifs)
	})

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.URL.Query().Get("tenant_id")
		if tenantID == "" {
			http.Error(w, "tenant_id is required", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade failed: %v", err)
			return
		}

		client := &Client{
			tenantID: tenantID,
			conn:     conn,
			send:     make(chan []byte, 256),
			hub:      hub,
		}

		hub.Register(tenantID, client)

		go client.writePump()
		go client.readPump()
	})

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http server failed: %v", err)
		}
	}()
}

func addCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
