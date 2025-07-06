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
	configFile   string
	tunnelConfig *config.Config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "tunn",
	Short:   "A powerful tunnel tool for secure connections",
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
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.json", "config file path")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{Use: "no-help", Hidden: true})
}

func runTunnel(cmd *cobra.Command, args []string) {
	fmt.Printf("Mode: %s\n", tunnelConfig.ConnectionMode)
	fmt.Printf("\n")

	// Create and start tunnel manager
	manager := tunnel.NewManager(tunnelConfig)
	if err := manager.Start(); err != nil {
		log.Fatalf("Failed to start tunnel: %v", err)
	}
}
