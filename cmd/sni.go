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

Example usage:
  # Basic SNI fronting with SOCKS5 proxy (default)
  tunn sni --front-domain google.com --proxy-host proxy.example.com --target-host target.example.com --ssh-username user --ssh-password pass
  
  # SNI fronting with HTTP proxy
  tunn --proxy-type http sni --front-domain cloudflare.com --proxy-host proxy.example.com --ssh-username user --ssh-password pass`,
	Run: runSNITunnel,
}

func runSNITunnel(cmd *cobra.Command, args []string) {
	if verbose {
		fmt.Println("Starting SNI fronted tunnel...")
	}

	// Validate proxy type
	globalProxyType := GetProxyType()
	if globalProxyType != "socks5" && globalProxyType != "http" {
		log.Fatal("Error: --proxy-type must be either 'socks5' or 'http'")
	}

	// Create configuration with SNI fronted specific defaults
	cfg := &tunnel.Config{
		Mode:            "sni",
		FrontDomain:     "config.rcs.mnc840.mcc405.pub.3gppnetwork.org",
		LocalSOCKSPort:  1080,
		ProxyHost:       "us1.ws-tun.me",
		ProxyPort:       "443", // SNI fronting typically uses HTTPS
		TargetHost:      "us1.ws-tun.me",
		TargetPort:      "80",
		SSHUsername:     "sshstores-ayan10",
		SSHPassword:     "1234",
		SSHPort:         "22",
		PayloadTemplate: "GET / HTTP/1.1[crlf]Host: [host][crlf]Upgrade: websocket[crlf][crlf]",
	}

	// Override with command line flags
	if sniFlags.frontDomain != "" {
		cfg.FrontDomain = sniFlags.frontDomain
	}
	if sniFlags.localPort != 0 {
		cfg.LocalSOCKSPort = sniFlags.localPort
	}
	if sniFlags.proxyHost != "" {
		cfg.ProxyHost = sniFlags.proxyHost
	}
	if sniFlags.proxyPort != "" {
		cfg.ProxyPort = sniFlags.proxyPort
	}
	if sniFlags.targetHost != "" {
		cfg.TargetHost = sniFlags.targetHost
	}
	if sniFlags.targetPort != "" {
		cfg.TargetPort = sniFlags.targetPort
	}
	if sniFlags.sshUsername != "" {
		cfg.SSHUsername = sniFlags.sshUsername
	}
	if sniFlags.sshPassword != "" {
		cfg.SSHPassword = sniFlags.sshPassword
	}
	if sniFlags.sshPort != "" {
		cfg.SSHPort = sniFlags.sshPort
	}
	if sniFlags.payload != "" {
		cfg.PayloadTemplate = sniFlags.payload
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
		cfg.PayloadTemplate,
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
	fmt.Printf("[+] All traffic through that proxy is forwarded over SSH via WS tunnel.\n")

	if globalProxyType == "socks5" {
		fmt.Printf("[+] Configure your applications to use SOCKS5 proxy 127.0.0.1:%d\n", cfg.LocalSOCKSPort)
	} else {
		fmt.Printf("[+] Configure your applications to use HTTP proxy 127.0.0.1:%d\n", cfg.LocalSOCKSPort)
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

func printSNIConfig(cfg *tunnel.Config) {
	fmt.Println("SNI Fronted Tunnel Configuration:")
	fmt.Printf("  Front Domain: %s\n", cfg.FrontDomain)
	fmt.Printf("  Local SOCKS Port: %d\n", cfg.LocalSOCKSPort)
	fmt.Printf("  Proxy Host: %s\n", cfg.ProxyHost)
	fmt.Printf("  Proxy Port: %s\n", cfg.ProxyPort)
	fmt.Printf("  Target Host: %s\n", cfg.TargetHost)
	fmt.Printf("  Target Port: %s\n", cfg.TargetPort)
	fmt.Printf("  SSH Username: %s\n", cfg.SSHUsername)
	fmt.Printf("  SSH Port: %s\n", cfg.SSHPort)
	fmt.Printf("  Payload Template: %s\n", cfg.PayloadTemplate)
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(sniCmd)

	// SNI fronting specific flags
	sniCmd.Flags().StringVar(&sniFlags.frontDomain, "front-domain", "", "front domain for SNI fronting (required)")

	// Network configuration flags
	sniCmd.Flags().StringVar(&sniFlags.proxyHost, "proxy-host", "", "proxy host (required)")
	sniCmd.Flags().StringVar(&sniFlags.proxyPort, "proxy-port", "443", "proxy port (usually 443 for SNI fronting)")
	sniCmd.Flags().StringVar(&sniFlags.targetHost, "target-host", "", "target host (required)")
	sniCmd.Flags().StringVar(&sniFlags.targetPort, "target-port", "80", "target port")

	// SSH configuration flags
	sniCmd.Flags().StringVarP(&sniFlags.sshUsername, "ssh-username", "u", "", "SSH username (required)")
	sniCmd.Flags().StringVarP(&sniFlags.sshPassword, "ssh-password", "p", "", "SSH password (required)")
	sniCmd.Flags().StringVar(&sniFlags.sshPort, "ssh-port", "22", "SSH port")

	// Advanced options
	sniCmd.Flags().StringVar(&sniFlags.payload, "payload", "", "custom HTTP payload template")
	sniCmd.Flags().IntVarP(&sniFlags.localPort, "local-port", "l", 1080, "local SOCKS proxy port")
	sniCmd.Flags().IntVarP(&sniFlags.timeout, "timeout", "t", 0, "connection timeout in seconds (0 = no timeout)")

	// Mark required flags
	sniCmd.MarkFlagRequired("front-domain")
	sniCmd.MarkFlagRequired("proxy-host")
	sniCmd.MarkFlagRequired("target-host")
	sniCmd.MarkFlagRequired("ssh-username")
	sniCmd.MarkFlagRequired("ssh-password")
}
