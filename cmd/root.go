package cmd

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

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
  1. Command line flags with mode specification: tunn [mode] --target-host example.com --ssh-username user
  2. JSON/YAML configuration file with profile: tunn --config config.json --profile myprofile

When using a configuration file with profiles, the mode is automatically determined from the profile,
eliminating the need to specify the mode separately.

Examples:
  # Traditional command line usage
  tunn proxy --proxy-host proxy.example.com --target-host target.example.com --ssh-username user --ssh-password pass
  
  # New config-based usage (mode determined from profile)
  tunn --config config.json --profile myprofile
  
Use 'tunn [mode] --help' for mode-specific options and examples.`,
	Version: "v0.1.1", Run: func(cmd *cobra.Command, args []string) {
		// If both config and profile are specified, run directly using the profile's mode
		if configFile != "" && profile != "" {
			runWithConfigProfile(cmd)
			return
		}

		// Otherwise, show help
		cmd.Help()
	},
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

// runWithConfigProfile executes tunnel using configuration profile without needing mode specification
func runWithConfigProfile(cmd *cobra.Command) {
	if configMgr == nil {
		log.Fatal("Error: Configuration manager not initialized")
	}

	profileConfig, err := configMgr.GetProfile(profile)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Determine proxy type with proper priority:
	// 1. User-provided --proxy-type flag (highest priority)
	// 2. Profile's proxyType setting
	// 3. Default "socks5" (lowest priority)
	var effectiveProxyType string

	// Check if --proxy-type flag was explicitly set by user
	proxyTypeFlag := cmd.PersistentFlags().Lookup("proxy-type")
	if proxyTypeFlag != nil && proxyTypeFlag.Changed {
		// User explicitly provided --proxy-type flag, use it
		effectiveProxyType = GetProxyType()
		if verbose {
			fmt.Printf("[*] Using --proxy-type flag: %s\n", effectiveProxyType)
		}
	} else if profileConfig.ProxyType != "" {
		// No explicit flag, use profile's proxyType
		effectiveProxyType = profileConfig.ProxyType
		if verbose {
			fmt.Printf("[*] Using profile proxyType: %s\n", effectiveProxyType)
		}
	} else {
		// Neither flag nor profile specify, use default
		effectiveProxyType = "socks5"
		if verbose {
			fmt.Printf("[*] Using default proxyType: %s\n", effectiveProxyType)
		}
	}

	// Validate proxy type
	if effectiveProxyType != "socks5" && effectiveProxyType != "http" {
		log.Fatal("Error: proxy type must be either 'socks5' or 'http'")
	}

	fmt.Printf("[*] Using profile: %s (mode: %s)\n", profileConfig.Name, profileConfig.Mode)
	fmt.Printf("[*] Starting tunnel using %s strategy with %s local proxy\n", profileConfig.Mode, effectiveProxyType)

	// Execute the appropriate tunnel function based on the profile mode
	switch profileConfig.Mode {
	case "proxy":
		runTunnelWithConfigProxy(profileConfig, effectiveProxyType)
	case "sni":
		runTunnelWithConfigSNI(profileConfig, effectiveProxyType)
	case "direct":
		runTunnelWithConfigDirect(profileConfig, effectiveProxyType)
	default:
		log.Fatalf("Error: Unsupported mode '%s' in profile '%s'", profileConfig.Mode, profileConfig.Name)
	}
}

// runTunnelWithConfigProxy executes proxy tunnel with the given profile configuration
func runTunnelWithConfigProxy(profileConfig *tunnel.ProfileConfig, globalProxyType string) {
	// Convert profile to legacy config
	cfg := configMgr.ConvertProfileToLegacyConfig(profileConfig)

	fmt.Printf("[*] Proxy: %s:%s\n", cfg.ProxyHost, cfg.ProxyPort)
	fmt.Printf("[*] Target: %s:%s\n", cfg.TargetHost, cfg.TargetPort)
	fmt.Printf("[*] SSH User: %s\n", cfg.SSHUsername)
	fmt.Printf("[*] Local %s proxy will be available on 127.0.0.1:%d\n", globalProxyType, cfg.LocalSOCKSPort)

	if cfg.FrontDomain != "" {
		fmt.Printf("[*] Front Domain: %s\n", cfg.FrontDomain)
	}

	// Establish HTTP proxy tunnel
	conn, err := tunnel.EstablishWSTunnel(
		cfg.ProxyHost,
		cfg.ProxyPort,
		cfg.TargetHost,
		cfg.TargetPort,
		cfg.PayloadTemplate,
		cfg.FrontDomain,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Error establishing proxy tunnel to target: %v", err)
	}
	defer conn.Close()

	fmt.Printf("[*] Connected to proxy %s:%s, tunneling to target %s:%s\n",
		cfg.ProxyHost, cfg.ProxyPort, cfg.TargetHost, cfg.TargetPort)

	fmt.Printf("[*] Starting SSH connection and %s proxy...\n", globalProxyType)

	startSSHConnection(conn, cfg, globalProxyType, profileConfig.Timeout)
}

// runTunnelWithConfigSNI executes SNI tunnel with the given profile configuration
func runTunnelWithConfigSNI(profileConfig *tunnel.ProfileConfig, globalProxyType string) {
	cfg := configMgr.ConvertProfileToLegacyConfig(profileConfig)

	fmt.Printf("[*] SNI Front Domain: %s\n", cfg.FrontDomain)
	fmt.Printf("[*] Proxy: %s:%s\n", cfg.ProxyHost, cfg.ProxyPort)
	fmt.Printf("[*] Target: %s:%s\n", cfg.TargetHost, cfg.TargetPort)
	fmt.Printf("[*] SSH User: %s\n", cfg.SSHUsername)
	fmt.Printf("[*] Local %s proxy will be available on 127.0.0.1:%d\n", globalProxyType, cfg.LocalSOCKSPort)

	// Connect to proxy with SNI fronting
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", cfg.ProxyHost, cfg.ProxyPort))
	if err != nil {
		log.Fatalf("Error connecting to proxy: %v", err)
	}

	// Setup TLS connection with SNI fronting
	tlsConfig := &tls.Config{
		ServerName:         cfg.FrontDomain,
		InsecureSkipVerify: true,
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		log.Fatalf("Error establishing TLS connection: %v", err)
	}

	fmt.Printf("[*] TLS connection established with SNI: %s\n", cfg.FrontDomain)

	// Establish WebSocket tunnel through the TLS connection
	wsConn, err := tunnel.EstablishWSTunnel(
		cfg.TargetHost,
		cfg.TargetPort,
		cfg.TargetHost,
		cfg.TargetPort,
		cfg.PayloadTemplate,
		cfg.FrontDomain,
		true,
		tlsConn,
	)
	if err != nil {
		log.Fatalf("Error establishing WebSocket tunnel: %v", err)
	}
	defer wsConn.Close()

	fmt.Printf("[*] SNI fronted connection established to target %s:%s\n", cfg.TargetHost, cfg.TargetPort)

	startSSHConnection(wsConn, cfg, globalProxyType, profileConfig.Timeout)
}

// runTunnelWithConfigDirect executes direct tunnel with the given profile configuration
func runTunnelWithConfigDirect(profileConfig *tunnel.ProfileConfig, globalProxyType string) {
	cfg := configMgr.ConvertProfileToLegacyConfig(profileConfig)

	fmt.Printf("[*] Target: %s:%s\n", cfg.TargetHost, cfg.TargetPort)
	fmt.Printf("[*] SSH User: %s\n", cfg.SSHUsername)
	fmt.Printf("[*] Local %s proxy will be available on 127.0.0.1:%d\n", globalProxyType, cfg.LocalSOCKSPort)

	if cfg.FrontDomain != "" {
		fmt.Printf("[*] Front Domain: %s\n", cfg.FrontDomain)
	}

	// Establish direct WebSocket connection
	conn, err := tunnel.EstablishWSTunnel(
		cfg.TargetHost,
		cfg.TargetPort,
		cfg.TargetHost,
		cfg.TargetPort,
		cfg.PayloadTemplate,
		cfg.FrontDomain,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Error establishing direct tunnel: %v", err)
	}
	defer conn.Close()

	fmt.Printf("[*] Direct connection established to target %s:%s\n", cfg.TargetHost, cfg.TargetPort)

	startSSHConnection(conn, cfg, globalProxyType, profileConfig.Timeout)
}

// startSSHConnection is a common function to start SSH connection and handle signals
func startSSHConnection(conn net.Conn, cfg *tunnel.Config, globalProxyType string, timeout int) {
	// Create a channel for the SSH connection result
	type sshResult struct {
		conn *tunnel.SSHOverWebSocket
		err  error
	}
	sshResultChan := make(chan sshResult, 1)

	// Start SSH connection in a goroutine with timeout
	go func() {
		var sshConn *tunnel.SSHOverWebSocket
		var err error

		if globalProxyType == "http" {
			sshConn, err = tunnel.ConnectViaWSAndStartHTTP(
				conn,
				cfg.SSHUsername,
				cfg.SSHPassword,
				cfg.SSHPort,
				cfg.LocalSOCKSPort,
			)
		} else {
			sshConn, err = tunnel.ConnectViaWSAndStartSOCKS(
				conn,
				cfg.SSHUsername,
				cfg.SSHPassword,
				cfg.SSHPort,
				cfg.LocalSOCKSPort,
			)
		}
		sshResultChan <- sshResult{conn: sshConn, err: err}
	}()

	// Wait for SSH connection with timeout
	var sshConn *tunnel.SSHOverWebSocket
	select {
	case result := <-sshResultChan:
		if result.err != nil {
			log.Fatalf("Error starting SSH connection: %v", result.err)
		}
		sshConn = result.conn
		fmt.Printf("[+] %s proxy up on 127.0.0.1:%d\n", globalProxyType, cfg.LocalSOCKSPort)
		fmt.Printf("[+] All traffic through that proxy is forwarded over SSH via WS tunnel.\n")

		if globalProxyType == "socks5" {
			fmt.Printf("[+] Configure your applications to use SOCKS5 proxy 127.0.0.1:%d\n", cfg.LocalSOCKSPort)
		} else {
			fmt.Printf("[+] Configure your applications to use HTTP proxy 127.0.0.1:%d\n", cfg.LocalSOCKSPort)
		}

	case <-time.After(30 * time.Second):
		fmt.Println("[!] SSH connection timed out after 30 seconds")
		conn.Close()
		log.Fatal("SSH connection establishment timed out")
	}

	defer sshConn.Close()

	// Wait for interrupt signal or timeout
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Keep running until interrupt or timeout
	timeoutDuration := time.Duration(timeout) * time.Second
	if timeout <= 0 {
		timeoutDuration = time.Hour * 24 * 365 // Effectively forever
	}

	select {
	case <-sigChan:
		fmt.Println("\n[*] Shutting down (interrupt signal received).")
	case <-time.After(timeoutDuration):
		fmt.Println("\n[*] Shutting down (timeout reached).")
	}
}
