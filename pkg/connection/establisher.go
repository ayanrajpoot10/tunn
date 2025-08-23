// Package connection provides network connection establishment functionality for Tunn.
//
// This package implements different strategies for establishing network connections
// to SSH servers, supporting both direct connections and connections through HTTP proxies.
// It also handles WebSocket upgrade procedures for enhanced bypass capabilities.
//
// The package supports multiple connection modes:
//   - Direct: Direct TCP or TLS connection to the target SSH server
//   - Proxy: Connection through an HTTP proxy server
//
// Both modes support optional WebSocket upgrade using custom HTTP payloads for
// improved network traversal and firewall bypass capabilities.
package connection

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"tunn/pkg/config"
)

// Establisher defines the interface for establishing network connections.
//
// This interface abstracts the connection establishment process, allowing
// different implementations for various connection strategies (direct, proxy, etc.).
// All establishers must be able to create a net.Conn given a configuration.
type Establisher interface {
	// Establish creates a network connection based on the provided configuration.
	// Returns a ready-to-use net.Conn or an error if connection fails.
	Establish(cfg *config.Config) (net.Conn, error)
}

// DirectEstablisher implements direct connection establishment with optional WebSocket upgrade.
//
// This establisher creates direct TCP or TLS connections to the target SSH server,
// automatically selecting TLS when port 443 is specified. It supports optional
// WebSocket upgrade using custom HTTP payloads for enhanced bypass capabilities.
type DirectEstablisher struct{}

// Establish creates a direct connection to the SSH server with optional WebSocket upgrade.
//
// The connection process:
//  1. Establishes TCP or TLS connection (TLS for port 443)
//  2. Performs WebSocket upgrade if HTTPPayload is configured
//  3. Returns the ready-to-use connection
//
// TLS connections use secure defaults with TLS 1.2 minimum version and proper
// server name indication (SNI) for certificate validation.
//
// Parameters:
//   - cfg: Configuration containing connection details and optional WebSocket payload
//
// Returns:
//   - net.Conn: Ready-to-use connection to the SSH server
//   - error: Connection or WebSocket upgrade error
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

// ProxyEstablisher implements HTTP proxy connection establishment with WebSocket upgrade.
//
// This establisher routes connections through HTTP proxy servers before reaching
// the target SSH server. It supports both plain HTTP and HTTPS proxy connections,
// with mandatory WebSocket upgrade for tunneling through the proxy.
type ProxyEstablisher struct{}

// Establish creates a connection through an HTTP proxy with WebSocket upgrade.
//
// The connection process:
//  1. Establishes TCP or TLS connection to the HTTP proxy server
//  2. Performs WebSocket upgrade through the proxy to reach the target
//  3. Returns the tunneled connection ready for SSH traffic
//
// This method requires an HTTPPayload configuration to perform the WebSocket
// upgrade, as proxy connections always tunnel through WebSocket.
//
// Parameters:
//   - cfg: Configuration containing proxy details and required WebSocket payload
//
// Returns:
//   - net.Conn: Tunneled connection through the proxy to the SSH server
//   - error: Connection or WebSocket upgrade error
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

// GetEstablisher returns the appropriate connection establisher for the specified mode.
//
// This factory function creates the correct Establisher implementation based on
// the connection mode specified in the configuration.
//
// Supported modes:
//   - "direct": Returns DirectEstablisher for direct connections
//   - "proxy": Returns ProxyEstablisher for HTTP proxy connections
//
// Parameters:
//   - mode: The connection mode string from configuration
//
// Returns:
//   - Establisher: The appropriate establisher implementation
//   - error: An error if the mode is not supported
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
