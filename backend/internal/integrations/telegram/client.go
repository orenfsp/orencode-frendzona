package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"orenkod/backend/internal/config"
)

// Client is a small Telegram Bot API client. This project uses long polling,
// so the local prototype does not need a public HTTPS webhook endpoint.
type Client struct {
	token         string
	defaultChatID string
	baseURL       string
	http          *http.Client
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		token:         strings.TrimSpace(cfg.TelegramBotToken),
		defaultChatID: strings.TrimSpace(cfg.TelegramDefaultChatID),
		baseURL:       strings.TrimRight(cfg.TelegramAPIBaseURL, "/"),
		http:          &http.Client{Timeout: 40 * time.Second},
	}
}

func (c *Client) Configured() bool {
	return c != nil && strings.TrimSpace(c.token) != ""
}

func (c *Client) DefaultChatConfigured() bool {
	return c != nil && strings.TrimSpace(c.defaultChatID) != ""
}

func (c *Client) RequestTimeout() time.Duration {
	if c == nil || c.http == nil || c.http.Timeout == 0 {
		return 8 * time.Second
	}
	return c.http.Timeout
}

func (c *Client) apiURL(method string) string {
	base := "https://api.telegram.org"
	if c != nil && strings.TrimSpace(c.baseURL) != "" {
		base = strings.TrimRight(c.baseURL, "/")
	}
	return base + "/bot" + c.token + "/" + method
}

func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	if !c.Configured() {
		return errors.New("TELEGRAM_BOT_TOKEN is not configured")
	}
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL(method), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	httpClient := c.http
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 40 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		// net/http wraps the full request URL (including the bot token) in url.Error.
		// Unwrap it before returning so normal application logs never print the token.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return fmt.Errorf("Telegram API %s request failed: %v", method, urlErr.Err)
		}
		return fmt.Errorf("Telegram API %s request failed (%T)", method, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	var envelope struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
		ErrorCode   int             `json:"error_code"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("Telegram API returned HTTP %s", resp.Status)
		}
		return fmt.Errorf("decode Telegram API response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !envelope.OK {
		return fmt.Errorf("Telegram API error (%d, HTTP %s): %s", envelope.ErrorCode, resp.Status, envelope.Description)
	}
	if result != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, result); err != nil {
			return fmt.Errorf("decode Telegram %s result: %w", method, err)
		}
	}
	return nil
}

func (c *Client) SendMessage(ctx context.Context, chatID string, message string) error {
	if strings.TrimSpace(chatID) == "" {
		return errors.New("Telegram chat_id is empty")
	}
	if strings.TrimSpace(message) == "" {
		return errors.New("Telegram message is empty")
	}
	return c.call(ctx, "sendMessage", map[string]any{"chat_id": chatID, "text": message, "disable_web_page_preview": true}, nil)
}

func (c *Client) NotifyChannel(title, message string) {
	if !c.Configured() || !c.DefaultChatConfigured() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		text := strings.TrimSpace(title)
		if strings.TrimSpace(message) != "" {
			text += "\n\n" + strings.TrimSpace(message)
		}
		if err := c.SendMessage(ctx, c.defaultChatID, text); err != nil {
			log.Printf("Telegram channel notification: %v", err)
		}
	}()
}

func (c *Client) NotifyDraftCreated(title, startsAt string) {
	title = strings.TrimSpace(title)
	startsAt = strings.TrimSpace(startsAt)
	message := fmt.Sprintf("Создан черновик соревнования «%s».", title)
	if startsAt != "" {
		message += fmt.Sprintf("\nНачало: %s", startsAt)
	}
	c.NotifyChannel("Новое соревнование", message)
}

// NotifyUser sends a direct message to a user who has explicitly linked their
// Telegram account. A user must have opened the bot at least once before this works.
func (c *Client) NotifyUser(userID int64, message string) {
	if !c.Configured() || userID <= 0 || strings.TrimSpace(message) == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.SendMessage(ctx, fmt.Sprintf("%d", userID), message); err != nil {
			log.Printf("Telegram notify user %d: %v", userID, err)
		}
	}()
}

func (c *Client) GetUpdates(ctx context.Context, offset int64, timeoutSeconds int) ([]Update, error) {
	if timeoutSeconds < 1 {
		timeoutSeconds = 25
	}
	if timeoutSeconds > 50 {
		timeoutSeconds = 50
	}
	var updates []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         timeoutSeconds,
		"allowed_updates": []string{"message"},
	}, &updates)
	return updates, err
}

func (c *Client) DeleteWebhook(ctx context.Context) error {
	return c.call(ctx, "deleteWebhook", map[string]any{"drop_pending_updates": false}, nil)
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}
