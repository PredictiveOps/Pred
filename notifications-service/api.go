package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"notifications-service/db"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for WebSocket (adjust as needed)
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func notificationsHandler(gdb *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		tenantID := r.Header.Get("X-Tenant-Id")
		if tenantID == "" {
			http.Error(w, "X-Tenant-Id header is required", http.StatusBadRequest)
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

		notifs, err := db.GetNotifications(r.Context(), gdb, tenantID, limit)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(notifs)
	}
}

func wsHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-Id")
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
	}
}

func healthHandler(gdb *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check database connectivity
		sqlDB, err := gdb.DB()
		if err != nil {
			http.Error(w, "database connection failed", http.StatusServiceUnavailable)
			return
		}

		if err := sqlDB.Ping(); err != nil {
			http.Error(w, "database ping failed", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"service": "notifications-service",
		})
	}
}

func startHTTPServer(gdb *gorm.DB, hub *Hub) {
	mux := http.NewServeMux()
	mux.HandleFunc("/list", notificationsHandler(gdb))
	mux.HandleFunc("/ws", wsHandler(hub))
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", healthHandler(gdb))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}
	server := &http.Server{
		Addr:              ":" + port,
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
