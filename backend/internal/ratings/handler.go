package ratings

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"orenkod/backend/internal/httpserver"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.List(r.Context(), strings.TrimSpace(r.URL.Query().Get("discipline")), 0)
	if err != nil {
		log.Printf("list rating: %v", err)
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось рассчитать рейтинг")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"calculation": "Демонстрационная сортировка по сумме очков; регламент расчёта необходимо согласовать с ФСП.",
	})
}

func (h *Handler) Team(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.List(r.Context(), "", 5)
	if err != nil {
		log.Printf("list team: %v", err)
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить сборную")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"basis": "Топ-5 демонстрационного рейтинга. Реальный состав должен формироваться по регламенту отбора.",
	})
}

func FormatTopMessage(items []RatingEntry) string {
	if len(items) == 0 {
		return "Рейтинг пока пуст."
	}
	var b strings.Builder
	b.WriteString("Топ рейтинга (демо):\n")
	for _, a := range items {
		b.WriteString(fmt.Sprintf("%d. %s — %d очк.\n", a.Rank, a.Athlete, a.Points))
	}
	return strings.TrimSpace(b.String())
}
