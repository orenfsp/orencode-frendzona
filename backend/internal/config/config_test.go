package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_DEFAULT_CHAT_ID", "")
	cfg := Load()
	if cfg.Port != DefaultPort {
		t.Fatalf("port: got %q want %q", cfg.Port, DefaultPort)
	}
	if cfg.DatabaseURL != DefaultDatabase {
		t.Fatal("database url mismatch")
	}
	if cfg.TelegramConfigured() || cfg.DefaultChatConfigured() {
		t.Fatal("Telegram flags must be false when environment is empty")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://demo")
	t.Setenv("TELEGRAM_BOT_TOKEN", " token ")
	t.Setenv("TELEGRAM_DEFAULT_CHAT_ID", "-100123")
	cfg := Load()
	if cfg.Port != "9090" {
		t.Fatalf("port: got %q", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://demo" {
		t.Fatalf("database: got %q", cfg.DatabaseURL)
	}
	if cfg.TelegramBotToken != "token" {
		t.Fatalf("token should be trimmed, got %q", cfg.TelegramBotToken)
	}
	if !cfg.TelegramConfigured() || !cfg.DefaultChatConfigured() {
		t.Fatal("Telegram flags must be true when values are set")
	}
	if cfg.TelegramAPIBaseURL != "https://api.telegram.org" {
		t.Fatalf("unexpected Telegram API base URL %q", cfg.TelegramAPIBaseURL)
	}
}
