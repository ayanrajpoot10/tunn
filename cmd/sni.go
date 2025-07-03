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

var sniFlags struct {
	frontDomain string
	proxyHost   string
	proxyPort   string
	targetHost  string
	targetPort  string
	sshUsername string
	sshPassword string
	sshPort     string
	payload     string
	localPort   int
	timeout     int
}

// sniCmd represents the sni mode command
var sniCmd = &cobra.Command{
	Use:   "sni",
	Short: "Start a tunnel using SNI fronting strategy",
	Long: `Start a tunnel connection using SNI fronting strategy.

This mode uses SNI (Server Name Indication) fronting to establish a connection
through a proxy server with a forged SNI header, then creates a WebSocket tunnel
to the target host.

The local proxy type can be controlled with the global --proxy-type flag:
  --proxy-type socks5  : Start a SOCKS5 proxy (default, works with all protocols)
  --proxy-type http    : Start an HTTP proxy (works with HTTP/HTTPS traffic)

Configuration Methods:
  1. Command line flags (this command)
  2. Configuration file with profile (use: tunn --config config.json --profile myprofile)

Example usage:
  # Basic SNI fronting with SOCKS5 proxy (default)
  tunn sni --front-domain google.com --proxy-host proxy.example.com --target-host target.example.com --ssh-username user --ssh-password pass
  
  # Using configuration file
  tunn --config config.json`,
	Run: runSNITunnel,
}

func runSNITunnel(cmd *cobra.Command, args []string) {
	// Check if using config file
	if configFile != "" {
		runWithConfigSNI()
		return
	}

	if verbose {
		fmt.Println("Starting SNI fronted tunnel...")
	}

	// Validate proxy type
	globalProxyType := GetProxyType()
	if globalProxyType != "socks5" && globalProxyType != "http" {
		log.Fatal("Error: --proxy-type must be either 'socks5' or 'http'")
	}

	// Validate required flags when not using profile
	if sniFlags.frontDomain == "" {
		log.Fatal("Error: --front-domain is required (or use --config and --profile)")
	}
	if sniFlags.proxyHost == "" {
		log.Fatal("Error: --proxy-host is required (or use --config and --profile)")
	}
	if sniFlags.targetHost == "" {
		log.Fatal("Error: --target-host is required (or use --config and --profile)")
	}
	if sniFlags.sshUsername == "" {
		log.Fatal("Error: --ssh-username is required (or use --config and --profile)")
	}
	if sniFlags.sshPassword == "" {
		log.Fatal("Error: --ssh-password is required (or use --config and --profile)")
	}

	// Set defaults
	if sniFlags.proxyPort == "" {
		sniFlags.proxyPort = "443" // SNI fronting typically uses HTTPS
	}
	if sniFlags.targetPort == "" {
		sniFlags.targetPort = "443"
	}
	if sniFlags.sshPort == "" {
		sniFlags.sshPort = "22"
	}
	if sniFlags.payload == "" {
		sniFlags.payload = fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\n\r\n", sniFlags.frontDomain)
	}
	if sniFlags.localPort == 0 {
		if globalProxyType == "http" {
			sniFlags.localPort = 8080
		} else {
			sniFlags.localPort = 1080
		}
	}

	// Create configuration
	cfg := &tunnel.Config{
		Mode:        "sni",
		FrontDomain: sniFlags.frontDomain,
		LocalPort:   sniFlags.localPort,
		ProxyHost:   sniFlags.proxyHost,
		ProxyPort:   sniFlags.proxyPort,
		TargetHost:  sniFlags.targetHost,
		TargetPort:  sniFlags.targetPort,
		SSH: tunnel.SSHConfig{
			Username: sniFlags.sshUsername,
			Password: sniFlags.sshPassword,
			Port:     sniFlags.sshPort,
		},
		Payload: sniFlags.payload,
	}

	if verbose {
		printSNIConfig(cfg)
	}

	// Establish SNI fronted tunnel
	// 1. Build a TLS socket to PROXY_HOST with forged SNI
	address := net.JoinHostPort(cfg.ProxyHost, cfg.ProxyPort)
	rawConn, err := net.Dial("tcp", address)
	if err != nil {
		log.Fatalf("Failed to connect to proxy: %v", err)
	}

	// 2. Configure TLS with fake SNI
	tlsConfig := &tls.Config{
		ServerName:         cfg.FrontDomain, // Fake SNI
		InsecureSkipVerify: true,
	}

	tlsConn := tls.Client(rawConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		rawConn.Close()
		log.Fatalf("TLS handshake failed: %v", err)
	}

	// 3. Use the TLS connection to establish WS tunnel to TARGET_HOST:TARGET_PORT
	conn, err := tunnel.EstablishWSTunnel(
		"", "", // proxy host/port not needed, we already have the connection
		cfg.TargetHost,
		cfg.TargetPort,
		cfg.Payload,
		"",      // no front domain spoofing for SNI fronted (using TLS SNI instead)
		true,    // use_tls
		tlsConn, // existing TLS connection
	)
	if err != nil {
		log.Fatalf("Error establishing SNI tunnel: %v", err)
	}
	defer conn.Close()

	// Start SSH connection and local proxy
	var sshConn *tunnel.SSHOverWebSocket

	if globalProxyType == "http" {
		sshConn, err = tunnel.ConnectViaWSAndStartHTTP(
			conn,
			cfg.SSH.Username,
			cfg.SSH.Password,
			cfg.SSH.Port,
			cfg.LocalPort,
		)
	} else {
		sshConn, err = tunnel.ConnectViaWSAndStartSOCKS(
			conn,
			cfg.SSH.Username,
			cfg.SSH.Password,
			cfg.SSH.Port,
			cfg.LocalPort,
		)
	}
	if err != nil {
		log.Fatalf("Error starting SSH connection: %v", err)
	}
	defer sshConn.Close()

	fmt.Printf("[+] %s proxy up on 127.0.0.1:%d\n", globalProxyType, cfg.LocalPort)
	fmt.Printf("[+] All traffic through that proxy is forwarded over SSH via WS tunnel.\n")

	if globalProxyType == "socks5" {
		fmt.Printf("[+] Configure your applications to use SOCKS5 proxy 127.0.0.1:%d\n", cfg.LocalPort)
	} else {
		fmt.Printf("[+] Configure your applications to use HTTP proxy 127.0.0.1:%d\n", cfg.LocalPort)
	}

	// Wait for interrupt signal or timeout
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Keep running until interrupt or timeout
	timeoutDuration := time.Duration(sniFlags.timeout) * time.Second
	if sniFlags.timeout <= 0 {
		timeoutDuration = time.Hour * 24 * 365 // Effectively forever
	}

	select {
	case <-sigChan:
		fmt.Println("\n[*] Shutting down (interrupt signal received).")
	case <-time.After(timeoutDuration):
		fmt.Println("\n[*] Shutting down (timeout reached).")
	}
}

// runWithConfigSNI executes SNI tunnel using configuration file
func runWithConfigSNI() {
	configMgr := GetConfigManager()
	if configMgr == nil {
		log.Fatal("Error: Configuration manager not initialized")
	}

	config := configMgr.GetConfig()
	if config == nil {
		log.Fatal("Error: No configuration loaded")
	}

	// Validate mode matches
	if config.Mode != "sni" {
		log.Fatalf("Error: Configuration is set for mode '%s', but sni mode was requested", config.Mode)
	}

	// Override with CLI proxy type if specified
	globalProxyType := GetProxyType()
	if globalProxyType != "socks5" && globalProxyType != "http" {
		log.Fatal("Error: --proxy-type must be either 'socks5' or 'http'")
	}

	fmt.Printf("[*] Starting tunnel using %s strategy with %s local proxy\n", config.Mode, globalProxyType)

	// Execute the tunnel with the configuration
	runSNITunnelWithConfig(config, globalProxyType)
}

// runSNITunnelWithConfig executes the SNI tunnel with the given configuration
func runSNITunnelWithConfig(config *tunnel.Config, globalProxyType string) {
	fmt.Printf("[*] Front Domain: %s\n", config.FrontDomain)
	fmt.Printf("[*] Proxy: %s:%s\n", config.ProxyHost, config.ProxyPort)
	fmt.Printf("[*] Target: %s:%s\n", config.TargetHost, config.TargetPort)
	fmt.Printf("[*] SSH User: %s\n", config.SSH.Username)
	fmt.Printf("[*] Local %s proxy will be available on 127.0.0.1:%d\n", globalProxyType, config.LocalPort)

	if verbose {
		printSNIConfig(config)
	}

	// Establish SNI fronted tunnel
	fmt.Printf("[*] Establishing TLS connection to %s:%s with SNI %s...\n",
		config.ProxyHost, config.ProxyPort, config.FrontDomain)

	// 1. Make raw TCP connection to proxy
	address := fmt.Sprintf("%s:%s", config.ProxyHost, config.ProxyPort)
	rawConn, err := net.Dial("tcp", address)
	if err != nil {
		log.Fatalf("Failed to connect to proxy: %v", err)
	}

	// 2. Establish TLS with SNI set to front domain
	tlsConfig := &tls.Config{
		ServerName: config.FrontDomain, // This is the key for SNI fronting
	}

	tlsConn := tls.Client(rawConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		rawConn.Close()
		log.Fatalf("TLS handshake failed: %v", err)
	}

	// 3. Use the TLS connection to establish WS tunnel to TARGET_HOST:TARGET_PORT
	conn, err := tunnel.EstablishWSTunnel(
		"", "", // proxy host/port not needed, we already have the connection
		config.TargetHost,
		config.TargetPort,
		config.Payload,
		"",      // no front domain spoofing for SNI fronted (using TLS SNI instead)
		true,    // use_tls
		tlsConn, // existing TLS connection
	)
	if err != nil {
		log.Fatalf("Error establishing SNI fronted tunnel: %v", err)
	}
	defer conn.Close()

	fmt.Printf("[*] Connected through SNI fronting, tunneling to target %s:%s\n",
		config.TargetHost, config.TargetPort)

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
				config.SSH.Username,
				config.SSH.Password,
				config.SSH.Port,
				config.LocalPort,
			)
		} else {
			sshConn, err = tunnel.ConnectViaWSAndStartSOCKS(
				conn,
				config.SSH.Username,
				config.SSH.Password,
				config.SSH.Port,
				config.LocalPort,
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
		fmt.Printf("[+] %s proxy up on 127.0.0.1:%d\n", globalProxyType, config.LocalPort)

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

// printSNIConfig prints the SNI configuration for debugging
func printSNIConfig(config *tunnel.Config) {
	fmt.Println("SNI Fronting Configuration:")
	fmt.Printf("  Front Domain: %s (used for SNI)\n", config.FrontDomain)
	fmt.Printf("  Proxy Host: %s\n", config.ProxyHost)
	fmt.Printf("  Proxy Port: %s\n", config.ProxyPort)
	fmt.Printf("  Target Host: %s\n", config.TargetHost)
	fmt.Printf("  Target Port: %s\n", config.TargetPort)
	fmt.Printf("  Local SOCKS Port: %d\n", config.LocalPort)
	fmt.Printf("  SSH Username: %s\n", config.SSH.Username)
	fmt.Printf("  SSH Port: %s\n", config.SSH.Port)
	fmt.Printf("  Payload Template: %s\n", config.Payload)
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(sniCmd)

	// SNI fronting specific flags
	sniCmd.Flags().StringVar(&sniFlags.frontDomain, "front-domain", "", "front domain for SNI fronting (required unless using config)")

	// Network configuration flags
	sniCmd.Flags().StringVar(&sniFlags.proxyHost, "proxy-host", "", "proxy host (required unless using config)")
	sniCmd.Flags().StringVar(&sniFlags.proxyPort, "proxy-port", "443", "proxy port (usually 443 for SNI fronting)")
	sniCmd.Flags().StringVar(&sniFlags.targetHost, "target-host", "", "target host (required unless using config)")
	sniCmd.Flags().StringVar(&sniFlags.targetPort, "target-port", "443", "target port")

	// SSH configuration flags
	sniCmd.Flags().StringVarP(&sniFlags.sshUsername, "ssh-username", "u", "", "SSH username (required unless using config)")
	sniCmd.Flags().StringVarP(&sniFlags.sshPassword, "ssh-password", "p", "", "SSH password (required unless using config)")
	sniCmd.Flags().StringVar(&sniFlags.sshPort, "ssh-port", "22", "SSH port")

	// Advanced options
	sniCmd.Flags().StringVar(&sniFlags.payload, "payload", "", "custom HTTP payload template")
	sniCmd.Flags().IntVarP(&sniFlags.localPort, "local-port", "l", 1080, "local SOCKS proxy port")
	sniCmd.Flags().IntVarP(&sniFlags.timeout, "timeout", "t", 0, "connection timeout in seconds (0 = no timeout)")

}
