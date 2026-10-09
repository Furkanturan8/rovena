package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env            string
	LogLevel       string
	HTTPPort       string
	DatabaseURL    string
	JWTSecret      string
	AllowedOrigins string
	TrustedProxies []string
}

const minJWTSecretLen = 32

func Load() (*Config, error) {
	cfg := &Config{
		Env:            get("APP_ENV", "development"),
		LogLevel:       get("LOG_LEVEL", "info"),
		HTTPPort:       get("HTTP_PORT", "8082"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AllowedOrigins: get("ALLOWED_ORIGINS", "http://localhost:5173"),
		TrustedProxies: strings.Split(get("TRUSTED_PROXIES", "127.0.0.1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"), ","),
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
	return nil
}

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
