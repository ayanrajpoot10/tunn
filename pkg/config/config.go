// Package config provides configuration management for the Tunn SSH tunneling tool.
//
// This package handles loading, parsing, validating, and managing configuration
// data from JSON files. It supports both direct and proxy tunnel modes with
// comprehensive validation to ensure all required settings are present and valid.
//
// Configuration files use JSON format and support environment variable substitution
// using the standard $VAR or ${VAR} syntax.
//
// Example usage:
//
//	cfg, err := config.LoadConfig("config.json")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use cfg.Mode, cfg.SSH.Host, etc.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents the complete tunnel configuration structure.
//
// It contains all settings required to establish tunnel connections including
// connection modes, SSH credentials, proxy settings, and local server options.
// The configuration supports both direct connections and connections through
// HTTP proxies, with optional WebSocket upgrade capabilities.
type Config struct {
	// Connection settings
	Mode      string `json:"mode"`                // Connection mode: "direct" or "proxy"
	ProxyHost string `json:"proxyHost,omitempty"` // Proxy server hostname (required for proxy mode)
	ProxyPort string `json:"proxyPort,omitempty"` // Proxy server port (required for proxy mode)

	// SSH connection settings
	SSH SSHConfig `json:"ssh"` // SSH connection settings and credentials

	// Local proxy server settings
	Listener ListenerConfig `json:"listener"` // Local listener configuration

	// Advanced connection settings
	HTTPPayload       string `json:"httpPayload,omitempty"`       // Custom HTTP payload for WebSocket upgrade
	ConnectionTimeout int    `json:"connectionTimeout,omitempty"` // Connection timeout in seconds (default: 30)
}

// SSHConfig defines SSH connection settings and credentials.
//
// Contains the connection information and authentication details required
// to establish SSH connections through the tunnel.
type SSHConfig struct {
	Host     string `json:"host"`     // SSH server hostname or IP address
	Port     int    `json:"port"`     // SSH server port
	Username string `json:"username"` // SSH username for authentication
	Password string `json:"password"` // SSH password for authentication
}

// ListenerConfig defines local proxy server settings.
//
// Contains the configuration for the local proxy server that will listen
// for client connections and forward them through the SSH tunnel.
type ListenerConfig struct {
	Port      int    `json:"port"`      // Local listener port (default: 1080)
	ProxyType string `json:"proxyType"` // Proxy protocol: "http", "socks5", etc. (default: "socks5")
}

// LoadConfig loads and validates configuration from a JSON file.
//
// This function reads the specified configuration file, performs environment
// variable substitution, parses the JSON content, validates all settings,
// and applies default values where appropriate.
//
// Environment variables in the configuration file are expanded using os.ExpandEnv,
// allowing for dynamic configuration values using $VAR or ${VAR} syntax.
//
// Parameters:
//   - configPath: Path to the JSON configuration file
//
// Returns:
//   - *Config: The loaded and validated configuration
//   - error: An error if file reading, parsing, or validation fails
//
// Example:
//
//	cfg, err := LoadConfig("/path/to/config.json")
//	if err != nil {
//	    return fmt.Errorf("config load failed: %w", err)
//	}
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		return nil, fmt.Errorf("no config file specified")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := &Config{}
	content := os.ExpandEnv(string(data))
	if err := json.Unmarshal([]byte(content), config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := config.validate(); err != nil {
		return nil, err
	}
	config.setDefaults()

	return config, nil
}

// validate performs comprehensive validation of the configuration settings.
//
// This method checks that all required fields are present and contain valid values,
// validates mode-specific requirements, and ensures the configuration is internally
// consistent and ready for use.
//
// Validation checks include:
//   - Mode must be either "direct" or "proxy""
//   - Required fields (SSH host, SSH username/password) must be non-empty
//   - Proxy mode requires proxyHost and proxyPort
//   - Field values must be reasonable and properly formatted
//
// Returns:
//   - error: A descriptive error if validation fails, nil if successful
func (c *Config) validate() error {
	validModes := map[string]bool{"direct": true, "proxy": true}
	if !validModes[c.Mode] {
		return fmt.Errorf("invalid mode '%s', must be one of: direct, proxy", c.Mode)
	}

	// Check required SSH fields
	if c.SSH.Host == "" {
		return fmt.Errorf("SSH host is required")
	}
	if c.SSH.Username == "" {
		return fmt.Errorf("SSH username is required")
	}
	if c.SSH.Password == "" {
		return fmt.Errorf("SSH password is required")
	}

	// Validate proxy mode requirements
	if c.Mode == "proxy" {
		if c.ProxyHost == "" || c.ProxyPort == "" {
			return fmt.Errorf("proxyHost and proxyPort are required for proxy mode")
		}
	}

	return nil
}

// setDefaults applies default values to optional configuration fields.
//
// This method sets sensible defaults for fields that were not explicitly
// configured, ensuring the configuration is complete and ready for use.
//
// Default values applied:
//   - SSH Port: 22 (standard SSH port)
//   - Listener Port: 1080 (HTTP proxy port)
//   - Listener ProxyType: "http" (http protocol)
//   - ConnectionTimeout: 30 seconds
func (c *Config) setDefaults() {
	if c.SSH.Port == 0 {
		c.SSH.Port = 22
	}
	if c.Listener.Port == 0 {
		c.Listener.Port = 1080
	}
	if c.Listener.ProxyType == "" {
		c.Listener.ProxyType = "http"
	}
	if c.ConnectionTimeout == 0 {
		c.ConnectionTimeout = 30
	}
}
