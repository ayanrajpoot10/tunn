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
	address := net.JoinHostPort(cfg.SSHHost, cfg.SSHPort)

	fmt.Printf("→ Connecting to %s\n", address)

	// Establish TCP or TLS connection first
	var conn net.Conn
	var err error
	if cfg.SSHPort == "443" {
		tlsConfig := &tls.Config{
			ServerName: cfg.SSHHost,
			MinVersion: tls.VersionTLS12,
		}
		conn, err = tls.DialWithDialer(
			&net.Dialer{Timeout: time.Duration(cfg.ConnectionTimeout) * time.Second},
			"tcp",
			address,
			tlsConfig,
		)
	} else {
		conn, err = net.DialTimeout("tcp", address, time.Duration(cfg.ConnectionTimeout)*time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect directly: %w", err)
	}

	// Perform WebSocket upgrade if payload is provided
	if cfg.HTTPPayload != "" {
		wsConn, err := EstablishWSTunnel(conn, cfg.HTTPPayload, cfg.SSHHost, cfg.SSHPort, cfg.SSHHost)
		if err != nil {
			return nil, fmt.Errorf("failed to establish WebSocket tunnel: %w", err)
		}
		return wsConn, nil
	}

	return conn, nil
}

// ProxyEstablisher handles HTTP proxy connections with WebSocket upgrade
type ProxyEstablisher struct{}

// Establish creates a connection through an HTTP proxy with WebSocket upgrade
func (p *ProxyEstablisher) Establish(cfg *config.Config) (net.Conn, error) {
	proxyAddress := net.JoinHostPort(cfg.ProxyHost, cfg.ProxyPort)
	fmt.Printf("→ Connecting to proxy %s for target %s\n", proxyAddress, cfg.SSHHost)

	// Establish TCP or TLS connection to proxy
	var conn net.Conn
	var err error
	if cfg.ProxyPort == "443" {
		tlsConfig := &tls.Config{
			ServerName: cfg.ProxyHost,
			MinVersion: tls.VersionTLS12,
		}
		conn, err = tls.DialWithDialer(
			&net.Dialer{Timeout: time.Duration(cfg.ConnectionTimeout) * time.Second},
			"tcp",
			proxyAddress,
			tlsConfig,
		)
	} else {
		conn, err = net.DialTimeout("tcp", proxyAddress, time.Duration(cfg.ConnectionTimeout)*time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}

	// Perform WebSocket upgrade through proxy
	wsConn, err := EstablishWSTunnel(conn, cfg.HTTPPayload, cfg.SSHHost, cfg.SSHPort, cfg.SSHHost)
	if err != nil {
		return nil, fmt.Errorf("failed to establish proxy WebSocket tunnel: %w", err)
	}

	fmt.Printf("✓ Proxy WebSocket connection established through %s\n", proxyAddress)
	return wsConn, nil
}

// GetEstablisher returns the appropriate connection establisher based on mode
func GetEstablisher(mode string) (Establisher, error) {
	switch mode {
	case "direct":
		return &DirectEstablisher{}, nil
	case "proxy":
		return &ProxyEstablisher{}, nil
	default:
		return nil, fmt.Errorf("unsupported connection mode: %s", mode)
	}
}
