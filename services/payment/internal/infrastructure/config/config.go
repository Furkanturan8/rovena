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

	// InternalAPIKey guards the service-to-service read endpoint.
	InternalAPIKey string

	RabbitMQURL string

	// MaxAttempts is how many times the provider is called for one payment while it
	// keeps failing to answer; after that the payment fails (see application.Config).
	MaxAttempts int
	// ChargeTimeout bounds a single provider call.
	ChargeTimeout time.Duration
	// RetryDelay is how long a message waits before it is redelivered after an error.
	RetryDelay time.Duration
	// ProviderLatency is the artificial delay of the fake provider.
	ProviderLatency time.Duration
}

const (
	minJWTSecretLen      = 32
	minInternalAPIKeyLen = 32
)

func Load() (*Config, error) {
	cfg := &Config{
		Env:            get("APP_ENV", "development"),
		LogLevel:       get("LOG_LEVEL", "info"),
		HTTPPort:       get("HTTP_PORT", "8085"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AllowedOrigins: get("ALLOWED_ORIGINS", "http://localhost:5173"),
		TrustedProxies: strings.Split(get("TRUSTED_PROXIES", "127.0.0.1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"), ","),
		InternalAPIKey: os.Getenv("INTERNAL_API_KEY"),
		RabbitMQURL:    os.Getenv("RABBITMQ_URL"),
	}

	var err error
	if cfg.MaxAttempts, err = positiveInt("PAYMENT_MAX_ATTEMPTS", 3); err != nil {
		return nil, err
	}
	if cfg.ChargeTimeout, err = duration("PAYMENT_CHARGE_TIMEOUT", 10*time.Second); err != nil {
		return nil, err
	}
	if cfg.RetryDelay, err = duration("PAYMENT_RETRY_DELAY", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.ProviderLatency, err = nonNegativeDuration("FAKE_PROVIDER_LATENCY", 0); err != nil {
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
