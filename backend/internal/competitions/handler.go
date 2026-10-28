package competitions

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"orenkod/backend/internal/httpserver"
)

type Notifier interface {
	NotifyDraftCreated(title, startsAt string)
}

type Handler struct {
	service  *Service
	notifier Notifier
}

func NewHandler(service *Service, notifier Notifier) *Handler {
	return &Handler{service: service, notifier: notifier}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		log.Printf("list competitions: %v", err)
		httpserver.WriteError(w, http.StatusInternalServerError, ErrListFailed.Error())
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var input CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, ErrInvalidJSON.Error())
		return
	}
	c, err := h.service.Create(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, ErrRequired), errors.Is(err, ErrInvalidDate), errors.Is(err, ErrInvalidJSON):
			httpserver.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrDuplicate):
			httpserver.WriteError(w, http.StatusConflict, err.Error())
		default:
			log.Printf("create competition: %v", err)
			httpserver.WriteError(w, http.StatusInternalServerError, ErrCreateFailed.Error())
		}
		return
	}
	if h.notifier != nil {
		h.notifier.NotifyDraftCreated(c.Title, c.StartsAt)
	}
	httpserver.WriteJSON(w, http.StatusCreated, c)
}
