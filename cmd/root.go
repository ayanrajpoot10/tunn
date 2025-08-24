// Package cmd provides the command-line interface for the Tunn SSH tunneling tool.
//
// This package implements the CLI commands using the Cobra library, handling
// configuration loading, command parsing, and tunnel management initialization.
// It supports multiple subcommands for configuration management and tunnel operations.
package cmd

import (
	"context"
	"fmt"
	"os"

	"tunn/internal/tunnel"
	"tunn/pkg/config"

	"github.com/spf13/cobra"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const configKey contextKey = "cfg"

// rootCmd represents the base command when called without any subcommands.
// It loads the configuration file and starts the tunnel manager.
var rootCmd = &cobra.Command{
	Use:     "tunn",
	Short:   "A powerful tunnel tool for secure connections",
	Version: "v0.1.2",

	PreRunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Store config in context for Run
		cmd.SetContext(context.WithValue(cmd.Context(), configKey, cfg))
		return nil
	},

	RunE: func(cmd *cobra.Command, args []string) error {
		// Retrieve config from context
		cfg, ok := cmd.Context().Value(configKey).(*config.Config)
		if !ok {
			return fmt.Errorf("failed to retrieve config from context")
		}

		fmt.Printf("Mode: %s\n\n", cfg.Mode)

		manager := tunnel.NewManager(cfg)
		if err := manager.Start(); err != nil {
			return fmt.Errorf("failed to start tunnel: %w", err)
		}
		return nil
	},
}

var configFile string

// init initializes the root command with persistent flags and configuration.
func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.json", "config file path")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{Use: "no-help", Hidden: true})
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
