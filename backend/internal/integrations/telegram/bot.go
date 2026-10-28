package telegram

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"orenkod/backend/internal/competitions"
	"orenkod/backend/internal/httpserver"
	"orenkod/backend/internal/ratings"
)

type BotHandler struct {
	polling      atomic.Bool
	db           *sql.DB
	client       *Client
	competitions *competitions.Service
	ratings      *ratings.Repository
}

func NewBotHandler(client *Client, competitionsService *competitions.Service, ratingsRepo *ratings.Repository, db *sql.DB) *BotHandler {
	return &BotHandler{db: db, client: client, competitions: competitionsService, ratings: ratingsRepo}
}

func (h *BotHandler) Status(w http.ResponseWriter, _ *http.Request) {
	configured := h != nil && h.client != nil && h.client.Configured()
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{
		"configured":              configured,
		"default_chat_configured": h != nil && h.client != nil && h.client.DefaultChatConfigured(),
		"mode":                    "long_polling",
		"polling_enabled":         h != nil && h.polling.Load(),
	})
}

// Run consumes Telegram updates using long polling. It works against localhost
// and deliberately does not require a public URL or webhook certificate.
func (h *BotHandler) Run(ctx context.Context) {
	if h == nil || h.client == nil || !h.client.Configured() {
		log.Printf("Telegram bot is not configured; set TELEGRAM_BOT_TOKEN to enable it")
		return
	}
	for {
		if ctx.Err() != nil {
			return
		}
		if err := h.client.DeleteWebhook(ctx); err != nil {
			h.polling.Store(false)
			log.Printf("Telegram deleteWebhook: %v; retrying", err)
			if !waitOrCancel(ctx, 5*time.Second) {
				return
			}
			continue
		}
		break
	}
	h.polling.Store(true)
	log.Printf("Telegram bot long polling started")
	var offset int64
	for {
		if ctx.Err() != nil {
			h.polling.Store(false)
			return
		}
		updates, err := h.client.GetUpdates(ctx, offset, 25)
		if err != nil {
			h.polling.Store(false)
			if ctx.Err() != nil {
				return
			}
			log.Printf("Telegram getUpdates: %v", err)
			if !waitOrCancel(ctx, 3*time.Second) {
				return
			}
			continue
		}
		h.polling.Store(true)
		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			if err := h.handleUpdate(ctx, update); err != nil {
				log.Printf("Telegram update %d: %v", update.UpdateID, err)
			}
		}
	}
}

func waitOrCancel(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (h *BotHandler) handleUpdate(ctx context.Context, update Update) error {
	if update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		return nil
	}
	message := update.Message
	text := strings.TrimSpace(message.Text)
	if !strings.HasPrefix(text, "/") {
		return nil
	}
	parts := strings.Fields(text)
	command := strings.ToLower(strings.SplitN(parts[0], "@", 2)[0])
	userID := message.From.ID
	chatID := message.Chat.ID

	var reply string
	switch command {
	case "/start", "/help":
		reply = "Привет! Я Telegram-бот цифровой платформы ФСП Оренбуржья.\n\nКоманды:\n/events — ближайшие соревнования\n/rating — топ рейтинга\n/link КОД — привязать аккаунт ФСП 56\n/chatid — показать ID этого чата\n/help — помощь."
	case "/events":
		if h.competitions == nil {
			reply = "Список соревнований сейчас недоступен."
		} else {
			items, err := h.competitions.ListUpcoming(ctx)
			if err != nil {
				log.Printf("Telegram /events: %v", err)
				reply = "Сейчас не удалось загрузить ближайшие соревнования. Попробуйте позже."
			} else {
				reply = competitions.FormatUpcomingMessage(items)
			}
		}
	case "/rating":
		if h.ratings == nil {
			reply = "Рейтинг сейчас недоступен."
		} else {
			items, err := h.ratings.List(ctx, "", 5)
			if err != nil {
				log.Printf("Telegram /rating: %v", err)
				reply = "Рейтинг пока недоступен. Попробуйте позже."
			} else {
				reply = ratings.FormatTopMessage(items)
			}
		}
	case "/link":
		if message.Chat.Type != "private" {
			reply = "Для безопасной привязки аккаунта откройте личный чат с ботом и отправьте /link КОД."
		} else if len(parts) != 2 || len(parts[1]) != 8 || !isDigits(parts[1]) {
			reply = "Создайте одноразовый код в разделе «Уведомления» ФСП 56 и отправьте: /link 12345678"
		} else if h.db == nil {
			reply = "Привязка аккаунта временно недоступна. Попробуйте позже."
		} else {
			name, err := h.linkAccount(ctx, userID, parts[1])
			switch {
			case err == nil:
				reply = "Готово, " + name + "! Аккаунт платформы ФСП 56 подключён. Персональные уведомления будут приходить сюда."
			case errors.Is(err, sql.ErrNoRows):
				reply = "Код не найден или истёк. Создайте новый код в разделе «Уведомления» платформы."
			default:
				log.Printf("Telegram /link: %v", err)
				reply = "Не удалось подключить аккаунт. Проверьте код и попробуйте ещё раз."
			}
		}
	case "/chatid":
		reply = fmt.Sprintf("ID этого Telegram-чата: %d\nЕсли это нужная группа, укажите его в TELEGRAM_DEFAULT_CHAT_ID в файле .env.", chatID)
	default:
		return nil
	}
	if !h.client.Configured() {
		return nil
	}
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return h.client.SendMessage(sendCtx, strconv.FormatInt(chatID, 10), reply)
}

func (h *BotHandler) linkAccount(ctx context.Context, telegramUserID int64, code string) (string, error) {
	if telegramUserID <= 0 || h.db == nil {
		return "", errors.New("invalid Telegram user ID or unavailable database")
	}
	var fullName string
	err := h.db.QueryRowContext(ctx, `UPDATE users
		SET telegram_user_id=$1, telegram_link_code=NULL, telegram_link_expires_at=NULL
		WHERE telegram_link_code=$2 AND telegram_link_expires_at > NOW()
		RETURNING full_name`, telegramUserID, code).Scan(&fullName)
	return fullName, err
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
