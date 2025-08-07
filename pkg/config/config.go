package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents the tunnel configuration structure
type Config struct {
	// Connection settings
	Mode      string `json:"Mode"`                // direct, proxy
	SSHHost   string `json:"sshHost"`             // SSH server hostname
	SSHPort   string `json:"sshPort,omitempty"`   // SSH server port (default: 22)
	ProxyHost string `json:"proxyHost,omitempty"` // Proxy server hostname (for proxy mode)
	ProxyPort string `json:"proxyPort,omitempty"` // Proxy server port

	// SSH settings
	SSH SSHConfig `json:"ssh"`

	// Local proxy settings
	ListenPort int    `json:"listenPort,omitempty"` // Local SOCKS/HTTP port (default: 1080)
	ProxyType  string `json:"proxyType,omitempty"`  // socks5 or http (default: socks5)

	// Advanced settings
	HTTPPayload       string `json:"httpPayload,omitempty"`       // Custom HTTP payload
	ConnectionTimeout int    `json:"connectionTimeout,omitempty"` // Connection timeout in seconds (default: 30)
}

// SSHConfig defines SSH connection settings
type SSHConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoadConfig loads and validates configuration from file
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

// validate validates the configuration
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

// setDefaults sets default values for optional fields
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
