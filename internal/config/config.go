package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port             string
	Host             string
	PostgresDSN      string
	JWTSecret        string
	ResendAPIKey     string
	ResendFrom       string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	OTPValidWindow   time.Duration
	TrustedProxyCIDR []string
	AdminEmails      []string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		Host:             getEnv("HOST", "0.0.0.0"),
		PostgresDSN:      strings.TrimSpace(getEnv("POSTGRES_DSN", "")),
		JWTSecret:        getEnv("AUTH_JWT_SECRET", "change-me"),
		ResendAPIKey:     strings.TrimSpace(getEnv("RESEND_API_KEY", "")),
		ResendFrom:       strings.TrimSpace(getEnv("RESEND_FROM", getEnv("RESEND_FROM_EMAIL", ""))),
		AccessTokenTTL:   time.Duration(getEnvInt("AUTH_ACCESS_TOKEN_MINUTES", 15)) * time.Minute,
		RefreshTokenTTL:  time.Duration(getEnvInt("AUTH_REFRESH_TOKEN_DAYS", 30)) * 24 * time.Hour,
		OTPValidWindow:   time.Duration(getEnvInt("AUTH_OTP_WINDOW_MINUTES", 10)) * time.Minute,
		TrustedProxyCIDR: parseCSV(getEnv("TRUSTED_PROXY_CIDR", "")),
		AdminEmails:      normalizeEmails(parseCSV(getEnv("ADMIN_EMAILS", ""))),
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func parseCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func normalizeEmails(emails []string) []string {
	if len(emails) == 0 {
		return nil
	}
	out := make([]string, 0, len(emails))
	seen := map[string]struct{}{}
	for _, e := range emails {
		n := strings.ToLower(strings.TrimSpace(e))
		if n == "" {
			continue
		}
		if _, exists := seen[n]; exists {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}
