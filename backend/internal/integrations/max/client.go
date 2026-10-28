package max

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

type Client struct {
	token         string
	defaultChatID string
	baseURL       string
	http          *http.Client
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		token:         cfg.MaxBotToken,
		defaultChatID: cfg.MaxDefaultChatID,
		baseURL:       cfg.MaxAPIBaseURL,
		http: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

func (c *Client) Configured() bool {
	return c != nil && strings.TrimSpace(c.token) != ""
}

func (c *Client) DefaultChatConfigured() bool {
	return c != nil && strings.TrimSpace(c.defaultChatID) != ""
}

func (c *Client) NotifyDraftCreated(title, startsAt string) {
	if !c.DefaultChatConfigured() || !c.Configured() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := c.SendMessage(ctx, "chat_id", c.defaultChatID, fmt.Sprintf("Создан черновик соревнования: %s · %s", title, startsAt)); err != nil {
			log.Printf("MAX notify draft: %v", err)
		}
	}()
}

func (c *Client) requestTimeout() time.Duration {
	if c == nil || c.http == nil || c.http.Timeout == 0 {
		return 8 * time.Second
	}
	return c.http.Timeout
}

func (c *Client) SendMessage(ctx context.Context, targetType, targetID, message string) error {
	if !c.Configured() {
		return errors.New("MAX_BOT_TOKEN is not configured")
	}
	parsed, err := url.Parse(c.baseURL + "/messages")
	if err != nil {
		return err
	}
	q := parsed.Query()
	q.Set(targetType, targetID)
	parsed.RawQuery = q.Encode()
	body, err := json.Marshal(map[string]any{"text": message, "notify": true})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("MAX API returned %s: %s", resp.Status, string(data))
	}
	return nil
}
