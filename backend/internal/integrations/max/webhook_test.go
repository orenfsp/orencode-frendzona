package max

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"orenkod/backend/internal/config"
)

func TestWebhookRejectsInvalidSecret(t *testing.T) {
	h := NewWebhookHandler("expected-secret", NewClient(config.Config{}), nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/max/webhook", strings.NewReader(`{"update_type":"message_created"}`))
	req.Header.Set("X-Max-Bot-Api-Secret", "wrong")
	rec := httptest.NewRecorder()
	h.Receive(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookAcceptsWhenSecretUnset(t *testing.T) {
	h := NewWebhookHandler("", NewClient(config.Config{}), nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/max/webhook", strings.NewReader(`{"update_type":"message_created"}`))
	rec := httptest.NewRecorder()
	h.Receive(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookRejectsInvalidJSON(t *testing.T) {
	h := NewWebhookHandler("", NewClient(config.Config{}), nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/max/webhook", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	h.Receive(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestStatusDoesNotExposeSecrets(t *testing.T) {
	h := NewWebhookHandler("super-secret", NewClient(config.Config{MaxBotToken: "token-value", MaxDefaultChatID: "42"}), nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/integrations/max/status", nil)
	rec := httptest.NewRecorder()
	h.Status(rec, req)
	body := rec.Body.String()
	if strings.Contains(body, "super-secret") || strings.Contains(body, "token-value") {
		t.Fatalf("secrets leaked: %s", body)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}
