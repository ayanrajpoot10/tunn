package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"tunn/internal/tunnel"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
	Long: `Manage Tunn configurations including creating templates, listing profiles, and validating configs.

This command provides utilities for working with Xray-like configuration files.`,
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a sample configuration file",
	Long:  `Generate a sample configuration file with examples for all supported features.`,
	Run:   generateConfig,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available profiles",
	Long:  `List all profiles available in the configuration file.`,
	Run:   listProfiles,
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long:  `Validate the syntax and content of a configuration file.`,
	Run:   validateConfig,
}

var generateFlags struct {
	output string
	format string
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(generateCmd)
	configCmd.AddCommand(listCmd)
	configCmd.AddCommand(validateCmd)

	generateCmd.Flags().StringVarP(&generateFlags.output, "output", "o", "tunn-config.json", "output file path")
	generateCmd.Flags().StringVarP(&generateFlags.format, "format", "f", "json", "output format: json or yaml")
}

func generateConfig(cmd *cobra.Command, args []string) {
	// Create a sample configuration
	sampleConfig := &tunnel.XrayConfig{
		Log: &tunnel.LogConfig{
			Level:  "info",
			Access: "/var/log/tunn/access.log",
			Error:  "/var/log/tunn/error.log",
			DNSLog: false,
		},
		Inbounds: []tunnel.InboundConfig{
			{
				Tag:      "socks-in",
				Port:     1080,
				Listen:   "127.0.0.1",
				Protocol: "socks",
				Settings: map[string]interface{}{
					"auth": "noauth",
					"udp":  true,
				},
			},
			{
				Tag:      "http-in",
				Port:     8080,
				Listen:   "127.0.0.1",
				Protocol: "http",
				Settings: map[string]interface{}{
					"timeout": 300,
				},
			},
		},
		Outbounds: []tunnel.OutboundConfig{
			{
				Tag:      "tunnel-out",
				Protocol: "tunnel",
				Settings: map[string]interface{}{
					"servers": []map[string]interface{}{
						{
							"address": "$TARGET_HOST",
							"port":    "$TARGET_PORT",
						},
					},
				},
				StreamSettings: &tunnel.StreamSettings{
					Network:  "ws",
					Security: "tls",
					WSSettings: &tunnel.WebSocketSettings{
						Path: "/ws",
						Headers: map[string]string{
							"Host": "$FRONT_DOMAIN",
						},
					},
					TLSSettings: &tunnel.TLSSettings{
						ServerName: "$FRONT_DOMAIN",
						ALPN:       []string{"h2", "http/1.1"},
					},
				},
			},
			{
				Tag:      "freedom",
				Protocol: "freedom",
				Settings: map[string]interface{}{
					"domainStrategy": "UseIP",
				},
			},
		},
		Routing: &tunnel.RoutingConfig{
			DomainStrategy: "IPIfNonMatch",
			Rules: []tunnel.RuleConfig{
				{
					Type:        "field",
					Domain:      []string{"geosite:cn"},
					OutboundTag: "freedom",
				},
				{
					Type:        "field",
					IP:          []string{"geoip:private"},
					OutboundTag: "freedom",
				},
				{
					Type:        "field",
					Network:     "tcp,udp",
					OutboundTag: "tunnel-out",
				},
			},
		},
		DNS: &tunnel.DNSConfig{
			Hosts: map[string]string{
				"example.com": "127.0.0.1",
			},
			Servers: []string{
				"8.8.8.8",
				"1.1.1.1",
			},
		},
		Profiles: []tunnel.ProfileConfig{
			{
				Name:        "default",
				Mode:        "proxy",
				ProxyHost:   "proxy.example.com",
				ProxyPort:   "80",
				TargetHost:  "target.example.com",
				TargetPort:  "22",
				FrontDomain: "google.com",
				SSH: tunnel.SSHConfig{
					Username: "user",
					Password: "password",
					Port:     "22",
				},
				LocalPort: 1080,
				ProxyType: "socks5",
				Payload:   "GET / HTTP/1.1\r\nHost: $FRONT_DOMAIN\r\nUpgrade: websocket\r\n\r\n",
				Timeout:   30,
			},
			{
				Name:        "sni-mode",
				Mode:        "sni",
				ProxyHost:   "proxy.example.com",
				ProxyPort:   "443",
				TargetHost:  "target.example.com",
				TargetPort:  "22",
				FrontDomain: "cloudflare.com",
				SSH: tunnel.SSHConfig{
					Username: "user",
					Password: "password",
					Port:     "22",
				},
				LocalPort: 1080,
				ProxyType: "socks5",
				Timeout:   30,
			},
			{
				Name:        "direct-mode",
				Mode:        "direct",
				TargetHost:  "target.example.com",
				TargetPort:  "443",
				FrontDomain: "microsoft.com",
				SSH: tunnel.SSHConfig{
					Username: "user",
					Password: "password",
					Port:     "22",
				},
				LocalPort: 1080,
				ProxyType: "socks5",
				Timeout:   30,
			},
		},
	}

	var data []byte
	var err error

	switch generateFlags.format {
	case "yaml", "yml":
		data, err = yaml.Marshal(sampleConfig)
	case "json":
		data, err = json.MarshalIndent(sampleConfig, "", "  ")
	default:
		fmt.Printf("❌ Unsupported format: %s\n", generateFlags.format)
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("❌ Failed to marshal config: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(generateFlags.output, data, 0644); err != nil {
		fmt.Printf("❌ Failed to write config file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Sample configuration generated: %s\n", generateFlags.output)
	fmt.Println("\n📝 Remember to:")
	fmt.Println("   1. Replace environment variables like $TARGET_HOST with actual values")
	fmt.Println("   2. Update SSH credentials")
	fmt.Println("   3. Modify proxy and target settings according to your setup")
	fmt.Println("   4. Validate the config with: tunn config validate --config", generateFlags.output)
}

func listProfiles(cmd *cobra.Command, args []string) {
	if configFile == "" {
		fmt.Println("❌ No config file specified. Use --config flag.")
		os.Exit(1)
	}

	configMgr := tunnel.NewConfigManager(configFile)
	if err := configMgr.LoadConfig(); err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	config := configMgr.GetConfig()
	if len(config.Profiles) == 0 {
		fmt.Println("📝 No profiles found in configuration file.")
		return
	}

	fmt.Printf("📋 Available profiles in %s:\n\n", configFile)
	for i, profile := range config.Profiles {
		fmt.Printf("%d. %s\n", i+1, profile.Name)
		fmt.Printf("   Mode: %s\n", profile.Mode)
		fmt.Printf("   Target: %s:%s\n", profile.TargetHost, profile.TargetPort)
		if profile.ProxyHost != "" {
			fmt.Printf("   Proxy: %s:%s\n", profile.ProxyHost, profile.ProxyPort)
		}
		if profile.FrontDomain != "" {
			fmt.Printf("   Front Domain: %s\n", profile.FrontDomain)
		}
		fmt.Printf("   SSH User: %s\n", profile.SSH.Username)
		fmt.Printf("   Local Port: %d\n", profile.LocalPort)
		fmt.Println()
	}
}

func validateConfig(cmd *cobra.Command, args []string) {
	if configFile == "" {
		fmt.Println("❌ No config file specified. Use --config flag.")
		os.Exit(1)
	}

	configMgr := tunnel.NewConfigManager(configFile)
	if err := configMgr.LoadConfig(); err != nil {
		fmt.Printf("❌ Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	config := configMgr.GetConfig()

	fmt.Printf("✓ Configuration file is valid: %s\n", configFile)
	fmt.Printf("📊 Summary:\n")
	fmt.Printf("   - Inbounds: %d\n", len(config.Inbounds))
	fmt.Printf("   - Outbounds: %d\n", len(config.Outbounds))
	fmt.Printf("   - Profiles: %d\n", len(config.Profiles))

	if config.Routing != nil {
		fmt.Printf("   - Routing rules: %d\n", len(config.Routing.Rules))
	}

	if config.DNS != nil {
		fmt.Printf("   - DNS hosts: %d\n", len(config.DNS.Hosts))
		fmt.Printf("   - DNS servers: %d\n", len(config.DNS.Servers))
	}
}
