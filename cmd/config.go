package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"tunn/pkg/config"

	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
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
	mode   string
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(generateCmd)
	configCmd.AddCommand(validateCmd)

	generateCmd.Flags().StringVarP(&generateFlags.output, "output", "o", "tunn-config.json", "output file path")
	generateCmd.Flags().StringVarP(&generateFlags.mode, "mode", "m", "proxy", "tunnel mode: proxy, sni, or direct")

	validateCmd.Flags().StringVarP(&validateFlags.configPath, "config", "c", "", "path to configuration file to validate (required)")
	validateCmd.MarkFlagRequired("config")
}

func generateConfig(cmd *cobra.Command, args []string) {
	// Create sample configurations for different modes
	var sampleConfig *config.Config

	switch generateFlags.mode {
	case "proxy":
		sampleConfig = &config.Config{
			ConnectionMode: "proxy",
			ServerHost:     "target.example.com",
			ServerPort:     "80",
			ProxyHost:      "proxy.example.com",
			ProxyPort:      "80",
			SpoofedHost:    "google.com",
			SSH: config.SSHConfig{
				Username: "user",
				Password: "password",
				Port:     "22",
			},
			ListenPort:        1080,
			ProxyType:         "socks5",
			HTTPPayload:       "GET / HTTP/1.1\r\nHost: [host]\r\nUpgrade: websocket\r\n\r\n",
			ConnectionTimeout: 30,
		}
	case "sni":
		sampleConfig = &config.Config{
			ConnectionMode: "sni",
			ServerHost:     "target.example.com",
			ServerPort:     "443",
			ProxyHost:      "proxy.example.com",
			ProxyPort:      "443",
			SpoofedHost:    "cloudflare.com",
			SSH: config.SSHConfig{
				Username: "user",
				Password: "password",
				Port:     "22",
			},
			ListenPort:        1080,
			ProxyType:         "socks5",
			HTTPPayload:       "GET / HTTP/1.1\r\nHost: [host]\r\nUpgrade: websocket\r\n\r\n",
			ConnectionTimeout: 30,
		}
	case "direct":
		sampleConfig = &config.Config{
			ConnectionMode: "direct",
			ServerHost:     "target.example.com",
			ServerPort:     "80",
			SpoofedHost:    "cloudflare.com",
			SSH: config.SSHConfig{
				Username: "user",
				Password: "password",
				Port:     "22",
			},
			ListenPort:        1080,
			ProxyType:         "http",
			HTTPPayload:       "GET / HTTP/1.1\r\nHost: [host]\r\nUpgrade: websocket\r\n\r\n",
			ConnectionTimeout: 30,
		}
	default:
		fmt.Printf("Error: Unsupported mode: %s (supported: proxy, sni, direct)\n", generateFlags.mode)
		os.Exit(1)
	}

	var data []byte
	var err error

	data, err = json.MarshalIndent(sampleConfig, "", "  ")

	if err != nil {
		fmt.Printf("Error: Failed to marshal config: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(generateFlags.output, data, 0644); err != nil {
		fmt.Printf("Error: Failed to write config file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Success: Sample %s mode configuration generated: %s\n", generateFlags.mode, generateFlags.output)
}

func validateConfig(cmd *cobra.Command, args []string) {
	configPath := validateFlags.configPath
	if configPath == "" {
		fmt.Println("Error: No config file specified. Use --config flag.")
		os.Exit(1)
	}

	config, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Error: Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Success: Configuration file is valid: %s\n", configPath)
	fmt.Printf("Configuration Summary:\n")
	fmt.Printf("   - Mode: %s\n", config.ConnectionMode)
	fmt.Printf("   - Target: %s:%s\n", config.ServerHost, config.ServerPort)
	if config.ProxyHost != "" {
		fmt.Printf("   - Proxy: %s:%s\n", config.ProxyHost, config.ProxyPort)
	}
	if config.SpoofedHost != "" {
		fmt.Printf("   - Spoofed Host: %s\n", config.SpoofedHost)
	}
	fmt.Printf("   - SSH User: %s\n", config.SSH.Username)
	fmt.Printf("   - Local Port: %d (%s)\n", config.ListenPort, config.ProxyType)
	fmt.Printf("   - Timeout: %d seconds\n", config.ConnectionTimeout)
}
