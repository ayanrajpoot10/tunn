package ssh

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// Client represents an SSH client that can operate over any net.Conn
type Client interface {
	Dial(network, address string) (net.Conn, error)
	Close() error
}

// OverWebSocket wraps an SSH connection over a WebSocket-upgraded connection
type OverWebSocket struct {
	conn      net.Conn
	sshClient *ssh.Client
	username  string
	password  string
	port      string
}

// NewOverWebSocket creates a new SSH over WebSocket connection
func NewOverWebSocket(conn net.Conn, username, password, port string) *OverWebSocket {
	return &OverWebSocket{
		conn:     conn,
		username: username,
		password: password,
		port:     port,
	}
}

// StartTransport initializes the SSH client over the WebSocket connection
func (s *OverWebSocket) StartTransport() error {
	fmt.Println("[*] Starting SSH transport over WebSocket connection...")

	// Set keepalive on the underlying WebSocket connection if it's TCP
	if tcpConn, ok := s.conn.(*net.TCPConn); ok {
		fmt.Println("[*] Setting keepalive on WebSocket connection...")
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	// Clear any previous deadlines
	s.conn.SetReadDeadline(time.Time{})
	s.conn.SetWriteDeadline(time.Time{})

	config := &ssh.ClientConfig{
		User: s.username,
		Auth: []ssh.AuthMethod{
			ssh.Password(s.password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // WARNING: This is insecure for production
		Timeout:         15 * time.Second,
	}

	fmt.Printf("[*] Attempting SSH connection with user: %s\n", s.username)

	// Create SSH client using the WebSocket connection
	sshConn, chans, reqs, err := ssh.NewClientConn(s.conn, "tcp", config)
	if err != nil {
		return fmt.Errorf("failed to create SSH connection: %v", err)
	}

	s.sshClient = ssh.NewClient(sshConn, chans, reqs)
	fmt.Println("[*] SSH transport established and authenticated.")
	return nil
}

// Dial opens a new connection to the given address through the SSH tunnel
func (s *OverWebSocket) Dial(network, address string) (net.Conn, error) {
	return s.sshClient.Dial(network, address)
}

// Close closes the SSH connection
func (s *OverWebSocket) Close() error {
	if s.sshClient != nil {
		return s.sshClient.Close()
	}
	return nil
}

// GetUnderlyingClient returns the underlying SSH client for advanced operations
func (s *OverWebSocket) GetUnderlyingClient() *ssh.Client {
	return s.sshClient
}
