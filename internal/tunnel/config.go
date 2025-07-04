package tunnel

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Config represents the tunnel configuration structure
type Config struct {
	// Connection settings
	Mode        string `json:"mode"`                  // proxy, sni, direct
	TargetHost  string `json:"targetHost"`            // Target server hostname
	TargetPort  string `json:"targetPort,omitempty"`  // Target server port (default: 22)
	ProxyHost   string `json:"proxyHost,omitempty"`   // Proxy server hostname (for proxy/sni modes)
	ProxyPort   string `json:"proxyPort,omitempty"`   // Proxy server port
	FrontDomain string `json:"frontDomain,omitempty"` // Front domain for SNI/payload

	// SSH settings
	SSH SSHConfig `json:"ssh"`

	// Local proxy settings
	LocalPort int    `json:"localPort,omitempty"` // Local SOCKS/HTTP port (default: 1080)
	ProxyType string `json:"proxyType,omitempty"` // socks5 or http (default: socks5)

	// Advanced settings
	Payload string `json:"payload,omitempty"` // Custom HTTP payload
	Timeout int    `json:"timeout,omitempty"` // Connection timeout in seconds (default: 30)
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

	// Check if file exists and is JSON
	if !strings.HasSuffix(strings.ToLower(configPath), ".json") {
		return nil, fmt.Errorf("config file must be a JSON file")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Substitute environment variables and parse JSON
	content := os.ExpandEnv(string(data))
	config := &Config{}
	if err := json.Unmarshal([]byte(content), config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON config: %w", err)
	}

	// Validate and set defaults
	if err := config.validate(); err != nil {
		return nil, err
	}
	config.setDefaults()

	return config, nil
}

// validate validates the configuration
func (c *Config) validate() error {
	// Validate mode
	validModes := []string{"proxy", "sni", "direct"}
	if !contains(validModes, c.Mode) {
		return fmt.Errorf("invalid mode '%s', must be one of: %s", c.Mode, strings.Join(validModes, ", "))
	}

	// Validate required fields
	if c.TargetHost == "" {
		return fmt.Errorf("targetHost is required")
	}
	if c.SSH.Username == "" {
		return fmt.Errorf("SSH username is required")
	}
	if c.SSH.Password == "" {
		return fmt.Errorf("SSH password is required")
	}

	// Mode-specific validation
	if c.Mode == "proxy" || c.Mode == "sni" {
		if c.ProxyHost == "" {
			return fmt.Errorf("proxyHost is required for %s mode", c.Mode)
		}
		if c.ProxyPort == "" {
			return fmt.Errorf("proxyPort is required for %s mode", c.Mode)
		}
		if c.Mode == "sni" && c.FrontDomain == "" {
			return fmt.Errorf("frontDomain is required for sni mode")
		}
	}

	return nil
}

// setDefaults sets default values for optional fields
func (c *Config) setDefaults() {
	if c.TargetPort == "" {
		c.TargetPort = "22"
	}
	if c.SSH.Port == "" {
		c.SSH.Port = "22"
	}
	if c.LocalPort == 0 {
		c.LocalPort = 1080
	}
	if c.ProxyType == "" {
		c.ProxyType = "socks5"
	}
	if c.Timeout == 0 {
		c.Timeout = 30
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
