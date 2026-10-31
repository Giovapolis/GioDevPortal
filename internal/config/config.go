package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env        string
	ServerAddr string
	DBPath     string
	DataDir    string

	JWTSecret  string
	AccessTTL  time.Duration
	RefreshTTL time.Duration

	GeminiAPIKey string
	GeminiModel  string

	WhatsAppPhoneID          string
	WhatsAppToken            string
	WhatsAppTicketTemplate   string
	WhatsAppResourceTemplate string
	WhatsAppPaymentTemplate  string
	WhatsAppSRSTemplate      string
	AdminWhatsAppNumber      string

	R2AccountID string
	R2AccessKey string
	R2SecretKey string
	R2Bucket    string
	R2PublicURL string

	ResendAPIKey string
	EmailFrom    string
	AppBaseURL   string

	SeedAdminEmail    string
	SeedAdminPassword string
	SeedAdminName     string
	SeedDemo          bool
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	c := &Config{
		Env:                      env("ENV", "development"),
		ServerAddr:               env("SERVER_ADDR", ":8080"),
		DBPath:                   env("DB_PATH", "./data/app.db"),
		DataDir:                  env("DATA_DIR", "./data"),
		JWTSecret:                env("JWT_SECRET", "dev-only-secret-change-me-please-32chars"),
		AccessTTL:                envDuration("JWT_ACCESS_TTL", 15*time.Minute),
		RefreshTTL:               envDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		GeminiAPIKey:             env("GEMINI_API_KEY", ""),
		GeminiModel:              env("GEMINI_MODEL", "gemini-2.0-flash"),
		WhatsAppPhoneID:          env("WHATSAPP_PHONE_NUMBER_ID", ""),
		WhatsAppToken:            env("WHATSAPP_ACCESS_TOKEN", ""),
		WhatsAppTicketTemplate:   env("WHATSAPP_TEMPLATE_TICKET_ALERT", "ticket_alert"),
		WhatsAppResourceTemplate: env("WHATSAPP_TEMPLATE_RESOURCE_ALERT", "resource_alert"),
		WhatsAppPaymentTemplate:  env("WHATSAPP_TEMPLATE_PAYMENT_ALERT", "payment_alert"),
		WhatsAppSRSTemplate:      env("WHATSAPP_TEMPLATE_SRS_ALERT", "srs_request_alert"),
		AdminWhatsAppNumber:      env("ADMIN_WHATSAPP_NUMBER", ""),
		R2AccountID:              env("R2_ACCOUNT_ID", ""),
		R2AccessKey:              env("R2_ACCESS_KEY_ID", ""),
		R2SecretKey:              env("R2_SECRET_ACCESS_KEY", ""),
		R2Bucket:                 env("R2_BUCKET", "giodevportal-files"),
		R2PublicURL:              env("R2_PUBLIC_URL", ""),
		ResendAPIKey:             env("RESEND_API_KEY", ""),
		EmailFrom:                env("EMAIL_FROM", "GioDevPortal <noreply@localhost>"),
		AppBaseURL:               env("APP_BASE_URL", "http://localhost:8080"),
		SeedAdminEmail:           env("SEED_ADMIN_EMAIL", "admin@giodevportal.local"),
		SeedAdminPassword:        env("SEED_ADMIN_PASSWORD", "Admin123!"),
		SeedAdminName:            env("SEED_ADMIN_NAME", "Administrador"),
		SeedDemo:                 envBool("SEED_DEMO", true),
	}

	if _, ok := os.LookupEnv("SEED_DEMO"); !ok {
		c.SeedDemo = c.Env != "production"
	}

	if len(c.JWTSecret) < 16 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 16 characters")
	}

	if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	return c, nil
}

func (c *Config) R2Enabled() bool {
	return c.R2AccountID != "" && c.R2AccessKey != "" && c.R2SecretKey != ""
}

func (c *Config) WhatsAppEnabled() bool {
	return c.WhatsAppPhoneID != "" && c.WhatsAppToken != ""
}

func (c *Config) GeminiEnabled() bool {
	return strings.TrimSpace(c.GeminiAPIKey) != ""
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	return fallback
}
