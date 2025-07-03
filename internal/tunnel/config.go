package tunnel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// XrayConfig represents the main configuration structure (Xray-like)
type XrayConfig struct {
	Log       *LogConfig       `json:"log,omitempty" yaml:"log,omitempty"`
	Inbounds  []InboundConfig  `json:"inbounds" yaml:"inbounds"`
	Outbounds []OutboundConfig `json:"outbounds" yaml:"outbounds"`
	Routing   *RoutingConfig   `json:"routing,omitempty" yaml:"routing,omitempty"`
	DNS       *DNSConfig       `json:"dns,omitempty" yaml:"dns,omitempty"`
	Profiles  []ProfileConfig  `json:"profiles,omitempty" yaml:"profiles,omitempty"`
}

// LogConfig defines logging configuration
type LogConfig struct {
	Level  string `json:"level,omitempty" yaml:"level,omitempty"`
	Access string `json:"access,omitempty" yaml:"access,omitempty"`
	Error  string `json:"error,omitempty" yaml:"error,omitempty"`
	DNSLog bool   `json:"dnsLog,omitempty" yaml:"dnsLog,omitempty"`
}

// InboundConfig defines inbound proxy configuration
type InboundConfig struct {
	Tag      string                 `json:"tag" yaml:"tag"`
	Port     int                    `json:"port" yaml:"port"`
	Listen   string                 `json:"listen,omitempty" yaml:"listen,omitempty"`
	Protocol string                 `json:"protocol" yaml:"protocol"` // socks, http
	Settings map[string]interface{} `json:"settings,omitempty" yaml:"settings,omitempty"`
}

// OutboundConfig defines outbound connection configuration
type OutboundConfig struct {
	Tag            string                 `json:"tag" yaml:"tag"`
	Protocol       string                 `json:"protocol" yaml:"protocol"` // tunnel, freedom, blackhole
	Settings       map[string]interface{} `json:"settings,omitempty" yaml:"settings,omitempty"`
	StreamSettings *StreamSettings        `json:"streamSettings,omitempty" yaml:"streamSettings,omitempty"`
}

// StreamSettings defines transport layer configuration
type StreamSettings struct {
	Network     string             `json:"network,omitempty" yaml:"network,omitempty"`   // ws, tcp
	Security    string             `json:"security,omitempty" yaml:"security,omitempty"` // tls, none
	WSSettings  *WebSocketSettings `json:"wsSettings,omitempty" yaml:"wsSettings,omitempty"`
	TLSSettings *TLSSettings       `json:"tlsSettings,omitempty" yaml:"tlsSettings,omitempty"`
}

// WebSocketSettings defines WebSocket-specific settings
type WebSocketSettings struct {
	Path    string            `json:"path,omitempty" yaml:"path,omitempty"`
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
}

// TLSSettings defines TLS-specific settings
type TLSSettings struct {
	ServerName string   `json:"serverName,omitempty" yaml:"serverName,omitempty"`
	ALPN       []string `json:"alpn,omitempty" yaml:"alpn,omitempty"`
}

// RoutingConfig defines routing rules
type RoutingConfig struct {
	DomainStrategy string       `json:"domainStrategy,omitempty" yaml:"domainStrategy,omitempty"`
	Rules          []RuleConfig `json:"rules,omitempty" yaml:"rules,omitempty"`
}

// RuleConfig defines a single routing rule
type RuleConfig struct {
	Type        string   `json:"type,omitempty" yaml:"type,omitempty"`
	Domain      []string `json:"domain,omitempty" yaml:"domain,omitempty"`
	IP          []string `json:"ip,omitempty" yaml:"ip,omitempty"`
	Port        string   `json:"port,omitempty" yaml:"port,omitempty"`
	Network     string   `json:"network,omitempty" yaml:"network,omitempty"`
	Protocol    []string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	OutboundTag string   `json:"outboundTag" yaml:"outboundTag"`
}

// DNSConfig defines DNS settings
type DNSConfig struct {
	Hosts   map[string]string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
	Servers []string          `json:"servers,omitempty" yaml:"servers,omitempty"`
}

// ProfileConfig defines a tunnel profile for easy switching
type ProfileConfig struct {
	Name        string    `json:"name" yaml:"name"`
	Mode        string    `json:"mode" yaml:"mode"` // proxy, sni, direct
	ProxyHost   string    `json:"proxyHost,omitempty" yaml:"proxyHost,omitempty"`
	ProxyPort   string    `json:"proxyPort,omitempty" yaml:"proxyPort,omitempty"`
	TargetHost  string    `json:"targetHost" yaml:"targetHost"`
	TargetPort  string    `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`
	FrontDomain string    `json:"frontDomain,omitempty" yaml:"frontDomain,omitempty"`
	SSH         SSHConfig `json:"ssh" yaml:"ssh"`
	LocalPort   int       `json:"localPort,omitempty" yaml:"localPort,omitempty"`
	ProxyType   string    `json:"proxyType,omitempty" yaml:"proxyType,omitempty"`
	Payload     string    `json:"payload,omitempty" yaml:"payload,omitempty"`
	Timeout     int       `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// SSHConfig defines SSH connection settings
type SSHConfig struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Port     string `json:"port,omitempty" yaml:"port,omitempty"`
}

// Config holds all configuration parameters (legacy structure, maintained for compatibility)
type Config struct {
	Mode            string
	FrontDomain     string
	LocalSOCKSPort  int
	ProxyHost       string
	ProxyPort       string
	TargetHost      string
	TargetPort      string
	SSHUsername     string
	SSHPassword     string
	SSHPort         string
	PayloadTemplate string
}

// ConfigManager handles loading and parsing of configuration files
type ConfigManager struct {
	configPath string
	xrayConfig *XrayConfig
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

	cm.xrayConfig = &XrayConfig{}

	switch ext {
	case ".json":
		if err := json.Unmarshal([]byte(content), cm.xrayConfig); err != nil {
			return fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal([]byte(content), cm.xrayConfig); err != nil {
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
	if cm.xrayConfig == nil {
		return fmt.Errorf("no configuration loaded")
	}

	// Validate inbounds
	if len(cm.xrayConfig.Inbounds) == 0 {
		return fmt.Errorf("at least one inbound configuration is required")
	}

	for i, inbound := range cm.xrayConfig.Inbounds {
		if inbound.Tag == "" {
			return fmt.Errorf("inbound[%d]: tag is required", i)
		}
		if inbound.Port <= 0 || inbound.Port > 65535 {
			return fmt.Errorf("inbound[%d]: invalid port %d", i, inbound.Port)
		}
		if inbound.Protocol != "socks" && inbound.Protocol != "http" {
			return fmt.Errorf("inbound[%d]: unsupported protocol %s", i, inbound.Protocol)
		}
	}

	// Validate outbounds
	if len(cm.xrayConfig.Outbounds) == 0 {
		return fmt.Errorf("at least one outbound configuration is required")
	}

	for i, outbound := range cm.xrayConfig.Outbounds {
		if outbound.Tag == "" {
			return fmt.Errorf("outbound[%d]: tag is required", i)
		}
		if outbound.Protocol != "tunnel" && outbound.Protocol != "freedom" && outbound.Protocol != "blackhole" {
			return fmt.Errorf("outbound[%d]: unsupported protocol %s", i, outbound.Protocol)
		}
	}

	// Validate profiles if present
	for i, profile := range cm.xrayConfig.Profiles {
		if profile.Name == "" {
			return fmt.Errorf("profile[%d]: name is required", i)
		}
		if profile.Mode != "proxy" && profile.Mode != "sni" && profile.Mode != "direct" {
			return fmt.Errorf("profile[%d]: unsupported mode %s", i, profile.Mode)
		}
		if profile.TargetHost == "" {
			return fmt.Errorf("profile[%d]: targetHost is required", i)
		}
		if profile.SSH.Username == "" {
			return fmt.Errorf("profile[%d]: SSH username is required", i)
		}
	}

	return nil
}

// GetConfig returns the loaded Xray configuration
func (cm *ConfigManager) GetConfig() *XrayConfig {
	return cm.xrayConfig
}

// GetProfile returns a specific profile by name
func (cm *ConfigManager) GetProfile(name string) (*ProfileConfig, error) {
	if cm.xrayConfig == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	for _, profile := range cm.xrayConfig.Profiles {
		if profile.Name == name {
			return &profile, nil
		}
	}

	return nil, fmt.Errorf("profile '%s' not found", name)
}

// ConvertProfileToLegacyConfig converts a profile to legacy Config structure
func (cm *ConfigManager) ConvertProfileToLegacyConfig(profile *ProfileConfig) *Config {
	config := &Config{
		Mode:            profile.Mode,
		FrontDomain:     profile.FrontDomain,
		LocalSOCKSPort:  profile.LocalPort,
		ProxyHost:       profile.ProxyHost,
		ProxyPort:       profile.ProxyPort,
		TargetHost:      profile.TargetHost,
		TargetPort:      profile.TargetPort,
		SSHUsername:     profile.SSH.Username,
		SSHPassword:     profile.SSH.Password,
		SSHPort:         profile.SSH.Port,
		PayloadTemplate: profile.Payload,
	}

	// Set defaults
	if config.TargetPort == "" {
		config.TargetPort = "22"
	}
	if config.SSHPort == "" {
		config.SSHPort = "22"
	}
	if config.LocalSOCKSPort == 0 {
		config.LocalSOCKSPort = 1080
	}

	return config
}
