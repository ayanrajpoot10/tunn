package cmd

import (
	"fmt"
	"os"

	"tunn/internal/tunnel"

	"github.com/spf13/cobra"
)

var (
	verbose    bool
	proxyType  string
	configFile string
	profile    string
	configMgr  *tunnel.ConfigManager
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tunn",
	Short: "A powerful tunnel tool for secure connections",
	Long: `Tunn is a versatile tunneling tool that supports multiple connection strategies
including HTTP payload tunneling, SNI fronting, and direct connections.

It creates secure SSH tunnels over WebSocket connections and provides a SOCKS proxy
for routing traffic through the established tunnel.

Available tunnel modes:
  proxy         - HTTP proxy tunneling strategy  
  sni           - SNI fronting strategy  
  direct        - Direct connection with front domain spoofing

All modes support both SOCKS5 and HTTP local proxy types via --proxy-type flag.

Configuration can be provided via:
  1. Command line flags (traditional method)
  2. JSON/YAML configuration file (Xray-like)
  3. Profile selection from config file

Use 'tunn [mode] --help' for mode-specific options and examples.
Use 'tunn --config config.json --profile myprofile' to use a specific profile.`,
	Version: "v0.1.1",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration if config file is provided
		if configFile != "" {
			configMgr = tunnel.NewConfigManager(configFile)
			if err := configMgr.LoadConfig(); err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if verbose {
				fmt.Printf("✓ Configuration loaded from: %s\n", configFile)
			}
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVar(&proxyType, "proxy-type", "socks5", "local proxy type: 'socks5' or 'http'")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "path to configuration file (JSON/YAML)")
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "", "", "profile name to use from config file")

	// Disable built-in commands
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})
}

// GetProxyType returns the global proxy type setting
func GetProxyType() string {
	return proxyType
}

// GetConfigManager returns the global config manager
func GetConfigManager() *tunnel.ConfigManager {
	return configMgr
}

// GetProfile returns the selected profile name
func GetProfile() string {
	return profile
}
