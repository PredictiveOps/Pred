package httpapi

import (
	"encoding/json"
	"net/http"

	"event-processing-service/internal/app"
)

type Handler struct {
	Svc *app.Service
}

func NewHandler(svc *app.Service) *Handler {
	return &Handler{Svc: svc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /events", h.createEvent)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) createEvent(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	raw, err := json.Marshal(body)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	id, err := h.Svc.Ingest(r.Context(), raw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "stored",
		"id":     id,
	})
}
