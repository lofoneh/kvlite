// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the server configuration
type Config struct {
	// Host to bind to (default: localhost)
	Host string

	// Port to listen on (default: 6380)
	Port int

	// MaxConnections limits concurrent connections (0 = unlimited)
	MaxConnections int
}

// Default returns the default configuration
func Default() *Config {
	return &Config{
		Host:           "localhost",
		Port:           6380,
		MaxConnections: 0,
	}
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() *Config {
	cfg := Default()

	if host := os.Getenv("KVLITE_HOST"); host != "" {
		cfg.Host = host
	}

	if port := os.Getenv("KVLITE_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}

	if maxConn := os.Getenv("KVLITE_MAX_CONNECTIONS"); maxConn != "" {
		if m, err := strconv.Atoi(maxConn); err == nil {
			cfg.MaxConnections = m
		}
	}

	return cfg
}

// Address returns the full address string (host:port)
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Port 0 is allowed for OS-assigned random port
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d (must be 0-65535)", c.Port)
	}
	if c.MaxConnections < 0 {
		return fmt.Errorf("invalid max connections: %d (must be >= 0)", c.MaxConnections)
	}
	return nil
}
