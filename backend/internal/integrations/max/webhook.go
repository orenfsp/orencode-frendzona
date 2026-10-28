package max

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"orenkod/backend/internal/competitions"
	"orenkod/backend/internal/httpserver"
	"orenkod/backend/internal/ratings"
)

type Update struct {
	UpdateType string `json:"update_type"`
	Message    struct {
		Sender struct {
			UserID int64 `json:"user_id"`
		} `json:"sender"`
		Recipient struct {
			ChatID int64 `json:"chat_id"`
		} `json:"recipient"`
		Body struct {
			Text string `json:"text"`
		} `json:"body"`
	} `json:"message"`
}

type WebhookHandler struct {
	secret       string
	client       *Client
	competitions *competitions.Service
	ratings      *ratings.Repository
}

func NewWebhookHandler(secret string, client *Client, competitionsService *competitions.Service, ratingsRepo *ratings.Repository) *WebhookHandler {
	return &WebhookHandler{
		secret:       strings.TrimSpace(secret),
		client:       client,
		competitions: competitionsService,
		ratings:      ratingsRepo,
	}
}

func (h *WebhookHandler) Status(w http.ResponseWriter, r *http.Request) {
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{
		"configured":                h.client.Configured(),
		"default_chat_configured":   h.client.DefaultChatConfigured(),
		"webhook_secret_configured": h.secret != "",
		"webhook_path":              "/api/integrations/max/webhook",
	})
}

func (h *WebhookHandler) Receive(w http.ResponseWriter, r *http.Request) {
	if h.secret != "" && r.Header.Get("X-Max-Bot-Api-Secret") != h.secret {
		httpserver.WriteError(w, http.StatusUnauthorized, "Неверный секрет webhook")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "Не удалось прочитать событие")
		return
	}
	var update Update
	if err = json.Unmarshal(payload, &update); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "Неверный JSON события MAX")
		return
	}
	if update.UpdateType == "message_created" || update.UpdateType == "bot_started" {
		if err := h.reply(r.Context(), update); err != nil {
			log.Printf("MAX webhook reply: %v", err)
		}
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *WebhookHandler) reply(ctx context.Context, update Update) error {
	text := strings.TrimSpace(update.Message.Body.Text)
	var reply string
	switch strings.ToLower(text) {
	case "/start", "start":
		reply = "Привет! Я помощник цифровой платформы ФСП Оренбуржья.\nКоманды: /events — ближайшие соревнования, /rating — рейтинг, /help — помощь."
	case "/events", "events":
		items, err := h.competitions.ListUpcoming(ctx)
		if err != nil {
			reply = "Сейчас нет ближайших соревнований."
			log.Printf("MAX /events: %v", err)
		} else {
			reply = competitions.FormatUpcomingMessage(items)
		}
	case "/rating", "rating":
		items, err := h.ratings.List(ctx, "", 5)
		if err != nil {
			reply = "Рейтинг пока пуст."
			log.Printf("MAX /rating: %v", err)
		} else {
			reply = ratings.FormatTopMessage(items)
		}
	case "/help", "help":
		reply = "Команды:\n/events — ближайшие соревнования\n/rating — топ рейтинга\n/help — список команд"
	default:
		if update.UpdateType == "bot_started" {
			reply = "Привет! Напиши /help, чтобы увидеть команды."
		}
	}
	if reply == "" || !h.client.Configured() {
		return nil
	}
	sendCtx, cancel := context.WithTimeout(context.Background(), h.client.requestTimeout())
	defer cancel()
	if update.Message.Recipient.ChatID != 0 {
		return h.client.SendMessage(sendCtx, "chat_id", strconv.FormatInt(update.Message.Recipient.ChatID, 10), reply)
	}
	if update.Message.Sender.UserID != 0 {
		return h.client.SendMessage(sendCtx, "user_id", strconv.FormatInt(update.Message.Sender.UserID, 10), reply)
	}
	return nil
}
