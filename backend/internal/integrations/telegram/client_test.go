package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"orenkod/backend/internal/config"
)

func TestSendMessageUsesTelegramBotAPI(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer server.Close()

	client := NewClient(config.Config{TelegramBotToken: "test-token", TelegramAPIBaseURL: server.URL})
	client.http = server.Client()
	if err := client.SendMessage(context.Background(), "12345", "Hello"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if gotPath != "/bottest-token/sendMessage" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["chat_id"] != "12345" || gotBody["text"] != "Hello" {
		t.Fatalf("unexpected payload: %#v", gotBody)
	}
}

func TestTelegramAPIErrorIsReturned(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":401,"description":"Unauthorized"}`))
	}))
	defer server.Close()
	client := NewClient(config.Config{TelegramBotToken: "bad-token", TelegramAPIBaseURL: server.URL})
	client.http = server.Client()
	err := client.SendMessage(context.Background(), "123", "Hi")
	if err == nil || !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("expected API error, got %v", err)
	}
}

func TestBotStartRepliesInSameChat(t *testing.T) {
	var gotChatID any
	var gotText string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			ChatID any    `json:"chat_id"`
			Text   string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		gotChatID, gotText = payload.ChatID, payload.Text
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer server.Close()
	client := NewClient(config.Config{TelegramBotToken: "token", TelegramAPIBaseURL: server.URL})
	client.http = server.Client()
	h := NewBotHandler(client, nil, nil, nil)
	update := Update{UpdateID: 1, Message: &Message{From: &User{ID: 50}, Chat: Chat{ID: 50, Type: "private"}, Text: "/start"}}
	if err := h.handleUpdate(context.Background(), update); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if gotChatID != "50" {
		t.Fatalf("chat_id = %#v", gotChatID)
	}
	if !strings.Contains(gotText, "/events") || !strings.Contains(gotText, "/link КОД") {
		t.Fatalf("unexpected start reply: %s", gotText)
	}
}

func TestNetworkErrorDoesNotExposeBotToken(t *testing.T) {
	client := NewClient(config.Config{TelegramBotToken: "secret-test-token", TelegramAPIBaseURL: "http://127.0.0.1:1"})
	client.http = &http.Client{Timeout: 100 * time.Millisecond}
	err := client.SendMessage(context.Background(), "123", "Hi")
	if err == nil {
		t.Fatal("expected a network error")
	}
	if strings.Contains(err.Error(), "secret-test-token") {
		t.Fatalf("error exposed Telegram bot token: %v", err)
	}
}
