package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL   string
	SMTPHost      string
	SMTPPort      int
	SMTPFrom      string
	SMTPTo        string
	SMTPUsername  string
	SMTPPassword  string
	SMTPTLSMode   string
	ClaimsBaseURL string
	Timezone      *time.Location
	WarningDays   int
}

func Load() (Config, error) {
	var cfg Config

	cfg.DatabaseURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	cfg.SMTPHost = strings.TrimSpace(os.Getenv("SMTP_HOST"))
	cfg.SMTPFrom = strings.TrimSpace(os.Getenv("CLAIM_NOTIFICATION_FROM"))
	cfg.SMTPTo = strings.TrimSpace(os.Getenv("CLAIM_NOTIFICATION_TO"))
	cfg.SMTPUsername = strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	cfg.SMTPPassword = os.Getenv("SMTP_PASSWORD")
	cfg.SMTPTLSMode = strings.ToLower(strings.TrimSpace(getEnv("SMTP_TLS_MODE", "none")))
	cfg.ClaimsBaseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("CLAIMS_BASE_URL")), "/")

	port, err := strconv.Atoi(getEnv("SMTP_PORT", "25"))
	if err != nil || port < 1 || port > 65535 {
		return cfg, fmt.Errorf("invalid SMTP_PORT")
	}
	cfg.SMTPPort = port

	warningDays, err := strconv.Atoi(getEnv("WARNING_DAYS", "14"))
	if err != nil || warningDays < 1 || warningDays > 365 {
		return cfg, fmt.Errorf("invalid WARNING_DAYS")
	}
	cfg.WarningDays = warningDays

	tzName := getEnv("APP_TIMEZONE", "Europe/Vienna")
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return cfg, fmt.Errorf("load APP_TIMEZONE %q: %w", tzName, err)
	}
	cfg.Timezone = loc

	for name, value := range map[string]string{
		"DATABASE_URL": cfg.DatabaseURL,
		"SMTP_HOST":    cfg.SMTPHost,
		"SMTP_FROM":    cfg.SMTPFrom,
		"SMTP_TO":      cfg.SMTPTo,
	} {
		if value == "" {
			return cfg, fmt.Errorf("%s is required", name)
		}
	}

	switch cfg.SMTPTLSMode {
	case "none", "opportunistic", "starttls":
	default:
		return cfg, fmt.Errorf("SMTP_TLS_MODE must be one of: none, opportunistic, starttls")
	}

	if (cfg.SMTPUsername == "") != (cfg.SMTPPassword == "") {
		return cfg, fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD must either both be set or both be empty")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
