package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose   bool
	proxyType string
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

Use 'tunn [mode] --help' for mode-specific options and examples.`,
	Version: "v0.1.1",
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
