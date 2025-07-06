package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Config represents the tunnel configuration structure
type Config struct {
	// Connection settings
	ConnectionMode string `json:"connectionMode"`        // proxy, sni, direct
	ServerHost     string `json:"serverHost"`            // Target server hostname
	ServerPort     string `json:"serverPort,omitempty"`  // Target server port (default: 22)
	ProxyHost      string `json:"proxyHost,omitempty"`   // Proxy server hostname (for proxy/sni modes)
	ProxyPort      string `json:"proxyPort,omitempty"`   // Proxy server port
	SpoofedHost    string `json:"spoofedHost,omitempty"` // Host header value for SNI and payload spoofing

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
	Port     string `json:"port,omitempty"` // SSH port (default: 22)
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
	validModes := []string{"proxy", "sni", "direct"}
	if !contains(validModes, c.ConnectionMode) {
		return fmt.Errorf("invalid mode '%s', must be one of: %s", c.ConnectionMode, strings.Join(validModes, ", "))
	}

	if c.ServerHost == "" {
		return fmt.Errorf("serverHost is required")
	}
	if c.SSH.Username == "" {
		return fmt.Errorf("SSH username is required")
	}
	if c.SSH.Password == "" {
		return fmt.Errorf("SSH password is required")
	}

	if c.ConnectionMode == "proxy" || c.ConnectionMode == "sni" {
		if c.ProxyHost == "" {
			return fmt.Errorf("proxyHost is required for %s mode", c.ConnectionMode)
		}
		if c.ProxyPort == "" {
			return fmt.Errorf("proxyPort is required for %s mode", c.ConnectionMode)
		}
		if c.ConnectionMode == "sni" && c.SpoofedHost == "" {
			return fmt.Errorf("spoofedHost is required for sni mode")
		}
	}

	return nil
}

// setDefaults sets default values for optional fields
func (c *Config) setDefaults() {
	if c.ServerPort == "" {
		c.ServerPort = "22"
	}
	if c.SSH.Port == "" {
		c.SSH.Port = "22"
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

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
