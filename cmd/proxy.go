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

Example usage:
  tunn proxy --proxy-host proxy.example.com --target-host target.example.com
  tunn proxy --proxy-host proxy.example.com --proxy-port 8080 --target-host ssh-server.com --ssh-username user
  tunn proxy --proxy-host proxy.example.com --target-host ssh-server.com --front-domain allowed-domain.com --ssh-username user`,
	Run: runProxyTunnel,
}

func runProxyTunnel(cmd *cobra.Command, args []string) {
	if verbose {
		fmt.Println("Starting HTTP proxy tunnel...")
	}

	// Create configuration with HTTP proxy specific defaults
	cfg := &tunnel.Config{
		Mode:            "proxy",
		LocalSOCKSPort:  1080,
		ProxyHost:       "us1.ws-tun.me",
		ProxyPort:       "80",
		TargetHost:      "us1.ws-tun.me",
		TargetPort:      "80",
		FrontDomain:     "",
		SSHUsername:     "sshstores-ayan10",
		SSHPassword:     "1234",
		SSHPort:         "22",
		PayloadTemplate: "GET / HTTP/1.1[crlf]Host: [host][crlf]Upgrade: websocket[crlf][crlf]",
	}

	// Override with command line flags
	if proxyFlags.localPort != 0 {
		cfg.LocalSOCKSPort = proxyFlags.localPort
	}
	if proxyFlags.proxyHost != "" {
		cfg.ProxyHost = proxyFlags.proxyHost
	}
	if proxyFlags.proxyPort != "" {
		cfg.ProxyPort = proxyFlags.proxyPort
	}
	if proxyFlags.targetHost != "" {
		cfg.TargetHost = proxyFlags.targetHost
	}
	if proxyFlags.targetPort != "" {
		cfg.TargetPort = proxyFlags.targetPort
	}
	if proxyFlags.frontDomain != "" {
		cfg.FrontDomain = proxyFlags.frontDomain
	}
	if proxyFlags.sshUsername != "" {
		cfg.SSHUsername = proxyFlags.sshUsername
	}
	if proxyFlags.sshPassword != "" {
		cfg.SSHPassword = proxyFlags.sshPassword
	}
	if proxyFlags.sshPort != "" {
		cfg.SSHPort = proxyFlags.sshPort
	}
	if proxyFlags.payload != "" {
		cfg.PayloadTemplate = proxyFlags.payload
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

	fmt.Println("[*] Starting SSH connection and SOCKS proxy...")

	// Create a channel for the SSH connection result
	type sshResult struct {
		conn *tunnel.SSHOverWebSocket
		err  error
	}
	sshResultChan := make(chan sshResult, 1)

	// Start SSH connection in a goroutine with timeout
	go func() {
		sshConn, err := tunnel.ConnectViaWSAndStartSOCKS(
			conn, // Use proxy tunnel connection for SSH
			cfg.SSHUsername,
			cfg.SSHPassword,
			cfg.SSHPort,
			cfg.LocalSOCKSPort,
		)
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
		fmt.Printf("[+] SOCKS proxy up on 127.0.0.1:%d\n", cfg.LocalSOCKSPort)
		fmt.Println("[+] All traffic through that proxy is forwarded over SSH via WS tunnel.")

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
