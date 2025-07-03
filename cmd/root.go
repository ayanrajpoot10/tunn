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
	configFile string
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

Available tunnel modes (configured via config file):
  proxy         - HTTP proxy tunneling strategy  
  sni           - SNI fronting strategy  
  direct        - Direct connection with front domain spoofing

Configuration must be provided via JSON/YAML configuration file:
  tunn --config config.json

Example configuration file (config.json):
{
  "mode": "proxy",
  "targetHost": "target.example.com",
  "proxyHost": "proxy.example.com",
  "ssh": {
    "username": "user",
    "password": "pass"
  }
}

To generate sample configurations:
  tunn config generate --mode proxy --output proxy-config.json
  tunn config generate --mode sni --output sni-config.yaml --format yaml
  tunn config generate --mode direct --output direct-config.json

To validate a configuration:
  tunn config validate --config myconfig.json`,
	Version: "v0.1.1",
	Run: func(cmd *cobra.Command, args []string) {
		// Config file is required
		if configFile == "" {
			fmt.Println("Error: Configuration file is required")
			fmt.Println("\nUsage: tunn --config config.json")
			fmt.Println("\nTo generate a sample config: tunn config generate")
			os.Exit(1)
		}

		runWithConfig(cmd)
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration if config file is provided
		if configFile != "" {
			configMgr = tunnel.NewConfigManager(configFile)
			if err := configMgr.LoadConfig(); err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			fmt.Printf("✓ Configuration loaded from: %s\n", configFile)
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
	// Config file flag (required for main command, but not for config subcommands)
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "path to configuration file (JSON/YAML) - required")

	// Disable built-in commands
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})
}

// GetConfigManager returns the global config manager
func GetConfigManager() *tunnel.ConfigManager {
	return configMgr
}

// runWithConfig executes tunnel using configuration file
func runWithConfig(cmd *cobra.Command) {
	if configMgr == nil {
		log.Fatal("Error: Configuration manager not initialized")
	}

	config := configMgr.GetConfig()
	if config == nil {
		log.Fatal("Error: No configuration loaded")
	}

	// Use proxy type from config (defaults to "socks5" if not specified)
	effectiveProxyType := config.ProxyType

	// Validate proxy type
	if effectiveProxyType != "socks5" && effectiveProxyType != "http" {
		log.Fatal("Error: proxy type must be either 'socks5' or 'http'")
	}

	fmt.Printf("[*] Using mode: %s\n", config.Mode)
	fmt.Printf("[*] Starting tunnel using %s strategy with %s local proxy\n", config.Mode, effectiveProxyType)

	// Execute the appropriate tunnel function based on the config mode
	switch config.Mode {
	case "proxy":
		runTunnelWithConfigProxy(config, effectiveProxyType)
	case "sni":
		runTunnelWithConfigSNI(config, effectiveProxyType)
	case "direct":
		runTunnelWithConfigDirect(config, effectiveProxyType)
	default:
		log.Fatalf("Error: Unsupported mode '%s' in config", config.Mode)
	}
}

// runTunnelWithConfigProxy executes proxy tunnel with the given configuration
func runTunnelWithConfigProxy(config *tunnel.Config, globalProxyType string) {
	fmt.Printf("[*] Proxy: %s:%s\n", config.ProxyHost, config.ProxyPort)
	fmt.Printf("[*] Target: %s:%s\n", config.TargetHost, config.TargetPort)
	fmt.Printf("[*] SSH User: %s\n", config.SSH.Username)
	fmt.Printf("[*] Local %s proxy will be available on 127.0.0.1:%d\n", globalProxyType, config.LocalPort)

	if config.FrontDomain != "" {
		fmt.Printf("[*] Front Domain: %s\n", config.FrontDomain)
	}

	// Establish HTTP proxy tunnel
	conn, err := tunnel.EstablishWSTunnel(
		config.ProxyHost,
		config.ProxyPort,
		config.TargetHost,
		config.TargetPort,
		config.Payload,
		config.FrontDomain,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Error establishing proxy tunnel to target: %v", err)
	}
	defer conn.Close()

	fmt.Printf("[*] Connected to proxy %s:%s, tunneling to target %s:%s\n",
		config.ProxyHost, config.ProxyPort, config.TargetHost, config.TargetPort)

	fmt.Printf("[*] Starting SSH connection and %s proxy...\n", globalProxyType)

	startSSHConnection(conn, config, globalProxyType, config.Timeout)
}

// runTunnelWithConfigSNI executes SNI tunnel with the given configuration
func runTunnelWithConfigSNI(config *tunnel.Config, globalProxyType string) {
	fmt.Printf("[*] SNI Front Domain: %s\n", config.FrontDomain)
	fmt.Printf("[*] Proxy: %s:%s\n", config.ProxyHost, config.ProxyPort)
	fmt.Printf("[*] Target: %s:%s\n", config.TargetHost, config.TargetPort)
	fmt.Printf("[*] SSH User: %s\n", config.SSH.Username)
	fmt.Printf("[*] Local %s proxy will be available on 127.0.0.1:%d\n", globalProxyType, config.LocalPort)

	// Connect to proxy with SNI fronting
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", config.ProxyHost, config.ProxyPort))
	if err != nil {
		log.Fatalf("Error connecting to proxy: %v", err)
	}

	// Setup TLS connection with SNI fronting
	tlsConfig := &tls.Config{
		ServerName:         config.FrontDomain,
		InsecureSkipVerify: true,
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		log.Fatalf("Error establishing TLS connection: %v", err)
	}

	fmt.Printf("[*] TLS connection established with SNI: %s\n", config.FrontDomain)

	// Establish WebSocket tunnel through the TLS connection
	wsConn, err := tunnel.EstablishWSTunnel(
		config.TargetHost,
		config.TargetPort,
		config.TargetHost,
		config.TargetPort,
		config.Payload,
		config.FrontDomain,
		true,
		tlsConn,
	)
	if err != nil {
		log.Fatalf("Error establishing WebSocket tunnel: %v", err)
	}
	defer wsConn.Close()

	fmt.Printf("[*] SNI fronted connection established to target %s:%s\n", config.TargetHost, config.TargetPort)

	startSSHConnection(wsConn, config, globalProxyType, config.Timeout)
}

// runTunnelWithConfigDirect executes direct tunnel with the given configuration
func runTunnelWithConfigDirect(config *tunnel.Config, globalProxyType string) {
	fmt.Printf("[*] Target: %s:%s\n", config.TargetHost, config.TargetPort)
	fmt.Printf("[*] SSH User: %s\n", config.SSH.Username)
	fmt.Printf("[*] Local %s proxy will be available on 127.0.0.1:%d\n", globalProxyType, config.LocalPort)

	if config.FrontDomain != "" {
		fmt.Printf("[*] Front Domain: %s\n", config.FrontDomain)
	}

	// Establish direct WebSocket connection
	conn, err := tunnel.EstablishWSTunnel(
		config.TargetHost,
		config.TargetPort,
		config.TargetHost,
		config.TargetPort,
		config.Payload,
		config.FrontDomain,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Error establishing direct tunnel: %v", err)
	}
	defer conn.Close()

	fmt.Printf("[*] Direct connection established to target %s:%s\n", config.TargetHost, config.TargetPort)

	startSSHConnection(conn, config, globalProxyType, config.Timeout)
}

// startSSHConnection is a common function to start SSH connection and handle signals
func startSSHConnection(conn net.Conn, config *tunnel.Config, globalProxyType string, timeout int) {
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

	// Wait for SSH connection with timeout
	var sshConn *tunnel.SSHOverWebSocket
	select {
	case result := <-sshResultChan:
		if result.err != nil {
			log.Fatalf("Error starting SSH connection: %v", result.err)
		}
		sshConn = result.conn
		fmt.Printf("[+] %s proxy up on 127.0.0.1:%d\n", globalProxyType, config.LocalPort)
		fmt.Printf("[+] All traffic through that proxy is forwarded over SSH via WS tunnel.\n")

		if globalProxyType == "socks5" {
			fmt.Printf("[+] Configure your applications to use SOCKS5 proxy 127.0.0.1:%d\n", config.LocalPort)
		} else {
			fmt.Printf("[+] Configure your applications to use HTTP proxy 127.0.0.1:%d\n", config.LocalPort)
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
