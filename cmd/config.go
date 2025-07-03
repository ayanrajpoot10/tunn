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
	Long: `Manage Tunn configurations including creating templates and validating configs.

This command provides utilities for working with simplified configuration files.`,
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a sample configuration file",
	Long:  `Generate a sample configuration file with examples for all supported modes.`,
	Run:   generateConfig,
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long:  `Validate the syntax and content of a configuration file.`,
	Run:   validateConfig,
}

var validateFlags struct {
	configPath string
}

var generateFlags struct {
	output string
	format string
	mode   string
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(generateCmd)
	configCmd.AddCommand(validateCmd)

	generateCmd.Flags().StringVarP(&generateFlags.output, "output", "o", "tunn-config.json", "output file path")
	generateCmd.Flags().StringVarP(&generateFlags.format, "format", "f", "json", "output format: json or yaml")
	generateCmd.Flags().StringVarP(&generateFlags.mode, "mode", "m", "proxy", "tunnel mode: proxy, sni, or direct")

	validateCmd.Flags().StringVarP(&validateFlags.configPath, "config", "c", "", "path to configuration file to validate (required)")
	validateCmd.MarkFlagRequired("config")
}

func generateConfig(cmd *cobra.Command, args []string) {
	// Create sample configurations for different modes
	var sampleConfig *tunnel.Config

	switch generateFlags.mode {
	case "proxy":
		sampleConfig = &tunnel.Config{
			Mode:        "proxy",
			TargetHost:  "target.example.com",
			TargetPort:  "22",
			ProxyHost:   "proxy.example.com",
			ProxyPort:   "80",
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
		}
	case "sni":
		sampleConfig = &tunnel.Config{
			Mode:        "sni",
			TargetHost:  "target.example.com",
			TargetPort:  "22",
			ProxyHost:   "proxy.example.com",
			ProxyPort:   "443",
			FrontDomain: "cloudflare.com",
			SSH: tunnel.SSHConfig{
				Username: "user",
				Password: "password",
				Port:     "22",
			},
			LocalPort: 1080,
			ProxyType: "socks5",
			Timeout:   30,
		}
	case "direct":
		sampleConfig = &tunnel.Config{
			Mode:        "direct",
			TargetHost:  "us2.ws-tun.me",
			TargetPort:  "80",
			FrontDomain: "config.rcs.mnc840.mcc405.pub.3gppnetwork.org",
			SSH: tunnel.SSHConfig{
				Username: "sshstores-ayan",
				Password: "1234",
				Port:     "22",
			},
			LocalPort: 1080,
			ProxyType: "http",
			Timeout:   30,
		}
	default:
		fmt.Printf("❌ Unsupported mode: %s (supported: proxy, sni, direct)\n", generateFlags.mode)
		os.Exit(1)
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

	fmt.Printf("✓ Sample %s mode configuration generated: %s\n", generateFlags.mode, generateFlags.output)
	fmt.Println("\n📝 Remember to:")
	fmt.Println("   1. Update target host and SSH credentials")
	fmt.Println("   2. Modify proxy settings if needed")
	fmt.Println("   3. Validate the config with: tunn config validate --config", generateFlags.output)
}

func validateConfig(cmd *cobra.Command, args []string) {
	configPath := validateFlags.configPath
	if configPath == "" {
		fmt.Println("❌ No config file specified. Use --config flag.")
		os.Exit(1)
	}

	configMgr := tunnel.NewConfigManager(configPath)
	if err := configMgr.LoadConfig(); err != nil {
		fmt.Printf("❌ Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	config := configMgr.GetConfig()

	fmt.Printf("✓ Configuration file is valid: %s\n", configPath)
	fmt.Printf("📊 Configuration Summary:\n")
	fmt.Printf("   - Mode: %s\n", config.Mode)
	fmt.Printf("   - Target: %s:%s\n", config.TargetHost, config.TargetPort)
	if config.ProxyHost != "" {
		fmt.Printf("   - Proxy: %s:%s\n", config.ProxyHost, config.ProxyPort)
	}
	if config.FrontDomain != "" {
		fmt.Printf("   - Front Domain: %s\n", config.FrontDomain)
	}
	fmt.Printf("   - SSH User: %s\n", config.SSH.Username)
	fmt.Printf("   - Local Port: %d (%s)\n", config.LocalPort, config.ProxyType)
	fmt.Printf("   - Timeout: %d seconds\n", config.Timeout)
}
