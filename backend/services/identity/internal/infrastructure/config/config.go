package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env         string
	LogLevel    string
	HTTPPort    string
	DatabaseURL string
	JWTSecret   string
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
	// RefreshGrace is how long a just-rotated refresh token still works once more (two tabs refreshing together).
	RefreshGrace   time.Duration
	AllowedOrigins string
	BcryptCost     int
	TrustedProxies []string

	// Only used by cmd/seed.
	AdminEmail    string
	AdminPassword string
}

const minJWTSecretLen = 32

func Load() (*Config, error) {
	cfg := &Config{
		Env:            get("APP_ENV", "development"),
		LogLevel:       get("LOG_LEVEL", "info"),
		HTTPPort:       get("HTTP_PORT", "8081"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AllowedOrigins: get("ALLOWED_ORIGINS", "http://localhost:5173"),
		TrustedProxies: strings.Split(get("TRUSTED_PROXIES", "127.0.0.1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"), ","),
		AdminEmail:     os.Getenv("ADMIN_EMAIL"),
		AdminPassword:  os.Getenv("ADMIN_PASSWORD"),
	}

	var err error
	if cfg.AccessTTL, err = duration("ACCESS_TOKEN_TTL", 15*time.Minute); err != nil {
		return nil, err
	}
	if cfg.RefreshTTL, err = duration("REFRESH_TOKEN_TTL", 30*24*time.Hour); err != nil {
		return nil, err
	}
	if cfg.RefreshGrace, err = nonNegativeDuration("REFRESH_TOKEN_GRACE", 10*time.Second); err != nil {
		return nil, err
	}
	if cfg.BcryptCost, err = integer("BCRYPT_COST", 12); err != nil {
		return nil, err
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

// ValidateServer checks settings that only the API process needs.
func (c *Config) ValidateServer() error {
	if len(c.JWTSecret) < minJWTSecretLen {
		return fmt.Errorf("JWT_SECRET must be at least %d characters", minJWTSecretLen)
	}
	return nil
}

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func duration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration, got %q", key, v)
	}
	return d, nil
}

// nonNegativeDuration is like duration but accepts 0, which turns a feature off.
func nonNegativeDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("%s must be a non-negative duration, got %q", key, v)
	}
	return d, nil
}

func integer(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, v)
	}
	return n, nil
}
