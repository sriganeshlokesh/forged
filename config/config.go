package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Version is stamped at build time via -ldflags "-X github.com/sriganeshlokesh/forged/config.Version=..."
var Version = "dev"

// Config holds all application configuration loaded from environment variables.
type Config struct {
	ServiceName     string
	Env             string
	Port            string
	LogLevel        string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	Version         string
}

// Load reads configuration from environment variables, applying defaults for any unset values.
// It returns an error if any value cannot be parsed.
func Load() (*Config, error) {
	port := getEnv("PORT", "8080")
	if _, err := strconv.Atoi(port); err != nil {
		return nil, fmt.Errorf("PORT must be numeric, got %q: %w", port, err)
	}

	readTimeout, err := parseDuration("HTTP_READ_TIMEOUT", "10s")
	if err != nil {
		return nil, err
	}
	writeTimeout, err := parseDuration("HTTP_WRITE_TIMEOUT", "30s")
	if err != nil {
		return nil, err
	}
	idleTimeout, err := parseDuration("HTTP_IDLE_TIMEOUT", "120s")
	if err != nil {
		return nil, err
	}
	shutdownTimeout, err := parseDuration("SHUTDOWN_TIMEOUT", "5s")
	if err != nil {
		return nil, err
	}

	return &Config{
		ServiceName:     getEnv("SERVICE_NAME", "forged"),
		Env:             getEnv("APP_ENV", "development"),
		Port:            port,
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		ReadTimeout:     readTimeout,
		WriteTimeout:    writeTimeout,
		IdleTimeout:     idleTimeout,
		ShutdownTimeout: shutdownTimeout,
		Version:         Version,
	}, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func parseDuration(envKey, defaultVal string) (time.Duration, error) {
	raw := getEnv(envKey, defaultVal)
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration, got %q: %w", envKey, raw, err)
	}
	return d, nil
}
