package notifications

import (
	"net/http"

	"orenkod/backend/internal/httpserver"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func DemoItems() []Notification {
	return []Notification{
		{1, "warning", "Проверить судейские категории", "У 3 судей скоро истекает срок категории.", "Сегодня"},
		{2, "info", "Новые заявки на проверке", "12 заявок ожидают модерации Федерации.", "2 ч назад"},
		{3, "success", "Протокол готов к публикации", "По последнему турниру сформирован черновик протокола.", "Вчера"},
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": DemoItems()})
}
