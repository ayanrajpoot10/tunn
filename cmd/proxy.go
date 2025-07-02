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

var proxyFlags struct {
	proxyHost   string
	proxyPort   string
	targetHost  string
	targetPort  string
	frontDomain string
	sshUsername string
	sshPassword string
	sshPort     string
	payload     string
	localPort   int
	timeout     int
}

// proxyCmd represents the proxy mode command
var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Start a tunnel using HTTP proxy strategy",
	Long: `Start a tunnel connection using HTTP proxy tunneling strategy.

This mode connects to a proxy server first, sends an HTTP payload to it,
and then establishes a WebSocket tunnel through the proxy to reach the target host for SSH.

You can optionally specify a front domain to spoof the Host header in the HTTP request,
which can help bypass certain proxy restrictions or filters.

The local proxy type can be controlled with the global --proxy-type flag:
  --proxy-type socks5  : Start a SOCKS5 proxy (default, works with all protocols)
  --proxy-type http    : Start an HTTP proxy (works with HTTP/HTTPS traffic)

Example usage:
  # Basic proxy mode with SOCKS5 local proxy (default)
  tunn proxy --proxy-host proxy.example.com --target-host target.example.com --ssh-username user --ssh-password pass
  
  # Proxy mode with HTTP local proxy
  tunn --proxy-type http proxy --proxy-host proxy.example.com --target-host ssh-server.com --ssh-username user --ssh-password pass
  
  # With custom configuration
  tunn --proxy-type socks5 proxy --proxy-host proxy.example.com --proxy-port 8080 --target-host ssh-server.com --front-domain allowed-domain.com --ssh-username user --ssh-password pass`,
	Run: runProxyTunnel,
}

func runProxyTunnel(cmd *cobra.Command, args []string) {
	if verbose {
		fmt.Println("[*] Running in verbose mode")
	}

	// Validate proxy type
	globalProxyType := GetProxyType()
	if globalProxyType != "socks5" && globalProxyType != "http" {
		log.Fatal("Error: --proxy-type must be either 'socks5' or 'http'")
	}

	// Validate required flags
	if proxyFlags.proxyHost == "" {
		log.Fatal("Error: --proxy-host is required")
	}
	if proxyFlags.targetHost == "" {
		log.Fatal("Error: --target-host is required")
	}
	if proxyFlags.sshUsername == "" {
		log.Fatal("Error: --ssh-username is required")
	}
	if proxyFlags.sshPassword == "" {
		log.Fatal("Error: --ssh-password is required")
	}

	// Set default values
	if proxyFlags.proxyPort == "" {
		proxyFlags.proxyPort = "80"
	}
	if proxyFlags.targetPort == "" {
		proxyFlags.targetPort = "80"
	}
	if proxyFlags.sshPort == "" {
		proxyFlags.sshPort = "22"
	}
	if proxyFlags.payload == "" {
		proxyFlags.payload = "GET / HTTP/1.1[crlf]Host: [host][crlf]Upgrade: websocket[crlf][crlf]"
	}
	if proxyFlags.localPort == 0 {
		if globalProxyType == "http" {
			proxyFlags.localPort = 8080
		} else {
			proxyFlags.localPort = 1080
		}
	}

	// Create configuration
	cfg := &tunnel.Config{
		Mode:            "proxy",
		LocalSOCKSPort:  proxyFlags.localPort,
		ProxyHost:       proxyFlags.proxyHost,
		ProxyPort:       proxyFlags.proxyPort,
		TargetHost:      proxyFlags.targetHost,
		TargetPort:      proxyFlags.targetPort,
		FrontDomain:     proxyFlags.frontDomain,
		SSHUsername:     proxyFlags.sshUsername,
		SSHPassword:     proxyFlags.sshPassword,
		SSHPort:         proxyFlags.sshPort,
		PayloadTemplate: proxyFlags.payload,
	}

	fmt.Printf("[*] Starting tunnel using proxy strategy with %s local proxy\n", globalProxyType)
	fmt.Printf("[*] Proxy: %s:%s\n", cfg.ProxyHost, cfg.ProxyPort)
	fmt.Printf("[*] Target: %s:%s\n", cfg.TargetHost, cfg.TargetPort)
	fmt.Printf("[*] SSH User: %s\n", cfg.SSHUsername)
	fmt.Printf("[*] Local %s proxy will be available on 127.0.0.1:%d\n", globalProxyType, cfg.LocalSOCKSPort)

	if cfg.FrontDomain != "" {
		fmt.Printf("[*] Front Domain: %s\n", cfg.FrontDomain)
	}

	if verbose {
		printProxyConfig(cfg)
	}

	// Establish HTTP proxy tunnel
	// Step 1: Connect to proxy-host and send payload
	// Step 2: Use the proxy connection to reach target-host for SSH
	conn, err := tunnel.EstablishWSTunnel(
		cfg.ProxyHost,  // Connect to proxy first
		cfg.ProxyPort,  // Proxy port
		cfg.TargetHost, // Target host (where we want to tunnel to)
		cfg.TargetPort, // Target port (where SSH server is)
		cfg.PayloadTemplate,
		cfg.FrontDomain, // front domain for Host header spoofing
		false,           // use_tls
		nil,             // sock
	)
	if err != nil {
		log.Fatalf("Error establishing proxy tunnel to target: %v", err)
	}
	defer conn.Close()

	fmt.Printf("[*] Connected to proxy %s:%s, tunneling to target %s:%s\n",
		cfg.ProxyHost, cfg.ProxyPort, cfg.TargetHost, cfg.TargetPort)

	fmt.Printf("[*] Starting SSH connection and %s proxy...\n", globalProxyType)

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
				conn, // Use proxy tunnel connection for SSH
				cfg.SSHUsername,
				cfg.SSHPassword,
				cfg.SSHPort,
				cfg.LocalSOCKSPort,
			)
		} else {
			sshConn, err = tunnel.ConnectViaWSAndStartSOCKS(
				conn, // Use proxy tunnel connection for SSH
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
	timeoutDuration := time.Duration(proxyFlags.timeout) * time.Second
	if proxyFlags.timeout <= 0 {
		timeoutDuration = time.Hour * 24 * 365 // Effectively forever
	}

	select {
	case <-sigChan:
		fmt.Println("\n[*] Shutting down (interrupt signal received).")
	case <-time.After(timeoutDuration):
		fmt.Println("\n[*] Shutting down (timeout reached).")
	}
}

func printProxyConfig(cfg *tunnel.Config) {
	fmt.Println("HTTP Proxy Tunnel Configuration:")
	fmt.Printf("  Proxy Host: %s (connect here first)\n", cfg.ProxyHost)
	fmt.Printf("  Proxy Port: %s\n", cfg.ProxyPort)
	fmt.Printf("  Target Host: %s (SSH server through proxy)\n", cfg.TargetHost)
	fmt.Printf("  Target Port: %s\n", cfg.TargetPort)
	if cfg.FrontDomain != "" {
		fmt.Printf("  Front Domain: %s (used for Host header spoofing)\n", cfg.FrontDomain)
	}
	fmt.Printf("  Local SOCKS Port: %d\n", cfg.LocalSOCKSPort)
	fmt.Printf("  SSH Username: %s\n", cfg.SSHUsername)
	fmt.Printf("  SSH Port: %s\n", cfg.SSHPort)
	fmt.Printf("  Payload Template: %s\n", cfg.PayloadTemplate)
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(proxyCmd)

	// Network configuration flags
	proxyCmd.Flags().StringVar(&proxyFlags.proxyHost, "proxy-host", "", "proxy server to connect to first (required)")
	proxyCmd.Flags().StringVar(&proxyFlags.proxyPort, "proxy-port", "80", "proxy server port")
	proxyCmd.Flags().StringVar(&proxyFlags.targetHost, "target-host", "", "target SSH server to reach through proxy (required)")
	proxyCmd.Flags().StringVar(&proxyFlags.targetPort, "target-port", "80", "target server port (for WebSocket, not SSH)")
	proxyCmd.Flags().StringVar(&proxyFlags.frontDomain, "front-domain", "", "front domain for Host header spoofing (optional)")

	// SSH configuration flags
	proxyCmd.Flags().StringVarP(&proxyFlags.sshUsername, "ssh-username", "u", "", "SSH username for target server (required)")
	proxyCmd.Flags().StringVarP(&proxyFlags.sshPassword, "ssh-password", "p", "", "SSH password for target server (required)")
	proxyCmd.Flags().StringVar(&proxyFlags.sshPort, "ssh-port", "22", "SSH port on target server")

	// Advanced options
	proxyCmd.Flags().StringVar(&proxyFlags.payload, "payload", "", "custom HTTP payload template")
	proxyCmd.Flags().IntVarP(&proxyFlags.localPort, "local-port", "l", 1080, "local SOCKS proxy port")
	proxyCmd.Flags().IntVarP(&proxyFlags.timeout, "timeout", "t", 0, "connection timeout in seconds (0 = no timeout)")

	// Mark required flags
	proxyCmd.MarkFlagRequired("proxy-host")
	proxyCmd.MarkFlagRequired("target-host")
	proxyCmd.MarkFlagRequired("ssh-username")
	proxyCmd.MarkFlagRequired("ssh-password")
}
