package dashboard

import (
	"database/sql"
	"log"
	"net/http"

	"orenkod/backend/internal/httpserver"
)

type Handler struct {
	db   *sql.DB
	repo *Repository
}

func NewHandler(db *sql.DB, repo *Repository) *Handler {
	return &Handler{db: db, repo: repo}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.db.PingContext(r.Context()); err != nil {
		httpserver.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "degraded", "database": false})
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok", "database": true, "service": "orenkod-api"})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.repo.Snapshot(r.Context())
	if err != nil {
		log.Printf("dashboard: %v", err)
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить сводку")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, snapshot)
}
