package proxy

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// SOCKS5 implements a SOCKS5 proxy server that forwards connections through SSH tunnels.
//
// This implementation provides full SOCKS5 protocol support including:
//   - No authentication method (method 0x00)
//   - CONNECT command for TCP connections
//   - IPv4, IPv6, and domain name address types
//   - Proper error response handling according to RFC 1928
//
// The SOCKS5 proxy accepts client connections on a local port and forwards
// all CONNECT requests through the established SSH tunnel to their destinations.
type SOCKS5 struct {
	server *Server // Embedded server for common proxy functionality
}

// NewSOCKS5 creates a new SOCKS5 proxy instance with the specified SSH client.
//
// The SOCKS5 proxy will use the provided SSH client to establish tunneled
// connections for incoming SOCKS5 requests. The SSH client must be properly
// initialized and ready to handle connection requests.
//
// Parameters:
//   - ssh: An initialized SSH client for tunnel connections
//
// Returns:
//   - *SOCKS5: A new SOCKS5 proxy server instance
func NewSOCKS5(ssh SSHClient) *SOCKS5 {
	return &SOCKS5{
		server: NewServer(ssh),
	}
}

// Start starts the SOCKS5 proxy server on the specified local port.
//
// This method begins listening for SOCKS5 client connections on the local
// interface at the specified port. Each client connection is handled according
// to the SOCKS5 protocol specification (RFC 1928).
//
// The server will continue running until the application is terminated or
// an unrecoverable error occurs.
//
// Parameters:
//   - localPort: Local port number to listen for SOCKS5 connections
//
// Returns:
//   - error: An error if the server fails to start listening
func (s *SOCKS5) Start(localPort int) error {
	return s.server.StartProxy("SOCKS5", localPort, s.handleClient)
}

// handleClient processes a single SOCKS5 client connection.
//
// This method manages the complete SOCKS5 client session including timeout
// handling, panic recovery, and protocol version detection. It supports both
// SOCKS5 (version 5) protocol, logging appropriate messages for unsupported versions.
//
// The method uses a 10-second timeout for initial protocol negotiation to prevent
// slow or malicious clients from consuming server resources.
//
// Parameters:
//   - clientConn: The incoming client connection to handle
func (s *SOCKS5) handleClient(clientConn net.Conn) {
	s.server.HandleClientWithTimeout(clientConn, "SOCKS5", 10*time.Second, func() {
		versionByte := make([]byte, 1)
		if _, err := clientConn.Read(versionByte); err != nil {
			fmt.Printf("✗ Error reading SOCKS version: %v\n", err)
			return
		}

		switch versionByte[0] {
		case 5:
			s.handleSOCKS5(clientConn)
		default:
			fmt.Printf("✗ Unsupported SOCKS version: %d (only SOCKS5 supported)\n", versionByte[0])
		}
	})
}

// handleSOCKS5 implements the complete SOCKS5 protocol handshake and connection establishment.
//
// This method performs the full SOCKS5 protocol sequence according to RFC 1928:
//  1. Method selection negotiation (supporting no authentication - method 0x00)
//  2. Connection request processing (supporting CONNECT command only)
//  3. Address parsing for IPv4, IPv6, and domain names
//  4. SSH tunnel establishment and data forwarding
//
// The implementation supports all standard SOCKS5 address types:
//   - Type 1: IPv4 address (4 bytes)
//   - Type 3: Domain name (variable length with length prefix)
//   - Type 4: IPv6 address (16 bytes)
//
// Only the CONNECT command (0x01) is supported, as it's the most common and
// useful for general proxy operations.
//
// Parameters:
//   - clientConn: The SOCKS5 client connection to process
func (s *SOCKS5) handleSOCKS5(clientConn net.Conn) {
	// Read number of methods
	nmethodsByte := make([]byte, 1)
	_, err := clientConn.Read(nmethodsByte)
	if err != nil {
		fmt.Printf("✗ Error reading SOCKS5 nmethods: %v\n", err)
		return
	}

	nmethods := int(nmethodsByte[0])
	methods := make([]byte, nmethods)
	_, err = io.ReadFull(clientConn, methods)
	if err != nil {
		fmt.Printf("✗ Error reading SOCKS5 methods: %v\n", err)
		return
	}

	// Send method selection (no auth)
	clientConn.Write([]byte{5, 0})

	// Read connection request
	requestHeader := make([]byte, 4) // ver, cmd, rsv, atyp
	_, err = io.ReadFull(clientConn, requestHeader)
	if err != nil {
		fmt.Printf("✗ Error reading SOCKS5 request header: %v\n", err)
		return
	}

	cmd := requestHeader[1]
	atyp := requestHeader[3]

	if cmd != 1 { // Only CONNECT supported
		s.sendError(clientConn, 7) // Command not supported
		return
	}

	var host string
	var port int

	// Parse address based on type
	switch atyp {
	case 1: // IPv4
		addr := make([]byte, 4)
		_, err = io.ReadFull(clientConn, addr)
		if err != nil {
			s.sendError(clientConn, 1)
			return
		}
		host = fmt.Sprintf("%d.%d.%d.%d", addr[0], addr[1], addr[2], addr[3])

	case 3: // Domain name
		lengthByte := make([]byte, 1)
		_, err = clientConn.Read(lengthByte)
		if err != nil {
			s.sendError(clientConn, 1)
			return
		}

		length := int(lengthByte[0])
		domain := make([]byte, length)
		_, err = io.ReadFull(clientConn, domain)
		if err != nil {
			s.sendError(clientConn, 1)
			return
		}
		host = string(domain)

	case 4: // IPv6
		addr := make([]byte, 16)
		_, err = io.ReadFull(clientConn, addr)
		if err != nil {
			s.sendError(clientConn, 1)
			return
		}
		host = fmt.Sprintf("[%x:%x:%x:%x:%x:%x:%x:%x]",
			binary.BigEndian.Uint16(addr[0:2]),
			binary.BigEndian.Uint16(addr[2:4]),
			binary.BigEndian.Uint16(addr[4:6]),
			binary.BigEndian.Uint16(addr[6:8]),
			binary.BigEndian.Uint16(addr[8:10]),
			binary.BigEndian.Uint16(addr[10:12]),
			binary.BigEndian.Uint16(addr[12:14]),
			binary.BigEndian.Uint16(addr[14:16]))

	default:
		s.sendError(clientConn, 8) // Address type not supported
		return
	}

	// Read port
	portBytes := make([]byte, 2)
	_, err = io.ReadFull(clientConn, portBytes)
	if err != nil {
		s.sendError(clientConn, 1)
		return
	}
	port = int(binary.BigEndian.Uint16(portBytes))

	// Send success response
	s.sendSuccess(clientConn)

	// Open SSH channel
	s.server.OpenSSHChannel(clientConn, host, port)
}

// sendError sends a SOCKS5 error response to the client.
//
// This method constructs and sends a proper SOCKS5 error response according to
// RFC 1928 format. The response includes the SOCKS version, error code, and
// placeholder address information.
//
// SOCKS5 error codes:
//   - 0x01: General SOCKS server failure
//   - 0x07: Command not supported
//   - 0x08: Address type not supported
//   - (other codes as defined in RFC 1928)
//
// Parameters:
//   - clientConn: The client connection to send the error response to
//   - errCode: The SOCKS5 error code to send (as defined in RFC 1928)
func (s *SOCKS5) sendError(clientConn net.Conn, errCode byte) {
	response := []byte{5, errCode, 0, 1, 0, 0, 0, 0, 0, 0}
	clientConn.Write(response)
}

// sendSuccess sends a SOCKS5 success response to the client.
//
// This method sends a successful connection response according to SOCKS5
// protocol specification. The response indicates that the connection request
// was accepted and the tunnel is ready for data forwarding.
//
// The success response includes:
//   - Version: 5 (SOCKS5)
//   - Reply code: 0 (success)
//   - Reserved: 0
//   - Address type: 1 (IPv4)
//   - Bound address: 0.0.0.0 (placeholder)
//   - Bound port: 0 (placeholder)
//
// Parameters:
//   - clientConn: The client connection to send the success response to
func (s *SOCKS5) sendSuccess(clientConn net.Conn) {
	response := []byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	clientConn.Write(response)
}
