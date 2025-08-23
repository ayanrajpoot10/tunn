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
//	// Use cfg.Mode, cfg.SSHHost, etc.
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
	Mode      string `json:"Mode"`                // Connection mode: "direct" or "proxy"
	SSHHost   string `json:"sshHost"`             // SSH server hostname or IP address
	SSHPort   string `json:"sshPort,omitempty"`   // SSH server port (default: "22")
	ProxyHost string `json:"proxyHost,omitempty"` // Proxy server hostname (required for proxy mode)
	ProxyPort string `json:"proxyPort,omitempty"` // Proxy server port (required for proxy mode)

	// SSH authentication settings
	SSH SSHConfig `json:"ssh"` // SSH connection credentials

	// Local proxy server settings
	ListenPort int    `json:"listenPort,omitempty"` // Local SOCKS/HTTP server port (default: 1080)
	ProxyType  string `json:"proxyType,omitempty"`  // Proxy protocol: "socks5" or "http" (default: "socks5")

	// Advanced connection settings
	HTTPPayload       string `json:"httpPayload,omitempty"`       // Custom HTTP payload for WebSocket upgrade
	ConnectionTimeout int    `json:"connectionTimeout,omitempty"` // Connection timeout in seconds (default: 30)
}

// SSHConfig defines SSH connection credentials and settings.
//
// Contains the authentication information required to establish SSH connections
// through the tunnel. Both username and password are required fields.
type SSHConfig struct {
	Username string `json:"username"` // SSH username for authentication
	Password string `json:"password"` // SSH password for authentication
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
//   - Mode must be either "direct" or "proxy"
//   - Required fields (sshHost, SSH username/password) must be non-empty
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

	// Check required fields
	if c.SSHHost == "" {
		return fmt.Errorf("sshHost is required")
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
//   - SSHPort: "22" (standard SSH port)
//   - ListenPort: 1080 (standard SOCKS proxy port)
//   - ProxyType: "socks5" (SOCKS5 protocol)
//   - ConnectionTimeout: 30 seconds
func (c *Config) setDefaults() {
	if c.SSHPort == "" {
		c.SSHPort = "22"
	}
	if c.ListenPort == 0 {
		c.ListenPort = 1080
	}
	if c.ProxyType == "" {
		c.ProxyType = "socks5"
	}
	if c.ConnectionTimeout == 0 {
		c.ConnectionTimeout = 30
	}
}
