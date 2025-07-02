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

Example usage:
  tunn direct --front-domain google.com --target-host target.example.com
  tunn direct --front-domain cloudflare.com --target-host target.example.com --target-port 443 --ssh-username user`,
	Run: runDirectTunnel,
}

func runDirectTunnel(cmd *cobra.Command, args []string) {
	if verbose {
		fmt.Println("Starting direct tunnel...")
	}

	// Create configuration with direct specific defaults
	cfg := &tunnel.Config{
		Mode:            "direct",
		FrontDomain:     "config.rcs.mnc840.mcc405.pub.3gppnetwork.org",
		LocalSOCKSPort:  1080,
		TargetHost:      "us1.ws-tun.me",
		TargetPort:      "80",
		SSHUsername:     "sshstores-ayan10",
		SSHPassword:     "1234",
		SSHPort:         "22",
		PayloadTemplate: "GET / HTTP/1.1[crlf]Host: [host][crlf]Upgrade: websocket[crlf][crlf]",
	}

	// Override with command line flags
	if directFlags.frontDomain != "" {
		cfg.FrontDomain = directFlags.frontDomain
	}
	if directFlags.localPort != 0 {
		cfg.LocalSOCKSPort = directFlags.localPort
	}
	if directFlags.targetHost != "" {
		cfg.TargetHost = directFlags.targetHost
	}
	if directFlags.targetPort != "" {
		cfg.TargetPort = directFlags.targetPort
	}
	if directFlags.sshUsername != "" {
		cfg.SSHUsername = directFlags.sshUsername
	}
	if directFlags.sshPassword != "" {
		cfg.SSHPassword = directFlags.sshPassword
	}
	if directFlags.sshPort != "" {
		cfg.SSHPort = directFlags.sshPort
	}
	if directFlags.payload != "" {
		cfg.PayloadTemplate = directFlags.payload
	}

	if verbose {
		printDirectConfig(cfg)
	}

	// Establish direct connection with front domain spoofing
	// For direct mode, we connect directly to the target host
	// but use the front domain in the Host header for spoofing
	conn, err := tunnel.EstablishWSTunnel(
		cfg.TargetHost, // Connect directly to target
		cfg.TargetPort, // Use target port
		cfg.TargetHost, // Target remains the same
		cfg.TargetPort, // Target port remains the same
		cfg.PayloadTemplate,
		cfg.FrontDomain, // Use front domain for Host header spoofing
		false,           // use_tls
		nil,             // sock
	)
	if err != nil {
		log.Fatalf("Error establishing direct tunnel: %v", err)
	}
	defer conn.Close()

	// Start SSH connection and SOCKS proxy
	sshConn, err := tunnel.ConnectViaWSAndStartSOCKS(
		conn,
		cfg.SSHUsername,
		cfg.SSHPassword,
		cfg.SSHPort,
		cfg.LocalSOCKSPort,
	)
	if err != nil {
		log.Fatalf("Error starting SSH connection: %v", err)
	}
	defer sshConn.Close()

	fmt.Printf("[+] SOCKS proxy up on 127.0.0.1:%d\n", cfg.LocalSOCKSPort)
	fmt.Println("[+] All traffic through that proxy is forwarded over SSH via WS tunnel.")

	// Wait for interrupt signal or timeout
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Keep running until interrupt or timeout
	timeoutDuration := time.Duration(directFlags.timeout) * time.Second
	if directFlags.timeout <= 0 {
		timeoutDuration = time.Hour * 24 * 365 // Effectively forever
	}

	select {
	case <-sigChan:
		fmt.Println("\n[*] Shutting down (interrupt signal received).")
	case <-time.After(timeoutDuration):
		fmt.Println("\n[*] Shutting down (timeout reached).")
	}
}

func printDirectConfig(cfg *tunnel.Config) {
	fmt.Println("Direct Tunnel Configuration:")
	fmt.Printf("  Front Domain: %s\n", cfg.FrontDomain)
	fmt.Printf("  Local SOCKS Port: %d\n", cfg.LocalSOCKSPort)
	fmt.Printf("  Target Host: %s\n", cfg.TargetHost)
	fmt.Printf("  Target Port: %s\n", cfg.TargetPort)
	fmt.Printf("  SSH Username: %s\n", cfg.SSHUsername)
	fmt.Printf("  SSH Port: %s\n", cfg.SSHPort)
	fmt.Printf("  Payload Template: %s\n", cfg.PayloadTemplate)
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(directCmd)

	// Front domain flag (unique to direct mode)
	directCmd.Flags().StringVar(&directFlags.frontDomain, "front-domain", "", "front domain for Host header spoofing (required)")

	// Network configuration flags
	directCmd.Flags().StringVar(&directFlags.targetHost, "target-host", "", "target host (required)")
	directCmd.Flags().StringVar(&directFlags.targetPort, "target-port", "80", "target port")

	// SSH configuration flags
	directCmd.Flags().StringVarP(&directFlags.sshUsername, "ssh-username", "u", "", "SSH username (required)")
	directCmd.Flags().StringVarP(&directFlags.sshPassword, "ssh-password", "p", "", "SSH password (required)")
	directCmd.Flags().StringVar(&directFlags.sshPort, "ssh-port", "22", "SSH port")

	// Advanced options
	directCmd.Flags().StringVar(&directFlags.payload, "payload", "", "custom HTTP payload template")
	directCmd.Flags().IntVarP(&directFlags.localPort, "local-port", "l", 1080, "local SOCKS proxy port")
	directCmd.Flags().IntVarP(&directFlags.timeout, "timeout", "t", 0, "connection timeout in seconds (0 = no timeout)")

	// Mark required flags
	directCmd.MarkFlagRequired("front-domain")
	directCmd.MarkFlagRequired("target-host")
	directCmd.MarkFlagRequired("ssh-username")
	directCmd.MarkFlagRequired("ssh-password")
}
