// Package tunnel provides the core tunnel management functionality for Tunn.
//
// This package implements the tunnel lifecycle management, including connection
// establishment, SSH client setup, and proxy server initialization. It coordinates
// between the configuration, connection, SSH, and proxy packages to provide
// a complete tunneling solution.
//
// The Manager type handles the entire tunnel lifecycle from initialization
// through graceful shutdown, managing both SSH connections and local proxy servers.
package tunnel

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"tunn/pkg/config"
	"tunn/pkg/connection"
	"tunn/pkg/proxy"
	"tunn/pkg/ssh"
)

// Manager manages the complete tunnel lifecycle including connection establishment,
// SSH client setup, proxy server initialization, and graceful shutdown.
//
// The Manager coordinates between different components to provide a seamless
// tunneling experience, handling both direct and proxy-based connection modes.
type Manager struct {
	config      *config.Config // The tunnel configuration
	sshClient   ssh.Client     // SSH client for tunneling
	proxyServer interface{}    // Local proxy server (SOCKS5 or HTTP)
}

// NewManager creates a new tunnel manager with the provided configuration.
//
// The manager will use the configuration to determine the appropriate connection
// method, SSH settings, and proxy type to establish.
//
// Parameters:
//   - cfg: The tunnel configuration containing all necessary settings
//
// Returns:
//   - *Manager: A new tunnel manager instance ready for startup
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config: cfg,
	}
}

// Start establishes the complete tunnel setup and starts all necessary services.
//
// This method performs the following operations in sequence:
//  1. Establishes the base connection (direct or through proxy)
//  2. Creates and initializes the SSH client over the connection
//  3. Starts the SSH transport layer
//  4. Launches the appropriate local proxy server (SOCKS5 or HTTP)
//  5. Waits for shutdown signals to gracefully terminate
//
// The method blocks until a shutdown signal is received, making it suitable
// for use in the main application loop.
//
// Returns:
//   - error: An error if any step of the setup process fails
func (m *Manager) Start() error {
	// Establish connection
	establisher, err := connection.GetEstablisher(m.config.Mode)
	if err != nil {
		return fmt.Errorf("failed to get connection establisher: %w", err)
	}

	conn, err := establisher.Establish(m.config)
	if err != nil {
		return fmt.Errorf("failed to establish connection: %w", err)
	}

	// Create SSH client
	m.sshClient = ssh.NewSSHClient(conn, m.config.SSH.Username, m.config.SSH.Password)

	// Start SSH transport
	if sshOverWS, ok := m.sshClient.(*ssh.SSHClient); ok {
		if err := sshOverWS.StartTransport(); err != nil {
			return fmt.Errorf("failed to start SSH transport: %w", err)
		}
	}

	// Start proxy server
	if err := m.startProxy(); err != nil {
		return fmt.Errorf("failed to start proxy: %w", err)
	}

	fmt.Printf("\n✓ Tunnel established and %s proxy running on port %d\n", m.config.Listener.ProxyType, m.config.Listener.Port)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Wait for shutdown signal
	m.waitForShutdown()

	return nil
}

// startProxy initializes and starts the appropriate local proxy server based on configuration.
//
// This method creates either a SOCKS5 or HTTP proxy server according to the ProxyType
// setting in the listener configuration. The proxy server will listen on the configured port
// and forward connections through the established SSH tunnel.
//
// Supported proxy types:
//   - "socks5" or "socks": Creates a SOCKS5 proxy server
//   - "http": Creates an HTTP proxy server
//
// Returns:
//   - error: An error if the proxy type is unsupported or proxy startup fails
func (m *Manager) startProxy() error {
	switch m.config.Listener.ProxyType {
	case "socks5", "socks":
		socksProxy := proxy.NewSOCKS5(m.sshClient)
		m.proxyServer = socksProxy
		return socksProxy.Start(m.config.Listener.Port)
	case "http":
		httpProxy := proxy.NewHTTP(m.sshClient)
		m.proxyServer = httpProxy
		return httpProxy.Start(m.config.Listener.Port)
	default:
		return fmt.Errorf("unsupported proxy type: %s", m.config.Listener.ProxyType)
	}
}

// waitForShutdown blocks and waits for system shutdown signals to gracefully terminate the tunnel.
//
// This method listens for SIGINT (Ctrl+C) and SIGTERM signals, providing a clean
// shutdown mechanism. When a signal is received, it closes the SSH client connection
// and performs cleanup operations.
//
// The method blocks the calling goroutine until a shutdown signal is received,
// making it suitable for use in the main application flow.
func (m *Manager) waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n→ Shutdown signal received, closing tunnel...")

	if m.sshClient != nil {
		m.sshClient.Close()
	}

	fmt.Println("✓ Tunnel closed.")
}
