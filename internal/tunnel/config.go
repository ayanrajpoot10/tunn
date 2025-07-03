package tunnel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the simplified main configuration structure
type Config struct {
	// Connection settings
	Mode        string `json:"mode" yaml:"mode"`                                   // proxy, sni, direct
	TargetHost  string `json:"targetHost" yaml:"targetHost"`                       // Target server hostname
	TargetPort  string `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`   // Target server port (default: 22)
	ProxyHost   string `json:"proxyHost,omitempty" yaml:"proxyHost,omitempty"`     // Proxy server hostname (for proxy/sni modes)
	ProxyPort   string `json:"proxyPort,omitempty" yaml:"proxyPort,omitempty"`     // Proxy server port
	FrontDomain string `json:"frontDomain,omitempty" yaml:"frontDomain,omitempty"` // Front domain for SNI/payload

	// SSH settings
	SSH SSHConfig `json:"ssh" yaml:"ssh"`

	// Local proxy settings
	LocalPort int    `json:"localPort,omitempty" yaml:"localPort,omitempty"` // Local SOCKS/HTTP port (default: 1080)
	ProxyType string `json:"proxyType,omitempty" yaml:"proxyType,omitempty"` // socks5 or http (default: socks5)

	// Advanced settings
	Payload string `json:"payload,omitempty" yaml:"payload,omitempty"` // Custom HTTP payload
	Timeout int    `json:"timeout,omitempty" yaml:"timeout,omitempty"` // Connection timeout in seconds (default: 30)

	// Legacy fields for backward compatibility
	LocalSOCKSPort  int    `json:"-" yaml:"-"`
	SSHUsername     string `json:"-" yaml:"-"`
	SSHPassword     string `json:"-" yaml:"-"`
	SSHPort         string `json:"-" yaml:"-"`
	PayloadTemplate string `json:"-" yaml:"-"`
}

// SSHConfig defines SSH connection settings
type SSHConfig struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Port     string `json:"port,omitempty" yaml:"port,omitempty"` // SSH port (default: 22)
}

// ConfigManager handles loading and parsing of configuration files
type ConfigManager struct {
	configPath string
	config     *Config
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configPath string) *ConfigManager {
	return &ConfigManager{
		configPath: configPath,
	}
}

// LoadConfig loads configuration from file
func (cm *ConfigManager) LoadConfig() error {
	if cm.configPath == "" {
		return fmt.Errorf("no config file specified")
	}

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Substitute environment variables
	content := cm.substituteEnvVars(string(data))

	// Determine file format by extension
	ext := strings.ToLower(filepath.Ext(cm.configPath))

	cm.config = &Config{}

	switch ext {
	case ".json":
		if err := json.Unmarshal([]byte(content), cm.config); err != nil {
			return fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal([]byte(content), cm.config); err != nil {
			return fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s (supported: .json, .yaml, .yml)", ext)
	}

	return cm.validateConfig()
}

// substituteEnvVars replaces environment variable placeholders in config
func (cm *ConfigManager) substituteEnvVars(content string) string {
	return os.ExpandEnv(content)
}

// validateConfig validates the loaded configuration
func (cm *ConfigManager) validateConfig() error {
	if cm.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	// Validate mode
	if cm.config.Mode != "proxy" && cm.config.Mode != "sni" && cm.config.Mode != "direct" {
		return fmt.Errorf("invalid mode '%s', must be: proxy, sni, or direct", cm.config.Mode)
	}

	// Validate target host
	if cm.config.TargetHost == "" {
		return fmt.Errorf("targetHost is required")
	}

	// Validate SSH settings
	if cm.config.SSH.Username == "" {
		return fmt.Errorf("SSH username is required")
	}
	if cm.config.SSH.Password == "" {
		return fmt.Errorf("SSH password is required")
	}

	// Mode-specific validation
	switch cm.config.Mode {
	case "proxy", "sni":
		if cm.config.ProxyHost == "" {
			return fmt.Errorf("proxyHost is required for %s mode", cm.config.Mode)
		}
		if cm.config.ProxyPort == "" {
			return fmt.Errorf("proxyPort is required for %s mode", cm.config.Mode)
		}
		if cm.config.Mode == "sni" && cm.config.FrontDomain == "" {
			return fmt.Errorf("frontDomain is required for sni mode")
		}
	}

	// Set defaults
	cm.setDefaults()

	return nil
}

// setDefaults sets default values for optional fields
func (cm *ConfigManager) setDefaults() {
	if cm.config.TargetPort == "" {
		cm.config.TargetPort = "22"
	}
	if cm.config.SSH.Port == "" {
		cm.config.SSH.Port = "22"
	}
	if cm.config.LocalPort == 0 {
		cm.config.LocalPort = 1080
	}
	if cm.config.ProxyType == "" {
		cm.config.ProxyType = "socks5"
	}
	if cm.config.Timeout == 0 {
		cm.config.Timeout = 30
	}

	// Set legacy fields for backward compatibility
	cm.config.LocalSOCKSPort = cm.config.LocalPort
	cm.config.SSHUsername = cm.config.SSH.Username
	cm.config.SSHPassword = cm.config.SSH.Password
	cm.config.SSHPort = cm.config.SSH.Port
	cm.config.PayloadTemplate = cm.config.Payload
}

// GetConfig returns the loaded configuration
func (cm *ConfigManager) GetConfig() *Config {
	return cm.config
}
