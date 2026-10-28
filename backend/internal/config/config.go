package config

import (
	"os"
	"strings"
)

const (
	DefaultPort     = "8080"
	DefaultDatabase = "postgres://orenkod:orenkod_local@localhost:5432/orenkod?sslmode=disable"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	TelegramBotToken      string
	TelegramDefaultChatID string
	TelegramAPIBaseURL    string
	SMTPHost, SMTPPort, SMTPUser, SMTPPassword, SMTPFrom string
	BrevoAPIKey                                          string
}

func Load() Config {
	return Config{
		Port:                  env("PORT", DefaultPort),
		DatabaseURL:           env("DATABASE_URL", DefaultDatabase),
		TelegramBotToken:      env("TELEGRAM_BOT_TOKEN", ""),
		TelegramDefaultChatID: env("TELEGRAM_DEFAULT_CHAT_ID", ""),
		TelegramAPIBaseURL:    "https://api.telegram.org",
		SMTPHost:              env("SMTP_HOST", ""),
		SMTPPort:              env("SMTP_PORT", "587"),
		SMTPUser:              env("SMTP_USER", ""),
		SMTPPassword:          env("SMTP_PASSWORD", ""),
		SMTPFrom:              env("SMTP_FROM", ""),
		BrevoAPIKey:           env("BREVO_API_KEY", ""),
	}
}

func (c Config) TelegramConfigured() bool {
	return strings.TrimSpace(c.TelegramBotToken) != ""
}

func (c Config) DefaultChatConfigured() bool {
	return strings.TrimSpace(c.TelegramDefaultChatID) != ""
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
