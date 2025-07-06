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

// Manager manages the tunnel lifecycle
type Manager struct {
	config      *config.Config
	sshClient   ssh.Client
	proxyServer interface{} // Can be either SOCKS5 or HTTP proxy
}

// NewManager creates a new tunnel manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config: cfg,
	}
}

// Start starts the tunnel and proxy server
func (m *Manager) Start() error {
	// Establish connection
	establisher, err := connection.GetEstablisher(m.config.ConnectionMode)
	if err != nil {
		return fmt.Errorf("failed to get connection establisher: %w", err)
	}

	conn, err := establisher.Establish(m.config)
	if err != nil {
		return fmt.Errorf("failed to establish connection: %w", err)
	}

	// Create SSH client
	m.sshClient = ssh.NewOverWebSocket(conn, m.config.SSH.Username, m.config.SSH.Password, m.config.SSH.Port)

	// Start SSH transport
	if sshOverWS, ok := m.sshClient.(*ssh.OverWebSocket); ok {
		if err := sshOverWS.StartTransport(); err != nil {
			return fmt.Errorf("failed to start SSH transport: %w", err)
		}
	}

	// Start proxy server
	if err := m.startProxy(); err != nil {
		return fmt.Errorf("failed to start proxy: %w", err)
	}

	fmt.Printf("\n✓ Tunnel established and %s proxy running on port %d\n", m.config.ProxyType, m.config.ListenPort)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Wait for shutdown signal
	m.waitForShutdown()

	return nil
}

// startProxy starts the appropriate proxy server
func (m *Manager) startProxy() error {
	switch m.config.ProxyType {
	case "socks5", "socks":
		socksProxy := proxy.NewSOCKS5(m.sshClient)
		m.proxyServer = socksProxy
		return socksProxy.Start(m.config.ListenPort)
	case "http":
		httpProxy := proxy.NewHTTP(m.sshClient)
		m.proxyServer = httpProxy
		return httpProxy.Start(m.config.ListenPort)
	default:
		return fmt.Errorf("unsupported proxy type: %s", m.config.ProxyType)
	}
}

// waitForShutdown waits for shutdown signals
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
