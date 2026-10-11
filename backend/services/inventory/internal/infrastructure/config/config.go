package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Env            string
	LogLevel       string
	HTTPPort       string
	DatabaseURL    string
	JWTSecret      string
	AllowedOrigins string
	TrustedProxies []string

	// InternalAPIKey guards the service-to-service reservation endpoints. Those are
	// not exposed through the gateway, so a shared key is enough to keep them
	// unreachable even if the service port is.
	InternalAPIKey string

	// ReservationTTL is how long a reservation holds stock before the expirer releases it.
	ReservationTTL time.Duration
	// ExpiryInterval is how often the expirer looks for due reservations.
	ExpiryInterval time.Duration

	// Only used by cmd/seed.
	CatalogBaseURL string
}

const (
	minJWTSecretLen      = 32
	minInternalAPIKeyLen = 32
)

func Load() (*Config, error) {
	cfg := &Config{
		Env:            get("APP_ENV", "development"),
		LogLevel:       get("LOG_LEVEL", "info"),
		HTTPPort:       get("HTTP_PORT", "8083"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AllowedOrigins: get("ALLOWED_ORIGINS", "http://localhost:5173"),
		TrustedProxies: strings.Split(get("TRUSTED_PROXIES", "127.0.0.1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"), ","),
		InternalAPIKey: os.Getenv("INTERNAL_API_KEY"),
		CatalogBaseURL: get("CATALOG_BASE_URL", "http://catalog:8082"),
	}

	var err error
	if cfg.ReservationTTL, err = duration("RESERVATION_TTL", 15*time.Minute); err != nil {
		return nil, err
	}
	if cfg.ExpiryInterval, err = duration("RESERVATION_EXPIRY_INTERVAL", time.Minute); err != nil {
		return nil, err
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("missing required env: DATABASE_URL")
	}
	return cfg, nil
}

// ValidateServer checks settings that only the API process needs.
func (c *Config) ValidateServer() error {
	if len(c.JWTSecret) < minJWTSecretLen {
		return fmt.Errorf("JWT_SECRET must be at least %d characters", minJWTSecretLen)
	}
	if len(c.InternalAPIKey) < minInternalAPIKeyLen {
		return fmt.Errorf("INTERNAL_API_KEY must be at least %d characters", minInternalAPIKeyLen)
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
