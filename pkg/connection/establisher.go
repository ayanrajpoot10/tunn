package connection

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"tunn/pkg/config"
)

// Establisher interface for different connection types
type Establisher interface {
	Establish(cfg *config.Config) (net.Conn, error)
}

// DirectEstablisher handles direct connections with WebSocket upgrade
type DirectEstablisher struct{}

// Establish creates a direct connection to the target with WebSocket upgrade
func (d *DirectEstablisher) Establish(cfg *config.Config) (net.Conn, error) {
	address := net.JoinHostPort(cfg.TargetHost, cfg.TargetPort)
	fmt.Printf("[*] Establishing direct connection to %s\n", address)

	// For direct mode, we still need to establish WebSocket tunnel if payload is provided
	if cfg.Payload != "" {
		conn, err := EstablishWSTunnel(
			"", "", // No proxy for direct mode
			cfg.TargetHost,
			cfg.TargetPort,
			cfg.Payload,
			cfg.FrontDomain,
			cfg.TargetPort == "443", // Use TLS if port is 443
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to establish WebSocket tunnel: %w", err)
		}
		fmt.Printf("[+] Direct WebSocket connection established to %s\n", address)
		return conn, nil
	}

	// Fallback to plain TCP connection
	conn, err := net.DialTimeout("tcp", address, time.Duration(cfg.Timeout)*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect directly: %w", err)
	}

	fmt.Printf("[+] Direct connection established to %s\n", address)
	return conn, nil
}

// ProxyEstablisher handles HTTP proxy connections with WebSocket upgrade
type ProxyEstablisher struct{}

// Establish creates a connection through an HTTP proxy with WebSocket upgrade
func (p *ProxyEstablisher) Establish(cfg *config.Config) (net.Conn, error) {
	proxyAddress := net.JoinHostPort(cfg.ProxyHost, cfg.ProxyPort)
	targetAddress := net.JoinHostPort(cfg.TargetHost, cfg.TargetPort)

	fmt.Printf("[*] Connecting to proxy %s for target %s\n", proxyAddress, targetAddress)

	// Use WebSocket tunnel through proxy
	conn, err := EstablishWSTunnel(
		cfg.ProxyHost,
		cfg.ProxyPort,
		cfg.TargetHost,
		cfg.TargetPort,
		cfg.Payload,
		cfg.FrontDomain,
		cfg.ProxyPort == "443", // Use TLS if port is 443
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to establish proxy WebSocket tunnel: %w", err)
	}

	fmt.Printf("[+] Proxy WebSocket connection established through %s\n", proxyAddress)
	return conn, nil
}

// SNIEstablisher handles SNI-fronted connections
type SNIEstablisher struct{}

// Establish creates a connection using SNI fronting
func (s *SNIEstablisher) Establish(cfg *config.Config) (net.Conn, error) {
	proxyAddress := net.JoinHostPort(cfg.ProxyHost, cfg.ProxyPort)

	fmt.Printf("[*] Establishing SNI connection to %s (fronting: %s)\n", cfg.TargetHost, cfg.FrontDomain)

	// Create TLS config with SNI
	tlsConfig := &tls.Config{
		ServerName: cfg.FrontDomain,
		MinVersion: tls.VersionTLS12,
	}

	// Connect with TLS
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: time.Duration(cfg.Timeout) * time.Second},
		"tcp",
		proxyAddress,
		tlsConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to establish SNI connection: %w", err)
	}

	// Send custom payload if provided
	if cfg.Payload != "" {
		if _, err := conn.Write([]byte(cfg.Payload)); err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to send payload: %w", err)
		}
	}

	fmt.Printf("[+] SNI connection established to %s\n", cfg.TargetHost)
	return conn, nil
}

// GetEstablisher returns the appropriate connection establisher based on mode
func GetEstablisher(mode string) (Establisher, error) {
	switch mode {
	case "direct":
		return &DirectEstablisher{}, nil
	case "proxy":
		return &ProxyEstablisher{}, nil
	case "sni":
		return &SNIEstablisher{}, nil
	default:
		return nil, fmt.Errorf("unsupported connection mode: %s", mode)
	}
}
