// internal/config/config_test.go
package config

import (
	"os"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Host != "localhost" {
		t.Errorf("Expected default host 'localhost', got '%s'", cfg.Host)
	}
	if cfg.Port != 6380 {
		t.Errorf("Expected default port 6380, got %d", cfg.Port)
	}
	if cfg.MaxConnections != 0 {
		t.Errorf("Expected default MaxConnections 0, got %d", cfg.MaxConnections)
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Save original env vars
	origHost := os.Getenv("KVLITE_HOST")
	origPort := os.Getenv("KVLITE_PORT")
	origMaxConn := os.Getenv("KVLITE_MAX_CONNECTIONS")

	// Restore at end
	defer func() {
		if origHost != "" {
			os.Setenv("KVLITE_HOST", origHost)
		} else {
			os.Unsetenv("KVLITE_HOST")
		}
		if origPort != "" {
			os.Setenv("KVLITE_PORT", origPort)
		} else {
			os.Unsetenv("KVLITE_PORT")
		}
		if origMaxConn != "" {
			os.Setenv("KVLITE_MAX_CONNECTIONS", origMaxConn)
		} else {
			os.Unsetenv("KVLITE_MAX_CONNECTIONS")
		}
	}()

	// Set test env vars
	os.Setenv("KVLITE_HOST", "0.0.0.0")
	os.Setenv("KVLITE_PORT", "9999")
	os.Setenv("KVLITE_MAX_CONNECTIONS", "100")

	cfg := LoadFromEnv()

	if cfg.Host != "0.0.0.0" {
		t.Errorf("Expected host '0.0.0.0', got '%s'", cfg.Host)
	}
	if cfg.Port != 9999 {
		t.Errorf("Expected port 9999, got %d", cfg.Port)
	}
	if cfg.MaxConnections != 100 {
		t.Errorf("Expected MaxConnections 100, got %d", cfg.MaxConnections)
	}
}

func TestLoadFromEnv_InvalidPort(t *testing.T) {
	origPort := os.Getenv("KVLITE_PORT")
	defer func() {
		if origPort != "" {
			os.Setenv("KVLITE_PORT", origPort)
		} else {
			os.Unsetenv("KVLITE_PORT")
		}
	}()

	// Set invalid port
	os.Setenv("KVLITE_PORT", "not_a_number")

	cfg := LoadFromEnv()

	// Should fall back to default
	if cfg.Port != 6380 {
		t.Errorf("Expected default port 6380 for invalid env, got %d", cfg.Port)
	}
}

func TestLoadFromEnv_EmptyVars(t *testing.T) {
	origHost := os.Getenv("KVLITE_HOST")
	origPort := os.Getenv("KVLITE_PORT")
	origMaxConn := os.Getenv("KVLITE_MAX_CONNECTIONS")

	defer func() {
		if origHost != "" {
			os.Setenv("KVLITE_HOST", origHost)
		}
		if origPort != "" {
			os.Setenv("KVLITE_PORT", origPort)
		}
		if origMaxConn != "" {
			os.Setenv("KVLITE_MAX_CONNECTIONS", origMaxConn)
		}
	}()

	// Clear all env vars
	os.Unsetenv("KVLITE_HOST")
	os.Unsetenv("KVLITE_PORT")
	os.Unsetenv("KVLITE_MAX_CONNECTIONS")

	cfg := LoadFromEnv()

	// Should use defaults
	if cfg.Host != "localhost" {
		t.Errorf("Expected default host 'localhost', got '%s'", cfg.Host)
	}
	if cfg.Port != 6380 {
		t.Errorf("Expected default port 6380, got %d", cfg.Port)
	}
}

func TestAddress(t *testing.T) {
	cfg := &Config{
		Host: "localhost",
		Port: 6380,
	}

	addr := cfg.Address()
	expected := "localhost:6380"
	if addr != expected {
		t.Errorf("Expected address '%s', got '%s'", expected, addr)
	}
}

func TestAddress_CustomHostPort(t *testing.T) {
	cfg := &Config{
		Host: "192.168.1.100",
		Port: 9999,
	}

	addr := cfg.Address()
	expected := "192.168.1.100:9999"
	if addr != expected {
		t.Errorf("Expected address '%s', got '%s'", expected, addr)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	testCases := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "default config",
			cfg:  Default(),
		},
		{
			name: "port 1",
			cfg:  &Config{Host: "localhost", Port: 1, MaxConnections: 0},
		},
		{
			name: "port 65535",
			cfg:  &Config{Host: "localhost", Port: 65535, MaxConnections: 0},
		},
		{
			name: "port 0 (random)",
			cfg:  &Config{Host: "localhost", Port: 0, MaxConnections: 0},
		},
		{
			name: "with max connections",
			cfg:  &Config{Host: "localhost", Port: 6380, MaxConnections: 100},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if err != nil {
				t.Errorf("Expected valid config, got error: %v", err)
			}
		})
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	testCases := []struct {
		name string
		port int
	}{
		{"negative port", -1},
		{"port too high", 65536},
		{"way too high", 100000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Host: "localhost",
				Port: tc.port,
			}

			err := cfg.Validate()
			if err == nil {
				t.Errorf("Expected error for port %d, got nil", tc.port)
			}
		})
	}
}

func TestValidate_InvalidMaxConnections(t *testing.T) {
	cfg := &Config{
		Host:           "localhost",
		Port:           6380,
		MaxConnections: -1,
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for negative MaxConnections, got nil")
	}
}

func TestValidate_ZeroMaxConnections(t *testing.T) {
	cfg := &Config{
		Host:           "localhost",
		Port:           6380,
		MaxConnections: 0, // 0 means unlimited
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected no error for MaxConnections 0 (unlimited), got: %v", err)
	}
}
