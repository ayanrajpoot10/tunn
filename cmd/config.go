package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"tunn/pkg/config"

	"github.com/spf13/cobra"
)

// configCmd represents the config command and its subcommands.
// It provides functionality for configuration file management including
// generation and validation operations.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
}

// generateCmd represents the config generate command.
// It creates sample configuration files for different tunnel modes.
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a sample configuration file",
	Run:   generateConfig,
}

// validateCmd represents the config validate command.
// It validates the syntax and content of configuration files.
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Run:   validateConfig,
}

// validateFlags holds the command-line flags for the validate subcommand.
var validateFlags struct {
	configPath string
}

// generateFlags holds the command-line flags for the generate subcommand.
var generateFlags struct {
	output string
	mode   string
}

// init initializes the config command and its subcommands with their respective flags.
func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(generateCmd)
	configCmd.AddCommand(validateCmd)

	generateCmd.Flags().StringVarP(&generateFlags.output, "output", "o", "config.json", "output file path")
	generateCmd.Flags().StringVarP(&generateFlags.mode, "mode", "m", "direct", "tunnel mode: direct or proxy")

	validateCmd.Flags().StringVarP(&validateFlags.configPath, "config", "c", "", "path to configuration file to validate (required)")
	validateCmd.MarkFlagRequired("config")
}

// generateConfig generates a sample configuration file based on the specified mode.
// It creates a configuration template with example values that users can customize
// for their specific tunneling needs.
func generateConfig(cmd *cobra.Command, args []string) {
	var sampleConfig *config.Config

	switch generateFlags.mode {
	case "direct":
		sampleConfig = &config.Config{
			Mode: "direct",
			SSH: config.SSHConfig{
				Host:     "target.example.com",
				Port:     80,
				Username: "user",
				Password: "password",
			},
			Listener: config.ListenerConfig{
				Port:      1080,
				ProxyType: "socks5",
			},
			HTTPPayload:       "GET / HTTP/1.1[crlf]Host: [host][crlf]Upgrade: websocket[crlf][crlf]",
			ConnectionTimeout: 30,
		}
	case "proxy":
		sampleConfig = &config.Config{
			Mode:      "proxy",
			ProxyHost: "proxy.example.com",
			ProxyPort: "80",
			SSH: config.SSHConfig{
				Host:     "target.example.com",
				Port:     80,
				Username: "user",
				Password: "password",
			},
			Listener: config.ListenerConfig{
				Port:      1080,
				ProxyType: "socks5",
			},
			HTTPPayload:       "GET / HTTP/1.1[crlf]Host: [host][crlf]Upgrade: websocket[crlf][crlf]",
			ConnectionTimeout: 30,
		}
	default:
		fmt.Printf("Error: Unsupported mode: %s (supported: direct, proxy)\n", generateFlags.mode)
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

// validateConfig validates an existing configuration file for syntax and content correctness.
// It loads the configuration file and performs comprehensive validation checks to ensure
// all required fields are present and valid for the specified tunnel mode.
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
	fmt.Printf("   - Mode: %s\n", config.Mode)
	fmt.Printf("   - SSH Target: %s:%d\n", config.SSH.Host, config.SSH.Port)
	if config.ProxyHost != "" {
		fmt.Printf("   - Proxy: %s:%s\n", config.ProxyHost, config.ProxyPort)
	}
	fmt.Printf("   - SSH User: %s\n", config.SSH.Username)
	fmt.Printf("   - Local Port: %d (%s)\n", config.Listener.Port, config.Listener.ProxyType)
	fmt.Printf("   - Timeout: %d seconds\n", config.ConnectionTimeout)
}
