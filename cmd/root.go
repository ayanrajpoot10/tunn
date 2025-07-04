package cmd

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"tunn/internal/tunnel"

	"github.com/spf13/cobra"
)

var (
	configFile string
	config     *tunnel.Config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "tunn",
	Short:   "A powerful tunnel tool for secure connections",
	Long:    "Tunn creates secure SSH tunnels over WebSocket connections with support for proxy, SNI, and direct modes.",
	Version: "v0.1.1",
	Run:     runTunnel,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if configFile != "" {
			var err error
			config, err = tunnel.LoadConfig(configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			fmt.Printf("Configuration loaded from: %s\n", configFile)
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "path to configuration file (JSON) - required")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{Use: "no-help", Hidden: true})
}

// GetConfig returns the global config
func GetConfig() *tunnel.Config {
	return config
}

// runTunnel is the main entry point for running the tunnel
func runTunnel(cmd *cobra.Command, args []string) {
	if configFile == "" {
		fmt.Println("Error: Configuration file is required")
		fmt.Println("\nUsage: tunn --config config.json")
		fmt.Println("\nTo generate a sample config: tunn config generate")
		os.Exit(1)
	}

	if config == nil {
		log.Fatal("Error: Configuration not loaded")
	}

	// Validate proxy type
	if config.ProxyType != "socks5" && config.ProxyType != "http" {
		log.Fatal("Error: proxy type must be either 'socks5' or 'http'")
	}

	fmt.Printf("[*] Using mode: %s\n", config.Mode)
	fmt.Printf("[*] Starting tunnel using %s strategy with %s local proxy\n", config.Mode, config.ProxyType)

	// Establish connection based on mode
	conn, err := establishConnection(config)
	if err != nil {
		log.Fatalf("Error establishing connection: %v", err)
	}
	defer conn.Close()

	// Start SSH connection and proxy
	startSSHConnection(conn, config)
}

// establishConnection creates the appropriate connection based on the mode
func establishConnection(config *tunnel.Config) (net.Conn, error) {
	printConnectionInfo(config)

	switch config.Mode {
	case "proxy":
		return establishProxyConnection(config)
	case "sni":
		return establishSNIConnection(config)
	case "direct":
		return establishDirectConnection(config)
	default:
		return nil, fmt.Errorf("unsupported mode: %s", config.Mode)
	}
}

// establishProxyConnection creates a proxy tunnel connection
func establishProxyConnection(config *tunnel.Config) (net.Conn, error) {
	conn, err := tunnel.EstablishWSTunnel(
		config.ProxyHost, config.ProxyPort,
		config.TargetHost, config.TargetPort,
		config.Payload, config.FrontDomain,
		false, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("proxy tunnel failed: %w", err)
	}

	fmt.Printf("[*] Connected to proxy %s:%s, tunneling to target %s:%s\n",
		config.ProxyHost, config.ProxyPort, config.TargetHost, config.TargetPort)
	return conn, nil
}

// establishSNIConnection creates an SNI fronted connection
func establishSNIConnection(config *tunnel.Config) (net.Conn, error) {
	// Connect to proxy
	conn, err := net.Dial("tcp", net.JoinHostPort(config.ProxyHost, config.ProxyPort))
	if err != nil {
		return nil, fmt.Errorf("proxy connection failed: %w", err)
	}

	// Setup TLS with SNI fronting
	tlsConn := tls.Client(conn, &tls.Config{
		ServerName:         config.FrontDomain,
		InsecureSkipVerify: true,
	})
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	fmt.Printf("[*] TLS connection established with SNI: %s\n", config.FrontDomain)

	// Establish WebSocket tunnel through TLS connection
	wsConn, err := tunnel.EstablishWSTunnel(
		config.TargetHost, config.TargetPort,
		config.TargetHost, config.TargetPort,
		config.Payload, config.FrontDomain,
		true, tlsConn,
	)
	if err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("WebSocket tunnel failed: %w", err)
	}

	fmt.Printf("[*] SNI fronted connection established to target %s:%s\n", config.TargetHost, config.TargetPort)
	return wsConn, nil
}

// establishDirectConnection creates a direct connection
func establishDirectConnection(config *tunnel.Config) (net.Conn, error) {
	conn, err := tunnel.EstablishWSTunnel(
		"", "",
		config.TargetHost, config.TargetPort,
		config.Payload, config.FrontDomain,
		false, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("direct tunnel failed: %w", err)
	}

	fmt.Printf("[*] Direct connection established to target %s:%s\n", config.TargetHost, config.TargetPort)
	return conn, nil
}

// printConnectionInfo prints connection information based on mode
func printConnectionInfo(config *tunnel.Config) {
	fmt.Printf("[*] Target: %s:%s\n", config.TargetHost, config.TargetPort)
	fmt.Printf("[*] SSH User: %s\n", config.SSH.Username)
	fmt.Printf("[*] Local %s proxy will be available on 127.0.0.1:%d\n", config.ProxyType, config.LocalPort)

	if config.Mode == "proxy" || config.Mode == "sni" {
		fmt.Printf("[*] Proxy: %s:%s\n", config.ProxyHost, config.ProxyPort)
	}
	if config.Mode == "sni" {
		fmt.Printf("[*] SNI Front Domain: %s\n", config.FrontDomain)
	}
	if config.FrontDomain != "" && config.Mode != "sni" {
		fmt.Printf("[*] Front Domain: %s\n", config.FrontDomain)
	}
}

// startSSHConnection establishes SSH connection and starts the proxy
func startSSHConnection(conn net.Conn, config *tunnel.Config) {
	fmt.Printf("[*] Starting SSH connection and %s proxy...\n", config.ProxyType)

	// Start SSH connection with timeout
	sshConn, err := connectSSHWithTimeout(conn, config)
	if err != nil {
		log.Fatalf("Error starting SSH connection: %v", err)
	}
	defer sshConn.Close()

	// Print success message
	fmt.Printf("[+] %s proxy up on 127.0.0.1:%d\n", config.ProxyType, config.LocalPort)
	fmt.Printf("[+] All traffic through that proxy is forwarded over SSH via WS tunnel.\n")
	fmt.Printf("[+] Configure your applications to use %s proxy 127.0.0.1:%d\n",
		strings.ToUpper(config.ProxyType), config.LocalPort)

	// Wait for interrupt signal
	waitForInterrupt()
}

// connectSSHWithTimeout establishes SSH connection with timeout
func connectSSHWithTimeout(conn net.Conn, config *tunnel.Config) (*tunnel.SSHOverWebSocket, error) {
	type sshResult struct {
		conn *tunnel.SSHOverWebSocket
		err  error
	}
	resultChan := make(chan sshResult, 1)

	// Start SSH connection in goroutine
	go func() {
		var sshConn *tunnel.SSHOverWebSocket
		var err error

		if config.ProxyType == "http" {
			sshConn, err = tunnel.ConnectViaWSAndStartHTTP(
				conn, config.SSH.Username, config.SSH.Password,
				config.SSH.Port, config.LocalPort,
			)
		} else {
			sshConn, err = tunnel.ConnectViaWSAndStartSOCKS(
				conn, config.SSH.Username, config.SSH.Password,
				config.SSH.Port, config.LocalPort,
			)
		}
		resultChan <- sshResult{conn: sshConn, err: err}
	}()

	// Wait for result or timeout
	select {
	case result := <-resultChan:
		return result.conn, result.err
	case <-time.After(time.Duration(config.Timeout) * time.Second):
		conn.Close()
		return nil, fmt.Errorf("SSH connection timed out after %d seconds", config.Timeout)
	}
}

// waitForInterrupt waits for interrupt signal
func waitForInterrupt() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	fmt.Println("\n[*] Shutting down (interrupt signal received).")
}
