package platform

import (
	cryptorand "crypto/rand"
	"fmt"
	"math/big"
	"net/http"

	"orenkod/backend/internal/auth"
	"orenkod/backend/internal/httpserver"
)

const telegramLinkCodeTTLSeconds = 600

// TelegramLinkStatus only reports whether the authenticated platform account is linked.
// The external Telegram identifier is never returned to the browser.
func (h *Handler) TelegramLinkStatus(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	var linked bool
	if err := h.db.QueryRowContext(r.Context(), `SELECT telegram_user_id IS NOT NULL FROM users WHERE id=$1`, user.ID).Scan(&linked); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось проверить подключение Telegram")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"linked": linked})
}

// CreateTelegramLinkCode issues a one-time eight-digit pairing code that expires in ten minutes.
func (h *Handler) CreateTelegramLinkCode(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	if h.notify == nil || !h.notify.Configured() {
		httpserver.WriteError(w, http.StatusServiceUnavailable, "Интеграция Telegram не настроена на сервере")
		return
	}
	_, _ = h.db.ExecContext(r.Context(), `UPDATE users SET telegram_link_code=NULL, telegram_link_expires_at=NULL WHERE telegram_link_expires_at <= NOW()`)
	for attempt := 0; attempt < 3; attempt++ {
		code, err := newTelegramLinkCode()
		if err != nil {
			httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось создать код подключения")
			return
		}
		_, err = h.db.ExecContext(r.Context(), `UPDATE users
			SET telegram_link_code=$1, telegram_link_expires_at=NOW() + INTERVAL '10 minutes'
			WHERE id=$2`, code, user.ID)
		if err == nil {
			h.audit(r, "создание кода Telegram", "users", "создан одноразовый код подключения")
			httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"code": code, "expires_in_seconds": telegramLinkCodeTTLSeconds})
			return
		}
		// A rare code collision is retried. Other database errors will fail all
		// attempts and are returned below instead of exposing SQL details.
		if attempt == 2 {
			httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить код подключения Telegram")
			return
		}
	}
}

func (h *Handler) UnlinkTelegram(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	if _, err := h.db.ExecContext(r.Context(), `UPDATE users SET telegram_user_id=NULL, telegram_link_code=NULL, telegram_link_expires_at=NULL WHERE id=$1`, user.ID); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось отключить Telegram")
		return
	}
	h.audit(r, "отключение Telegram", "users", "пользователь отключил Telegram уведомления")
	httpserver.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true, "linked": false})
}

func newTelegramLinkCode() (string, error) {
	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(100_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%08d", n.Int64()), nil
}
