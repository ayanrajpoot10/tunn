// Package cmd provides the command-line interface for the Tunn SSH tunneling tool.
//
// This package implements the CLI commands using the Cobra library, handling
// configuration loading, command parsing, and tunnel management initialization.
// It supports multiple subcommands for configuration management and tunnel operations.
package cmd

import (
	"fmt"
	"log"
	"os"

	"tunn/internal/tunnel"
	"tunn/pkg/config"

	"github.com/spf13/cobra"
)

var (
	// configFile holds the path to the configuration file specified by the user.
	configFile string

	// tunnelConfig contains the loaded and validated tunnel configuration.
	tunnelConfig *config.Config
)

// rootCmd represents the base command when called without any subcommands.
// It loads the configuration file and starts the tunnel manager.
var rootCmd = &cobra.Command{
	Use:   "tunn",
	Short: "A powerful tunnel tool for secure connections",
	Long: `Tunn is a cross-platform SSH tunneling tool that creates secure connections
through direct connections over WebSocket and HTTP proxies.

Features:
• Multiple tunnel modes: Direct connection and Proxy
• WebSocket-based SSH tunnels for better bypass capabilities
• SOCKS5 and HTTP proxy support
• Domain spoofing capabilities
• Cross-platform support (Windows, Linux, macOS)

Examples:
  tunn --config config.json
  tunn config generate --mode direct --output myconfig.json
  tunn config validate --config myconfig.json`,
	Version: "v0.1.2",
	PreRun: func(cmd *cobra.Command, args []string) {
		var err error
		tunnelConfig, err = config.LoadConfig(configFile)
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
		}
	},
	Run: runTunnel,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// init initializes the root command with persistent flags and configuration.
func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.json", "config file path")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{Use: "no-help", Hidden: true})
}

// runTunnel is the main execution function that starts the tunnel manager
// with the loaded configuration.
func runTunnel(cmd *cobra.Command, args []string) {
	fmt.Printf("Mode: %s\n", tunnelConfig.Mode)
	fmt.Printf("\n")

	// Create and start tunnel manager
	manager := tunnel.NewManager(tunnelConfig)
	if err := manager.Start(); err != nil {
		log.Fatalf("Failed to start tunnel: %v", err)
	}
}
