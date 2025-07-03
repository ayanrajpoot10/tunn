package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tunn/internal/tunnel"

	"github.com/spf13/cobra"
)

var directFlags struct {
	frontDomain string
	targetHost  string
	targetPort  string
	sshUsername string
	sshPassword string
	sshPort     string
	payload     string
	localPort   int
	timeout     int
}

// directCmd represents the direct mode command
var directCmd = &cobra.Command{
	Use:   "direct",
	Short: "Start a tunnel using direct connection strategy with front domain",
	Long: `Start a tunnel connection using direct connection strategy with front domain host spoofing.

This mode establishes a direct connection to the target host and uses
a front domain in the Host header to bypass filtering, creating a WebSocket 
connection directly to the target.

The local proxy type can be controlled with the global --proxy-type flag:
  --proxy-type socks5  : Start a SOCKS5 proxy (default, works with all protocols)
  --proxy-type http    : Start an HTTP proxy (works with HTTP/HTTPS traffic)

Example usage:
  # Basic direct connection with SOCKS5 proxy (default)
  tunn direct --front-domain google.com --target-host target.example.com --ssh-username user --ssh-password pass
  
  # Direct connection with HTTP proxy
  tunn --proxy-type http direct --front-domain cloudflare.com --target-host target.example.com --target-port 443 --ssh-username user --ssh-password pass`,
	Run: runDirectTunnel,
}

func runDirectTunnel(cmd *cobra.Command, args []string) {
	// Check if using config file and profile
	if configFile != "" && profile != "" {
		runWithProfileDirect("direct")
		return
	}

	if verbose {
		fmt.Println("Starting direct tunnel...")
	}

	// Validate proxy type
	globalProxyType := GetProxyType()
	if globalProxyType != "socks5" && globalProxyType != "http" {
		log.Fatal("Error: --proxy-type must be either 'socks5' or 'http'")
	}

	// Validate required flags when not using profile
	if directFlags.targetHost == "" {
		log.Fatal("Error: --target-host is required (or use --config and --profile)")
	}
	if directFlags.sshUsername == "" {
		log.Fatal("Error: --ssh-username is required (or use --config and --profile)")
	}
	if directFlags.sshPassword == "" {
		log.Fatal("Error: --ssh-password is required (or use --config and --profile)")
	}

	// Set defaults
	if directFlags.targetPort == "" {
		directFlags.targetPort = "443"
	}
	if directFlags.sshPort == "" {
		directFlags.sshPort = "22"
	}
	if directFlags.payload == "" {
		if directFlags.frontDomain != "" {
			directFlags.payload = fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\n\r\n", directFlags.frontDomain)
		} else {
			directFlags.payload = fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\n\r\n", directFlags.targetHost)
		}
	}
	if directFlags.localPort == 0 {
		if globalProxyType == "http" {
			directFlags.localPort = 8080
		} else {
			directFlags.localPort = 1080
		}
	}

	// Create configuration
	cfg := &tunnel.Config{
		Mode:            "direct",
		FrontDomain:     directFlags.frontDomain,
		LocalSOCKSPort:  directFlags.localPort,
		TargetHost:      directFlags.targetHost,
		TargetPort:      directFlags.targetPort,
		SSHUsername:     directFlags.sshUsername,
		SSHPassword:     directFlags.sshPassword,
		SSHPort:         directFlags.sshPort,
		PayloadTemplate: directFlags.payload,
	}

	fmt.Printf("[*] Starting tunnel using direct strategy with %s local proxy\n", globalProxyType)
	fmt.Printf("[*] Target: %s:%s\n", cfg.TargetHost, cfg.TargetPort)
	if cfg.FrontDomain != "" {
		fmt.Printf("[*] Front Domain: %s\n", cfg.FrontDomain)
	}
	fmt.Printf("[*] SSH User: %s\n", cfg.SSHUsername)
	fmt.Printf("[*] Local %s proxy will be available on 127.0.0.1:%d\n", globalProxyType, cfg.LocalSOCKSPort)

	if verbose {
		printDirectConfig(cfg)
	}

	// Establish direct tunnel
	conn, err := tunnel.EstablishWSTunnel(
		"", "", // No proxy for direct mode
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

	fmt.Printf("[*] Connected directly to target %s:%s\n", cfg.TargetHost, cfg.TargetPort)
	fmt.Printf("[*] Starting SSH connection and %s proxy...\n", globalProxyType)

	// Start SSH connection
	var sshConn *tunnel.SSHOverWebSocket
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

	if err != nil {
		log.Fatalf("Error starting SSH connection: %v", err)
	}
	defer sshConn.Close()

	fmt.Printf("[+] %s proxy up on 127.0.0.1:%d\n", globalProxyType, cfg.LocalSOCKSPort)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		fmt.Println("\n[*] Shutting down (interrupt signal received).")
	}
}

// runWithProfileDirect executes direct tunnel using configuration profile
func runWithProfileDirect(expectedMode string) {
	configMgr := GetConfigManager()
	if configMgr == nil {
		log.Fatal("Error: Configuration manager not initialized")
	}

	profileConfig, err := configMgr.GetProfile(profile)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Validate mode matches
	if profileConfig.Mode != expectedMode {
		log.Fatalf("Error: Profile '%s' is configured for mode '%s', but '%s' mode was requested",
			profile, profileConfig.Mode, expectedMode)
	}

	// Convert profile to legacy config
	cfg := configMgr.ConvertProfileToLegacyConfig(profileConfig)

	// Determine proxy type: use profile setting if available, otherwise use CLI flag
	var effectiveProxyType string
	if profileConfig.ProxyType != "" {
		effectiveProxyType = profileConfig.ProxyType
	} else {
		effectiveProxyType = GetProxyType()
	}

	// Validate proxy type
	if effectiveProxyType != "socks5" && effectiveProxyType != "http" {
		log.Fatal("Error: proxy type must be either 'socks5' or 'http'")
	}

	fmt.Printf("[*] Using profile: %s\n", profileConfig.Name)
	fmt.Printf("[*] Starting tunnel using %s strategy with %s local proxy\n", cfg.Mode, effectiveProxyType)

	// Execute the tunnel with the profile configuration
	runDirectTunnelWithConfig(cfg, effectiveProxyType)
}

// runDirectTunnelWithConfig executes the direct tunnel with the given configuration
func runDirectTunnelWithConfig(cfg *tunnel.Config, globalProxyType string) {
	fmt.Printf("[*] Target: %s:%s\n", cfg.TargetHost, cfg.TargetPort)
	if cfg.FrontDomain != "" {
		fmt.Printf("[*] Front Domain: %s\n", cfg.FrontDomain)
	}
	fmt.Printf("[*] SSH User: %s\n", cfg.SSHUsername)
	fmt.Printf("[*] Local %s proxy will be available on 127.0.0.1:%d\n", globalProxyType, cfg.LocalSOCKSPort)

	if verbose {
		printDirectConfig(cfg)
	}

	// Establish direct tunnel
	conn, err := tunnel.EstablishWSTunnel(
		"", "", // No proxy for direct mode
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

	fmt.Printf("[*] Connected directly to target %s:%s\n", cfg.TargetHost, cfg.TargetPort)
	fmt.Printf("[*] Starting SSH connection and %s proxy...\n", globalProxyType)

	// Start SSH connection
	type sshResult struct {
		conn *tunnel.SSHOverWebSocket
		err  error
	}
	sshResultChan := make(chan sshResult, 1)

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

	var sshConn *tunnel.SSHOverWebSocket
	select {
	case result := <-sshResultChan:
		if result.err != nil {
			log.Fatalf("Error starting SSH connection: %v", result.err)
		}
		sshConn = result.conn
		fmt.Printf("[+] %s proxy up on 127.0.0.1:%d\n", globalProxyType, cfg.LocalSOCKSPort)

	case <-time.After(30 * time.Second):
		fmt.Println("[!] SSH connection timed out after 30 seconds")
		conn.Close()
		log.Fatal("SSH connection establishment timed out")
	}

	defer sshConn.Close()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		fmt.Println("\n[*] Shutting down (interrupt signal received).")
	}
}

// printDirectConfig prints the direct configuration for debugging
func printDirectConfig(cfg *tunnel.Config) {
	fmt.Println("Direct Connection Configuration:")
	fmt.Printf("  Target Host: %s\n", cfg.TargetHost)
	fmt.Printf("  Target Port: %s\n", cfg.TargetPort)
	if cfg.FrontDomain != "" {
		fmt.Printf("  Front Domain: %s (for Host header spoofing)\n", cfg.FrontDomain)
	}
	fmt.Printf("  Local SOCKS Port: %d\n", cfg.LocalSOCKSPort)
	fmt.Printf("  SSH Username: %s\n", cfg.SSHUsername)
	fmt.Printf("  SSH Port: %s\n", cfg.SSHPort)
	fmt.Printf("  Payload Template: %s\n", cfg.PayloadTemplate)
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(directCmd)

	// Front domain flag (unique to direct mode)
	directCmd.Flags().StringVar(&directFlags.frontDomain, "front-domain", "", "front domain for Host header spoofing (optional)")

	// Network configuration flags
	directCmd.Flags().StringVar(&directFlags.targetHost, "target-host", "", "target host (required unless using config)")
	directCmd.Flags().StringVar(&directFlags.targetPort, "target-port", "443", "target port")

	// SSH configuration flags
	directCmd.Flags().StringVarP(&directFlags.sshUsername, "ssh-username", "u", "", "SSH username (required unless using config)")
	directCmd.Flags().StringVarP(&directFlags.sshPassword, "ssh-password", "p", "", "SSH password (required unless using config)")
	directCmd.Flags().StringVar(&directFlags.sshPort, "ssh-port", "22", "SSH port")

	// Advanced options
	directCmd.Flags().StringVar(&directFlags.payload, "payload", "", "custom HTTP payload template")
	directCmd.Flags().IntVarP(&directFlags.localPort, "local-port", "l", 1080, "local SOCKS proxy port")
	directCmd.Flags().IntVarP(&directFlags.timeout, "timeout", "t", 0, "connection timeout in seconds (0 = no timeout)")

	// Note: Required flags are now validated conditionally in the run function
}
