package proxy

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// SOCKS5 handles SOCKS5 proxy functionality
type SOCKS5 struct {
	server *Server
}

// NewSOCKS5 creates a new SOCKS5 proxy instance
func NewSOCKS5(ssh SSHClient) *SOCKS5 {
	return &SOCKS5{
		server: NewServer(ssh),
	}
}

// Start starts the SOCKS5 proxy server on the specified local port
func (s *SOCKS5) Start(localPort int) error {
	return s.server.StartProxy("SOCKS5", localPort, s.handleClient)
}

// handleClient handles a SOCKS5 client connection
func (s *SOCKS5) handleClient(clientConn net.Conn) {
	s.server.HandleClientWithTimeout(clientConn, "SOCKS5", 10*time.Second, func() {
		versionByte := make([]byte, 1)
		if _, err := clientConn.Read(versionByte); err != nil {
			fmt.Printf("✗ Error reading SOCKS version: %v\n", err)
			return
		}

		switch versionByte[0] {
		case 5:
			s.handleSOCKS5(clientConn, versionByte[0])
		default:
			fmt.Printf("✗ Unsupported SOCKS version: %d (only SOCKS5 supported)\n", versionByte[0])
		}
	})
}

// handleSOCKS5 handles SOCKS5 protocol
func (s *SOCKS5) handleSOCKS5(clientConn net.Conn, version byte) {
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

// sendError sends a SOCKS5 error response
func (s *SOCKS5) sendError(clientConn net.Conn, errCode byte) {
	response := []byte{5, errCode, 0, 1, 0, 0, 0, 0, 0, 0}
	clientConn.Write(response)
}

// sendSuccess sends a SOCKS5 success response
func (s *SOCKS5) sendSuccess(clientConn net.Conn) {
	response := []byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	clientConn.Write(response)
}
