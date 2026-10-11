package config

import (
	"fmt"
	"os"
	"strconv"
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

	// InternalAPIKey is sent to Catalog and Inventory on service-to-service calls.
	InternalAPIKey string
	CatalogURL     string
	InventoryURL   string
	// ClientTimeout bounds every call to Catalog and Inventory.
	ClientTimeout time.Duration

	RabbitMQURL string
	// OutboxRetention is how long published outbox rows and inbox markers are kept before the cleaner removes them.
	OutboxRetention time.Duration
	// OutboxCleanInterval is how often the cleaner runs.
	OutboxCleanInterval time.Duration
	// RetryDelay is how long a payment result waits before it is redelivered after an error.
	RetryDelay time.Duration

	// Saga recovery; see application.Config.
	RecoveryInterval   time.Duration
	StuckAfter         time.Duration
	PendingMaxAge      time.Duration
	PaymentTimeout     time.Duration
	MaxPaymentRequests int
}

const (
	minJWTSecretLen      = 32
	minInternalAPIKeyLen = 32
)

func Load() (*Config, error) {
	cfg := &Config{
		Env:            get("APP_ENV", "development"),
		LogLevel:       get("LOG_LEVEL", "info"),
		HTTPPort:       get("HTTP_PORT", "8086"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AllowedOrigins: get("ALLOWED_ORIGINS", "http://localhost:5173"),
		TrustedProxies: strings.Split(get("TRUSTED_PROXIES", "127.0.0.1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"), ","),
		InternalAPIKey: os.Getenv("INTERNAL_API_KEY"),
		CatalogURL:     strings.TrimRight(get("CATALOG_URL", "http://catalog:8082"), "/"),
		InventoryURL:   strings.TrimRight(get("INVENTORY_URL", "http://inventory:8083"), "/"),
		RabbitMQURL:    os.Getenv("RABBITMQ_URL"),
	}

	var err error
	if cfg.ClientTimeout, err = duration("CLIENT_TIMEOUT", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.RetryDelay, err = duration("ORDER_RETRY_DELAY", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.RecoveryInterval, err = duration("ORDER_RECOVERY_INTERVAL", 15*time.Second); err != nil {
		return nil, err
	}
	if cfg.StuckAfter, err = duration("ORDER_STUCK_AFTER", 30*time.Second); err != nil {
		return nil, err
	}
	if cfg.PendingMaxAge, err = duration("ORDER_PENDING_MAX_AGE", 5*time.Minute); err != nil {
		return nil, err
	}
	if cfg.PaymentTimeout, err = duration("ORDER_PAYMENT_TIMEOUT", 2*time.Minute); err != nil {
		return nil, err
	}
	if cfg.MaxPaymentRequests, err = positiveInt("ORDER_MAX_PAYMENT_REQUESTS", 3); err != nil {
		return nil, err
	}

	if cfg.OutboxRetention, err = duration("OUTBOX_RETENTION", 7*24*time.Hour); err != nil {
		return nil, err
	}

	if cfg.OutboxCleanInterval, err = duration("OUTBOX_CLEAN_INTERVAL", time.Hour); err != nil {
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
	if c.RabbitMQURL == "" {
		return fmt.Errorf("missing required env: RABBITMQ_URL")
	}
	return nil
}

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func positiveInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", key, v)
	}
	return n, nil
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
